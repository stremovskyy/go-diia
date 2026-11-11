package faults

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

var (
	ErrValidation          = errors.New("diia: validation failed")
	ErrTransport           = errors.New("diia: transport failed")
	ErrEncode              = errors.New("diia: encode failed")
	ErrDecode              = errors.New("diia: decode failed")
	ErrUnauthorized        = errors.New("diia: unauthorized")
	ErrBadRequest          = errors.New("diia: bad request")
	ErrNotFound            = errors.New("diia: not found")
	ErrUnprocessableEntity = errors.New("diia: unprocessable entity")
	ErrServer              = errors.New("diia: server error")
	ErrCallback            = errors.New("diia: callback failed")
	ErrCrypto              = errors.New("diia: crypto failed")
)

// ValidationError describes an invalid SDK request before any network call.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field == "" {
		return fmt.Sprintf("%s: %s", ErrValidation, e.Message)
	}

	return fmt.Sprintf("%s: %s: %s", ErrValidation, e.Field, e.Message)
}

func (e *ValidationError) Unwrap() error { return ErrValidation }

// APIError is returned for non-2xx Diia API responses.
type APIError struct {
	Name       string          `json:"name,omitempty"`
	Message    string          `json:"message,omitempty"`
	Code       int             `json:"code,omitempty"`
	Type       string          `json:"type,omitempty"`
	Data       json.RawMessage `json:"data,omitempty"`
	HTTPStatus int             `json:"-"`
	RawBody    []byte          `json:"-"`
}

func (e *APIError) Error() string {
	if e == nil || e.HTTPStatus < 100 || e.HTTPStatus > 599 {
		return "diia: API request failed"
	}

	return fmt.Sprintf("diia: API request failed (HTTP %d)", e.HTTPStatus)
}

func (e *APIError) Unwrap() error {
	switch e.HTTPStatus {
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusBadRequest:
		return ErrBadRequest
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusUnprocessableEntity:
		return ErrUnprocessableEntity

	default:
		if e.HTTPStatus >= 500 {
			return ErrServer
		}

		return nil
	}
}

func NewValidationError(field, message string) error {
	return &ValidationError{Field: field, Message: message}
}
