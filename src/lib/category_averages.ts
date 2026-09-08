import { CategoryType } from '@/core/category.ts';
import type { ColorValue } from '@/core/color.ts';
import type { TransactionStatisticTrendsResponseItem } from '@/models/transaction.ts';

export interface CategoryAveragesCategoryInfo {
    readonly id: string;
    readonly name: string;
    readonly parentId: string;
    readonly type: CategoryType;
    readonly icon: string;
    readonly color: ColorValue;
}

export interface CategoryAveragesOptions {
    // whole month totals per category, for the baseline months and the current month
    readonly fullMonthTrends: TransactionStatisticTrendsResponseItem[];
    // same range, but only up to the current day of the month
    readonly toDateTrends: TransactionStatisticTrendsResponseItem[];
    // the complete months the averages are calculated over, as year * 100 + month
    readonly baselineYearMonths: number[];
    readonly currentYearMonth: number;
    readonly resolveCategory: (categoryId: string) => CategoryAveragesCategoryInfo | undefined;
    // converts an amount to the default currency, returning null when no rate is known
    readonly resolveAmount: (amount: number, accountId: string) => number | null;
    // whether the category is filtered out of the home page overview. Excluded categories keep a
    // row of their own but are left out of every total.
    readonly isExcludedCategory: (categoryId: string) => boolean;
}

export interface CategoryAveragesRow {
    readonly categoryId: string;
    readonly name: string;
    readonly icon: string;
    readonly color: ColorValue;
    readonly spentSoFar: number;
    readonly averageToDate: number;
    readonly averageFullMonth: number;
    readonly excluded: boolean;
    readonly subRows: CategoryAveragesRow[];
}

export interface CategoryAveragesTotal {
    readonly spentSoFar: number;
    readonly averageToDate: number;
    readonly averageFullMonth: number;
}

export interface CategoryAveragesResult {
    readonly rows: CategoryAveragesRow[];
    readonly total: CategoryAveragesTotal;
    readonly hasUnconvertedAmounts: boolean;
}

interface WritableAmounts {
    spentSoFar: number;
    averageToDate: number;
    averageFullMonth: number;
}

interface WritableRow extends WritableAmounts {
    readonly category: CategoryAveragesCategoryInfo;
    // a row counts towards the totals as soon as one amount reaches it from a category that is
    // not filtered out, so a primary stays included while any of its sub categories is
    hasIncludedAmount: boolean;
    // what filtered out sub categories contributed, kept apart so a primary that is only
    // partly filtered still totals correctly, while one that is filtered out entirely can
    // still show what it really costs instead of a row of zeroes
    readonly excludedAmounts: WritableAmounts;
    readonly subRows: Record<string, WritableRow>;
}

function emptyAmounts(): WritableAmounts {
    return { spentSoFar: 0, averageToDate: 0, averageFullMonth: 0 };
}

type AmountField = keyof WritableAmounts;

export interface CurrentPeriod {
    // the period's key, named after the month it starts in, as year * 100 + month
    readonly yearMonth: number;
    // how many days of the period have passed, counting the first day as 1
    readonly daysElapsed: number;
    readonly startYear: number;
    readonly startMonth: number;
    readonly startDay: number;
}

function daysInMonth(year: number, month: number): number {
    return new Date(Date.UTC(year, month, 0)).getUTCDate();
}

// getCurrentPeriod locates today within a spending period that starts on periodStartDay of the
// month. A periodStartDay of 0 or 1 gives plain calendar months; 25 gives a salary style cycle
// where the period named August runs from 25 August to 24 September.
export function getCurrentPeriod(year: number, month: number, day: number, periodStartDay: number): CurrentPeriod {
    if (periodStartDay <= 1) {
        return { yearMonth: year * 100 + month, daysElapsed: day, startYear: year, startMonth: month, startDay: 1 };
    }

    if (day >= periodStartDay) {
        return {
            yearMonth: year * 100 + month,
            daysElapsed: day - periodStartDay + 1,
            startYear: year,
            startMonth: month,
            startDay: periodStartDay
        };
    }

    let startYear = year;
    let startMonth = month - 1;

    if (startMonth < 1) {
        startMonth = 12;
        startYear -= 1;
    }

    return {
        yearMonth: startYear * 100 + startMonth,
        daysElapsed: daysInMonth(startYear, startMonth) - periodStartDay + 1 + day,
        startYear: startYear,
        startMonth: startMonth,
        startDay: periodStartDay
    };
}

// getBaselineYearMonths returns the given number of complete months before the current month,
// oldest first, as year * 100 + month
export function getBaselineYearMonths(currentYear: number, currentMonth: number, monthCount: number): number[] {
    const yearMonths: number[] = [];

    for (let i = monthCount; i >= 1; i--) {
        let year = currentYear;
        let month = currentMonth - i;

        while (month < 1) {
            month += 12;
            year -= 1;
        }

        yearMonths.push(year * 100 + month);
    }

    return yearMonths;
}

// buildCategoryAverages turns two sets of monthly statistics trends into per category rows
// comparing what has been spent in the current month against what is normally spent by the
// same day of the month, and over a whole month
export function buildCategoryAverages(options: CategoryAveragesOptions): CategoryAveragesResult {
    const { fullMonthTrends, toDateTrends, baselineYearMonths, currentYearMonth, resolveCategory, resolveAmount, isExcludedCategory } = options;

    const baselineMonthCount = baselineYearMonths.length;
    const baselineYearMonthSet = new Set<number>(baselineYearMonths);
    const rows: Record<string, WritableRow> = {};
    let hasUnconvertedAmounts = false;

    function getRow(category: CategoryAveragesCategoryInfo): WritableRow {
        let row = rows[category.id];

        if (!row) {
            row = { category, ...emptyAmounts(), hasIncludedAmount: false, excludedAmounts: emptyAmounts(), subRows: {} };
            rows[category.id] = row;
        }

        return row;
    }

    function addAmount(categoryId: string, accountId: string, amount: number, field: AmountField, divisor: number): void {
        const category = resolveCategory(categoryId);

        if (!category || category.type !== CategoryType.Expense) {
            return;
        }

        const convertedAmount = resolveAmount(amount, accountId);

        if (convertedAmount === null) {
            hasUnconvertedAmounts = true;
            return;
        }

        const primaryCategory = category.parentId && category.parentId !== '0' ? resolveCategory(category.parentId) : category;

        if (!primaryCategory) {
            return;
        }

        // exclusion is decided on the category the transaction actually carries: the filter marks
        // every partially checked parent too, so trusting a parent's own flag would drop a whole
        // primary as soon as one of its sub categories was unchecked
        const excluded = isExcludedCategory(category.id);
        const primaryRow = getRow(primaryCategory);

        const share = convertedAmount / divisor;

        if (primaryCategory.id === category.id) {
            if (excluded) {
                primaryRow.excludedAmounts[field] += share;
            } else {
                primaryRow[field] += share;
                primaryRow.hasIncludedAmount = true;
            }

            return;
        }

        let subRow = primaryRow.subRows[category.id];

        if (!subRow) {
            subRow = { category, ...emptyAmounts(), hasIncludedAmount: !excluded, excludedAmounts: emptyAmounts(), subRows: {} };
            primaryRow.subRows[category.id] = subRow;
        }

        subRow[field] += share;

        if (excluded) {
            primaryRow.excludedAmounts[field] += share;
        } else {
            primaryRow[field] += share;
            primaryRow.hasIncludedAmount = true;
        }
    }

    function accumulate(trends: TransactionStatisticTrendsResponseItem[], baselineField: AmountField, currentMonthField: AmountField | null): void {
        for (const monthlyTrend of trends) {
            const yearMonth = monthlyTrend.year * 100 + monthlyTrend.month;
            const isBaseline = baselineYearMonthSet.has(yearMonth);
            const isCurrentMonth = yearMonth === currentYearMonth;

            if (!isBaseline && !isCurrentMonth) {
                continue;
            }

            for (const item of monthlyTrend.items) {
                if (isBaseline && baselineMonthCount > 0) {
                    addAmount(item.categoryId, item.accountId, item.amount, baselineField, baselineMonthCount);
                }

                if (isCurrentMonth && currentMonthField) {
                    addAmount(item.categoryId, item.accountId, item.amount, currentMonthField, 1);
                }
            }
        }
    }

    accumulate(fullMonthTrends, 'averageFullMonth', null);
    accumulate(toDateTrends, 'averageToDate', 'spentSoFar');

    const finalRows = Object.values(rows)
        .map(row => finalizeRow(row))
        .sort(compareRows);

    const includedRows = finalRows.filter(row => !row.excluded);
    const total: CategoryAveragesTotal = {
        spentSoFar: sumOf(includedRows, 'spentSoFar'),
        averageToDate: sumOf(includedRows, 'averageToDate'),
        averageFullMonth: sumOf(includedRows, 'averageFullMonth')
    };

    return { rows: finalRows, total, hasUnconvertedAmounts };
}

function finalizeRow(row: WritableRow): CategoryAveragesRow {
    // an included row shows only what it contributes to the totals, so a partly filtered primary
    // still adds up. A row that contributes nothing shows what it actually costs instead of a row
    // of zeroes - whether that amount sits on the row itself (a filtered sub category) or was
    // aggregated from filtered children (a primary whose sub categories are all filtered).
    const excluded = !row.hasIncludedAmount;
    const amountOf = (field: AmountField): number => excluded ? row[field] + row.excludedAmounts[field] : row[field];

    return {
        categoryId: row.category.id,
        name: row.category.name,
        icon: row.category.icon,
        color: row.category.color,
        spentSoFar: amountOf('spentSoFar'),
        averageToDate: amountOf('averageToDate'),
        averageFullMonth: amountOf('averageFullMonth'),
        excluded: excluded,
        subRows: Object.values(row.subRows).map(subRow => finalizeRow(subRow)).sort(compareRows)
    };
}

function compareRows(rowA: CategoryAveragesRow, rowB: CategoryAveragesRow): number {
    if (rowA.excluded !== rowB.excluded) {
        return rowA.excluded ? 1 : -1;
    }

    if (rowA.averageFullMonth !== rowB.averageFullMonth) {
        return rowB.averageFullMonth - rowA.averageFullMonth;
    }

    return rowB.spentSoFar - rowA.spentSoFar;
}

function sumOf(rows: CategoryAveragesRow[], field: keyof CategoryAveragesTotal): number {
    let total = 0;

    for (const row of rows) {
        total += row[field];
    }

    return total;
}
