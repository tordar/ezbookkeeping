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
}

export interface CategoryAveragesRow {
    readonly categoryId: string;
    readonly name: string;
    readonly icon: string;
    readonly color: ColorValue;
    readonly spentSoFar: number;
    readonly averageToDate: number;
    readonly averageFullMonth: number;
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
    readonly subRows: Record<string, WritableRow>;
}

type AmountField = keyof WritableAmounts;

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
    const { fullMonthTrends, toDateTrends, baselineYearMonths, currentYearMonth, resolveCategory, resolveAmount } = options;

    const baselineMonthCount = baselineYearMonths.length;
    const baselineYearMonthSet = new Set<number>(baselineYearMonths);
    const rows: Record<string, WritableRow> = {};
    let hasUnconvertedAmounts = false;

    function getRow(category: CategoryAveragesCategoryInfo): WritableRow {
        let row = rows[category.id];

        if (!row) {
            row = { category, spentSoFar: 0, averageToDate: 0, averageFullMonth: 0, subRows: {} };
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

        const primaryRow = getRow(primaryCategory);
        primaryRow[field] += convertedAmount / divisor;

        if (primaryCategory.id === category.id) {
            return;
        }

        let subRow = primaryRow.subRows[category.id];

        if (!subRow) {
            subRow = { category, spentSoFar: 0, averageToDate: 0, averageFullMonth: 0, subRows: {} };
            primaryRow.subRows[category.id] = subRow;
        }

        subRow[field] += convertedAmount / divisor;
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

    const total: CategoryAveragesTotal = {
        spentSoFar: sumOf(finalRows, 'spentSoFar'),
        averageToDate: sumOf(finalRows, 'averageToDate'),
        averageFullMonth: sumOf(finalRows, 'averageFullMonth')
    };

    return { rows: finalRows, total, hasUnconvertedAmounts };
}

function finalizeRow(row: WritableRow): CategoryAveragesRow {
    return {
        categoryId: row.category.id,
        name: row.category.name,
        icon: row.category.icon,
        color: row.category.color,
        spentSoFar: row.spentSoFar,
        averageToDate: row.averageToDate,
        averageFullMonth: row.averageFullMonth,
        subRows: Object.values(row.subRows).map(subRow => finalizeRow(subRow)).sort(compareRows)
    };
}

function compareRows(rowA: CategoryAveragesRow, rowB: CategoryAveragesRow): number {
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
