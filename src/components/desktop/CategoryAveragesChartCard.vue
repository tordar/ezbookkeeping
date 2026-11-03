<template>
    <v-card :class="{ 'disabled': disabled }" class="overview-widget category-averages-chart-card h-100">
        <template #title>
            <overview-widget-header :title="title || tt('Category Averages')" :icon="mdiChartLine" />
        </template>

        <v-card-text class="overview-widget__body d-flex flex-column">
            <div class="overview-widget__caption text-caption mb-2" v-if="periodTitle">{{ periodTitle }}</div>

            <div class="d-flex flex-column align-center justify-center flex-grow-1" v-if="loading && !series.length">
                <v-skeleton-loader type="image" class="w-100 h-100" :loading="true"></v-skeleton-loader>
            </div>

            <div class="text-center text-medium-emphasis py-8" v-else-if="!series.length">
                {{ tt('No data') }}
            </div>

            <v-chart autoresize class="category-averages-chart" :option="chartOptions"
                     :update-options="{ notMerge: true }" v-else />
        </v-card-text>
    </v-card>
</template>

<script setup lang="ts">
import OverviewWidgetHeader from '@/views/desktop/overview/widgets/OverviewWidgetHeader.vue';

import { computed } from 'vue';
import type { CallbackDataParams } from 'echarts/types/dist/shared';

import { useI18n } from '@/locales/helpers.ts';

import { useCategoryAveragesWidgetBase } from '@/views/base/overview/CategoryAveragesWidgetBase.ts';

import { useSettingsStore } from '@/stores/setting.ts';

import { AMOUNT_FACTOR } from '@/consts/numeral.ts';
import { DEFAULT_CATEGORY_COLOR } from '@/consts/color.ts';

import type { CategoryAveragesDailySeries } from '@/lib/category_averages.ts';
import { getUnixTimeAfterUnixTime, parseDateTimeFromUnixTime } from '@/lib/datetime.ts';

import {
    mdiChartLine
} from '@mdi/js';

const props = defineProps<{
    loading: boolean;
    disabled: boolean;
    series: CategoryAveragesDailySeries[];
    periodLength: number;
    periodStartUnixTime: number;
    periodTitle: string;
    isDarkMode?: boolean;
    title?: string;
}>();

const { tt, formatDateTimeToLongMonthDay } = useI18n();
const { displayAmount } = useCategoryAveragesWidgetBase();

const settingsStore = useSettingsStore();

const showAmountInHomePage = computed<boolean>(() => settingsStore.appSettings.showAmountInHomePage);

const dayDates = computed<string[]>(() => {
    const dates: string[] = [];

    for (let i = 0; i < props.periodLength; i++) {
        dates.push(formatDateTimeToLongMonthDay(parseDateTimeFromUnixTime(getUnixTimeAfterUnixTime(props.periodStartUnixTime, i, 'days'))));
    }

    return dates;
});

const dayLabels = computed<string[]>(() => {
    const labels: string[] = [];

    for (let i = 0; i < props.periodLength; i++) {
        labels.push(parseDateTimeFromUnixTime(getUnixTimeAfterUnixTime(props.periodStartUnixTime, i, 'days')).getGregorianCalendarDay().toString());
    }

    return labels;
});

// categories often share a colour, or keep the default one, so fall back to the chart palette to
// keep every line apart
const seriesColors = computed<string[]>(() => {
    const usedColors = new Set<string>();
    const palette = settingsStore.chartColorList;
    let paletteIndex = 0;

    return props.series.map(item => {
        let color = item.color;

        if (!color || color === DEFAULT_CATEGORY_COLOR || usedColors.has(color)) {
            do {
                color = palette[paletteIndex % palette.length] as string;
                paletteIndex++;
            } while (usedColors.has(color) && paletteIndex < palette.length * 2);
        }

        usedColors.add(color);
        return '#' + color;
    });
});

function formatAxisAmount(value: number): string {
    const units = value / AMOUNT_FACTOR;

    if (Math.abs(units) >= 1000) {
        return `${Math.round(units / 100) / 10}k`;
    }

    return Math.round(units).toString();
}

const chartOptions = computed<object>(() => {
    const textColor = props.isDarkMode ? '#eee' : '#333';
    const lineSeries: object[] = [];

    props.series.forEach((item, index) => {
        const color = seriesColors.value[index];

        lineSeries.push({
            type: 'line',
            id: `${item.categoryId}-spent`,
            name: item.name,
            data: item.spentSoFar,
            showSymbol: false,
            lineStyle: { width: 2.5, color: color },
            itemStyle: { color: color },
            emphasis: { focus: 'series' },
            z: 3
        });

        lineSeries.push({
            type: 'line',
            id: `${item.categoryId}-average`,
            name: item.name,
            data: item.average,
            showSymbol: false,
            lineStyle: { width: 1.5, type: 'dashed', color: color, opacity: 0.7 },
            itemStyle: { color: color },
            emphasis: { focus: 'series' },
            z: 2
        });
    });

    return {
        tooltip: {
            trigger: 'axis',
            confine: true,
            backgroundColor: props.isDarkMode ? '#333' : '#fff',
            borderColor: props.isDarkMode ? '#333' : '#fff',
            textStyle: { color: textColor },
            formatter: (params: CallbackDataParams[]) => {
                const dayIndex = params[0]?.dataIndex ?? 0;
                const shownCategoryIds = new Set<string>(params.map(param => (param.seriesId ?? '').replace(/-(spent|average)$/, '')));
                let rows = '';

                props.series.forEach((item, index) => {
                    if (!shownCategoryIds.has(item.categoryId)) {
                        return;
                    }

                    const spent = item.spentSoFar[dayIndex];
                    const average = item.average[dayIndex] ?? 0;

                    if (!spent && !average) {
                        return;
                    }

                    rows += `<tr>` +
                        `<td><span class="category-averages-chart-tooltip-indicator me-1" style="background-color: ${seriesColors.value[index]}"></span><span class="me-4">${item.name}</span></td>` +
                        `<td class="text-end ps-3"><strong>${spent !== undefined ? displayAmount(spent) : ''}</strong></td>` +
                        `<td class="text-end ps-3 text-medium-emphasis">${displayAmount(average)}</td>` +
                        `</tr>`;
                });

                return `<table>` +
                    `<thead>` +
                    `<tr>` +
                    `<td class="text-start">${dayDates.value[dayIndex] ?? ''}</td>` +
                    `<td class="text-end ps-3">${tt('This Period')}</td>` +
                    `<td class="text-end ps-3">${tt('Usually')}</td>` +
                    `</tr>` +
                    `</thead>` +
                    `<tbody>${rows}</tbody>` +
                    `</table>`;
            }
        },
        legend: {
            type: 'scroll',
            bottom: 0,
            itemWidth: 10,
            itemHeight: 10,
            icon: 'circle',
            textStyle: { color: textColor },
            pageTextStyle: { color: textColor },
            data: props.series.map((item, index) => ({ name: item.name, itemStyle: { color: seriesColors.value[index] } }))
        },
        grid: {
            left: 8,
            right: 12,
            top: 10,
            bottom: 36,
            containLabel: true
        },
        xAxis: {
            type: 'category',
            boundaryGap: false,
            data: dayLabels.value,
            axisTick: { show: false },
            axisLine: { lineStyle: { color: props.isDarkMode ? '#555' : '#ccc' } },
            axisLabel: { color: textColor, interval: 'auto' }
        },
        yAxis: {
            type: 'value',
            axisLabel: {
                show: showAmountInHomePage.value,
                color: textColor,
                formatter: formatAxisAmount
            },
            splitLine: {
                lineStyle: { color: props.isDarkMode ? 'rgba(255, 255, 255, 0.08)' : 'rgba(0, 0, 0, 0.06)' }
            }
        },
        series: lineSeries
    };
});
</script>

<style>
.category-averages-chart-card .category-averages-chart {
    flex: 1 1 auto;
    min-block-size: 240px;
}

.category-averages-chart-tooltip-indicator {
    display: inline-block;
    inline-size: 8px;
    block-size: 8px;
    border-radius: 50%;
}
</style>
