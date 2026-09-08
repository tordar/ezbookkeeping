import { describe, expect, test } from 'vitest';

import { CategoryType } from '@/core/category.ts';
import type { TransactionStatisticTrendsResponseItem } from '@/models/transaction.ts';

import {
    getBaselineYearMonths,
    getCurrentPeriod,
    buildCategoryAverages,
    type CategoryAveragesCategoryInfo
} from '@/lib/category_averages.ts';

const CATEGORIES: Record<string, CategoryAveragesCategoryInfo> = {
    '1': { id: '1', name: 'Mat', parentId: '0', type: CategoryType.Expense, icon: '1', color: 'ff0000' },
    '11': { id: '11', name: 'Matbutikk', parentId: '1', type: CategoryType.Expense, icon: '2', color: '00ff00' },
    '12': { id: '12', name: 'Mat ute', parentId: '1', type: CategoryType.Expense, icon: '3', color: '0000ff' },
    '2': { id: '2', name: 'Lån og leilighet', parentId: '0', type: CategoryType.Expense, icon: '4', color: 'ffff00' },
    '21': { id: '21', name: 'Huslån', parentId: '2', type: CategoryType.Expense, icon: '5', color: '00ffff' },
    '9': { id: '9', name: 'Lønn', parentId: '0', type: CategoryType.Income, icon: '6', color: 'ff00ff' }
};

function month(yearMonth: number, items: [string, number][]): TransactionStatisticTrendsResponseItem {
    return {
        year: Math.trunc(yearMonth / 100),
        month: yearMonth % 100,
        items: items.map(([categoryId, amount]) => ({
            categoryId,
            accountId: 'a1',
            amount
        }))
    };
}

const resolveCategory = (categoryId: string): CategoryAveragesCategoryInfo | undefined => CATEGORIES[categoryId];
const resolveAmount = (amount: number): number | null => amount;
const excludeNothing = (): boolean => false;

function build(fullMonthTrends: TransactionStatisticTrendsResponseItem[], toDateTrends: TransactionStatisticTrendsResponseItem[], baselineYearMonths: number[], currentYearMonth: number) {
    return buildCategoryAverages({
        fullMonthTrends,
        toDateTrends,
        baselineYearMonths,
        currentYearMonth,
        resolveCategory,
        resolveAmount,
        isExcludedCategory: excludeNothing
    });
}

function buildWithExcluded(excludedCategoryIds: string[], fullMonthTrends: TransactionStatisticTrendsResponseItem[], toDateTrends: TransactionStatisticTrendsResponseItem[], baselineYearMonths: number[], currentYearMonth: number) {
    return buildCategoryAverages({
        fullMonthTrends,
        toDateTrends,
        baselineYearMonths,
        currentYearMonth,
        resolveCategory,
        resolveAmount,
        isExcludedCategory: (categoryId: string) => excludedCategoryIds.indexOf(categoryId) >= 0
    });
}

describe('getBaselineYearMonths', () => {
    test('returns the complete months before the current one, oldest first', () => {
        expect(getBaselineYearMonths(2026, 9, 3)).toEqual([202606, 202607, 202608]);
    });

    test('crosses the year boundary', () => {
        expect(getBaselineYearMonths(2026, 2, 4)).toEqual([202510, 202511, 202512, 202601]);
    });

    test('never includes the current month', () => {
        expect(getBaselineYearMonths(2026, 9, 12)).not.toContain(202609);
        expect(getBaselineYearMonths(2026, 9, 12)).toHaveLength(12);
    });
});

describe('buildCategoryAverages', () => {
    test('returns no rows for no data', () => {
        const result = build([], [], [202608], 202609);

        expect(result.rows).toEqual([]);
        expect(result.total.averageFullMonth).toBe(0);
        expect(result.total.spentSoFar).toBe(0);
    });

    test('rolls secondary categories up into their primary category', () => {
        const result = build(
            [month(202608, [['11', 1000], ['12', 500]])],
            [month(202608, [['11', 400], ['12', 100]])],
            [202608],
            202609
        );

        expect(result.rows).toHaveLength(1);
        expect(result.rows[0]!.categoryId).toBe('1');
        expect(result.rows[0]!.name).toBe('Mat');
        expect(result.rows[0]!.averageFullMonth).toBe(1500);
        expect(result.rows[0]!.averageToDate).toBe(500);

        expect(result.rows[0]!.subRows).toHaveLength(2);
        expect(result.rows[0]!.subRows[0]!.categoryId).toBe('11');
        expect(result.rows[0]!.subRows[0]!.averageFullMonth).toBe(1000);
        expect(result.rows[0]!.subRows[1]!.categoryId).toBe('12');
        expect(result.rows[0]!.subRows[1]!.averageFullMonth).toBe(500);
    });

    test('averages over every baseline month, including months with no spending', () => {
        const result = build(
            [month(202607, [['11', 1200]])],
            [],
            [202607, 202608, 202609 - 1],
            202609
        );

        // 1200 spread over three baseline months
        expect(result.rows[0]!.averageFullMonth).toBe(400);
    });

    test('keeps the current month out of the averages and reports it as spent so far', () => {
        const result = build(
            [month(202608, [['11', 1000]]), month(202609, [['11', 9999]])],
            [month(202608, [['11', 300]]), month(202609, [['11', 250]])],
            [202608],
            202609
        );

        expect(result.rows[0]!.averageFullMonth).toBe(1000);
        expect(result.rows[0]!.averageToDate).toBe(300);
        expect(result.rows[0]!.spentSoFar).toBe(250);
    });

    test('ignores categories that are not expenses', () => {
        const result = build(
            [month(202608, [['11', 1000], ['9', 50000]])],
            [],
            [202608],
            202609
        );

        expect(result.rows).toHaveLength(1);
        expect(result.rows[0]!.categoryId).toBe('1');
        expect(result.total.averageFullMonth).toBe(1000);
    });

    test('ignores unknown categories rather than crashing', () => {
        const result = build(
            [month(202608, [['11', 1000], ['404', 700]])],
            [],
            [202608],
            202609
        );

        expect(result.rows).toHaveLength(1);
        expect(result.total.averageFullMonth).toBe(1000);
    });

    test('sorts rows by average full month spending, descending', () => {
        const result = build(
            [month(202608, [['11', 1000], ['21', 18000]])],
            [],
            [202608],
            202609
        );

        expect(result.rows.map(row => row.categoryId)).toEqual(['2', '1']);
    });

    test('a fixed cost that has not hit yet shows zero rather than a pro-rata estimate', () => {
        const result = build(
            [month(202608, [['21', 18000]])],
            [month(202608, [])],
            [202608],
            202609
        );

        expect(result.rows[0]!.averageFullMonth).toBe(18000);
        expect(result.rows[0]!.averageToDate).toBe(0);
        expect(result.rows[0]!.spentSoFar).toBe(0);
    });

    test('totals every row', () => {
        const result = build(
            [month(202608, [['11', 1000], ['21', 18000]])],
            [month(202608, [['11', 400]]), month(202609, [['11', 250]])],
            [202608],
            202609
        );

        expect(result.total.averageFullMonth).toBe(19000);
        expect(result.total.averageToDate).toBe(400);
        expect(result.total.spentSoFar).toBe(250);
    });

    test('skips amounts that cannot be converted and reports them', () => {
        const result = buildCategoryAverages({
            fullMonthTrends: [month(202608, [['11', 1000], ['12', 700]])],
            toDateTrends: [],
            baselineYearMonths: [202608],
            currentYearMonth: 202609,
            resolveCategory,
            resolveAmount: (amount: number) => amount === 700 ? null : amount,
            isExcludedCategory: excludeNothing
        });

        expect(result.rows[0]!.averageFullMonth).toBe(1000);
        expect(result.hasUnconvertedAmounts).toBe(true);
    });

    test('reports no unconverted amounts when every amount converts', () => {
        const result = build([month(202608, [['11', 1000]])], [], [202608], 202609);
        expect(result.hasUnconvertedAmounts).toBe(false);
    });
});

describe('buildCategoryAverages with excluded categories', () => {
    test('an excluded sub category keeps its own row but leaves the total alone', () => {
        const result = buildWithExcluded(['21'],
            [month(202608, [['11', 1000], ['21', 18000]])],
            [month(202608, [['11', 400], ['21', 9000]])],
            [202608], 202609
        );

        const husl = result.rows.find(row => row.categoryId === '2')!;
        expect(husl.excluded).toBe(true);
        expect(husl.averageFullMonth).toBe(18000);

        expect(result.total.averageFullMonth).toBe(1000);
        expect(result.total.averageToDate).toBe(400);
    });

    test('an excluded sub does not contribute to its primary', () => {
        const result = buildWithExcluded(['12'],
            [month(202608, [['11', 1000], ['12', 500]])],
            [], [202608], 202609
        );

        const mat = result.rows.find(row => row.categoryId === '1')!;
        expect(mat.excluded).toBe(false);
        expect(mat.averageFullMonth).toBe(1000);

        const excludedSub = mat.subRows.find(row => row.categoryId === '12')!;
        expect(excludedSub.excluded).toBe(true);
        expect(excludedSub.averageFullMonth).toBe(500);

        expect(result.total.averageFullMonth).toBe(1000);
    });

    test('a primary is only excluded when every one of its subs is', () => {
        const partly = buildWithExcluded(['12'], [month(202608, [['11', 1000], ['12', 500]])], [], [202608], 202609);
        expect(partly.rows.find(row => row.categoryId === '1')!.excluded).toBe(false);

        const fully = buildWithExcluded(['11', '12'], [month(202608, [['11', 1000], ['12', 500]])], [], [202608], 202609);
        expect(fully.rows.find(row => row.categoryId === '1')!.excluded).toBe(true);
        expect(fully.total.averageFullMonth).toBe(0);
    });

    test('excluded rows sort below included ones regardless of size', () => {
        const result = buildWithExcluded(['21'],
            [month(202608, [['11', 1000], ['21', 18000]])],
            [], [202608], 202609
        );

        expect(result.rows.map(row => row.categoryId)).toEqual(['1', '2']);
    });

    test('excluding nothing leaves every row included', () => {
        const result = buildWithExcluded([], [month(202608, [['11', 1000]])], [], [202608], 202609);
        expect(result.rows[0]!.excluded).toBe(false);
        expect(result.total.averageFullMonth).toBe(1000);
    });

    test('spent so far in the current month also ignores excluded categories', () => {
        const result = buildWithExcluded(['21'],
            [month(202608, [['11', 1000], ['21', 18000]])],
            [month(202608, [['11', 400]]), month(202609, [['11', 250], ['21', 2799]])],
            [202608], 202609
        );

        expect(result.total.spentSoFar).toBe(250);
        expect(result.rows.find(row => row.categoryId === '2')!.spentSoFar).toBe(2799);
    });
});

describe('getCurrentPeriod', () => {
    test('plain calendar months when the period starts on the first', () => {
        for (const startDay of [0, 1]) {
            const period = getCurrentPeriod(2026, 9, 8, startDay);
            expect(period.yearMonth).toBe(202609);
            expect(period.daysElapsed).toBe(8);
            expect(period.startYear).toBe(2026);
            expect(period.startMonth).toBe(9);
            expect(period.startDay).toBe(1);
        }
    });

    test('a day before the start day belongs to the period that began last month', () => {
        // 25-31 Aug is 7 days, plus 1-8 Sep is 8, so 8 September is 15 days in
        const period = getCurrentPeriod(2026, 9, 8, 25);
        expect(period.yearMonth).toBe(202608);
        expect(period.daysElapsed).toBe(15);
        expect(period.startYear).toBe(2026);
        expect(period.startMonth).toBe(8);
        expect(period.startDay).toBe(25);
    });

    test('the start day itself begins a new period', () => {
        const period = getCurrentPeriod(2026, 9, 25, 25);
        expect(period.yearMonth).toBe(202609);
        expect(period.daysElapsed).toBe(1);
        expect(period.startMonth).toBe(9);
    });

    test('the day before the start day closes the previous period', () => {
        const period = getCurrentPeriod(2026, 9, 24, 25);
        expect(period.yearMonth).toBe(202608);
        expect(period.daysElapsed).toBe(31); // 25-31 Aug plus 1-24 Sep
    });

    test('short months shorten the period', () => {
        expect(getCurrentPeriod(2026, 3, 24, 25).daysElapsed).toBe(28);
        expect(getCurrentPeriod(2024, 3, 24, 25).daysElapsed).toBe(29); // leap year
    });

    test('crosses the year boundary', () => {
        const period = getCurrentPeriod(2026, 1, 3, 25);
        expect(period.yearMonth).toBe(202512);
        expect(period.daysElapsed).toBe(10);
        expect(period.startYear).toBe(2025);
        expect(period.startMonth).toBe(12);
    });

    test('baseline periods are the ones before the current period', () => {
        const period = getCurrentPeriod(2026, 9, 8, 25);
        const baseline = getBaselineYearMonths(period.startYear, period.startMonth, 12);

        expect(baseline).toHaveLength(12);
        expect(baseline).not.toContain(202608);
        expect(baseline[baseline.length - 1]).toBe(202607);
    });
});
