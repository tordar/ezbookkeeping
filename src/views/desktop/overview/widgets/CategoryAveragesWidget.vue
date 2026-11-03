<template>
    <category-averages-chart-card :series="chartSeries"
                                  :period-length="categoryAveragesStore.currentPeriodLength"
                                  :period-start-unix-time="categoryAveragesStore.currentPeriodStartUnixTime"
                                  :period-title="periodTitle" :title="title" :is-dark-mode="isDarkMode"
                                  :loading="!!loading" :disabled="!!loading"
                                  v-if="displayMode === 'lineChart'" />
    <category-averages-card :data="categoryAveragesStore.categoryAveragesData"
                            :period-title="periodTitle" :title="title"
                            :loading="!!loading" :disabled="!!loading" v-else />
</template>

<script setup lang="ts">
import CategoryAveragesCard from '@/components/desktop/CategoryAveragesCard.vue';
import CategoryAveragesChartCard from '@/components/desktop/CategoryAveragesChartCard.vue';

import { computed } from 'vue';
import { useTheme } from 'vuetify';

import { useCategoryAveragesWidgetBase } from '@/views/base/overview/CategoryAveragesWidgetBase.ts';

import { useCategoryAveragesStore } from '@/stores/categoryAverages.ts';
import { useTransactionCategoriesStore } from '@/stores/transactionCategory.ts';

import { ThemeType } from '@/core/theme.ts';

import type { CategoryAveragesDailySeries } from '@/lib/category_averages.ts';

const props = defineProps<{
    loading?: boolean;
    editing?: boolean;
    title?: string;
    displayMode?: string;
    chartCategoryIds?: string[];
}>();

const theme = useTheme();

const { periodTitle } = useCategoryAveragesWidgetBase();

const categoryAveragesStore = useCategoryAveragesStore();
const transactionCategoriesStore = useTransactionCategoriesStore();

// lines are drawn per primary category, so a primary stays on the chart while it or any of its
// sub categories is selected. Nothing selected means every category is drawn.
const chartSeries = computed<CategoryAveragesDailySeries[]>(() => {
    const series = categoryAveragesStore.categoryAveragesDailyData;
    const selectedCategoryIds = props.chartCategoryIds;

    if (!selectedCategoryIds || !selectedCategoryIds.length) {
        return series;
    }

    const shownPrimaryCategoryIds = new Set<string>();

    for (const categoryId of selectedCategoryIds) {
        const category = transactionCategoriesStore.allTransactionCategoriesMap[categoryId];

        shownPrimaryCategoryIds.add(categoryId);

        if (category && category.parentId && category.parentId !== '0') {
            shownPrimaryCategoryIds.add(category.parentId);
        }
    }

    return series.filter(item => shownPrimaryCategoryIds.has(item.categoryId));
});

const isDarkMode = computed<boolean>(() => theme.global.name.value === ThemeType.Dark);
</script>
