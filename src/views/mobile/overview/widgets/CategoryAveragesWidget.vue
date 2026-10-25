<template>
    <f7-list strong inset dividers class="overview-widget-list overview-transaction-list no-margin-top margin-bottom">
        <f7-list-item group-title v-if="showTitle">
            <small>{{ title || tt('Category Averages') }}</small>
        </f7-list-item>
        <f7-list-item class="item-title-full-line">
            <template #title>
                <small class="skeleton-text" v-if="loading && !periodTitle">Oct 25 - Oct 25</small>
                <small v-else-if="periodTitle">{{ periodTitle }}</small>
                <div class="overview-widget-primary-amount skeleton-text" v-if="loading && !rows.length">0 USD</div>
                <div class="overview-widget-primary-amount" :class="{ 'text-expense': total.spentSoFar > 0 }" v-else>
                    {{ displayAmount(total.spentSoFar) }}
                </div>
                <div class="display-flex justify-content-space-between">
                    <small>{{ tt('Monthly Average') }}</small>
                    <small class="margin-left-half text-truncate skeleton-text" v-if="loading && !rows.length">0 USD</small>
                    <small class="margin-left-half text-truncate" v-else>{{ displayAmount(total.averageFullMonth) }}</small>
                </div>
            </template>
        </f7-list-item>
        <f7-list-item :key="row.categoryId" :title="row.name" :after="displayAmount(row.spentSoFar)"
                      v-for="row in includedRows">
            <template #media>
                <ItemIcon icon-type="category" :icon-id="row.icon" :color="row.color" />
            </template>
            <template #footer>
                <div class="overview-transaction-footer">
                    <span>{{ tt('Monthly Average') }}</span>
                    <span>{{ displayAmount(row.averageFullMonth) }}</span>
                </div>
            </template>
        </f7-list-item>
        <template v-if="loading && !rows.length">
            <f7-list-item title="Category" after="0 USD" :key="idx" v-for="idx in 5">
                <template #media>
                    <f7-icon f7="app_fill"></f7-icon>
                </template>
            </f7-list-item>
        </template>
        <f7-list-item :title="tt('No data')" v-else-if="!loading && !includedRows.length"></f7-list-item>
    </f7-list>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';
import { useCategoryAveragesWidgetBase } from '@/views/base/overview/CategoryAveragesWidgetBase.ts';

import { useCategoryAveragesStore } from '@/stores/categoryAverages.ts';

import type { CategoryAveragesRow, CategoryAveragesTotal } from '@/lib/category_averages.ts';

defineProps<{
    loading: boolean;
    title?: string;
    showTitle: boolean;
}>();

const { tt } = useI18n();
const { periodTitle, displayAmount } = useCategoryAveragesWidgetBase();

const categoryAveragesStore = useCategoryAveragesStore();

const rows = computed<CategoryAveragesRow[]>(() => categoryAveragesStore.categoryAveragesData.rows);
// categories filtered out of the home page totals are left off here, since there is no room on a
// phone to show them greyed out the way the desktop card does
const includedRows = computed<CategoryAveragesRow[]>(() => rows.value.filter(row => !row.excluded));
const total = computed<CategoryAveragesTotal>(() => categoryAveragesStore.categoryAveragesData.total);
</script>
