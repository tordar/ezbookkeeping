package api

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/duplicatechecker"
	"github.com/mayswind/ezbookkeeping/pkg/enablebanking"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

const bankAuthStateExpiration = 10 * time.Minute

// BankIntegrationConnectionsApi represents bank integration (Enable Banking) API
type BankIntegrationConnectionsApi struct {
	ApiUsingConfig
	ApiUsingDuplicateChecker
	connections *services.UserBankConnectionService
}

// Initialize bank integration connections API singleton
var (
	BankIntegrationConnections = &BankIntegrationConnectionsApi{
		ApiUsingConfig: ApiUsingConfig{
			container: settings.Container,
		},
		ApiUsingDuplicateChecker: ApiUsingDuplicateChecker{
			ApiUsingConfig: ApiUsingConfig{
				container: settings.Container,
			},
			container: duplicatechecker.Container,
		},
		connections: services.UserBankConnections,
	}
)

func (a *BankIntegrationConnectionsApi) getClient() (*enablebanking.Client, *errs.Error) {
	cfg := a.CurrentConfig()
	if !cfg.EnableBankIntegration {
		return nil, errs.ErrBankIntegrationDisabled
	}
	if cfg.EnableBankingAppID == "" || cfg.EnableBankingPrivateKey == "" {
		return nil, errs.ErrBankIntegrationNotConfigured
	}
	client, err := enablebanking.NewClient(&enablebanking.Config{
		APIBaseURL: cfg.EnableBankingAPIURL,
		AppID:      cfg.EnableBankingAppID,
		PrivateKey: cfg.EnableBankingPrivateKey,
	})
	if err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}
	return client, nil
}

// ListConnectionsHandler returns the current user's bank connections
func (a *BankIntegrationConnectionsApi) ListConnectionsHandler(c *core.WebContext) (any, *errs.Error) {
	cfg := a.CurrentConfig()
	if !cfg.EnableBankIntegration {
		return []*models.UserBankConnectionResponse{}, nil
	}

	uid := c.GetCurrentUid()
	list, err := a.connections.GetConnectionsByUid(c, uid)
	if err != nil {
		log.Errorf(c, "[bank_integration.ListConnectionsHandler] failed to get connections for uid %d: %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	resp := make([]*models.UserBankConnectionResponse, len(list))
	for i := range list {
		resp[i] = list[i].ToResponse()
	}
	return resp, nil
}

// GetAspspsHandler returns list of supported banks (ASPSPs), optionally filtered by country
func (a *BankIntegrationConnectionsApi) GetAspspsHandler(c *core.WebContext) (any, *errs.Error) {
	client, err := a.getClient()
	if err != nil {
		return nil, err
	}

	country := c.Query("country")
	data, goErr := client.GetAspsps(country)
	if goErr != nil {
		log.Errorf(c, "[bank_integration.GetAspspsHandler] Enable Banking GetAspsps failed: %s", goErr.Error())
		return nil, errs.Or(goErr, errs.ErrOperationFailed)
	}
	return data, nil
}

// StartAuthHandler starts bank authorization and returns the redirect URL
func (a *BankIntegrationConnectionsApi) StartAuthHandler(c *core.WebContext) (any, *errs.Error) {
	client, err := a.getClient()
	if err != nil {
		return nil, err
	}

	var req models.StartBankAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf(c, "[bank_integration.StartAuthHandler] parse request failed: %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	state, goErr := utils.GetRandomNumberOrLowercaseLetter(32)
	if goErr != nil {
		return nil, errs.Or(goErr, errs.ErrSystemError)
	}

	cfg := a.CurrentConfig()
	redirectURL := cfg.EnableBankingCallbackURL
	if redirectURL == "" {
		rootURL := strings.TrimSuffix(cfg.RootUrl, "/")
		redirectURL = rootURL + "/api/bank_integration/callback"
	}

	a.SetSubmissionRemarkWithCustomExpiration(
		duplicatechecker.DUPLICATE_CHECKER_TYPE_BANK_AUTH_STATE,
		0,
		state,
		strconv.FormatInt(uid, 10),
		bankAuthStateExpiration,
	)

	result, goErr := client.StartAuth(req.AspspName, req.AspspCountry, state, redirectURL)
	if goErr != nil {
		log.Errorf(c, "[bank_integration.StartAuthHandler] Enable Banking StartAuth failed: %s", goErr.Error())
		return nil, errs.Or(goErr, errs.ErrOperationFailed)
	}

	return &models.StartBankAuthResponse{Url: result.URL}, nil
}

// CallbackHandler handles the redirect from Enable Banking after user authorizes (no JWT)
func (a *BankIntegrationConnectionsApi) CallbackHandler(c *core.WebContext) (string, *errs.Error) {
	cfg := a.CurrentConfig()
	if !cfg.EnableBankIntegration {
		return a.redirectToSettings(c, "error", "Bank integration is disabled")
	}

	code := c.Query("code")
	state := c.Query("state")
	errParam := c.Query("error")

	if errParam != "" {
		desc := c.Query("error_description")
		if desc == "" {
			desc = errParam
		}
		log.Warnf(c, "[bank_integration.CallbackHandler] user returned with error: %s - %s", errParam, desc)
		return a.redirectToSettings(c, "error", desc)
	}

	if code == "" || state == "" {
		return a.redirectToSettings(c, "error", "Missing code or state")
	}

	found, remark := a.GetSubmissionRemark(duplicatechecker.DUPLICATE_CHECKER_TYPE_BANK_AUTH_STATE, 0, state)
	if !found {
		log.Warnf(c, "[bank_integration.CallbackHandler] invalid or expired state")
		return a.redirectToSettings(c, "error", "Authorization expired or invalid. Please try again.")
	}

	uid, err := strconv.ParseInt(remark, 10, 64)
	if err != nil || uid <= 0 {
		return a.redirectToSettings(c, "error", "Invalid state")
	}

	a.RemoveSubmissionRemark(duplicatechecker.DUPLICATE_CHECKER_TYPE_BANK_AUTH_STATE, 0, state)

	client, apiErr := a.getClient()
	if apiErr != nil {
		return a.redirectToSettings(c, "error", apiErr.Message)
	}

	session, goErr := client.AuthorizeSession(code)
	if goErr != nil {
		log.Errorf(c, "[bank_integration.CallbackHandler] AuthorizeSession failed: %s", goErr.Error())
		return a.redirectToSettings(c, "error", "Failed to complete bank authorization")
	}

	conn := &models.UserBankConnection{
		Uid:          uid,
		SessionId:    session.SessionID,
		AspspName:    session.Aspsp.Name,
		AspspCountry: session.Aspsp.Country,
		ValidUntil:   session.Access.ValidUntil,
	}
	if err := a.connections.CreateConnection(c, conn); err != nil {
		log.Errorf(c, "[bank_integration.CallbackHandler] CreateConnection failed: %s", err.Error())
		return a.redirectToSettings(c, "error", "Failed to save connection")
	}

	return a.redirectToSettings(c, "success", "")
}

func (a *BankIntegrationConnectionsApi) redirectToSettings(c *core.WebContext, status, message string) (string, *errs.Error) {
	cfg := a.CurrentConfig()
	base := cfg.RootUrl
	if len(base) > 0 && base[len(base)-1] == '/' {
		base = base[:len(base)-1]
	}
	redirectURL := fmt.Sprintf("%sdesktop#/user/settings?tab=bankIntegrationSetting&bank=%s", base, status)
	if message != "" {
		redirectURL += "&bank_message=" + url.QueryEscape(message)
	}
	return redirectURL, nil
}

// DisconnectHandler removes a bank connection
func (a *BankIntegrationConnectionsApi) DisconnectHandler(c *core.WebContext) (any, *errs.Error) {
	cfg := a.CurrentConfig()
	if !cfg.EnableBankIntegration {
		return nil, errs.ErrBankIntegrationDisabled
	}

	var req models.DisconnectBankRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	_, err := a.connections.GetConnectionBySessionId(c, uid, req.SessionId)
	if err != nil {
		return nil, errs.Or(err, errs.ErrBankConnectionNotFound)
	}

	// Revoke session at Enable Banking if configured
	if client, apiErr := a.getClient(); apiErr == nil {
		if delErr := client.DeleteSession(req.SessionId); delErr != nil {
			log.Warnf(c, "[bank_integration.DisconnectHandler] Enable Banking DeleteSession failed: %s", delErr.Error())
		}
	}

	if err := a.connections.DeleteConnection(c, uid, req.SessionId); err != nil {
		log.Errorf(c, "[bank_integration.DisconnectHandler] DeleteConnection failed: %s", err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return true, nil
}

const bankConnectionTransactionsLimit = 10
const bankConnectionTransactionsDays = 90

// GetConnectionTransactionsHandler returns the latest N transactions from the bank for a connection
func (a *BankIntegrationConnectionsApi) GetConnectionTransactionsHandler(c *core.WebContext) (any, *errs.Error) {
	cfg := a.CurrentConfig()
	if !cfg.EnableBankIntegration {
		return nil, errs.ErrBankIntegrationDisabled
	}

	sessionId := c.Query("sessionId")
	if sessionId == "" {
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(fmt.Errorf("sessionId is required"))
	}

	uid := c.GetCurrentUid()
	_, err := a.connections.GetConnectionBySessionId(c, uid, sessionId)
	if err != nil {
		return nil, errs.Or(err, errs.ErrBankConnectionNotFound)
	}

	client, apiErr := a.getClient()
	if apiErr != nil {
		return nil, apiErr
	}

	session, goErr := client.GetSession(sessionId)
	if goErr != nil {
		log.Errorf(c, "[bank_integration.GetConnectionTransactionsHandler] GetSession failed: %s", goErr.Error())
		return nil, errs.Or(goErr, errs.ErrOperationFailed)
	}

	// Collect account UIDs: prefer accounts_data, fallback to accounts (array of strings)
	accountUIDs := make([]string, 0, len(session.AccountsData)+len(session.Accounts))
	for _, acc := range session.AccountsData {
		if acc.UID != "" {
			accountUIDs = append(accountUIDs, acc.UID)
		}
	}
	if len(accountUIDs) == 0 {
		accountUIDs = append(accountUIDs, session.Accounts...)
	}
	log.Infof(c, "[bank_integration.GetConnectionTransactionsHandler] session %s: got %d account(s)", sessionId, len(accountUIDs))
	if len(accountUIDs) == 0 {
		return &models.BankConnectionTransactionsResponse{Transactions: []*models.BankConnectionTransactionItem{}}, nil
	}

	now := time.Now().UTC()
	dateTo := now.Format("2006-01-02")
	dateFrom := now.AddDate(0, 0, -bankConnectionTransactionsDays).Format("2006-01-02")

	type txWithDate struct {
		tx   *enablebanking.BankTransaction
		date time.Time
	}
	var all []txWithDate

	for _, accountUID := range accountUIDs {
		hal, goErr := client.GetAccountTransactions(accountUID, dateFrom, dateTo, "")
		if goErr != nil {
			log.Warnf(c, "[bank_integration.GetConnectionTransactionsHandler] GetAccountTransactions for account %s failed: %s", accountUID, goErr.Error())
			continue
		}
		// Some ASPSPs (e.g. Bank Norwegian) only return transactions when using strategy=longest
		if len(hal.Transactions) == 0 {
			halLongest, errLongest := client.GetAccountTransactions(accountUID, dateFrom, dateTo, "longest")
			if errLongest == nil && len(halLongest.Transactions) > 0 {
				hal = halLongest
				log.Infof(c, "[bank_integration.GetConnectionTransactionsHandler] account %s: got %d transaction(s) using strategy=longest", accountUID, len(hal.Transactions))
			}
		}
		log.Infof(c, "[bank_integration.GetConnectionTransactionsHandler] account %s: got %d transaction(s) from API", accountUID, len(hal.Transactions))
		for i := range hal.Transactions {
			tx := &hal.Transactions[i]
			var t time.Time
			if tx.TransactionDate != "" {
				t, _ = time.Parse("2006-01-02", tx.TransactionDate)
			} else if tx.BookingDate != "" {
				t, _ = time.Parse("2006-01-02", tx.BookingDate)
			}
			all = append(all, txWithDate{tx: tx, date: t})
		}
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].date.After(all[j].date)
	})

	if len(all) > bankConnectionTransactionsLimit {
		all = all[:bankConnectionTransactionsLimit]
	}

	result := make([]*models.BankConnectionTransactionItem, 0, len(all))
	for _, item := range all {
		tx := item.tx
		amount := "0"
		currency := ""
		if tx.TransactionAmount != nil {
			amount = tx.TransactionAmount.Amount
			currency = tx.TransactionAmount.Currency
		}
		if tx.CreditDebitIndicator == "DBIT" && amount != "0" && !strings.HasPrefix(amount, "-") {
			amount = "-" + amount
		}
		desc := ""
		if len(tx.RemittanceInformation) > 0 {
			desc = tx.RemittanceInformation[0]
		}
		if desc == "" && tx.BankTransactionCode != nil {
			desc = tx.BankTransactionCode.Description
		}
		counterparty := ""
		if tx.CreditDebitIndicator == "DBIT" && tx.Creditor != nil {
			counterparty = tx.Creditor.Name
		} else if tx.Debtor != nil {
			counterparty = tx.Debtor.Name
		}
		result = append(result, &models.BankConnectionTransactionItem{
			TransactionID:    tx.TransactionID,
			Date:             tx.TransactionDate,
			Amount:           amount,
			Currency:         currency,
			CreditDebit:      tx.CreditDebitIndicator,
			Description:      desc,
			CounterpartyName: counterparty,
		})
	}

	return &models.BankConnectionTransactionsResponse{Transactions: result}, nil
}
