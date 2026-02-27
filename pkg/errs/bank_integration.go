package errs

import "net/http"

// Error codes related to bank integration (Enable Banking)
var (
	ErrBankIntegrationDisabled     = NewNormalError(NormalSubcategoryBankIntegration, 0, http.StatusBadRequest, "bank integration is disabled")
	ErrBankIntegrationNotConfigured = NewNormalError(NormalSubcategoryBankIntegration, 1, http.StatusBadRequest, "bank integration is not configured")
	ErrBankAuthStateInvalid        = NewNormalError(NormalSubcategoryBankIntegration, 2, http.StatusBadRequest, "authorization state is invalid or expired")
	ErrBankConnectionNotFound      = NewNormalError(NormalSubcategoryBankIntegration, 3, http.StatusNotFound, "bank connection not found")
)
