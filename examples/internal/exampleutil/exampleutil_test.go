package exampleutil

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	diia "github.com/stremovskyy/go-diia"
)

func TestLoadAndProductionGate(t *testing.T) {
	t.Setenv("DIIA_ENV", "")
	t.Setenv("DIIA_ACQUIRER_TOKEN", "test-acquirer")
	t.Setenv("DIIA_AUTH_ACQUIRER_TOKEN", "test-basic")
	t.Setenv("DIIA_ALLOW_PRODUCTION_MUTATIONS", "")

	config, err := Load()

	if err != nil || config.Client == nil || config.Environment != "sandbox" {
		t.Fatalf("sandbox config: environment=%q, err=%v", config.Environment, SafeError(err))
	}

	if err := config.AllowMutation(); err != nil {
		t.Fatalf("sandbox gate: %s", SafeError(err))
	}

	t.Setenv("DIIA_AUTH_ACQUIRER_TOKEN", "")

	if _, err := Load(); SafeError(err) != "DIIA_AUTH_ACQUIRER_TOKEN is required" {
		t.Fatalf("missing sandbox credential: %s", SafeError(err))
	}

	t.Setenv("DIIA_ENV", "production")
	config, err = Load()

	if err != nil || config.Client == nil || config.Environment != "production" {
		t.Fatalf("production config: environment=%q, err=%v", config.Environment, SafeError(err))
	}

	if err := config.AllowMutation(); SafeError(
		err,
	) != "DIIA_ALLOW_PRODUCTION_MUTATIONS=1 is required for production changes" {
		t.Fatalf("production gate: %s", SafeError(err))
	}

	t.Setenv("DIIA_ALLOW_PRODUCTION_MUTATIONS", "1")

	if err := config.AllowMutation(); err != nil {
		t.Fatalf("production opt-in: %s", SafeError(err))
	}

	t.Setenv("DIIA_ENV", "other")

	if _, err := Load(); SafeError(err) != "DIIA_ENV must be sandbox or production" {
		t.Fatalf("invalid environment: %s", SafeError(err))
	}
}

func TestRequiredEnvAndRequestID(t *testing.T) {
	t.Setenv("DIIA_BRANCH_ID", " ")

	if _, err := RequiredEnv("DIIA_BRANCH_ID"); SafeError(err) != "DIIA_BRANCH_ID is required" {
		t.Fatalf("missing variable: %s", SafeError(err))
	}

	t.Setenv("DIIA_BRANCH_ID", "__SET_BRANCH_ID__")

	if _, err := RequiredEnv("DIIA_BRANCH_ID"); SafeError(err) != "DIIA_BRANCH_ID is required" {
		t.Fatalf("placeholder should be rejected: %s", SafeError(err))
	}

	t.Setenv("DIIA_BRANCH_ID", " branch-1 ")

	if got, err := RequiredEnv("DIIA_BRANCH_ID"); err != nil || got != "branch-1" {
		t.Fatalf("required variable: got=%q err=%s", got, SafeError(err))
	}

	t.Setenv("DIIA_REQUEST_ID", "request-12345")
	first, err := RequestID()
	if err != nil {
		t.Fatal(SafeError(err))
	}

	second, err := RequestID()
	if err != nil {
		t.Fatal(SafeError(err))
	}

	if first == "request-12345" || first == second || !strings.HasPrefix(first, "example-") ||
		len(first) != len("example-")+32 {
		t.Fatalf("generated IDs were invalid or repeated: lengths=%d,%d", len(first), len(second))
	}
}

func TestSafeErrorRedactsUpstreamText(t *testing.T) {
	const secret = "secret-marker"
	tests := []error{
		fmt.Errorf("%w: GET /api/v1/auth/acquirer/%s", diia.ErrTransport, secret),
		&diia.APIError{HTTPStatus: 401, Message: secret, RawBody: []byte(secret)},
		errors.New("unknown failure with " + secret),
		&diia.ValidationError{Field: "returnLink", Message: secret},
		&diia.ValidationError{Field: secret, Message: secret},
	}

	for _, err := range tests {
		message := SafeError(err)

		if strings.Contains(message, secret) || strings.Contains(message, "/api/") || message == "" {
			t.Fatalf("unsafe diagnostic: %q", message)
		}
	}

	if got := SafeError(tests[1]); got != "Diia API request failed (HTTP 401)" {
		t.Fatalf("HTTP status diagnostic: %q", got)
	}
	if got := SafeError(tests[3]); got != "request validation failed: returnLink" {
		t.Fatalf("validation field diagnostic: %q", got)
	}
	if got := SafeError(tests[4]); got != "request validation failed" {
		t.Fatalf("unknown validation field diagnostic: %q", got)
	}
}
