<template>
    <f7-page ptr @ptr:refresh="reload">
        <f7-navbar>
            <f7-nav-left :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="tt('Bank Integration')"></f7-nav-title>
        </f7-navbar>

        <f7-block v-if="callbackMessage" class="margin-top">
            <f7-block strong inset
                      :class="callbackStatus === 'success' ? 'color-green' : 'color-red'">
                {{ callbackMessage }}
            </f7-block>
        </f7-block>

        <f7-list strong inset dividers class="margin-top skeleton-text" v-if="loading">
            <f7-list-item title="Bank name" after="Reauth"></f7-list-item>
            <f7-list-item title="Bank name" after="Reauth"></f7-list-item>
        </f7-list>

        <f7-list strong inset dividers class="margin-top" v-else-if="!loading && connections.length === 0">
            <f7-list-item :title="tt('No bank connections yet.')"></f7-list-item>
        </f7-list>

        <f7-list strong inset dividers class="margin-top" v-else>
            <f7-list-item v-for="conn in connections" :key="conn.sessionId"
                          :title="conn.aspspName"
                          :after="conn.aspspCountry">
                <template #media>
                    <f7-icon f7="building_columns"></f7-icon>
                </template>
                <template #footer>
                    <div v-if="conn.selectedAccountUid" class="margin-top-half text-color-gray">
                        {{ tt('Account') }}: {{ conn.selectedAccountName || accountLabel(conn.sessionId, conn.selectedAccountUid) }}
                    </div>
                    <div v-if="conn.defaultAccountId" class="margin-top-half text-color-gray">
                        {{ tt('Default') }}: {{ accountOptions.find(a => a.id === String(conn.defaultAccountId))?.name || conn.defaultAccountId }}
                    </div>
                    <div class="margin-top-half">
                        <f7-button small outline color="teal"
                                   class="margin-right-half display-inline-flex"
                                   :disabled="loadingAccountsFor === conn.sessionId || reauthingSessionId === conn.sessionId || disconnectingSessionId === conn.sessionId"
                                   @click="openAccountPicker(conn.sessionId)">
                            {{ loadingAccountsFor === conn.sessionId ? tt('Loading...') : tt('Choose account') }}
                        </f7-button>
                        <f7-button small outline color="indigo"
                                   class="margin-right-half display-inline-flex"
                                   @click="openDefaultAccountSheet(conn.sessionId)">
                            {{ tt('Default account') }}
                        </f7-button>
                        <f7-button small outline color="blue"
                                   class="margin-right-half display-inline-flex"
                                   :disabled="reauthingSessionId === conn.sessionId || disconnectingSessionId === conn.sessionId"
                                   @click="reauth(conn.sessionId)">
                            {{ reauthingSessionId === conn.sessionId ? tt('Redirecting...') : tt('Reauth') }}
                        </f7-button>
                        <f7-button small outline color="red"
                                   class="display-inline-flex"
                                   :disabled="disconnectingSessionId === conn.sessionId || reauthingSessionId === conn.sessionId"
                                   @click="confirmDisconnect(conn.sessionId)">
                            {{ disconnectingSessionId === conn.sessionId ? tt('Disconnecting...') : tt('Disconnect') }}
                        </f7-button>
                    </div>
                </template>
            </f7-list-item>
        </f7-list>

        <!-- Default account sheet -->
        <f7-sheet
            class="account-picker-sheet"
            :opened="defaultAccountSheetOpen"
            @sheet:closed="defaultAccountSheetOpen = false"
            swipe-to-close
        >
            <f7-page-content>
                <f7-block-title>{{ tt('Set default account') }}</f7-block-title>
                <f7-block>
                    <p class="text-color-gray">{{ tt('Transactions from this bank will pre-fill this account when categorising.') }}</p>
                </f7-block>
                <f7-list v-if="accountOptions.length > 0">
                    <f7-list-item
                        v-for="acc in accountOptions"
                        :key="acc.id"
                        :title="acc.name"
                        link
                        @click="selectDefaultAccount(acc.id)"
                    ></f7-list-item>
                </f7-list>
                <f7-block v-else>
                    <p>{{ tt('No accounts found.') }}</p>
                </f7-block>
            </f7-page-content>
        </f7-sheet>

        <!-- Account picker sheet -->
        <f7-sheet
            class="account-picker-sheet"
            :opened="accountPickerOpen"
            @sheet:closed="accountPickerOpen = false"
            swipe-to-close
        >
            <f7-page-content>
                <f7-block-title>{{ tt('Choose account') }}</f7-block-title>
                <f7-list v-if="pickerAccounts.length > 0">
                    <f7-list-item
                        v-for="acc in pickerAccounts"
                        :key="acc.uid"
                        :title="acc.name || acc.uid"
                        :after="acc.iban || acc.bban || (acc.currency && acc.balance ? acc.currency + ' ' + acc.balance : acc.currency) || ''"
                        link
                        @click="selectAccount(acc.uid)"
                    ></f7-list-item>
                </f7-list>
                <f7-block v-else>
                    <p>{{ tt('No accounts found.') }}</p>
                </f7-block>
            </f7-page-content>
        </f7-sheet>
    </f7-page>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import type { Router } from 'framework7/types';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents } from '@/lib/ui/mobile.ts';
import services from '@/lib/services.ts';
import type { BankConnectionResponse, BankConnectionAccount } from '@/models/bank_integration.ts';
import { useAccountsStore } from '@/stores/account.ts';

const props = defineProps<{
    f7router: Router.Router;
    f7route: Router.Route;
}>();

const { tt } = useI18n();
const { showToast, showConfirm } = useI18nUIComponents();
const accountsStore = useAccountsStore();

const connections = ref<BankConnectionResponse[]>([]);
const loading = ref(true);
const reauthingSessionId = ref<string | null>(null);
const disconnectingSessionId = ref<string | null>(null);
const callbackStatus = ref<string | null>(null);
const callbackMessage = ref('');

const loadingAccountsFor = ref<string | null>(null);
const accountPickerOpen = ref(false);
const pickerSessionId = ref<string | null>(null);
const pickerAccounts = ref<BankConnectionAccount[]>([]);
// Cache: sessionId -> accounts
const accountsCache = ref<Record<string, BankConnectionAccount[]>>({});

const defaultAccountSheetOpen = ref(false);
const defaultAccountSessionId = ref<string | null>(null);
const defaultAccountId = ref<string | null>(null);

const accountOptions = computed(() => (accountsStore.allVisiblePlainAccounts ?? []).map(a => ({ id: String(a.id), name: a.name })));

function openDefaultAccountSheet(sessionId: string): void {
    defaultAccountSessionId.value = sessionId;
    const conn = connections.value.find(c => c.sessionId === sessionId);
    defaultAccountId.value = conn?.defaultAccountId ? String(conn.defaultAccountId) : null;
    void accountsStore.loadAllAccounts({ force: false });
    defaultAccountSheetOpen.value = true;
}

async function selectDefaultAccount(accountId: string): Promise<void> {
    defaultAccountSheetOpen.value = false;
    const sessionId = defaultAccountSessionId.value;
    if (!sessionId || !accountId) return;
    try {
        await services.setBankConnectionDefaultAccount({ sessionId, defaultAccountId: accountId });
        const id = Number(accountId);
        connections.value = connections.value.map(c =>
            c.sessionId === sessionId ? { ...c, defaultAccountId: id } : c
        );
    } catch (error: unknown) {
        if (!(error as { processed?: boolean }).processed) {
            showToast((error as Error).message || tt('Failed to set default account'));
        }
    }
}

function accountLabel(sessionId: string, uid: string): string {
    const cached = accountsCache.value[sessionId];
    if (cached) {
        const acc = cached.find(a => a.uid === uid);
        if (acc) return acc.name || acc.iban || uid;
    }
    return uid;
}

async function openAccountPicker(sessionId: string): Promise<void> {
    loadingAccountsFor.value = sessionId;
    try {
        const res = await services.getBankConnectionAccounts(sessionId);
        const accounts = (res.data.result as { accounts?: BankConnectionAccount[] })?.accounts ?? [];
        accountsCache.value = { ...accountsCache.value, [sessionId]: accounts };
        pickerSessionId.value = sessionId;
        pickerAccounts.value = accounts;
        accountPickerOpen.value = true;
    } catch (error: unknown) {
        if (!(error as { processed?: boolean }).processed) {
            showToast((error as Error).message || tt('Failed to load accounts'));
        }
    } finally {
        loadingAccountsFor.value = null;
    }
}

async function selectAccount(accountUid: string): Promise<void> {
    accountPickerOpen.value = false;
    const sessionId = pickerSessionId.value;
    if (!sessionId) return;
    const accountName = pickerAccounts.value.find(a => a.uid === accountUid)?.name;
    try {
        await services.setBankConnectionAccount({ sessionId, accountUid, accountName });
        connections.value = connections.value.map(c =>
            c.sessionId === sessionId ? { ...c, selectedAccountUid: accountUid, selectedAccountName: accountName } : c
        );
    } catch (error: unknown) {
        if (!(error as { processed?: boolean }).processed) {
            showToast((error as Error).message || tt('Failed to set account'));
        }
    }
}

function parseCallbackParams(): void {
    const bank = props.f7route.query['bank'] as string | undefined;
    const msg = props.f7route.query['bank_message'] as string | undefined;
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

async function loadConnections(autoPickAccount = false): Promise<void> {
    loading.value = true;
    try {
        const res = await services.getBankIntegrationConnections();
        connections.value = (res.data.result ?? []) as BankConnectionResponse[];
        if (autoPickAccount) {
            const unpicked = connections.value.find(c => !c.selectedAccountUid);
            if (unpicked) {
                await openAccountPicker(unpicked.sessionId);
            }
        }
    } catch (error: unknown) {
        if (!(error as { processed?: boolean }).processed) {
            showToast((error as Error).message || 'Failed to load bank connections');
        }
    } finally {
        loading.value = false;
    }
}

async function reauth(sessionId: string): Promise<void> {
    reauthingSessionId.value = sessionId;
    try {
        const res = await services.startBankIntegrationReauth({ sessionId });
        const url = (res.data.result as { url?: string })?.url;
        if (url) {
            window.location.href = url;
            return;
        }
    } catch (error: unknown) {
        if (!(error as { processed?: boolean }).processed) {
            showToast((error as Error).message || 'Failed to start re-authorization');
        }
    } finally {
        reauthingSessionId.value = null;
    }
}

function confirmDisconnect(sessionId: string): void {
    showConfirm('Are you sure you want to disconnect this bank?', async () => {
        disconnectingSessionId.value = sessionId;
        try {
            await services.disconnectBankIntegration({ sessionId });
            connections.value = connections.value.filter(c => c.sessionId !== sessionId);
        } catch (error: unknown) {
            if (!(error as { processed?: boolean }).processed) {
                showToast((error as Error).message || 'Failed to disconnect bank');
            }
        } finally {
            disconnectingSessionId.value = null;
        }
    });
}

function reload(done?: () => void): void {
    loadConnections().then(() => done?.());
}

parseCallbackParams();
loadConnections(callbackStatus.value === 'success');
</script>
