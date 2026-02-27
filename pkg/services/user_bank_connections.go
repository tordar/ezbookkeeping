package services

import (
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// UserBankConnectionService represents user bank connection service
type UserBankConnectionService struct {
	ServiceUsingDB
}

// Initialize a user bank connection service singleton instance
var (
	UserBankConnections = &UserBankConnectionService{
		ServiceUsingDB: ServiceUsingDB{
			container: datastore.Container,
		},
	}
)

// GetConnectionsByUid returns all bank connections for a user
func (s *UserBankConnectionService) GetConnectionsByUid(c core.Context, uid int64) ([]*models.UserBankConnection, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}
	var list []*models.UserBankConnection
	err := s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).Asc("created_unix_time").Find(&list)
	return list, err
}

// GetConnectionBySessionId returns a connection by session ID and uid
func (s *UserBankConnectionService) GetConnectionBySessionId(c core.Context, uid int64, sessionId string) (*models.UserBankConnection, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}
	conn := &models.UserBankConnection{}
	has, err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND session_id=?", uid, sessionId).Get(conn)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, errs.ErrBankConnectionNotFound
	}
	return conn, nil
}

// CreateConnection saves a new bank connection for the user
func (s *UserBankConnectionService) CreateConnection(c core.Context, conn *models.UserBankConnection) error {
	if conn.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}
	conn.CreatedUnixTime = time.Now().Unix()
	_, err := s.UserDataDB(conn.Uid).NewSession(c).Insert(conn)
	return err
}

// DeleteConnection removes a bank connection by session ID for the user
func (s *UserBankConnectionService) DeleteConnection(c core.Context, uid int64, sessionId string) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}
	n, err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND session_id=?", uid, sessionId).Delete(&models.UserBankConnection{})
	if err != nil {
		return err
	}
	if n == 0 {
		return errs.ErrBankConnectionNotFound
	}
	return nil
}
