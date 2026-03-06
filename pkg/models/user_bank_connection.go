package models

// UserBankConnection represents a user's bank connection (Enable Banking session) stored in database
type UserBankConnection struct {
	Id                  int64  `xorm:"PK AUTOINCR"`
	Uid                 int64  `xorm:"INDEX NOT NULL"`
	SessionId           string `xorm:"VARCHAR(64) UNIQUE NOT NULL"`
	AspspName           string `xorm:"VARCHAR(128) NOT NULL"`
	AspspCountry        string `xorm:"VARCHAR(2) NOT NULL"`
	ValidUntil          string `xorm:"VARCHAR(32)"`
	SelectedAccountUID  string `xorm:"selected_account_uid VARCHAR(128)"`
	SelectedAccountName string `xorm:"selected_account_name VARCHAR(256)"`
	DefaultAccountId    int64  `xorm:"default_account_id BIGINT"`
	CreatedUnixTime     int64
}

// TableName returns the table name for xorm
func (u *UserBankConnection) TableName() string {
	return "user_bank_connection"
}

// UserBankConnectionResponse represents a view-object of a bank connection
type UserBankConnectionResponse struct {
	SessionId           string `json:"sessionId"`
	AspspName           string `json:"aspspName"`
	AspspCountry        string `json:"aspspCountry"`
	ValidUntil          string `json:"validUntil,omitempty"`
	SelectedAccountUID  string `json:"selectedAccountUid,omitempty"`
	SelectedAccountName string `json:"selectedAccountName,omitempty"`
	DefaultAccountId    int64  `json:"defaultAccountId,string,omitempty"`
	CreatedAt           int64  `json:"createdAt"`
}

// ToResponse returns a view-object for API response
func (u *UserBankConnection) ToResponse() *UserBankConnectionResponse {
	return &UserBankConnectionResponse{
		SessionId:           u.SessionId,
		AspspName:           u.AspspName,
		AspspCountry:        u.AspspCountry,
		ValidUntil:          u.ValidUntil,
		SelectedAccountUID:  u.SelectedAccountUID,
		SelectedAccountName: u.SelectedAccountName,
		DefaultAccountId:    u.DefaultAccountId,
		CreatedAt:           u.CreatedUnixTime,
	}
}

// BankConnectionAccount represents a single account available for a bank connection
type BankConnectionAccount struct {
	UID      string `json:"uid"`
	Name     string `json:"name,omitempty"`
	IBAN     string `json:"iban,omitempty"`
	BBAN     string `json:"bban,omitempty"`
	Currency string `json:"currency,omitempty"`
	Balance  string `json:"balance,omitempty"`
}

// BankConnectionAccountsResponse holds the available accounts for a bank connection
type BankConnectionAccountsResponse struct {
	Accounts []*BankConnectionAccount `json:"accounts"`
}

// SetConnectionAccountRequest is the request to set the selected account for a connection
type SetConnectionAccountRequest struct {
	SessionId   string `json:"sessionId" binding:"required,notBlank"`
	AccountUID  string `json:"accountUid" binding:"required,notBlank"`
	AccountName string `json:"accountName"`
}

// SetConnectionDefaultAccountRequest is the request to set the default ledger account for a connection
type SetConnectionDefaultAccountRequest struct {
	SessionId        string `json:"sessionId" binding:"required,notBlank"`
	DefaultAccountId int64  `json:"defaultAccountId,string" binding:"required,min=1"`
}

// StartBankAuthRequest represents request to start bank authorization
type StartBankAuthRequest struct {
	AspspName    string `json:"aspspName" binding:"required,notBlank"`
	AspspCountry string `json:"aspspCountry" binding:"required,len=2"`
}

// StartBankAuthResponse represents response with redirect URL
type StartBankAuthResponse struct {
	Url string `json:"url"`
}

// DisconnectBankRequest represents request to disconnect a bank
type DisconnectBankRequest struct {
	SessionId string `json:"sessionId" binding:"required,notBlank"`
}

// BankConnectionTransactionItem represents a single transaction from the bank for API response
type BankConnectionTransactionItem struct {
	TransactionID    string  `json:"transactionId"`
	Date             string  `json:"date"`
	Amount           string  `json:"amount"`
	Currency         string  `json:"currency"`
	CreditDebit      string  `json:"creditDebit"` // CRDT or DBIT
	Description      string  `json:"description"`
	CounterpartyName string  `json:"counterpartyName,omitempty"`
}

// BankConnectionTransactionsResponse holds latest N bank transactions for a connection
type BankConnectionTransactionsResponse struct {
	Transactions []*BankConnectionTransactionItem `json:"transactions"`
}

// UserBankTransactionAction represents a user's action on a bank transaction (accepted or dismissed)
type UserBankTransactionAction struct {
	Id                    int64  `xorm:"PK AUTOINCR"`
	Uid                   int64  `xorm:"UNIQUE(UQE_user_bank_tx_action) INDEX NOT NULL"`
	ConnectionSessionId   string `xorm:"VARCHAR(64) UNIQUE(UQE_user_bank_tx_action) NOT NULL"`
	BankTransactionId     string `xorm:"VARCHAR(128) UNIQUE(UQE_user_bank_tx_action) NOT NULL"`
	Status                string `xorm:"VARCHAR(16) NOT NULL"` // "accepted" | "dismissed"
	CreatedTransactionId  int64  `xorm:"NOT NULL"`              // set when status=accepted
	CreatedUnixTime       int64
}

// TableName returns the table name for xorm
func (u *UserBankTransactionAction) TableName() string {
	return "user_bank_transaction_action"
}

const (
	UserBankTransactionActionStatusAccepted = "accepted"
	UserBankTransactionActionStatusDismissed = "dismissed"
)

// NewBankTransactionItem is a bank transaction with connection info for the "new transactions" list
type NewBankTransactionItem struct {
	SessionId        string `json:"sessionId"`
	AspspName        string `json:"aspspName"`
	TransactionID    string `json:"transactionId"`
	Date             string `json:"date"`             // transaction date, or booking date as fallback for display
	BookingDate      string `json:"bookingDate"`       // raw booking date from API; used as fallback when accepting if no transaction date
	Amount           string `json:"amount"`
	Currency         string `json:"currency"`
	CreditDebit      string `json:"creditDebit"`
	Description      string `json:"description"`
	CounterpartyName string `json:"counterpartyName,omitempty"`
	DefaultAccountId int64  `json:"defaultAccountId,string,omitempty"`
}

// NewBankTransactionsResponse holds pending new transactions (last 48h, not yet accepted/dismissed)
type NewBankTransactionsResponse struct {
	Transactions []*NewBankTransactionItem `json:"transactions"`
}

// AcceptNewBankTransactionRequest is the request to accept (categorise) a new bank transaction
type AcceptNewBankTransactionRequest struct {
	SessionId        string `json:"sessionId" binding:"required,notBlank"`
	BankTransactionId string `json:"bankTransactionId" binding:"required,notBlank"`
	AccountId        int64  `json:"accountId,string" binding:"required,min=1"`
	CategoryId       int64  `json:"categoryId,string" binding:"required,min=1"`
	Amount           string `json:"amount" binding:"required"`
	TransactionDate  string `json:"transactionDate"` // optional; backend falls back to bookingDate, then today
	BookingDate      string `json:"bookingDate"`     // optional fallback when transactionDate is empty
	Description      string `json:"description"`
	CreditDebit      string `json:"creditDebit" binding:"required"` // CRDT or DBIT
	Currency         string `json:"currency"`
}

// DismissNewBankTransactionRequest is the request to dismiss a new bank transaction
type DismissNewBankTransactionRequest struct {
	SessionId         string `json:"sessionId" binding:"required,notBlank"`
	BankTransactionId string `json:"bankTransactionId" binding:"required,notBlank"`
}
