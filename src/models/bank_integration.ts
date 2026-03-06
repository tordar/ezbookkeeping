export interface BankConnectionResponse {
    sessionId: string;
    aspspName: string;
    aspspCountry: string;
    validUntil?: string;
    selectedAccountUid?: string;
    selectedAccountName?: string;
    defaultAccountId?: string | number;
    createdAt: number;
}

export interface BankConnectionAccount {
    uid: string;
    name?: string;
    iban?: string;
    bban?: string;
    currency?: string;
    balance?: string;
}

export interface BankConnectionAccountsResponse {
    accounts: BankConnectionAccount[];
}

export interface SetConnectionAccountRequest {
    sessionId: string;
    accountUid: string;
    accountName?: string;
}

export interface SetConnectionDefaultAccountRequest {
    sessionId: string;
    defaultAccountId: number | string;
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

export interface NewBankTransactionItem {
    sessionId: string;
    aspspName: string;
    transactionId: string;
    date: string;
    bookingDate?: string;
    amount: string;
    currency: string;
    creditDebit: string;
    description: string;
    counterpartyName?: string;
    defaultAccountId?: string | number;
}

export interface NewBankTransactionsResponse {
    transactions: NewBankTransactionItem[];
}

export interface AcceptNewBankTransactionRequest {
    sessionId: string;
    bankTransactionId: string;
    accountId: number | string;
    categoryId: number | string;
    amount: string;
    transactionDate: string;
    bookingDate?: string;
    description?: string;
    creditDebit: string;
    currency?: string;
}

export interface DismissNewBankTransactionRequest {
    sessionId: string;
    bankTransactionId: string;
}
