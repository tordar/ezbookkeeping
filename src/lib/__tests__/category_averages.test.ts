import { describe, expect, test } from 'vitest';

import { CategoryType } from '@/core/category.ts';
import type { TransactionStatisticTrendsResponseItem } from '@/models/transaction.ts';

import {
    getBaselineYearMonths,
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

function build(fullMonthTrends: TransactionStatisticTrendsResponseItem[], toDateTrends: TransactionStatisticTrendsResponseItem[], baselineYearMonths: number[], currentYearMonth: number) {
    return buildCategoryAverages({
        fullMonthTrends,
        toDateTrends,
        baselineYearMonths,
        currentYearMonth,
        resolveCategory,
        resolveAmount
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
            resolveAmount: (amount: number) => amount === 700 ? null : amount
        });

        expect(result.rows[0]!.averageFullMonth).toBe(1000);
        expect(result.hasUnconvertedAmounts).toBe(true);
    });

    test('reports no unconverted amounts when every amount converts', () => {
        const result = build([month(202608, [['11', 1000]])], [], [202608], 202609);
        expect(result.hasUnconvertedAmounts).toBe(false);
    });
});
