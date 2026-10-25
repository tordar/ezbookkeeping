import { computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useSettingsStore } from '@/stores/setting.ts';
import { useUserStore } from '@/stores/user.ts';
import { useOverviewStore } from '@/stores/overview.ts';
import { useCategoryAveragesStore } from '@/stores/categoryAverages.ts';

import { AMOUNT_FACTOR, DISPLAY_HIDDEN_AMOUNT } from '@/consts/numeral.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import { parseDateTimeFromUnixTime } from '@/lib/datetime.ts';

export function useCategoryAveragesWidgetBase() {
    const { formatAmountToLocalizedNumeralsWithCurrency, formatDateTimeToLongMonthDay } = useI18n();

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

    return {
        periodTitle,
        displayAmount
    };
}
