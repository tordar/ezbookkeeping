package enablebanking

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	defaultAPIBaseURL = "https://api.enablebanking.com"
	jwtIssuer         = "enablebanking.com"
	jwtAudience       = "api.enablebanking.com"
	jwtMaxTTL         = 86400 // 24 hours in seconds
)

// Client calls Enable Banking API with JWT authentication
type Client struct {
	baseURL    string
	appID      string
	privateKey *rsa.PrivateKey
	httpClient *http.Client
}

// Config holds Enable Banking client configuration
type Config struct {
	APIBaseURL string // e.g. https://api.enablebanking.com
	AppID      string
	PrivateKey string // PEM-encoded RSA private key
}

// NewClient creates a new Enable Banking API client
func NewClient(cfg *Config) (*Client, error) {
	if cfg == nil || cfg.AppID == "" || cfg.PrivateKey == "" {
		return nil, fmt.Errorf("enablebanking: app_id and private_key are required")
	}
	baseURL := strings.TrimSuffix(cfg.APIBaseURL, "/")
	if baseURL == "" {
		baseURL = defaultAPIBaseURL
	}
	block, _ := pem.Decode([]byte(cfg.PrivateKey))
	if block == nil {
		return nil, fmt.Errorf("enablebanking: failed to decode PEM private key")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		keyInterface, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err2 != nil {
			return nil, fmt.Errorf("enablebanking: parse private key: %w", err)
		}
		var ok bool
		key, ok = keyInterface.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("enablebanking: key is not RSA")
		}
	}
	return &Client{
		baseURL:    baseURL,
		appID:      cfg.AppID,
		privateKey: key,
		httpClient: &http.Client{Timeout: 90 * time.Second},
	}, nil
}

// buildJWT returns a signed JWT for API authentication (max TTL 24h)
func (c *Client) buildJWT(ttlSec int) (string, error) {
	if ttlSec <= 0 || ttlSec > jwtMaxTTL {
		ttlSec = jwtMaxTTL
	}
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"iss": jwtIssuer,
		"aud": jwtAudience,
		"iat": now.Unix(),
		"exp": now.Unix() + int64(ttlSec),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = c.appID
	signed, err := token.SignedString(c.privateKey)
	if err != nil {
		return "", err
	}
	return signed, nil
}

// doRequest sends an authenticated request to the Enable Banking API
func (c *Client) doRequest(method, path string, body io.Reader) (*http.Response, error) {
	jwtStr, err := c.buildJWT(3600)
	if err != nil {
		return nil, err
	}
	url := c.baseURL + path
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+jwtStr)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return c.httpClient.Do(req)
}

// ASPSP is a bank/similar institution from Enable Banking
type ASPSP struct {
	Name    string `json:"name"`
	Country string `json:"country"`
	Logo    string `json:"logo,omitempty"`
	Bic     string `json:"bic,omitempty"`
	Beta    bool   `json:"beta"`
}

// GetAspspsResponse is the response from GET /aspsps
type GetAspspsResponse struct {
	Aspsps []ASPSPData `json:"aspsps"`
}

// ASPSPData includes auth methods and metadata
type ASPSPData struct {
	Name    string `json:"name"`
	Country string `json:"country"`
	Logo    string `json:"logo"`
	Bic     string `json:"bic,omitempty"`
	Beta    bool   `json:"beta"`
}

// GetAspsps returns list of ASPSPs (optionally filtered by country)
func (c *Client) GetAspsps(country string) (*GetAspspsResponse, error) {
	path := "/aspsps"
	if country != "" {
		path += "?country=" + country
	}
	resp, err := c.doRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("enablebanking GET aspsps: %s %s", resp.Status, string(body))
	}
	var out GetAspspsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// StartAuthRequest is the body for POST /auth
type StartAuthRequest struct {
	Access struct {
		ValidUntil string `json:"valid_until"` // RFC3339
	} `json:"access"`
	Aspsp struct {
		Name    string `json:"name"`
		Country string `json:"country"`
	} `json:"aspsp"`
	State       string `json:"state"`
	RedirectURL string `json:"redirect_url"`
	PsuType     string `json:"psu_type,omitempty"` // personal | business
}

// StartAuthResponse is the response from POST /auth
type StartAuthResponse struct {
	URL             string `json:"url"`
	AuthorizationID  string `json:"authorization_id"`
	PsuIDHash       string `json:"psu_id_hash,omitempty"`
}

// StartAuth starts user authorization and returns the URL to redirect the user to
func (c *Client) StartAuth(aspspName, aspspCountry, state, redirectURL string) (*StartAuthResponse, error) {
	validUntil := time.Now().UTC().Add(90 * 24 * time.Hour).Format(time.RFC3339) // 90 days
	body := map[string]interface{}{
		"access": map[string]string{"valid_until": validUntil},
		"aspsp":  map[string]string{"name": aspspName, "country": aspspCountry},
		"state":  state,
		"redirect_url": redirectURL,
		"psu_type": "personal",
	}
	raw, _ := json.Marshal(body)
	resp, err := c.doRequest(http.MethodPost, "/auth", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("enablebanking POST auth: %s %s", resp.Status, string(b))
	}
	var out StartAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AuthorizeSessionRequest is the body for POST /sessions
type AuthorizeSessionRequest struct {
	Code string `json:"code"`
}

// AuthorizeSessionResponse is the response from POST /sessions
type AuthorizeSessionResponse struct {
	SessionID string         `json:"session_id"`
	Accounts  []AccountInfo `json:"accounts"`
	Aspsp     struct {
		Name    string `json:"name"`
		Country string `json:"country"`
	} `json:"aspsp"`
	Access struct {
		ValidUntil string `json:"valid_until"`
	} `json:"access"`
}

// AccountInfo is minimal account data from session response
type AccountInfo struct {
	AccountID struct {
		IBAN string `json:"iban,omitempty"`
	} `json:"account_id"`
	Name     string `json:"name,omitempty"`
	Currency string `json:"currency,omitempty"`
}

// AuthorizeSession exchanges the authorization code for a session
func (c *Client) AuthorizeSession(code string) (*AuthorizeSessionResponse, error) {
	raw, _ := json.Marshal(AuthorizeSessionRequest{Code: code})
	resp, err := c.doRequest(http.MethodPost, "/sessions", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("enablebanking POST sessions: %s %s", resp.Status, string(b))
	}
	var out AuthorizeSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteSession revokes the session at Enable Banking
func (c *Client) DeleteSession(sessionID string) error {
	path := "/sessions/" + sessionID
	resp, err := c.doRequest(http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("enablebanking DELETE session: %s %s", resp.Status, string(b))
	}
	return nil
}

// GetSessionResponse is the response from GET /sessions/{session_id}
type GetSessionResponse struct {
	Status       string                  `json:"status"`
	Accounts     []string                `json:"accounts"`
	AccountsData []GetSessionAccountData `json:"accounts_data"`
	Aspsp        struct {
		Name    string `json:"name"`
		Country string `json:"country"`
	} `json:"aspsp"`
	Access     struct{ ValidUntil string } `json:"access"`
	Authorized string                     `json:"authorized"`
	Created    string                     `json:"created"`
	PsuType    string                     `json:"psu_type"`
}

// GetSessionAccountData holds account uid from session
type GetSessionAccountData struct {
	UID                string `json:"uid"`
	IdentificationHash string `json:"identification_hash"`
}

// GetSession returns session data including account IDs
func (c *Client) GetSession(sessionID string) (*GetSessionResponse, error) {
	path := "/sessions/" + sessionID
	resp, err := c.doRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("enablebanking GET session: %s %s", resp.Status, string(b))
	}
	var out GetSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// BankTransactionAmount is amount with currency
type BankTransactionAmount struct {
	Currency string `json:"currency"`
	Amount   string `json:"amount"`
}

// BankTransaction is a single transaction from Enable Banking API
type BankTransaction struct {
	TransactionID          string                 `json:"transaction_id"`
	EntryReference         string                 `json:"entry_reference"`
	TransactionAmount       *BankTransactionAmount `json:"transaction_amount"`
	CreditDebitIndicator   string                 `json:"credit_debit_indicator"`
	TransactionDate        string                 `json:"transaction_date"`
	BookingDate            string                 `json:"booking_date"`
	ValueDate              string                 `json:"value_date"`
	RemittanceInformation  []string               `json:"remittance_information"`
	Debtor                 *struct{ Name string } `json:"debtor"`
	Creditor               *struct{ Name string } `json:"creditor"`
	BankTransactionCode     *struct{ Description string } `json:"bank_transaction_code"`
	Status                 string                 `json:"status"`
}

// HalTransactions is the response from GET /accounts/{account_id}/transactions
type HalTransactions struct {
	Transactions    []BankTransaction `json:"transactions"`
	ContinuationKey string            `json:"continuation_key"`
}

// GetAccountTransactions fetches transactions for an account (date format: 2006-01-02).
// strategy is optional; use "longest" to request the longest available period (some ASPSPs need this).
// bookingStatus is optional; use "booked", "pending", or "both" to filter by booking status.
func (c *Client) GetAccountTransactions(accountID, dateFrom, dateTo, strategy, bookingStatus string) (*HalTransactions, error) {
	path := "/accounts/" + accountID + "/transactions"
	var params []string
	if dateFrom != "" {
		params = append(params, "date_from="+dateFrom)
	}
	if dateTo != "" {
		params = append(params, "date_to="+dateTo)
	}
	if strategy != "" {
		params = append(params, "strategy="+strategy)
	}
	if bookingStatus != "" {
		params = append(params, "booking_status="+bookingStatus)
	}
	if len(params) > 0 {
		path += "?" + strings.Join(params, "&")
	}
	resp, err := c.doRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, readErr
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("enablebanking GET account transactions: %s %s", resp.Status, string(body))
	}
	var out HalTransactions
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	// Some ASPSPs return HAL-style or different keys; try to fill Transactions from alternate shapes
	if len(out.Transactions) == 0 {
		var raw map[string]json.RawMessage
		if json.Unmarshal(body, &raw) == nil {
			// HAL: _embedded.transactions
			if emb, ok := raw["_embedded"]; ok {
				var embedded map[string]json.RawMessage
				if json.Unmarshal(emb, &embedded) == nil {
					if txJSON, ok := embedded["transactions"]; ok {
						_ = json.Unmarshal(txJSON, &out.Transactions)
					}
				}
			}
			// Top-level alternate keys
			if len(out.Transactions) == 0 {
				for _, key := range []string{"transaction_list", "transactionList", "TransactionList"} {
					if txJSON, ok := raw[key]; ok {
						if json.Unmarshal(txJSON, &out.Transactions) == nil {
							break
						}
					}
				}
			}
		}
	}
	return &out, nil
}

