<template>
    <category-averages-card :data="categoryAveragesStore.categoryAveragesData"
                            :period-title="periodTitle" :title="title"
                            :loading="!!loading" :disabled="!!loading" />
</template>

<script setup lang="ts">
import CategoryAveragesCard from '@/components/desktop/CategoryAveragesCard.vue';

import { useCategoryAveragesWidgetBase } from '@/views/base/overview/CategoryAveragesWidgetBase.ts';

import { useCategoryAveragesStore } from '@/stores/categoryAverages.ts';

defineProps<{
    loading?: boolean;
    editing?: boolean;
    title?: string;
}>();

const { periodTitle } = useCategoryAveragesWidgetBase();

const categoryAveragesStore = useCategoryAveragesStore();
</script>
