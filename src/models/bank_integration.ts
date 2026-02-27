export interface BankConnectionResponse {
    sessionId: string;
    aspspName: string;
    aspspCountry: string;
    validUntil?: string;
    createdAt: number;
}

export interface AspspData {
    name: string;
    country: string;
    logo: string;
    bic?: string;
    beta: boolean;
}

export interface AspspsResponse {
    aspsps: AspspData[];
}

export interface StartBankAuthRequest {
    aspspName: string;
    aspspCountry: string;
}

export interface StartBankAuthResponse {
    url: string;
}

export interface DisconnectBankRequest {
    sessionId: string;
}

export interface BankConnectionTransactionItem {
    transactionId: string;
    date: string;
    amount: string;
    currency: string;
    creditDebit: string;
    description: string;
    counterpartyName?: string;
}

export interface BankConnectionTransactionsResponse {
    transactions: BankConnectionTransactionItem[];
}
