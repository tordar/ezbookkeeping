<template>
    <v-row>
        <v-col cols="12">
            <v-card :title="tt('Bank Integration')">
                <v-card-text>
                    <span class="text-body-1">{{ tt('Manage your bank connections and linked accounts.') }}</span>
                </v-card-text>

                <v-card-text v-if="callbackMessage">
                    <v-alert
                        :type="callbackStatus === 'success' ? 'success' : 'error'"
                        variant="tonal"
                        closable
                        class="mb-4"
                    >
                        {{ callbackMessage }}
                    </v-alert>
                </v-card-text>

                <v-card-text>
                    <v-card variant="outlined" class="mb-4">
                        <v-card-title class="text-subtitle-1">{{ tt('Connected banks') }}</v-card-title>
                        <v-card-text>
                            <v-progress-linear v-if="loadingConnections" indeterminate color="primary" class="mb-3" />
                            <template v-else-if="connections.length === 0">
                                <p class="text-body-2 text-medium-emphasis">{{ tt('No bank connections yet.') }}</p>
                            </template>
                            <template v-else>
                                <v-list v-for="conn in connections" :key="conn.sessionId" class="py-0">
                                    <v-list-item class="px-0">
                                        <template #prepend>
                                            <v-icon :icon="mdiBankOutline" class="me-2" />
                                        </template>
                                        <v-list-item-title>{{ conn.aspspName }} ({{ conn.aspspCountry }})</v-list-item-title>
                                        <template #append>
                                            <div class="d-flex align-center flex-shrink-0">
                                                <v-btn
                                                    size="small"
                                                    variant="text"
                                                    class="me-1"
                                                    :loading="reauthSessionId === conn.sessionId"
                                                    @click="reauth(conn.sessionId)"
                                                >
                                                    {{ tt('Reauth') }}
                                                </v-btn>
                                                <v-btn
                                                    size="small"
                                                    color="error"
                                                    variant="text"
                                                    :loading="disconnectingSessionId === conn.sessionId"
                                                    @click="disconnect(conn.sessionId)"
                                                >
                                                    {{ tt('Disconnect') }}
                                                </v-btn>
                                            </div>
                                        </template>
                                    </v-list-item>
                                    <v-list-item class="px-0 pt-0">
                                        <v-card variant="outlined" class="mt-2 ms-8 flex-grow-1">
                                            <v-card-title class="text-subtitle-2">{{ tt('Latest 10 transactions') }}</v-card-title>
                                            <v-card-text>
                                                <v-progress-linear v-if="connectionTransactions[conn.sessionId]?.loading" indeterminate color="primary" class="mb-2" />
                                                <v-alert v-else-if="connectionTransactions[conn.sessionId]?.error" type="warning" variant="tonal" density="compact" class="mb-0">
                                                    {{ tt('Failed to load transactions.') }}
                                                    <v-btn size="small" variant="text" class="mt-1" @click="loadTransactionsForConnection(conn.sessionId)">
                                                        {{ tt('Retry') }}
                                                    </v-btn>
                                                </v-alert>
                                                <template v-else-if="(connectionTransactions[conn.sessionId]?.transactions?.length ?? 0) > 0">
                                                    <v-table density="compact" class="text-body-2">
                                                        <thead>
                                                            <tr>
                                                                <th>{{ tt('Date') }}</th>
                                                                <th>{{ tt('Description') }}</th>
                                                                <th>{{ tt('Counterparty') }}</th>
                                                                <th class="text-end">{{ tt('Amount') }}</th>
                                                            </tr>
                                                        </thead>
                                                        <tbody>
                                                            <tr v-for="tx in (connectionTransactions[conn.sessionId]?.transactions ?? [])" :key="tx.transactionId">
                                                                <td>{{ tx.date }}</td>
                                                                <td>{{ tx.description || '–' }}</td>
                                                                <td>{{ tx.counterpartyName || '–' }}</td>
                                                                <td class="text-end" :class="tx.creditDebit === 'CRDT' ? 'text-success' : 'text-error'">
                                                                    {{ tx.amount }} {{ tx.currency }}
                                                                </td>
                                                            </tr>
                                                        </tbody>
                                                    </v-table>
                                                </template>
                                                <p v-else-if="connectionTransactions[conn.sessionId] && !connectionTransactions[conn.sessionId]?.loading && !connectionTransactions[conn.sessionId]?.error" class="text-body-2 text-medium-emphasis mb-0">
                                                    {{ tt('No recent transactions.') }}
                                                </p>
                                            </v-card-text>
                                        </v-card>
                                    </v-list-item>
                                </v-list>
                            </template>
                        </v-card-text>
                    </v-card>

                    <v-card variant="outlined">
                        <v-card-title class="text-subtitle-1">{{ tt('Connect a bank') }}</v-card-title>
                        <v-card-text>
                            <v-row>
                                <v-col cols="12" md="4">
                                    <v-text-field
                                        v-model="connectCountry"
                                        :label="tt('Country code')"
                                        placeholder="NO"
                                        hint="Two-letter country code (e.g. NO, SE, FI)"
                                        persistent-hint
                                        maxlength="2"
                                        density="comfortable"
                                        @input="connectCountry = (connectCountry || '').toUpperCase().slice(0, 2)"
                                    />
                                </v-col>
                                <v-col cols="12" md="4" class="d-flex align-center">
                                    <v-btn
                                        color="primary"
                                        :loading="loadingAspsps"
                                        :disabled="!connectCountry || connectCountry.length !== 2"
                                        @click="loadBanks"
                                    >
                                        {{ tt('Load banks') }}
                                    </v-btn>
                                </v-col>
                            </v-row>
                            <v-progress-linear v-if="loadingAspsps" indeterminate color="primary" class="mb-3" />
                            <v-list v-else-if="aspsps.length > 0" class="mt-2">
                                <v-list-item
                                    v-for="bank in aspsps"
                                    :key="bank.name + bank.country"
                                    class="px-0"
                                    @click="connectBank(bank)"
                                >
                                    <template #prepend>
                                        <v-avatar v-if="bank.logo" size="36" rounded class="me-3">
                                            <v-img :src="bank.logo" :alt="bank.name" />
                                        </v-avatar>
                                        <v-icon v-else :icon="mdiBankOutline" class="me-3" />
                                    </template>
                                    <v-list-item-title>{{ bank.name }}</v-list-item-title>
                                    <v-list-item-subtitle>{{ bank.country }}</v-list-item-subtitle>
                                    <template #append>
                                        <v-btn size="small" variant="tonal" color="primary" :loading="startingAuthFor === bank.name + bank.country">
                                            {{ tt('Connect') }}
                                        </v-btn>
                                    </template>
                                </v-list-item>
                            </v-list>
                            <p v-else-if="loadedBanksOnce" class="text-body-2 text-medium-emphasis mt-2">
                                {{ tt('No banks found for this country, or bank integration is not configured.') }}
                            </p>
                        </v-card-text>
                    </v-card>
                </v-card-text>
            </v-card>
        </v-col>
    </v-row>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue';
import { useRoute } from 'vue-router';
import { useI18n } from '@/locales/helpers.ts';
import services from '@/lib/services.ts';
import type { BankConnectionResponse, AspspData, BankConnectionTransactionItem } from '@/models/bank_integration.ts';
import { mdiBankOutline } from '@mdi/js';

const { tt } = useI18n();
const route = useRoute();

const connections = ref<BankConnectionResponse[]>([]);
const loadingConnections = ref(false);
const disconnectingSessionId = ref<string | null>(null);
const reauthSessionId = ref<string | null>(null);

type ConnectionTransactionsState = { transactions: BankConnectionTransactionItem[]; loading: boolean; error?: boolean };
const connectionTransactions = ref<Record<string, ConnectionTransactionsState>>({});

const connectCountry = ref('NO');
const aspsps = ref<AspspData[]>([]);
const loadingAspsps = ref(false);
const loadedBanksOnce = ref(false);
const startingAuthFor = ref<string | null>(null);

const callbackStatus = ref<string | null>(null);
const callbackMessage = ref('');

function parseCallbackParams() {
    const bank = route.query['bank'] as string | undefined;
    const msg = route.query['bank_message'] as string | undefined;
    if (bank === 'success') {
        callbackStatus.value = 'success';
        callbackMessage.value = tt('Bank connected successfully.');
    } else if (bank === 'error' && msg) {
        callbackStatus.value = 'error';
        callbackMessage.value = decodeURIComponent(msg);
    } else if (bank === 'error') {
        callbackStatus.value = 'error';
        callbackMessage.value = tt('Connection was cancelled or failed.');
    }
}

async function loadTransactionsForConnection(sessionId: string) {
    // Replace entire object so Vue reactivity picks up the new key
    connectionTransactions.value = {
        ...connectionTransactions.value,
        [sessionId]: { transactions: [], loading: true, error: false }
    };
    try {
        const res = await services.getBankIntegrationConnectionTransactions(sessionId);
        const data = res.data.result as { transactions?: BankConnectionTransactionItem[] } | undefined;
        connectionTransactions.value = {
            ...connectionTransactions.value,
            [sessionId]: { transactions: data?.transactions ?? [], loading: false, error: false }
        };
    } catch {
        connectionTransactions.value = {
            ...connectionTransactions.value,
            [sessionId]: { transactions: [], loading: false, error: true }
        };
    }
}

async function loadConnections() {
    loadingConnections.value = true;
    try {
        const res = await services.getBankIntegrationConnections();
        connections.value = (res.data.result ?? []) as BankConnectionResponse[];
        connectionTransactions.value = {};
        for (const conn of connections.value) {
            void loadTransactionsForConnection(conn.sessionId);
        }
    } catch {
        connections.value = [];
    } finally {
        loadingConnections.value = false;
    }
}

// When connections load, fetch transactions for any that don't have state yet (e.g. after tab switch)
watch(connections, (newConnections) => {
    for (const conn of newConnections) {
        if (!connectionTransactions.value[conn.sessionId]) {
            void loadTransactionsForConnection(conn.sessionId);
        }
    }
}, { immediate: false });

async function loadBanks() {
    if (!connectCountry.value || connectCountry.value.length !== 2) return;
    loadingAspsps.value = true;
    loadedBanksOnce.value = true;
    aspsps.value = [];
    try {
        const res = await services.getBankIntegrationAspsps(connectCountry.value);
        const data = res.data.result as { aspsps?: AspspData[] } | undefined;
        aspsps.value = data?.aspsps ?? [];
    } catch {
        aspsps.value = [];
    } finally {
        loadingAspsps.value = false;
    }
}

async function connectBank(bank: AspspData) {
    const key = bank.name + bank.country;
    startingAuthFor.value = key;
    try {
        const res = await services.startBankIntegrationAuth({
            aspspName: bank.name,
            aspspCountry: bank.country
        });
        const url = (res.data.result as { url?: string })?.url;
        if (url) {
            window.location.href = url;
            return;
        }
    } catch {
        callbackStatus.value = 'error';
        callbackMessage.value = tt('Failed to start bank connection.');
    } finally {
        startingAuthFor.value = null;
    }
}

async function reauth(sessionId: string) {
    reauthSessionId.value = sessionId;
    try {
        const res = await services.startBankIntegrationReauth({ sessionId });
        const url = (res.data.result as { url?: string })?.url;
        if (url) {
            window.location.href = url;
            return;
        }
    } catch {
        callbackStatus.value = 'error';
        callbackMessage.value = tt('Failed to start re-authorization.');
    } finally {
        reauthSessionId.value = null;
    }
}

async function disconnect(sessionId: string) {
    disconnectingSessionId.value = sessionId;
    try {
        await services.disconnectBankIntegration({ sessionId });
        await loadConnections();
    } catch {
        callbackStatus.value = 'error';
        callbackMessage.value = tt('Failed to disconnect bank.');
    } finally {
        disconnectingSessionId.value = null;
    }
}

onMounted(() => {
    parseCallbackParams();
    loadConnections();
});
</script>
