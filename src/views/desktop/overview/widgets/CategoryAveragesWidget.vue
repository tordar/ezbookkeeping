<template>
    <category-averages-card :data="categoryAveragesStore.categoryAveragesData"
                            :period-title="periodTitle" :title="title"
                            :loading="!!loading" :disabled="!!loading" />
</template>

<script setup lang="ts">
import CategoryAveragesCard from '@/components/desktop/CategoryAveragesCard.vue';

import { computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useOverviewStore } from '@/stores/overview.ts';
import { useCategoryAveragesStore } from '@/stores/categoryAverages.ts';

import { parseDateTimeFromUnixTime } from '@/lib/datetime.ts';

defineProps<{
    loading?: boolean;
    editing?: boolean;
    title?: string;
}>();

const { formatDateTimeToLongMonthDay } = useI18n();

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
</script>
