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

// MCPModifyTransactionRequest represents all parameters of the modify transaction request.
// Only the fields that are provided (non-null) are changed; the transaction type cannot be changed.
type MCPModifyTransactionRequest struct {
	TransactionId          string  `json:"transaction_id" jsonschema_description:"The id of the transaction to modify (as returned by query_transactions)"`
	Time                   *string `json:"time,omitempty" jsonschema:"format=date-time" jsonschema_description:"New transaction time in RFC 3339 format (optional, leave unset to keep unchanged)"`
	SecondaryCategoryName  *string `json:"category_name,omitempty" jsonschema_description:"New secondary category name (optional, must match the transaction's existing type, leave unset to keep unchanged)"`
	AccountName            *string `json:"account_name,omitempty" jsonschema_description:"New account name (optional, leave unset to keep unchanged)"`
	Amount                 *string `json:"amount,omitempty" jsonschema_description:"New transaction amount (optional, leave unset to keep unchanged)"`
	DestinationAccountName *string `json:"destination_account_name,omitempty" jsonschema_description:"New destination account name for transfer transactions (optional)"`
	DestinationAmount      *string `json:"destination_amount,omitempty" jsonschema_description:"New destination amount for transfer transactions (optional)"`
	Comment                *string `json:"comment,omitempty" jsonschema_description:"New transaction description (optional, leave unset to keep unchanged)"`
	DryRun                 bool    `json:"dry_run,omitempty" jsonschema_description:"If true, the change will only be validated, not saved (optional)"`
}

// MCPModifyTransactionResponse represents the response structure for modify transaction
type MCPModifyTransactionResponse struct {
	Success        bool   `json:"success" jsonschema_description:"Indicates whether this operation is successful"`
	DryRun         bool   `json:"dry_run,omitempty" jsonschema_description:"Indicates whether this operation is a dry run (transaction not modified actually)"`
	AccountBalance string `json:"account_balance,omitempty" jsonschema_description:"Source account balance (or outstanding balance for debt accounts) after the modification"`
}

type mcpModifyTransactionToolHandler struct{}

// MCPModifyTransactionToolHandler is the MCP tool handler for modifying a transaction
var MCPModifyTransactionToolHandler = &mcpModifyTransactionToolHandler{}

// Name returns the name of the MCP tool
func (h *mcpModifyTransactionToolHandler) Name() string {
	return "modify_transaction"
}

// Description returns the description of the MCP tool
func (h *mcpModifyTransactionToolHandler) Description() string {
	return "Modify an existing transaction in ezBookkeeping by its id. Only the provided fields are changed; the transaction type cannot be changed. Tags and pictures are left untouched."
}

// InputType returns the input type for the MCP tool request
func (h *mcpModifyTransactionToolHandler) InputType() reflect.Type {
	return reflect.TypeOf(&MCPModifyTransactionRequest{})
}

// OutputType returns the output type for the MCP tool response
func (h *mcpModifyTransactionToolHandler) OutputType() reflect.Type {
	return reflect.TypeOf(&MCPModifyTransactionResponse{})
}

// Handle processes the MCP call tool request and returns the response
func (h *mcpModifyTransactionToolHandler) Handle(c *core.WebContext, callToolReq *MCPCallToolRequest, user *models.User, currentConfig *settings.Config, services MCPAvailableServices) (any, []*MCPTextContent, error) {
	var modifyTransactionRequest MCPModifyTransactionRequest

	if callToolReq.Arguments != nil {
		if err := json.Unmarshal(callToolReq.Arguments, &modifyTransactionRequest); err != nil {
			return nil, nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
		}
	} else {
		return nil, nil, errs.ErrIncompleteOrIncorrectSubmission
	}

	transactionId, err := utils.StringToInt64(modifyTransactionRequest.TransactionId)

	if err != nil {
		return nil, nil, errs.ErrTransactionIdInvalid
	}

	uid := user.Uid
	transaction, err := services.GetTransactionService().GetTransactionByTransactionId(c, uid, transactionId)

	if err != nil {
		log.Errorf(c, "[modify_transaction.Handle] failed to get transaction \"id:%d\" for user \"uid:%d\", because %s", transactionId, uid, err.Error())
		return nil, nil, err
	}

	if transaction.Type == models.TRANSACTION_DB_TYPE_TRANSFER_IN {
		log.Warnf(c, "[modify_transaction.Handle] cannot modify transaction \"id:%d\" for user \"uid:%d\", because transaction type is transfer in", transactionId, uid)
		return nil, nil, errs.ErrTransactionTypeInvalid
	}

	if transaction.Type == models.TRANSACTION_DB_TYPE_MODIFY_BALANCE {
		log.Warnf(c, "[modify_transaction.Handle] cannot modify balance-modification transaction \"id:%d\" for user \"uid:%d\"", transactionId, uid)
		return nil, nil, errs.ErrTransactionTypeInvalid
	}

	allAccounts, err := services.GetAccountService().GetAllAccountsByUid(c, uid)

	if err != nil {
		log.Warnf(c, "[modify_transaction.Handle] get account error, because %s", err.Error())
		return nil, nil, err
	}

	accountsByName := services.GetAccountService().GetVisibleAccountNameMapByList(allAccounts)
	accountsById := services.GetAccountService().GetAccountMapByList(allAccounts)

	// Start from the existing transaction so unspecified fields stay unchanged
	newTransaction := *transaction
	newTransaction.Uid = uid

	if modifyTransactionRequest.Time != nil {
		transactionTime, err := utils.ParseFromLongDateTimeWithTimezoneRFC3339Format(*modifyTransactionRequest.Time)

		if err != nil {
			return nil, nil, errs.ErrIncompleteOrIncorrectSubmission
		}

		newTransaction.TransactionTime = utils.GetMinTransactionTimeFromUnixTime(transactionTime.Unix())
		newTransaction.TimezoneUtcOffset = utils.GetTimezoneOffsetMinutes(transactionTime.Unix(), transactionTime.Location())
	}

	if modifyTransactionRequest.Amount != nil {
		amount, err := utils.ParseAmount(*modifyTransactionRequest.Amount)

		if err != nil {
			return nil, nil, errs.ErrIncompleteOrIncorrectSubmission
		}

		newTransaction.Amount = amount
	}

	if modifyTransactionRequest.Comment != nil {
		newTransaction.Comment = *modifyTransactionRequest.Comment
	}

	if modifyTransactionRequest.AccountName != nil {
		account, exists := accountsByName[*modifyTransactionRequest.AccountName]

		if !exists {
			log.Warnf(c, "[modify_transaction.Handle] source account \"%s\" not found for user \"uid:%d\"", *modifyTransactionRequest.AccountName, uid)
			return nil, nil, errs.ErrSourceAccountNotFound
		}

		newTransaction.AccountId = account.AccountId
	}

	if modifyTransactionRequest.SecondaryCategoryName != nil {
		category := h.findSecondaryCategoryByName(services, c, uid, *modifyTransactionRequest.SecondaryCategoryName, transaction.Type)

		if category == nil {
			log.Warnf(c, "[modify_transaction.Handle] secondary category \"%s\" not found for user \"uid:%d\"", *modifyTransactionRequest.SecondaryCategoryName, uid)
			return nil, nil, errs.ErrTransactionCategoryNotFound
		}

		newTransaction.CategoryId = category.CategoryId
	}

	if transaction.Type == models.TRANSACTION_DB_TYPE_TRANSFER_OUT {
		if modifyTransactionRequest.DestinationAccountName != nil {
			account, exists := accountsByName[*modifyTransactionRequest.DestinationAccountName]

			if !exists {
				log.Warnf(c, "[modify_transaction.Handle] destination account \"%s\" not found for user \"uid:%d\"", *modifyTransactionRequest.DestinationAccountName, uid)
				return nil, nil, errs.ErrDestinationAccountNotFound
			}

			newTransaction.RelatedAccountId = account.AccountId
		}

		if modifyTransactionRequest.DestinationAmount != nil {
			destinationAmount, err := utils.ParseAmount(*modifyTransactionRequest.DestinationAmount)

			if err != nil {
				return nil, nil, errs.ErrIncompleteOrIncorrectSubmission
			}

			newTransaction.RelatedAccountAmount = destinationAmount
		}
	}

	// Editable check on both the old and the new transaction time, using each transaction's own timezone
	oldTimeZone := time.FixedZone("Transaction Timezone", int(transaction.TimezoneUtcOffset)*60)
	newTimeZone := time.FixedZone("Transaction Timezone", int(newTransaction.TimezoneUtcOffset)*60)
	oldEditable := user.CanEditTransactionByTransactionTime(transaction.TransactionTime, oldTimeZone, accountsById[transaction.AccountId], accountsById[transaction.RelatedAccountId])
	newEditable := user.CanEditTransactionByTransactionTime(newTransaction.TransactionTime, newTimeZone, accountsById[newTransaction.AccountId], accountsById[newTransaction.RelatedAccountId])

	if !oldEditable || !newEditable {
		return nil, nil, errs.ErrCannotModifyTransactionWithThisTransactionTime
	}

	if modifyTransactionRequest.DryRun {
		return h.createNewMCPModifyTransactionResponse(true, "")
	}

	allTransactionTagIds, err := services.GetTransactionTagService().GetAllTagIdsOfTransactions(c, uid, []int64{transaction.TransactionId})

	if err != nil {
		log.Errorf(c, "[modify_transaction.Handle] failed to get transaction tag ids for user \"uid:%d\", because %s", uid, err.Error())
		return nil, nil, err
	}

	currentTagIds := allTransactionTagIds[transaction.TransactionId]
	currentTagIdsCount := len(currentTagIds)

	err = services.GetTransactionService().ModifyTransaction(c, &newTransaction, currentTagIdsCount, nil, nil, nil, nil)

	if err != nil {
		log.Errorf(c, "[modify_transaction.Handle] failed to modify transaction \"id:%d\" for user \"uid:%d\", because %s", transactionId, uid, err.Error())
		return nil, nil, err
	}

	log.Infof(c, "[modify_transaction.Handle] user \"uid:%d\" has modified transaction \"id:%d\" successfully", uid, transactionId)

	accountBalance := ""
	newAccounts, err := services.GetAccountService().GetAccountsByAccountIds(c, uid, []int64{newTransaction.AccountId})

	if err != nil {
		log.Warnf(c, "[modify_transaction.Handle] failed to get latest account info after modify, because %s", err.Error())
	} else if account, exists := newAccounts[newTransaction.AccountId]; exists && account != nil {
		accountInfo := account.ToAccountInfoResponse()

		if accountInfo.IsAsset {
			accountBalance = utils.FormatAmount(accountInfo.Balance)
		} else if accountInfo.IsLiability {
			accountBalance = utils.FormatAmount(-accountInfo.Balance)
		}
	}

	return h.createNewMCPModifyTransactionResponse(false, accountBalance)
}

func (h *mcpModifyTransactionToolHandler) findSecondaryCategoryByName(services MCPAvailableServices, c *core.WebContext, uid int64, categoryName string, transactionDbType models.TransactionDbType) *models.TransactionCategory {
	allCategories, err := services.GetTransactionCategoryService().GetAllCategoriesByUid(c, uid, 0, -1)

	if err != nil {
		log.Warnf(c, "[modify_transaction.findSecondaryCategoryByName] get transaction category error, because %s", err.Error())
		return nil
	}

	for i := 0; i < len(allCategories); i++ {
		category := allCategories[i]

		if category.Hidden || category.ParentCategoryId == models.LevelOneTransactionCategoryParentId {
			continue
		}

		if category.Name != categoryName {
			continue
		}

		if category.Type == models.CATEGORY_TYPE_INCOME && transactionDbType == models.TRANSACTION_DB_TYPE_INCOME {
			return category
		} else if category.Type == models.CATEGORY_TYPE_EXPENSE && transactionDbType == models.TRANSACTION_DB_TYPE_EXPENSE {
			return category
		} else if category.Type == models.CATEGORY_TYPE_TRANSFER && transactionDbType == models.TRANSACTION_DB_TYPE_TRANSFER_OUT {
			return category
		}
	}

	return nil
}

func (h *mcpModifyTransactionToolHandler) createNewMCPModifyTransactionResponse(dryRun bool, accountBalance string) (any, []*MCPTextContent, error) {
	response := MCPModifyTransactionResponse{
		Success:        true,
		DryRun:         dryRun,
		AccountBalance: accountBalance,
	}

	content, err := json.Marshal(response)

	if err != nil {
		return nil, nil, err
	}

	return response, []*MCPTextContent{
		NewMCPTextContent(string(content)),
	}, nil
}
