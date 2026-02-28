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
                    <div class="margin-top-half">
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
    </f7-page>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import type { Router } from 'framework7/types';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents } from '@/lib/ui/mobile.ts';
import services from '@/lib/services.ts';
import type { BankConnectionResponse } from '@/models/bank_integration.ts';

const props = defineProps<{
    f7router: Router.Router;
    f7route: Router.Route;
}>();

const { tt } = useI18n();
const { showToast, showConfirm } = useI18nUIComponents();

const connections = ref<BankConnectionResponse[]>([]);
const loading = ref(true);
const reauthingSessionId = ref<string | null>(null);
const disconnectingSessionId = ref<string | null>(null);
const callbackStatus = ref<string | null>(null);
const callbackMessage = ref('');

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

async function loadConnections(): Promise<void> {
    loading.value = true;
    try {
        const res = await services.getBankIntegrationConnections();
        connections.value = (res.data.result ?? []) as BankConnectionResponse[];
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
loadConnections();
</script>
