import { ref } from 'vue';
import { defineStore } from 'pinia';

import { useSettingsStore } from './setting.ts';
import { useUserStore } from './user.ts';
import { useAccountsStore } from './account.ts';
import { useTransactionCategoriesStore } from './transactionCategory.ts';
import { useExchangeRatesStore } from './exchangeRates.ts';

import { TimezoneTypeForStatistics } from '@/core/timezone.ts';

import type { TransactionStatisticTrendsResponseItem } from '@/models/transaction.ts';

import { isNumber } from '@/lib/common.ts';
import { getCurrentDateTime, getYearMonthDayDateTime } from '@/lib/datetime.ts';
import {
    getBaselineYearMonths,
    getCurrentPeriod,
    buildCategoryAverages,
    type CategoryAveragesCategoryInfo,
    type CategoryAveragesResult
} from '@/lib/category_averages.ts';

import services from '@/lib/services.ts';
import logger from '@/lib/logger.ts';

// the number of complete periods the averages are calculated over
export const CATEGORY_AVERAGES_BASELINE_MONTH_COUNT: number = 12;

// spending periods run from this day of the month to the day before it in the next month, so they
// line up with when salary lands rather than with the calendar
export const CATEGORY_AVERAGES_PERIOD_START_DAY: number = 25;

const EMPTY_RESULT: CategoryAveragesResult = {
    rows: [],
    total: { spentSoFar: 0, averageToDate: 0, averageFullMonth: 0 },
    hasUnconvertedAmounts: false
};

function formatYearMonth(yearMonth: number): string {
    return `${Math.trunc(yearMonth / 100)}-${(yearMonth % 100).toString().padStart(2, '0')}`;
}

export const useCategoryAveragesStore = defineStore('categoryAverages', () => {
    const settingsStore = useSettingsStore();
    const userStore = useUserStore();
    const accountsStore = useAccountsStore();
    const transactionCategoriesStore = useTransactionCategoriesStore();
    const exchangeRatesStore = useExchangeRatesStore();

    const categoryAveragesData = ref<CategoryAveragesResult>(EMPTY_RESULT);
    const categoryAveragesStateInvalid = ref<boolean>(true);
    const currentDayOfMonth = ref<number>(0);
    // the first day of the period the card is reporting on, for the heading
    const currentPeriodStartUnixTime = ref<number>(0);

    function updateCategoryAveragesInvalidState(invalidState: boolean): void {
        categoryAveragesStateInvalid.value = invalidState;
    }

    function resetCategoryAverages(): void {
        categoryAveragesData.value = EMPTY_RESULT;
        categoryAveragesStateInvalid.value = true;
        currentDayOfMonth.value = 0;
        currentPeriodStartUnixTime.value = 0;
    }

    function resolveCategory(categoryId: string): CategoryAveragesCategoryInfo | undefined {
        return transactionCategoriesStore.allTransactionCategoriesMap[categoryId];
    }

    // the home page overview category filter marks a category id true when it is filtered out
    function isExcludedCategory(categoryId: string): boolean {
        return settingsStore.appSettings.overviewTransactionCategoryFilterInHomePage[categoryId] === true;
    }

    function resolveAmount(amount: number, accountId: string): number | null {
        const defaultCurrency = userStore.currentUserDefaultCurrency;
        const account = accountsStore.allAccountsMap[accountId];

        if (!account) {
            return null;
        }

        if (account.currency === defaultCurrency) {
            return amount;
        }

        const exchangedAmount = exchangeRatesStore.getExchangedAmount(amount, account.currency, defaultCurrency);

        if (!isNumber(exchangedAmount)) {
            return null;
        }

        return Math.trunc(exchangedAmount);
    }

    function loadCategoryAverages({ force }: { force: boolean }): Promise<CategoryAveragesResult> {
        const today = getCurrentDateTime().toGregorianCalendarYearMonthDay();
        const currentYear = today.year;
        const currentMonth = today.month;
        const currentDay = today.day;

        if (!force && !categoryAveragesStateInvalid.value && currentDayOfMonth.value === currentDay) {
            return Promise.resolve(categoryAveragesData.value);
        }

        const period = getCurrentPeriod(currentYear, currentMonth, currentDay, CATEGORY_AVERAGES_PERIOD_START_DAY);
        const currentYearMonth = period.yearMonth;
        const baselineYearMonths = getBaselineYearMonths(period.startYear, period.startMonth, CATEGORY_AVERAGES_BASELINE_MONTH_COUNT);
        const useTransactionTimezone = settingsStore.appSettings.timezoneUsedForStatisticsInHomePage === TimezoneTypeForStatistics.TransactionTimezone.type;

        const baseRequest = {
            startYearMonth: formatYearMonth(baselineYearMonths[0] as number),
            endYearMonth: formatYearMonth(currentYearMonth),
            tagFilter: '',
            keyword: '',
            matchMode: 0,
            useTransactionTimezone: useTransactionTimezone,
            periodStartDay: CATEGORY_AVERAGES_PERIOD_START_DAY
        };

        // accounts and categories are needed to resolve every amount, so make sure they are loaded
        // before the statistics are aggregated, however this store is called
        return Promise.all([
            accountsStore.loadAllAccounts({ force: false }),
            transactionCategoriesStore.loadAllCategories({ force: false })
        ]).then(() => Promise.all([
            services.getTransactionStatisticsTrends(baseRequest),
            services.getTransactionStatisticsTrends({ ...baseRequest, maxDaysIntoPeriod: period.daysElapsed })
        ])).then(([fullMonthResponse, toDateResponse]) => {
            const fullMonthTrends = fullMonthResponse.data?.result as TransactionStatisticTrendsResponseItem[] | undefined;
            const toDateTrends = toDateResponse.data?.result as TransactionStatisticTrendsResponseItem[] | undefined;

            if (!fullMonthResponse.data || !fullMonthResponse.data.success || !fullMonthTrends
                || !toDateResponse.data || !toDateResponse.data.success || !toDateTrends) {
                logger.error('[categoryAverages.loadCategoryAverages] cannot load category averages, because response is not success');
                return Promise.reject({ message: 'Unable to retrieve category averages' });
            }

            const result = buildCategoryAverages({
                fullMonthTrends: fullMonthTrends,
                toDateTrends: toDateTrends,
                baselineYearMonths: baselineYearMonths,
                currentYearMonth: currentYearMonth,
                resolveCategory: resolveCategory,
                resolveAmount: resolveAmount,
                isExcludedCategory: isExcludedCategory
            });

            categoryAveragesData.value = result;
            categoryAveragesStateInvalid.value = false;
            currentDayOfMonth.value = currentDay;
            currentPeriodStartUnixTime.value = getYearMonthDayDateTime(period.startYear, period.startMonth, period.startDay).getUnixTime();

            return result;
        }).catch(error => {
            if (error.response || error.message) {
                logger.error('[categoryAverages.loadCategoryAverages] failed to load category averages', error);
            }

            return Promise.reject(error);
        });
    }

    return {
        // states
        categoryAveragesData,
        categoryAveragesStateInvalid,
        currentDayOfMonth,
        currentPeriodStartUnixTime,
        // functions
        updateCategoryAveragesInvalidState,
        resetCategoryAverages,
        loadCategoryAverages
    };
});
