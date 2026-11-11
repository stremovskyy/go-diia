package go_diia

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"time"
)

const (
	ProductionBaseURL = "https://api2.diia.gov.ua"
	SandboxBaseURL    = "https://api2s.diia.gov.ua"
)

type clientConfig struct {
	environment                    Environment
	baseURL                        string
	acquirerToken                  string
	authAcquirerToken              string
	httpClient                     *http.Client
	timeout                        time.Duration
	recorder                       Recorder
	crypto                         CryptoProvider
	multisharingArchive            MultisharingArchiveProcessor
	callbackToken                  string
	callbackRequestIDAuthorizer    CallbackRequestIDAuthorizer
	callbackOfferRequestAuthorizer CallbackOfferRequestAuthorizer
	callbackErrorMapper            CallbackErrorMapper

	now func() time.Time
}

// CallbackErrorMapper explicitly maps an internal callback error to the
// sanitized message returned to Diia. When it is not configured, the SDK
// always returns a generic failure message.
type CallbackErrorMapper func(error) string

// CallbackRequestIDAuthorizer checks an active application-owned request ID
// when a callback arrives without an Authorization header. It must not consume
// the ID: the application still has to atomically claim it before persistence.
// A present, invalid Authorization header is always rejected.
type CallbackRequestIDAuthorizer func(ctx context.Context, requestID string) (bool, error)

// CallbackOfferRequestAuthorizer binds a callback trace ID to the signed
// offerRequestId when neither signed encodeData nor signed ZIP metadata
// contains a requestId. It should read the same durable pending-request store
// used by the callback handler.
type CallbackOfferRequestAuthorizer func(ctx context.Context, requestID, offerRequestID string) (bool, error)

func defaultConfig(acquirerToken string) *clientConfig {
	return &clientConfig{
		environment:   EnvironmentProduction,
		baseURL:       ProductionBaseURL,
		acquirerToken: strings.TrimSpace(acquirerToken),
		httpClient:    http.DefaultClient,
		timeout:       30 * time.Second,
		now:           time.Now,
	}
}

// Option configures a client.
type Option func(*clientConfig)

func WithEnvironment(environment Environment) Option {
	return func(c *clientConfig) {
		c.environment = environment

		switch environment {
		case EnvironmentSandbox:
			c.baseURL = SandboxBaseURL
		case EnvironmentProduction:
			c.baseURL = ProductionBaseURL
		}
	}
}

func WithBaseURL(baseURL string) Option {
	return func(c *clientConfig) {
		if value := strings.TrimRight(strings.TrimSpace(baseURL), "/"); value != "" {
			c.baseURL = value
		}
	}
}

func WithAuthAcquirerToken(token string) Option {
	return func(c *clientConfig) { c.authAcquirerToken = strings.TrimSpace(token) }
}

func WithHTTPClient(client *http.Client) Option {
	return func(c *clientConfig) {
		if client != nil {
			c.httpClient = client
		}
	}
}

// WithClient is a compatibility alias matching the other stremovskyy SDKs.
func WithClient(client *http.Client) Option { return WithHTTPClient(client) }

func WithTimeout(timeout time.Duration) Option {
	return func(c *clientConfig) {
		if timeout >= 0 {
			c.timeout = timeout
		}
	}
}

func WithRecorder(recorder Recorder) Option {
	return func(c *clientConfig) { c.recorder = recorder }
}

func WithCryptoProvider(provider CryptoProvider) Option {
	return func(c *clientConfig) {
		if isNilProvider(provider) {
			c.crypto = nil
			return
		}
		c.crypto = provider
	}
}

func WithMultisharingArchiveProcessor(processor MultisharingArchiveProcessor) Option {
	return func(c *clientConfig) {
		if isNilProvider(processor) {
			c.multisharingArchive = nil
			return
		}
		c.multisharingArchive = processor
	}
}

func isNilProvider(provider any) bool {
	if provider == nil {
		return true
	}
	value := reflect.ValueOf(provider)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func WithCallbackToken(token string) Option {
	return func(c *clientConfig) { c.callbackToken = strings.TrimSpace(token) }
}

// WithCallbackRequestIDAuthorizer explicitly permits an absent callback
// Authorization header after an active request ID check. Cryptographic
// verification by Process is still required before acknowledging success.
func WithCallbackRequestIDAuthorizer(authorizer CallbackRequestIDAuthorizer) Option {
	return func(c *clientConfig) { c.callbackRequestIDAuthorizer = authorizer }
}

func WithCallbackOfferRequestAuthorizer(authorizer CallbackOfferRequestAuthorizer) Option {
	return func(c *clientConfig) { c.callbackOfferRequestAuthorizer = authorizer }
}

// WithCallbackErrorMapper configures an explicit, application-owned mapping
// from internal errors to safe callback response messages.
func WithCallbackErrorMapper(mapper CallbackErrorMapper) Option {
	return func(c *clientConfig) { c.callbackErrorMapper = mapper }
}
