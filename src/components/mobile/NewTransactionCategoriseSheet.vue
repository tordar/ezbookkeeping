<template>
    <f7-sheet swipe-to-close swipe-handler=".swipe-handler"
              class="new-transaction-categorise-sheet" :opened="show"
              @sheet:closed="onSheetClosed">
        <f7-toolbar class="toolbar-with-swipe-handler">
            <div class="swipe-handler"></div>
            <div class="left">
                <f7-link sheet-close icon-f7="xmark"></f7-link>
            </div>
        </f7-toolbar>
        <f7-page-content class="margin-top">
            <f7-block class="no-margin-top margin-bottom" v-if="transaction">
                <p class="no-margin-top"><strong>{{ transaction.description || '–' }}</strong></p>
                <p class="no-margin-bottom">
                    <span :class="transaction.creditDebit === 'CRDT' ? 'text-income' : 'text-expense'">
                        {{ transaction.amount }} {{ transaction.currency }}
                    </span>
                    &nbsp;·&nbsp;{{ transaction.date }}&nbsp;·&nbsp;{{ transaction.aspspName }}
                </p>
            </f7-block>

            <f7-list strong inset dividers class="margin-vertical">
                <f7-list-item link="#" no-chevron
                              class="list-item-with-header-and-title"
                              :header="tt('Account')"
                              :title="selectedAccountName || tt('Select Account')"
                              @click="showAccountSheet = true">
                    <template #media v-if="selectedAccount">
                        <ItemIcon icon-type="account" :icon-id="selectedAccount.icon" :color="selectedAccount.color" />
                    </template>
                </f7-list-item>
                <f7-list-item link="#" no-chevron
                              class="list-item-with-header-and-title"
                              :header="tt('Category')"
                              :title="selectedCategoryName || tt('Select Category')"
                              @click="showCategorySheet = true">
                    <template #media v-if="selectedCategory">
                        <ItemIcon icon-type="category" :icon-id="selectedCategory.icon" :color="selectedCategory.color" />
                    </template>
                </f7-list-item>
            </f7-list>

            <div class="margin">
                <f7-button fill round large
                           :disabled="submitting || !selectedAccountId || !selectedCategoryId"
                           @click="accept">
                    {{ tt('Add to ledger') }}
                </f7-button>
                <f7-button round large class="margin-top" sheet-close>{{ tt('Cancel') }}</f7-button>
            </div>
        </f7-page-content>

        <list-item-selection-sheet value-type="item"
                                   key-field="id"
                                   value-field="id"
                                   title-field="name"
                                   icon-field="icon"
                                   icon-type="account"
                                   color-field="color"
                                   :items="accountOptions"
                                   :model-value="selectedAccountId"
                                   v-model:show="showAccountSheet"
                                   @update:model-value="selectedAccountId = ($event as string)">
        </list-item-selection-sheet>

        <list-item-selection-sheet value-type="item"
                                   key-field="id"
                                   value-field="id"
                                   title-field="name"
                                   icon-field="icon"
                                   icon-type="category"
                                   color-field="color"
                                   :items="categoryOptions"
                                   :model-value="selectedCategoryId"
                                   v-model:show="showCategorySheet"
                                   @update:model-value="selectedCategoryId = ($event as string)">
        </list-item-selection-sheet>
    </f7-sheet>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents } from '@/lib/ui/mobile.ts';
import services from '@/lib/services.ts';
import { invalidateNewTransactionsCache } from '@/lib/bank_transactions_cache.ts';
import type { NewBankTransactionItem } from '@/models/bank_integration.ts';
import { useAccountsStore } from '@/stores/account.ts';
import { useTransactionCategoriesStore } from '@/stores/transactionCategory.ts';
import { CategoryType } from '@/core/category.ts';

import ItemIcon from '@/components/mobile/ItemIcon.vue';
import ListItemSelectionSheet from '@/components/mobile/ListItemSelectionSheet.vue';

const props = defineProps<{
    show: boolean;
    transaction: NewBankTransactionItem | null;
}>();

const emit = defineEmits<{
    (e: 'update:show', value: boolean): void;
    (e: 'accepted', tx: NewBankTransactionItem): void;
}>();

const { tt } = useI18n();
const { showToast, showAlert } = useI18nUIComponents();

const accountsStore = useAccountsStore();
const transactionCategoriesStore = useTransactionCategoriesStore();

const selectedAccountId = ref<string | null>(null);
const selectedCategoryId = ref<string | null>(null);
const showAccountSheet = ref(false);
const showCategorySheet = ref(false);
const submitting = ref(false);

const accountOptions = computed(() => accountsStore.allVisiblePlainAccounts ?? []);

const categoryOptions = computed(() => {
    if (!props.transaction) return [];
    const categoryType = props.transaction.creditDebit === 'CRDT' ? CategoryType.Income : CategoryType.Expense;
    const list = transactionCategoriesStore.allTransactionCategories?.[categoryType] ?? [];
    const options: Array<{ id: string; name: string; icon: string; color: string }> = [];
    for (const primary of list) {
        if (primary.hidden) continue;
        if (primary.subCategories?.length) {
            for (const sub of primary.subCategories) {
                if (sub.hidden) continue;
                options.push({ id: String(sub.id), name: `${primary.name} › ${sub.name}`, icon: sub.icon, color: sub.color });
            }
        } else {
            options.push({ id: String(primary.id), name: primary.name, icon: primary.icon, color: primary.color });
        }
    }
    return options;
});

const selectedAccount = computed(() => {
    if (!selectedAccountId.value) return null;
    return accountOptions.value.find(a => String(a.id) === selectedAccountId.value) ?? null;
});

const selectedCategory = computed(() => {
    if (!selectedCategoryId.value) return null;
    return categoryOptions.value.find(c => c.id === selectedCategoryId.value) ?? null;
});

const selectedAccountName = computed(() => selectedAccount.value?.name ?? null);
const selectedCategoryName = computed(() => selectedCategory.value?.name ?? null);

watch(() => props.transaction, () => {
    selectedAccountId.value = null;
    selectedCategoryId.value = null;
});

async function accept(): Promise<void> {
    const tx = props.transaction;
    if (!tx) return;

    if (!selectedAccountId.value) {
        showAlert('Please select an account');
        return;
    }
    if (!selectedCategoryId.value) {
        showAlert('Please select a category');
        return;
    }

    submitting.value = true;
    try {
        await services.acceptBankIntegrationNewTransaction({
            sessionId: tx.sessionId,
            bankTransactionId: tx.transactionId,
            accountId: selectedAccountId.value,
            categoryId: selectedCategoryId.value,
            amount: tx.amount,
            transactionDate: tx.date,
            bookingDate: tx.bookingDate ?? '',
            description: tx.description ?? '',
            creditDebit: tx.creditDebit,
            currency: tx.currency
        });
        invalidateNewTransactionsCache();
        emit('update:show', false);
        emit('accepted', tx);
        showToast('Transaction added to ledger');
    } catch (error: unknown) {
        if (!(error as { processed?: boolean }).processed) {
            showToast((error as Error).message || 'Failed to add transaction');
        }
    } finally {
        submitting.value = false;
    }
}

function onSheetClosed(): void {
    emit('update:show', false);
}
</script>

<style>
.new-transaction-categorise-sheet {
    height: 70%;
}
</style>
