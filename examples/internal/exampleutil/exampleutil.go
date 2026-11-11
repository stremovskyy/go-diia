// Package exampleutil contains the shared setup for executable Diia examples.
package exampleutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	diia "github.com/stremovskyy/go-diia"
)

const requestTimeout = 20 * time.Second

var envNamePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// Config holds the environment-specific client used by an example.
type Config struct {
	Client      diia.Diia
	Environment string
}

type inputError struct{ message string }

func (e *inputError) Error() string { return e.message }

// Load selects sandbox by default and reads credentials from the environment.
func Load() (Config, error) {
	environment := strings.ToLower(strings.TrimSpace(os.Getenv("DIIA_ENV")))

	if environment == "" {
		environment = "sandbox"
	}

	if environment != "sandbox" && environment != "production" {
		return Config{}, &inputError{message: "DIIA_ENV must be sandbox or production"}
	}

	acquirerToken, err := RequiredEnv("DIIA_ACQUIRER_TOKEN")
	if err != nil {
		return Config{}, err
	}

	config := Config{Environment: environment}
	if environment == "sandbox" {
		authAcquirerToken, err := RequiredEnv("DIIA_AUTH_ACQUIRER_TOKEN")
		if err != nil {
			return Config{}, err
		}

		config.Client = diia.NewSandboxClient(acquirerToken, authAcquirerToken, diia.WithTimeout(requestTimeout))
	} else {
		config.Client = diia.NewClient(acquirerToken, diia.WithTimeout(requestTimeout))
	}

	return config, nil
}

// AllowMutation requires an explicit opt-in when an example targets production.
func (c Config) AllowMutation() error {
	if c.Environment == "production" && os.Getenv("DIIA_ALLOW_PRODUCTION_MUTATIONS") != "1" {
		return &inputError{message: "DIIA_ALLOW_PRODUCTION_MUTATIONS=1 is required for production changes"}
	}

	return nil
}

// RequiredEnv reads a nonempty value and names only the missing variable in errors.
func RequiredEnv(name string) (string, error) {
	if !envNamePattern.MatchString(name) {
		return "", &inputError{message: "invalid environment variable name"}
	}

	value := strings.TrimSpace(os.Getenv(name))

	if value == "" || (strings.HasPrefix(value, "__SET_") && strings.HasSuffix(value, "__")) {
		return "", &inputError{message: name + " is required"}
	}

	return value, nil
}

// RequestID generates a fresh ID without relying on a clock.
func RequestID() (string, error) {
	var random [16]byte

	if _, err := rand.Read(random[:]); err != nil {
		return "", &inputError{message: "could not generate a request ID"}
	}

	return "example-" + hex.EncodeToString(random[:]), nil
}

// SafeError reports an error category without returning upstream text or URLs.
func SafeError(err error) string {
	if err == nil {
		return ""
	}

	var input *inputError

	if errors.As(err, &input) {
		return input.message
	}

	var apiErr *diia.APIError

	if errors.As(err, &apiErr) {
		if apiErr.HTTPStatus >= 100 && apiErr.HTTPStatus <= 599 {
			return fmt.Sprintf("Diia API request failed (HTTP %d)", apiErr.HTTPStatus)
		}

		return "Diia API request failed"
	}

	var validationErr *diia.ValidationError
	if errors.As(err, &validationErr) {
		switch validationErr.Field {
		case "branchId", "offerId", "requestId", "returnLink":
			return "request validation failed: " + validationErr.Field
		}
	}

	switch {
	case errors.Is(err, context.Canceled):
		return "request canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "request timed out"
	case errors.Is(err, diia.ErrValidation):
		return "request validation failed"
	case errors.Is(err, diia.ErrUnauthorized):
		return "Diia authorization failed"
	case errors.Is(err, diia.ErrTransport):
		return "Diia transport failed"
	case errors.Is(err, diia.ErrEncode):
		return "request encoding failed"
	case errors.Is(err, diia.ErrDecode):
		return "Diia response decoding failed"
	case errors.Is(err, diia.ErrCallback):
		return "callback processing failed"
	case errors.Is(err, diia.ErrCrypto):
		return "cryptographic processing failed"
	default:
		return "operation failed"
	}
}
