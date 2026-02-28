package services

import (
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// UserBankTransactionActionService represents user bank transaction action service
type UserBankTransactionActionService struct {
	ServiceUsingDB
}

// Initialize a user bank transaction action service singleton instance
var (
	UserBankTransactionActions = &UserBankTransactionActionService{
		ServiceUsingDB: ServiceUsingDB{
			container: datastore.Container,
		},
	}
)

// RecordAction records an action (accepted or dismissed) for a bank transaction
func (s *UserBankTransactionActionService) RecordAction(c core.Context, uid int64, sessionId, bankTxId, status string, createdTransactionId int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}
	action := &models.UserBankTransactionAction{
		Uid:                   uid,
		ConnectionSessionId:   sessionId,
		BankTransactionId:     bankTxId,
		Status:                status,
		CreatedTransactionId:  createdTransactionId,
		CreatedUnixTime:       time.Now().Unix(),
	}
	_, err := s.UserDataDB(uid).NewSession(c).Insert(action)
	return err
}

// HasAction returns true if the user has already accepted or dismissed this bank transaction
func (s *UserBankTransactionActionService) HasAction(c core.Context, uid int64, sessionId, bankTxId string) (bool, error) {
	if uid <= 0 {
		return false, errs.ErrUserIdInvalid
	}
	return s.UserDataDB(uid).NewSession(c).Where("uid=? AND connection_session_id=? AND bank_transaction_id=?", uid, sessionId, bankTxId).Exist(&models.UserBankTransactionAction{})
}

// GetActionKeysByUid returns a set of (sessionId, bankTransactionId) that the user has already acted on
func (s *UserBankTransactionActionService) GetActionKeysByUid(c core.Context, uid int64) (map[string]bool, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}
	var list []*models.UserBankTransactionAction
	err := s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).Find(&list)
	if err != nil {
		return nil, err
	}
	keys := make(map[string]bool, len(list))
	for _, a := range list {
		keys[a.ConnectionSessionId+"\x00"+a.BankTransactionId] = true
	}
	return keys, nil
}
