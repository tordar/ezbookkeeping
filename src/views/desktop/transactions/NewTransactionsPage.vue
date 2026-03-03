<template>
    <v-row>
        <v-col cols="12">
            <v-card :title="tt('New transactions')">
                <v-card-text>
                    <span class="text-body-1">{{ tt('Incoming bank transactions from the last 48 hours. Categorise to add to your ledger, or dismiss to ignore.') }}</span>
                </v-card-text>
                <v-card-text>
                    <v-progress-linear v-if="loading" indeterminate color="primary" class="mb-3" />
                    <template v-else-if="transactions.length === 0">
                        <p class="text-body-2 text-medium-emphasis">{{ tt('No new transactions in the last 48 hours.') }}</p>
                    </template>
                    <template v-else>
                        <v-table density="compact" class="text-body-2">
                            <thead>
                                <tr>
                                    <th>{{ tt('Date') }}</th>
                                    <th>{{ tt('Description') }}</th>
                                    <th>{{ tt('Counterparty') }}</th>
                                    <th>{{ tt('Bank') }}</th>
                                    <th class="text-end">{{ tt('Amount') }}</th>
                                    <th class="text-end">{{ tt('Actions') }}</th>
                                </tr>
                            </thead>
                            <tbody>
                                <tr v-for="tx in transactions" :key="tx.sessionId + '-' + getBankTransactionId(tx)">
                                    <td>{{ tx.date || tt('No date') }}</td>
                                    <td>{{ tx.description || '–' }}</td>
                                    <td>{{ tx.counterpartyName || '–' }}</td>
                                    <td>{{ tx.aspspName }}</td>
                                    <td class="text-end" :class="tx.creditDebit === 'CRDT' ? 'text-success' : 'text-error'">
                                        {{ tx.amount }} {{ tx.currency }}
                                    </td>
                                    <td class="text-end">
                                        <v-btn
                                            size="small"
                                            variant="text"
                                            color="primary"
                                            :loading="categorisingId === tx.sessionId + '-' + getBankTransactionId(tx)"
                                            :disabled="dismissingId !== null"
                                            @click="openCategoriseDialog(tx)"
                                        >
                                            {{ tt('Categorise') }}
                                        </v-btn>
                                        <v-btn
                                            size="small"
                                            variant="text"
                                            color="error"
                                            :loading="dismissingId === tx.sessionId + '-' + getBankTransactionId(tx)"
                                            :disabled="categorisingId !== null"
                                            @click="dismiss(tx)"
                                        >
                                            {{ tt('Dismiss') }}
                                        </v-btn>
                                    </td>
                                </tr>
                            </tbody>
                        </v-table>
                    </template>
                </v-card-text>
            </v-card>
        </v-col>
    </v-row>

    <v-dialog v-model="categoriseDialog" persistent max-width="500">
        <v-card>
            <v-card-title>{{ tt('Categorise transaction') }}</v-card-title>
            <v-card-text>
                <v-select
                    v-model="selectedAccountId"
                    :items="accountOptions"
                    item-title="name"
                    item-value="id"
                    :label="tt('Account')"
                    density="comfortable"
                    clearable
                    :menu-props="{ maxHeight: 320 }"
                    class="mb-3"
                />
                <v-select
                    v-model="selectedCategoryId"
                    :items="categoryOptionsFlat"
                    item-title="name"
                    item-value="id"
                    :label="tt('Category')"
                    density="comfortable"
                    clearable
                    :menu-props="{ maxHeight: 320 }"
                />
            </v-card-text>
            <v-card-actions>
                <v-spacer />
                <v-btn variant="text" type="button" @click="categoriseDialog = false">{{ tt('Cancel') }}</v-btn>
                <v-btn type="button" color="primary" :loading="accepting" @click="accept()">
                    {{ tt('Add to ledger') }}
                </v-btn>
            </v-card-actions>
        </v-card>
    </v-dialog>

    <snack-bar ref="snackbarRef" />
</template>

<script setup lang="ts">
import { ref, computed, onMounted, useTemplateRef, nextTick } from 'vue';
import { useI18n } from '@/locales/helpers.ts';
import services from '@/lib/services.ts';
import type { NewBankTransactionItem } from '@/models/bank_integration.ts';
import { useAccountsStore } from '@/stores/account.ts';
import { useTransactionCategoriesStore } from '@/stores/transactionCategory.ts';
import { CategoryType } from '@/core/category.ts';
import SnackBar from '@/components/desktop/SnackBar.vue';

type SnackBarType = InstanceType<typeof SnackBar>;
const snackbarRef = useTemplateRef<SnackBarType>('snackbarRef');

const { tt } = useI18n();
const transactions = ref<NewBankTransactionItem[]>([]);
const loading = ref(true);
const categorisingId = ref<string | null>(null);
const dismissingId = ref<string | null>(null);
const categoriseDialog = ref(false);
const accepting = ref(false);
const pendingTx = ref<NewBankTransactionItem | null>(null);
const pendingBankTransactionId = ref<string>('');
const selectedAccountId = ref<string | number | null>(null);
const selectedCategoryId = ref<string | null>(null);

const accountsStore = useAccountsStore();
const transactionCategoriesStore = useTransactionCategoriesStore();

const accountOptions = computed(() => {
    const accounts = accountsStore.allVisiblePlainAccounts ?? [];
    return accounts.map(a => ({ id: String(a.id), name: a.name }));
});

const categoryTypeForPending = computed(() =>
    pendingTx.value?.creditDebit === 'CRDT' ? CategoryType.Income : CategoryType.Expense
);

const categoryOptionsFlat = computed(() => {
    const type = categoryTypeForPending.value;
    const list = transactionCategoriesStore.allTransactionCategories?.[type] ?? [];
    const options: { id: string; name: string }[] = [];
    for (const primary of list) {
        if (primary.hidden) continue;
        if (primary.subCategories?.length) {
            for (const sub of primary.subCategories) {
                if (sub.hidden) continue;
                options.push({ id: String(sub.id), name: `${primary.name} › ${sub.name}` });
            }
        } else {
            options.push({ id: String(primary.id), name: primary.name });
        }
    }
    return options;
});

async function load() {
    loading.value = true;
    try {
        const res = await services.getBankIntegrationNewTransactions();
        const data = res.data.result as { transactions?: NewBankTransactionItem[] } | undefined;
        transactions.value = data?.transactions ?? [];
    } catch {
        transactions.value = [];
    } finally {
        loading.value = false;
    }
}

function openCategoriseDialog(tx: NewBankTransactionItem) {
    pendingTx.value = tx;
    pendingBankTransactionId.value = getBankTransactionId(tx);
    selectedAccountId.value = tx.defaultAccountId ? String(tx.defaultAccountId) : null;
    selectedCategoryId.value = null;
    categoriseDialog.value = true;
    void accountsStore.loadAllAccounts({ force: false });
    void transactionCategoriesStore.loadAllCategories({ force: false });
}

function getBankTransactionId(tx: NewBankTransactionItem): string {
    const txAny = tx as unknown as Record<string, unknown>;
    const keys = ['transactionId', 'transaction_id', 'TransactionID'];
    for (const k of keys) {
        const v = txAny[k];
        if (v != null && String(v).trim() !== '') return String(v).trim();
    }
    if (tx.transactionId != null && String(tx.transactionId).trim() !== '') return String(tx.transactionId).trim();
    return '';
}

async function accept() {
    await nextTick();
    const tx = pendingTx.value;
    const categoryIdRaw = selectedCategoryId.value;
    const accountIdRaw = selectedAccountId.value;
    const bankTransactionId = pendingBankTransactionId.value || (tx ? getBankTransactionId(tx) : '');
    const accountIdStr = accountIdRaw != null && String(accountIdRaw).trim() !== '' ? String(accountIdRaw).trim() : '';
    const categoryIdStr = categoryIdRaw != null && String(categoryIdRaw).trim() !== '' ? String(categoryIdRaw).trim() : '';
    if (!tx || !bankTransactionId) {
        snackbarRef.value?.showMessage('Missing transaction. Please try again.');
        return;
    }
    if (!accountIdStr) {
        snackbarRef.value?.showMessage('Please select an account.');
        return;
    }
    if (!categoryIdStr) {
        snackbarRef.value?.showMessage('Please select a category.');
        return;
    }
    accepting.value = true;
    const key = tx.sessionId + '-' + bankTransactionId;
    categorisingId.value = key;
    try {
        await services.acceptBankIntegrationNewTransaction({
            sessionId: tx.sessionId,
            bankTransactionId,
            accountId: accountIdStr,
            categoryId: categoryIdStr,
            amount: tx.amount,
            transactionDate: tx.date,
            bookingDate: tx.bookingDate ?? '',
            description: tx.description ?? '',
            creditDebit: tx.creditDebit,
            currency: tx.currency
        });
        categoriseDialog.value = false;
        pendingTx.value = null;
        pendingBankTransactionId.value = '';
        transactions.value = transactions.value.filter(t => t.sessionId !== tx.sessionId || getBankTransactionId(t) !== bankTransactionId);
        snackbarRef.value?.showMessage('Transaction added to ledger.');
    } catch (err: unknown) {
        if (!(err as { processed?: boolean }).processed) {
            const data = (err as { response?: { data?: Record<string, unknown> } }).response?.data;
            const message = (data?.['errorMessage'] ?? data?.['message'] ?? (err as Error).message) as string | undefined;
            snackbarRef.value?.showError(message ?? 'Failed to add transaction.');
        }
    } finally {
        accepting.value = false;
        categorisingId.value = null;
    }
}

async function dismiss(tx: NewBankTransactionItem) {
    const bankTransactionId = getBankTransactionId(tx);
    if (!bankTransactionId) return;
    dismissingId.value = tx.sessionId + '-' + bankTransactionId;
    try {
        await services.dismissBankIntegrationNewTransaction({
            sessionId: tx.sessionId,
            bankTransactionId
        });
        transactions.value = transactions.value.filter(t => t.sessionId !== tx.sessionId || getBankTransactionId(t) !== bankTransactionId);
    } finally {
        dismissingId.value = null;
    }
}

onMounted(() => {
    void accountsStore.loadAllAccounts({ force: false });
    void transactionCategoriesStore.loadAllCategories({ force: false });
    void load();
});
</script>
