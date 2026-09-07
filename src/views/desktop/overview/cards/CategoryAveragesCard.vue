<template>
    <v-card :class="{ 'disabled': disabled }" class="category-averages-card">
        <template #title>
            <div class="d-flex align-center">
                <span>{{ tt('Category Averages') }}</span>
                <v-spacer/>
                <span class="text-caption text-medium-emphasis">{{ periodTitle }}</span>
            </div>
        </template>

        <v-card-text class="pt-0">
            <div class="d-flex text-caption text-medium-emphasis category-averages-header pb-2">
                <span class="category-averages-name">{{ tt('Category') }}</span>
                <span class="category-averages-value">{{ tt('So Far') }}</span>
                <span class="category-averages-value">{{ tt('Usually By Now') }}</span>
                <span class="category-averages-value">{{ tt('Monthly Average') }}</span>
                <span class="category-averages-pace"></span>
            </div>

            <template v-if="loading && !rows.length">
                <v-skeleton-loader type="text" class="skeleton-no-margin my-4" :loading="true"
                                   v-for="index in 5" :key="index"></v-skeleton-loader>
            </template>

            <div class="text-center text-medium-emphasis py-8" v-else-if="!rows.length">
                {{ tt('No data') }}
            </div>

            <template v-else>
                <div v-for="row in rows" :key="row.categoryId">
                    <div class="d-flex align-center category-averages-row"
                         :class="{ 'category-averages-row-expandable': row.subRows.length }"
                         @click="toggleRow(row)">
                        <span class="category-averages-name d-flex align-center">
                            <v-icon class="category-averages-chevron" size="16"
                                    :icon="expandedCategoryIds[row.categoryId] ? mdiChevronDown : mdiChevronRight"
                                    v-if="row.subRows.length" />
                            <span class="category-averages-chevron" v-else></span>
                            <item-icon size="20px" icon-type="category" class="ms-1 me-2"
                                       :icon-id="row.icon" :color="row.color" />
                            <span class="text-truncate">{{ row.name }}</span>
                        </span>
                        <span class="category-averages-value">{{ displayAmount(row.spentSoFar) }}</span>
                        <span class="category-averages-value text-medium-emphasis">{{ displayAmount(row.averageToDate) }}</span>
                        <span class="category-averages-value text-medium-emphasis">{{ displayAmount(row.averageFullMonth) }}</span>
                        <span class="category-averages-pace" :class="paceClass(row)">{{ displayPace(row) }}</span>
                    </div>

                    <div class="d-flex align-center category-averages-row category-averages-subrow"
                         v-for="subRow in (expandedCategoryIds[row.categoryId] ? row.subRows : [])" :key="subRow.categoryId">
                        <span class="category-averages-name d-flex align-center">
                            <span class="category-averages-chevron"></span>
                            <item-icon size="18px" icon-type="category" class="ms-4 me-2"
                                       :icon-id="subRow.icon" :color="subRow.color" />
                            <span class="text-truncate">{{ subRow.name }}</span>
                        </span>
                        <span class="category-averages-value">{{ displayAmount(subRow.spentSoFar) }}</span>
                        <span class="category-averages-value text-medium-emphasis">{{ displayAmount(subRow.averageToDate) }}</span>
                        <span class="category-averages-value text-medium-emphasis">{{ displayAmount(subRow.averageFullMonth) }}</span>
                        <span class="category-averages-pace" :class="paceClass(subRow)">{{ displayPace(subRow) }}</span>
                    </div>
                </div>

                <v-divider class="my-2"/>

                <div class="d-flex align-center category-averages-row font-weight-bold">
                    <span class="category-averages-name">
                        <span class="category-averages-chevron"></span>
                        <span class="ms-1">{{ tt('Total') }}</span>
                    </span>
                    <span class="category-averages-value">{{ displayAmount(total.spentSoFar) }}</span>
                    <span class="category-averages-value">{{ displayAmount(total.averageToDate) }}</span>
                    <span class="category-averages-value">{{ displayAmount(total.averageFullMonth) }}</span>
                    <span class="category-averages-pace" :class="paceClass(total)">{{ displayPace(total) }}</span>
                </div>

                <div class="text-caption text-medium-emphasis mt-3" v-if="hasUnconvertedAmounts">
                    {{ tt('Some amounts could not be converted to the default currency and are not included.') }}
                </div>
            </template>
        </v-card-text>
    </v-card>
</template>

<script setup lang="ts">
import ItemIcon from '@/components/desktop/ItemIcon.vue';

import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useSettingsStore } from '@/stores/setting.ts';
import { useUserStore } from '@/stores/user.ts';

import { AMOUNT_FACTOR, DISPLAY_HIDDEN_AMOUNT } from '@/consts/numeral.ts';

import type { CategoryAveragesRow, CategoryAveragesTotal, CategoryAveragesResult } from '@/lib/category_averages.ts';

import {
    mdiChevronRight,
    mdiChevronDown
} from '@mdi/js';

// a category is only called over or under its usual pace once it is at least this far off, so that
// rounding noise on small categories does not show up as a signal
const PACE_THRESHOLD_PERCENT: number = 5;

const props = defineProps<{
    loading: boolean;
    disabled: boolean;
    data: CategoryAveragesResult;
    periodTitle: string;
}>();

const { tt, formatAmountToLocalizedNumeralsWithCurrency, formatNumberToLocalizedNumerals } = useI18n();

const settingsStore = useSettingsStore();
const userStore = useUserStore();

const expandedCategoryIds = ref<Record<string, boolean>>({});

const rows = computed<CategoryAveragesRow[]>(() => props.data?.rows ?? []);
const total = computed<CategoryAveragesTotal>(() => props.data?.total ?? { spentSoFar: 0, averageToDate: 0, averageFullMonth: 0 });
const hasUnconvertedAmounts = computed<boolean>(() => props.data?.hasUnconvertedAmounts ?? false);

// averages and month-to-date totals are only meaningful to the krone, and the decimals make the
// columns much harder to scan. The currency formatter always renders two decimals and takes no
// precision option, so read the decimal separator off a known value rather than assuming one.
const decimalSeparator = computed<string>(() => {
    const oneUnit = formatAmountToLocalizedNumeralsWithCurrency(AMOUNT_FACTOR, false);
    return oneUnit.charAt(oneUnit.length - 3);
});

function displayAmount(amount: number): string {
    if (!settingsStore.appSettings.showAmountInHomePage) {
        return formatAmountToLocalizedNumeralsWithCurrency(DISPLAY_HIDDEN_AMOUNT, userStore.currentUserDefaultCurrency);
    }

    const wholeUnits = Math.round(amount / AMOUNT_FACTOR) * AMOUNT_FACTOR;
    const formatted = formatAmountToLocalizedNumeralsWithCurrency(wholeUnits, userStore.currentUserDefaultCurrency);

    return formatted.replace(decimalSeparator.value + '00', '');
}

function getPacePercent(row: CategoryAveragesRow | CategoryAveragesTotal): number | null {
    if (row.averageToDate <= 0) {
        return null;
    }

    return (row.spentSoFar - row.averageToDate) * 100 / row.averageToDate;
}

function displayPace(row: CategoryAveragesRow | CategoryAveragesTotal): string {
    if (!settingsStore.appSettings.showAmountInHomePage) {
        return '';
    }

    const percent = getPacePercent(row);

    if (percent === null || Math.abs(percent) < PACE_THRESHOLD_PERCENT) {
        return '';
    }

    return (percent > 0 ? '+' : '') + formatNumberToLocalizedNumerals(Math.round(percent)) + '%';
}

function paceClass(row: CategoryAveragesRow | CategoryAveragesTotal): string {
    const percent = getPacePercent(row);

    if (percent === null || Math.abs(percent) < PACE_THRESHOLD_PERCENT) {
        return '';
    }

    return percent > 0 ? 'text-expense' : 'text-income';
}

function toggleRow(row: CategoryAveragesRow): void {
    if (!row.subRows.length) {
        return;
    }

    expandedCategoryIds.value[row.categoryId] = !expandedCategoryIds.value[row.categoryId];
}
</script>

<style>
.category-averages-card .category-averages-header,
.category-averages-card .category-averages-row {
    gap: 0.5rem;
}

.category-averages-card .category-averages-row {
    padding-block: 0.3rem;
}

.category-averages-card .category-averages-row-expandable {
    cursor: pointer;
}

.category-averages-card .category-averages-row-expandable:hover {
    background-color: rgba(var(--v-theme-on-surface), 0.04);
}

.category-averages-card .category-averages-name {
    flex: 1 1 auto;
    min-inline-size: 0;
    overflow: hidden;
}

.category-averages-card .category-averages-chevron {
    inline-size: 16px;
    flex: 0 0 auto;
}

.category-averages-card .category-averages-value {
    flex: 0 0 auto;
    inline-size: 6.5rem;
    text-align: end;
    white-space: nowrap;
}

.category-averages-card .category-averages-pace {
    flex: 0 0 auto;
    inline-size: 3.5rem;
    text-align: end;
    white-space: nowrap;
    font-size: 0.75rem;
}

.category-averages-card .category-averages-subrow {
    font-size: 0.875rem;
}
</style>
