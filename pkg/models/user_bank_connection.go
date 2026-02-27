package models

// UserBankConnection represents a user's bank connection (Enable Banking session) stored in database
type UserBankConnection struct {
	Id               int64  `xorm:"PK AUTOINCR"`
	Uid              int64  `xorm:"INDEX NOT NULL"`
	SessionId        string `xorm:"VARCHAR(64) UNIQUE NOT NULL"`
	AspspName        string `xorm:"VARCHAR(128) NOT NULL"`
	AspspCountry     string `xorm:"VARCHAR(2) NOT NULL"`
	ValidUntil       string `xorm:"VARCHAR(32)"`
	CreatedUnixTime  int64
}

// TableName returns the table name for xorm
func (u *UserBankConnection) TableName() string {
	return "user_bank_connection"
}

// UserBankConnectionResponse represents a view-object of a bank connection
type UserBankConnectionResponse struct {
	SessionId    string `json:"sessionId"`
	AspspName    string `json:"aspspName"`
	AspspCountry string `json:"aspspCountry"`
	ValidUntil   string `json:"validUntil,omitempty"`
	CreatedAt    int64  `json:"createdAt"`
}

// ToResponse returns a view-object for API response
func (u *UserBankConnection) ToResponse() *UserBankConnectionResponse {
	return &UserBankConnectionResponse{
		SessionId:    u.SessionId,
		AspspName:    u.AspspName,
		AspspCountry: u.AspspCountry,
		ValidUntil:   u.ValidUntil,
		CreatedAt:    u.CreatedUnixTime,
	}
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
