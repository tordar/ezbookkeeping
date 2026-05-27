package mcp

import (
	"encoding/json"
	"reflect"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// MCPDeleteTransactionRequest represents all parameters of the delete transaction request
type MCPDeleteTransactionRequest struct {
	TransactionId string `json:"transaction_id" jsonschema_description:"The id of the transaction to delete (as returned by query_transactions)"`
	Confirm       bool   `json:"confirm" jsonschema_description:"Must be set to true to actually delete the transaction. Safeguard against accidental deletion."`
}

// MCPDeleteTransactionResponse represents the response structure for delete transaction
type MCPDeleteTransactionResponse struct {
	Success bool `json:"success" jsonschema_description:"Indicates whether this operation is successful"`
}

type mcpDeleteTransactionToolHandler struct{}

// MCPDeleteTransactionToolHandler is the MCP tool handler for deleting a transaction
var MCPDeleteTransactionToolHandler = &mcpDeleteTransactionToolHandler{}

// Name returns the name of the MCP tool
func (h *mcpDeleteTransactionToolHandler) Name() string {
	return "delete_transaction"
}

// Description returns the description of the MCP tool
func (h *mcpDeleteTransactionToolHandler) Description() string {
	return "Delete an existing transaction in ezBookkeeping by its id. Requires confirm=true. This operation is irreversible."
}

// InputType returns the input type for the MCP tool request
func (h *mcpDeleteTransactionToolHandler) InputType() reflect.Type {
	return reflect.TypeOf(&MCPDeleteTransactionRequest{})
}

// OutputType returns the output type for the MCP tool response
func (h *mcpDeleteTransactionToolHandler) OutputType() reflect.Type {
	return reflect.TypeOf(&MCPDeleteTransactionResponse{})
}

// Handle processes the MCP call tool request and returns the response
func (h *mcpDeleteTransactionToolHandler) Handle(c *core.WebContext, callToolReq *MCPCallToolRequest, user *models.User, currentConfig *settings.Config, services MCPAvailableServices) (any, []*MCPTextContent, error) {
	var deleteTransactionRequest MCPDeleteTransactionRequest

	if callToolReq.Arguments != nil {
		if err := json.Unmarshal(callToolReq.Arguments, &deleteTransactionRequest); err != nil {
			return nil, nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
		}
	} else {
		return nil, nil, errs.ErrIncompleteOrIncorrectSubmission
	}

	if !deleteTransactionRequest.Confirm {
		return nil, nil, errs.ErrIncompleteOrIncorrectSubmission
	}

	transactionId, err := utils.StringToInt64(deleteTransactionRequest.TransactionId)

	if err != nil {
		return nil, nil, errs.ErrTransactionIdInvalid
	}

	uid := user.Uid
	transaction, err := services.GetTransactionService().GetTransactionByTransactionId(c, uid, transactionId)

	if err != nil {
		log.Errorf(c, "[delete_transaction.Handle] failed to get transaction \"id:%d\" for user \"uid:%d\", because %s", transactionId, uid, err.Error())
		return nil, nil, err
	}

	if transaction.Type == models.TRANSACTION_DB_TYPE_TRANSFER_IN {
		log.Warnf(c, "[delete_transaction.Handle] cannot delete transaction \"id:%d\" for user \"uid:%d\", because transaction type is transfer in", transactionId, uid)
		return nil, nil, errs.ErrTransactionTypeInvalid
	}

	allAccounts, err := services.GetAccountService().GetAllAccountsByUid(c, uid)

	if err != nil {
		log.Warnf(c, "[delete_transaction.Handle] get account error, because %s", err.Error())
		return nil, nil, err
	}

	accountMap := services.GetAccountService().GetAccountMapByList(allAccounts)
	transactionTimeZone := time.FixedZone("Transaction Timezone", int(transaction.TimezoneUtcOffset)*60)
	transactionEditable := user.CanEditTransactionByTransactionTime(transaction.TransactionTime, transactionTimeZone, accountMap[transaction.AccountId], accountMap[transaction.RelatedAccountId])

	if !transactionEditable {
		return nil, nil, errs.ErrCannotDeleteTransactionWithThisTransactionTime
	}

	err = services.GetTransactionService().DeleteTransaction(c, uid, transactionId)

	if err != nil {
		log.Errorf(c, "[delete_transaction.Handle] failed to delete transaction \"id:%d\" for user \"uid:%d\", because %s", transactionId, uid, err.Error())
		return nil, nil, err
	}

	log.Infof(c, "[delete_transaction.Handle] user \"uid:%d\" has deleted transaction \"id:%d\" successfully", uid, transactionId)

	response := MCPDeleteTransactionResponse{
		Success: true,
	}

	content, err := json.Marshal(response)

	if err != nil {
		return nil, nil, err
	}

	return response, []*MCPTextContent{
		NewMCPTextContent(string(content)),
	}, nil
}
