import services from '@/lib/services.ts';
import type { NewBankTransactionItem } from '@/models/bank_integration.ts';

const CACHE_TTL_MS = 5 * 60 * 1000; // 5 minutes

let cachedTransactions: NewBankTransactionItem[] | null = null;
let lastFetchTime = 0;
let inflight: Promise<NewBankTransactionItem[]> | null = null;

export async function getCachedNewTransactions(force = false): Promise<NewBankTransactionItem[]> {
    const now = Date.now();

    if (!force && cachedTransactions !== null && (now - lastFetchTime) < CACHE_TTL_MS) {
        return cachedTransactions;
    }

    // Deduplicate concurrent calls — return the same promise if one is already in flight
    if (inflight) {
        return inflight;
    }

    inflight = services.getBankIntegrationNewTransactions()
        .then(res => {
            cachedTransactions = res.data.result?.transactions ?? [];
            lastFetchTime = Date.now();
            inflight = null;
            return cachedTransactions;
        })
        .catch(err => {
            inflight = null;
            throw err;
        });

    return inflight;
}

export function invalidateNewTransactionsCache(): void {
    cachedTransactions = null;
    lastFetchTime = 0;
}
