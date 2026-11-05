import { computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useSettingsStore } from '@/stores/setting.ts';
import { useUserStore } from '@/stores/user.ts';
import { useOverviewStore } from '@/stores/overview.ts';
import { useCategoryAveragesStore } from '@/stores/categoryAverages.ts';

import { AMOUNT_FACTOR, DISPLAY_HIDDEN_AMOUNT } from '@/consts/numeral.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import { parseDateTimeFromUnixTime } from '@/lib/datetime.ts';

import type { CategoryAveragesRow, CategoryAveragesTotal } from '@/lib/category_averages.ts';

// a category is only called over or under its usual pace once it is at least this far off, so that
// rounding noise on small categories does not show up as a signal
const PACE_THRESHOLD_PERCENT: number = 5;

export function useCategoryAveragesWidgetBase() {
    const { formatAmountToLocalizedNumeralsWithCurrency, formatDateTimeToLongMonthDay, formatNumberToLocalizedNumerals } = useI18n();

    const settingsStore = useSettingsStore();
    const userStore = useUserStore();
    const overviewStore = useOverviewStore();
    const categoryAveragesStore = useCategoryAveragesStore();

    const periodTitle = computed<string>(() => {
        const periodStartUnixTime = categoryAveragesStore.currentPeriodStartUnixTime;

        if (!periodStartUnixTime) {
            return '';
        }

        const periodStart = formatDateTimeToLongMonthDay(parseDateTimeFromUnixTime(periodStartUnixTime));
        const today = formatDateTimeToLongMonthDay(parseDateTimeFromUnixTime(overviewStore.transactionDataRange.today.startTime));

        return `${periodStart} - ${today}`;
    });

    // averages and month-to-date totals are only meaningful to the krone, and the decimals make the
    // columns much harder to scan. The currency formatter always renders two decimals and takes no
    // precision option, so read the decimal separator off a known value rather than assuming one.
    const decimalSeparator = computed<string>(() => {
        const oneUnit = formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(AMOUNT_FACTOR), false);
        return oneUnit.charAt(oneUnit.length - 3);
    });

    function displayAmount(amount: number): string {
        if (!settingsStore.appSettings.showAmountInHomePage) {
            return formatAmountToLocalizedNumeralsWithCurrency(DISPLAY_HIDDEN_AMOUNT, userStore.currentUserDefaultCurrency);
        }

        const wholeUnits = Math.round(amount / AMOUNT_FACTOR) * AMOUNT_FACTOR;
        const formatted = formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(wholeUnits), userStore.currentUserDefaultCurrency);

        return formatted.replace(decimalSeparator.value + '00', '');
    }

    function getPacePercent(row: CategoryAveragesRow | CategoryAveragesTotal): number | null {
        if ('excluded' in row && row.excluded) {
            return null;
        }

        if (row.averageToDate <= 0) {
            return null;
        }

        const percent = (row.spentSoFar - row.averageToDate) * 100 / row.averageToDate;

        return Math.abs(percent) < PACE_THRESHOLD_PERCENT ? null : percent;
    }

    function displayPace(row: CategoryAveragesRow | CategoryAveragesTotal): string {
        if (!settingsStore.appSettings.showAmountInHomePage) {
            return '';
        }

        const percent = getPacePercent(row);

        if (percent === null) {
            return '';
        }

        return (percent > 0 ? '+' : '') + formatNumberToLocalizedNumerals(Math.round(percent)) + '%';
    }

    function paceClass(row: CategoryAveragesRow | CategoryAveragesTotal): string {
        const percent = getPacePercent(row);

        if (percent === null) {
            return '';
        }

        return percent > 0 ? 'text-expense' : 'text-income';
    }

    return {
        periodTitle,
        displayAmount,
        displayPace,
        paceClass
    };
}
