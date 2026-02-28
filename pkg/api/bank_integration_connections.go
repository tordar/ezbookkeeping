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
	connections  *services.UserBankConnectionService
	actions      *services.UserBankTransactionActionService
	transactions *services.TransactionService
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
		connections:  services.UserBankConnections,
		actions:      services.UserBankTransactionActions,
		transactions: services.Transactions,
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

const reauthRemarkPrefix = "reauth:"

// StartReauthHandler starts re-authorization for an existing connection (new session token)
func (a *BankIntegrationConnectionsApi) StartReauthHandler(c *core.WebContext) (any, *errs.Error) {
	cfg := a.CurrentConfig()
	if !cfg.EnableBankIntegration {
		return nil, errs.ErrBankIntegrationDisabled
	}

	var req models.DisconnectBankRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf(c, "[bank_integration.StartReauthHandler] parse request failed: %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	conn, err := a.connections.GetConnectionBySessionId(c, uid, req.SessionId)
	if err != nil {
		return nil, errs.Or(err, errs.ErrBankConnectionNotFound)
	}

	client, apiErr := a.getClient()
	if apiErr != nil {
		return nil, apiErr
	}

	state, goErr := utils.GetRandomNumberOrLowercaseLetter(32)
	if goErr != nil {
		return nil, errs.Or(goErr, errs.ErrSystemError)
	}

	redirectURL := cfg.EnableBankingCallbackURL
	if redirectURL == "" {
		rootURL := strings.TrimSuffix(cfg.RootUrl, "/")
		redirectURL = rootURL + "/api/bank_integration/callback"
	}

	remark := reauthRemarkPrefix + strconv.FormatInt(uid, 10) + ":" + conn.SessionId
	a.SetSubmissionRemarkWithCustomExpiration(
		duplicatechecker.DUPLICATE_CHECKER_TYPE_BANK_AUTH_STATE,
		0,
		state,
		remark,
		bankAuthStateExpiration,
	)

	result, goErr := client.StartAuth(conn.AspspName, conn.AspspCountry, state, redirectURL)
	if goErr != nil {
		log.Errorf(c, "[bank_integration.StartReauthHandler] Enable Banking StartAuth failed: %s", goErr.Error())
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

	var uid int64
	var oldSessionId string
	isReauth := strings.HasPrefix(remark, reauthRemarkPrefix)
	if isReauth {
		parts := strings.SplitN(remark, ":", 3)
		if len(parts) != 3 {
			return a.redirectToSettings(c, "error", "Invalid reauth state")
		}
		var parseErr error
		uid, parseErr = strconv.ParseInt(parts[1], 10, 64)
		if parseErr != nil || uid <= 0 {
			return a.redirectToSettings(c, "error", "Invalid reauth state")
		}
		oldSessionId = parts[2]
	} else {
		var err error
		uid, err = strconv.ParseInt(remark, 10, 64)
		if err != nil || uid <= 0 {
			return a.redirectToSettings(c, "error", "Invalid state")
		}
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

	if isReauth {
		if err := a.connections.UpdateConnectionSession(c, uid, oldSessionId, session.SessionID, session.Aspsp.Name, session.Aspsp.Country, session.Access.ValidUntil); err != nil {
			log.Errorf(c, "[bank_integration.CallbackHandler] UpdateConnectionSession failed: %s", err.Error())
			return a.redirectToSettings(c, "error", "Failed to update connection")
		}
	} else {
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
	}

	return a.redirectToSettings(c, "success", "")
}

func (a *BankIntegrationConnectionsApi) redirectToSettings(c *core.WebContext, status, message string) (string, *errs.Error) {
	cfg := a.CurrentConfig()
	base := strings.TrimSuffix(cfg.RootUrl, "/")
	redirectURL := fmt.Sprintf("%s/desktop#/user/settings?tab=bankIntegrationSetting&bank=%s", base, status)
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
const bankNewTransactionsHours = 48

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

// fetchConnectionTransactions48h returns all transactions in the last 48h for one connection (for new-transactions list)
func (a *BankIntegrationConnectionsApi) fetchConnectionTransactions48h(c *core.WebContext, client *enablebanking.Client, conn *models.UserBankConnection) []*models.NewBankTransactionItem {
	session, goErr := client.GetSession(conn.SessionId)
	if goErr != nil {
		log.Warnf(c, "[bank_integration.fetchConnectionTransactions48h] GetSession %s failed: %s", conn.SessionId, goErr.Error())
		return nil
	}
	accountUIDs := make([]string, 0, len(session.AccountsData)+len(session.Accounts))
	for _, acc := range session.AccountsData {
		if acc.UID != "" {
			accountUIDs = append(accountUIDs, acc.UID)
		}
	}
	if len(accountUIDs) == 0 {
		accountUIDs = append(accountUIDs, session.Accounts...)
	}
	// DNB returns multiple accounts; only fetch from the first (Brukskonto)
	if conn.AspspName == "DNB" && len(accountUIDs) > 1 {
		accountUIDs = accountUIDs[:1]
	}
	now := time.Now().UTC()
	dateTo := now.Format("2006-01-02")
	dateFrom := now.Add(-bankNewTransactionsHours * time.Hour).Format("2006-01-02")
	seenKeys := make(map[string]bool)
	appendTx := func(accountUID string, tx *enablebanking.BankTransaction, list *[]*models.NewBankTransactionItem) {
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
		// Skip dated transactions outside the 48h window (some ASPSPs ignore date params with strategy=longest)
		if tx.TransactionDate != "" && tx.TransactionDate < dateFrom {
			return
		}
		date := tx.TransactionDate
		if date == "" {
			date = tx.BookingDate
		}
		bankTxID := tx.TransactionID
		if bankTxID == "" && tx.EntryReference != "" && tx.EntryReference != "0" {
			bankTxID = tx.EntryReference
		}
		if bankTxID == "" {
			bankTxID = fmt.Sprintf("%s|%s|%s|%s", accountUID, date, amount, desc)
		}
		key := conn.SessionId + "\x00" + bankTxID
		if seenKeys[key] {
			return
		}
		seenKeys[key] = true
		*list = append(*list, &models.NewBankTransactionItem{
			SessionId:        conn.SessionId,
			AspspName:        conn.AspspName,
			TransactionID:    bankTxID,
			Date:             date,
			BookingDate:      tx.BookingDate,
			Amount:           amount,
			Currency:         currency,
			CreditDebit:      tx.CreditDebitIndicator,
			Description:      desc,
			CounterpartyName: counterparty,
		})
	}
	var items []*models.NewBankTransactionItem
	for _, accountUID := range accountUIDs {
		hal, goErr := client.GetAccountTransactions(accountUID, dateFrom, dateTo, "")
		if goErr != nil {
			log.Warnf(c, "[bank_integration.fetchConnectionTransactions48h] GetAccountTransactions %s failed: %s", accountUID, goErr.Error())
			continue
		}
		if len(hal.Transactions) == 0 {
			hal, goErr = client.GetAccountTransactions(accountUID, dateFrom, dateTo, "longest")
			if goErr != nil {
				continue
			}
		}
		for i := range hal.Transactions {
			appendTx(accountUID, &hal.Transactions[i], &items)
		}
		// Also fetch without date filter to include pending transactions (no transaction date yet)
		halNoDate, goErr := client.GetAccountTransactions(accountUID, "", "", "longest")
		if goErr == nil {
			for i := range halNoDate.Transactions {
				tx := &halNoDate.Transactions[i]
				if tx.TransactionDate == "" {
					appendTx(accountUID, tx, &items)
				}
			}
		}
	}
	return items
}

// ListNewTransactionsHandler returns pending new transactions (last 48h, not yet accepted/dismissed)
func (a *BankIntegrationConnectionsApi) ListNewTransactionsHandler(c *core.WebContext) (any, *errs.Error) {
	cfg := a.CurrentConfig()
	if !cfg.EnableBankIntegration {
		return &models.NewBankTransactionsResponse{Transactions: []*models.NewBankTransactionItem{}}, nil
	}
	uid := c.GetCurrentUid()
	conns, err := a.connections.GetConnectionsByUid(c, uid)
	if err != nil {
		log.Errorf(c, "[bank_integration.ListNewTransactionsHandler] GetConnectionsByUid failed: %s", err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}
	actionKeys, err := a.actions.GetActionKeysByUid(c, uid)
	if err != nil {
		log.Errorf(c, "[bank_integration.ListNewTransactionsHandler] GetActionKeysByUid failed: %s", err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}
	client, apiErr := a.getClient()
	if apiErr != nil {
		return nil, apiErr
	}
	var all []*models.NewBankTransactionItem
	for _, conn := range conns {
		items := a.fetchConnectionTransactions48h(c, client, conn)
		for _, item := range items {
			key := item.SessionId + "\x00" + item.TransactionID
			if !actionKeys[key] {
				all = append(all, item)
			}
		}
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].Date > all[j].Date
	})
	return &models.NewBankTransactionsResponse{Transactions: all}, nil
}

// normalizeAmountToTwoDecimals truncates amount string to at most 2 decimal places so ParseAmount accepts it.
func normalizeAmountToTwoDecimals(amount string) string {
	amount = strings.TrimSpace(amount)
	if amount == "" {
		return amount
	}
	idx := strings.Index(amount, ".")
	if idx < 0 {
		return amount
	}
	if len(amount)-idx-1 <= 2 {
		return amount
	}
	return amount[:idx+3]
}

// AcceptNewTransactionHandler creates a transaction from a bank transaction and records it as accepted
func (a *BankIntegrationConnectionsApi) AcceptNewTransactionHandler(c *core.WebContext) (any, *errs.Error) {
	cfg := a.CurrentConfig()
	if !cfg.EnableBankIntegration {
		return nil, errs.ErrBankIntegrationDisabled
	}
	var req models.AcceptNewBankTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf(c, "[bank_integration.AcceptNewTransactionHandler] parse request failed: %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}
	uid := c.GetCurrentUid()
	_, err := a.connections.GetConnectionBySessionId(c, uid, req.SessionId)
	if err != nil {
		return nil, errs.Or(err, errs.ErrBankConnectionNotFound)
	}
	has, err := a.actions.HasAction(c, uid, req.SessionId, req.BankTransactionId)
	if err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}
	if has {
		return map[string]interface{}{"accepted": true, "transactionId": int64(0)}, nil
	}
	amountStr := normalizeAmountToTwoDecimals(req.Amount)
	amount, err := utils.ParseAmount(amountStr)
	if err != nil {
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(fmt.Errorf("invalid amount: %s", req.Amount))
	}
	var txType models.TransactionDbType
	if req.CreditDebit == "CRDT" {
		txType = models.TRANSACTION_DB_TYPE_INCOME
	} else {
		txType = models.TRANSACTION_DB_TYPE_EXPENSE
		if amount > 0 {
			amount = -amount
		}
	}
	var transactionTime int64
	dateStr := req.TransactionDate
	if dateStr == "" {
		dateStr = req.BookingDate
	}
	if dateStr != "" {
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return nil, errs.NewIncompleteOrIncorrectSubmissionError(fmt.Errorf("invalid transaction date"))
		}
		transactionTime = utils.GetMinTransactionTimeFromUnixTime(t.Unix())
	} else {
		transactionTime = utils.GetMinTransactionTimeFromUnixTime(time.Now().UTC().Unix())
	}
	comment := req.Description
	if len(comment) > 255 {
		comment = comment[:255]
	}
	transaction := &models.Transaction{
		Uid:                  uid,
		Type:                 txType,
		CategoryId:           req.CategoryId,
		AccountId:            req.AccountId,
		TransactionTime:      transactionTime,
		TimezoneUtcOffset:    0,
		Amount:               amount,
		RelatedId:            0,
		RelatedAccountId:     0,
		RelatedAccountAmount: 0,
		HideAmount:           false,
		Comment:              comment,
		CreatedIp:            c.ClientIP(),
	}
	if err := a.transactions.CreateTransaction(c, transaction, nil, nil); err != nil {
		log.Errorf(c, "[bank_integration.AcceptNewTransactionHandler] CreateTransaction failed: %s", err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}
	if err := a.actions.RecordAction(c, uid, req.SessionId, req.BankTransactionId, models.UserBankTransactionActionStatusAccepted, transaction.TransactionId); err != nil {
		log.Errorf(c, "[bank_integration.AcceptNewTransactionHandler] RecordAction failed: %s", err.Error())
	}
	return map[string]interface{}{"accepted": true, "transactionId": transaction.TransactionId}, nil
}

// DismissNewTransactionHandler records a bank transaction as dismissed
func (a *BankIntegrationConnectionsApi) DismissNewTransactionHandler(c *core.WebContext) (any, *errs.Error) {
	cfg := a.CurrentConfig()
	if !cfg.EnableBankIntegration {
		return nil, errs.ErrBankIntegrationDisabled
	}
	var req models.DismissNewBankTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf(c, "[bank_integration.DismissNewTransactionHandler] parse request failed: %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}
	uid := c.GetCurrentUid()
	_, err := a.connections.GetConnectionBySessionId(c, uid, req.SessionId)
	if err != nil {
		return nil, errs.Or(err, errs.ErrBankConnectionNotFound)
	}
	has, err := a.actions.HasAction(c, uid, req.SessionId, req.BankTransactionId)
	if err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}
	if has {
		return true, nil
	}
	if err := a.actions.RecordAction(c, uid, req.SessionId, req.BankTransactionId, models.UserBankTransactionActionStatusDismissed, 0); err != nil {
		log.Errorf(c, "[bank_integration.DismissNewTransactionHandler] RecordAction failed: %s", err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}
	return true, nil
}
