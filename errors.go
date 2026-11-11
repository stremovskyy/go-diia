package go_diia

import "github.com/stremovskyy/go-diia/internal/faults"

var (
	ErrValidation          = faults.ErrValidation
	ErrTransport           = faults.ErrTransport
	ErrEncode              = faults.ErrEncode
	ErrDecode              = faults.ErrDecode
	ErrUnauthorized        = faults.ErrUnauthorized
	ErrBadRequest          = faults.ErrBadRequest
	ErrNotFound            = faults.ErrNotFound
	ErrUnprocessableEntity = faults.ErrUnprocessableEntity
	ErrServer              = faults.ErrServer
	ErrCallback            = faults.ErrCallback
	ErrCrypto              = faults.ErrCrypto
)

type (
	ValidationError = faults.ValidationError
	APIError        = faults.APIError
)

func newValidationError(field, message string) error {
	return faults.NewValidationError(field, message)
}
