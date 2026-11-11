package go_diia

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	diialog "github.com/stremovskyy/go-diia/log"
)

const (
	sessionLifetime    = 2 * time.Hour
	sessionRefreshLead = 5 * time.Minute
	refreshRetryDelay  = 30 * time.Second
	maxAPIResponseSize = 8 << 20
)

type client struct {
	cfg    *clientConfig
	logger *diialog.Logger

	tokenMu      sync.Mutex
	session      string
	sessionEnds  time.Time
	refreshAfter time.Time
	authFlight   *authFlight

	auth      AuthService
	branches  BranchService
	offers    OfferService
	sharing   SharingService
	callbacks CallbackService
}

type authFlight struct {
	done    chan struct{}
	err     error
	aborted bool
}

// safeCause preserves errors.Is/As without printing a transport URL or body.
type safeCause struct{ cause error }

func (e safeCause) Error() string { return "request failed" }
func (e safeCause) Unwrap() error { return e.cause }

func transportError(err error) error {
	var urlErr *url.Error

	if errors.As(err, &urlErr) {
		redacted := *urlErr
		redacted.URL = "[redacted]"

		if redacted.Err != nil {
			redacted.Err = safeCause{cause: redacted.Err}
		}

		return fmt.Errorf("%w: %w", ErrTransport, &redacted)
	}

	return fmt.Errorf("%w: %w", ErrTransport, safeCause{cause: err})
}

// NewClient creates a production Diia Document Sharing client.
func NewClient(acquirerToken string, options ...Option) Diia {
	cfg := defaultConfig(acquirerToken)

	for _, option := range options {
		if option != nil {
			option(cfg)
		}
	}

	return newClient(cfg)
}

// NewDefaultClient creates a production client with production-safe defaults.
func NewDefaultClient(acquirerToken string) Diia { return NewClient(acquirerToken) }

// NewSandboxClient creates a sandbox client and configures the Basic
// auth-acquirer token required only by the sandbox session endpoint.
func NewSandboxClient(acquirerToken, authAcquirerToken string, options ...Option) Diia {
	cfg := defaultConfig(acquirerToken)
	cfg.environment = EnvironmentSandbox
	cfg.baseURL = SandboxBaseURL
	cfg.authAcquirerToken = strings.TrimSpace(authAcquirerToken)

	for _, option := range options {
		if option != nil {
			option(cfg)
		}
	}

	return newClient(cfg)
}

func newClient(cfg *clientConfig) Diia {
	// All SDK requests carry a credential in the URL or Authorization header.
	// Do not forward either credential to a redirect target.
	httpClient := *cfg.httpClient
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	cfg.httpClient = &httpClient

	c := &client{cfg: cfg, logger: diialog.NewLogger("go-diia")}
	c.auth = &authService{client: c}
	c.branches = &branchService{client: c}
	c.offers = &offerService{client: c}
	c.sharing = &sharingService{client: c}
	c.callbacks = &callbackService{client: c}

	return c
}

func (c *client) Auth() AuthService               { return c.auth }
func (c *client) Branches() BranchService         { return c.branches }
func (c *client) Offers() OfferService            { return c.offers }
func (c *client) Sharing() SharingService         { return c.sharing }
func (c *client) Callbacks() CallbackService      { return c.callbacks }
func (c *client) SetLogLevel(level diialog.Level) { c.logger.SetLevel(level) }

func (c *client) sessionToken(ctx context.Context) (*SessionToken, error) {
	if c.cfg.acquirerToken == "" {
		return nil, newValidationError("acquirerToken", "is required")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		c.tokenMu.Lock()
		now := c.cfg.now()

		if c.session != "" && (now.Before(c.sessionEnds.Add(-sessionRefreshLead)) ||
			(now.Before(c.sessionEnds) && now.Before(c.refreshAfter))) {
			result := &SessionToken{Token: c.session, ExpiresAt: c.sessionEnds}
			c.tokenMu.Unlock()

			return result, nil
		}

		if flight := c.authFlight; flight != nil {
			c.tokenMu.Unlock()

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-flight.done:
				if err := ctx.Err(); err != nil {
					return nil, err
				}

				if flight.err != nil && !flight.aborted {
					return nil, flight.err
				}

				continue
			}
		}

		flight := &authFlight{done: make(chan struct{})}
		c.authFlight = flight
		c.tokenMu.Unlock()

		result, err := c.fetchSessionToken(ctx)

		c.tokenMu.Lock()
		if err == nil {
			c.session = result.Token
			c.sessionEnds = result.ExpiresAt
			c.refreshAfter = time.Time{}
		} else if ctx.Err() == nil && c.session != "" && c.cfg.now().Before(c.sessionEnds) {
			// Keep using the unexpired token if only the early refresh failed.
			// A 401 will invalidate it and force another refresh immediately.
			result = &SessionToken{Token: c.session, ExpiresAt: c.sessionEnds}
			c.refreshAfter = c.cfg.now().Add(refreshRetryDelay)
			err = nil
		}
		flight.err = err
		flight.aborted = ctx.Err() != nil
		c.authFlight = nil
		close(flight.done)
		c.tokenMu.Unlock()

		return result, err
	}
}

func (c *client) fetchSessionToken(ctx context.Context) (*SessionToken, error) {
	path := "/api/v1/auth/acquirer/" + url.PathEscape(c.cfg.acquirerToken)
	headers := make(http.Header)

	if c.cfg.environment == EnvironmentSandbox && c.cfg.authAcquirerToken != "" {
		headers.Set("Authorization", "Basic "+c.cfg.authAcquirerToken)
	}

	var response struct {
		Token string `json:"token"`
	}

	if err := c.do(ctx, http.MethodGet, path, nil, headers, false, &response); err != nil {
		return nil, err
	}

	if strings.TrimSpace(response.Token) == "" {
		return nil, fmt.Errorf("%w: auth response token is empty", ErrDecode)
	}

	return &SessionToken{Token: response.Token, ExpiresAt: c.cfg.now().Add(sessionLifetime)}, nil
}

func (c *client) invalidateSession(used string) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	if used == "" || c.session == used {
		c.session = ""
		c.sessionEnds = time.Time{}
		c.refreshAfter = time.Time{}
	}
}

func (c *client) do(
	ctx context.Context,
	method, path string,
	payload any,
	headers http.Header,
	authorized bool,
	result any,
) error {
	body, err := encodePayload(payload)
	if err != nil {
		return err
	}

	if authorized {
		token, tokenErr := c.sessionToken(ctx)
		if tokenErr != nil {
			return tokenErr
		}

		err = c.doOnce(ctx, method, path, body, headers, token.Token, result)

		if errorsIsUnauthorized(err) {
			c.invalidateSession(token.Token)

			if method != http.MethodGet && method != http.MethodHead {
				return err
			}

			refreshed, refreshErr := c.sessionToken(ctx)
			if refreshErr != nil {
				return refreshErr
			}

			return c.doOnce(ctx, method, path, body, headers, refreshed.Token, result)
		}

		return err
	}

	return c.doOnce(ctx, method, path, body, headers, "", result)
}

func encodePayload(payload any) ([]byte, error) {
	if payload == nil {
		return nil, nil
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrEncode, err)
	}

	return data, nil
}

func errorsIsUnauthorized(err error) bool {
	if err == nil {
		return false
	}

	var apiError *APIError
	ok := errors.As(err, &apiError)

	return ok && apiError.HTTPStatus == http.StatusUnauthorized
}

func (c *client) doOnce(
	ctx context.Context,
	method, path string,
	body []byte,
	headers http.Header,
	bearerToken string,
	result any,
) (requestErr error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.cfg.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.cfg.timeout)
		defer cancel()
	}

	if err := validateBaseURL(c.cfg.baseURL); err != nil {
		return err
	}

	endpoint := c.cfg.baseURL + path
	request, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return transportError(err)
	}

	request.Header.Set("Accept", "application/json")

	if len(body) != 0 {
		request.Header.Set("Content-Type", "application/json")
	}

	for key, values := range headers {
		for _, value := range values {
			request.Header.Add(key, value)
		}
	}

	if bearerToken != "" {
		request.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	started := time.Now()
	status := 0
	defer func() {
		duration := time.Since(started)
		safePath := sanitizedPath(request.URL.Path)
		c.logger.Debug(
			"HTTP request completed",
			"method",
			method,
			"path",
			safePath,
			"status",
			status,
			"duration",
			duration.String(),
		)

		if c.cfg.recorder == nil {
			return
		}

		record := HTTPRecord{Method: method, Path: safePath, StatusCode: status, Duration: duration}

		if requestErr != nil {
			record.Error = errorCategory(requestErr)
		}

		if recorderErr := c.cfg.recorder.Record(ctx, record); recorderErr != nil {
			c.logger.Warn("recorder failed", "category", "recorder")
		}
	}()

	response, err := c.cfg.httpClient.Do(request)
	if err != nil {
		requestErr = transportError(err)

		return requestErr
	}

	defer func() { _ = response.Body.Close() }()

	status = response.StatusCode
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxAPIResponseSize+1))
	if err != nil {
		requestErr = fmt.Errorf("%w: read response: %w", ErrDecode, err)

		return requestErr
	}

	if len(responseBody) > maxAPIResponseSize {
		requestErr = fmt.Errorf("%w: response exceeds %d bytes", ErrDecode, maxAPIResponseSize)

		return requestErr
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		requestErr = decodeAPIError(response.StatusCode, responseBody)

		return requestErr
	}

	if result == nil {
		return nil
	}

	if len(bytes.TrimSpace(responseBody)) == 0 {
		requestErr = fmt.Errorf("%w: empty response body", ErrDecode)

		return requestErr
	}

	if err := json.Unmarshal(responseBody, result); err != nil {
		requestErr = fmt.Errorf("%w: unmarshal response: %w", ErrDecode, err)

		return requestErr
	}

	return nil
}

func sanitizedPath(path string) string {
	if index := strings.Index(path, "/api/v1/auth/acquirer/"); index >= 0 {
		return path[:index] + "/api/v1/auth/acquirer/{acquirer_token}"
	}

	if index := strings.Index(path, "/api/v1/acquirers/offer-request/"); index >= 0 &&
		strings.HasSuffix(path, "/password") {
		return path[:index] + "/api/v1/acquirers/offer-request/{offer_request_id}/password"
	}

	return path
}

func validateBaseURL(raw string) error {
	parsed, err := url.Parse(raw)

	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Opaque != "" || strings.ContainsAny(raw, "?#") {
		return newValidationError("baseURL", "must be an absolute HTTPS URL or an HTTP loopback URL")
	}

	if parsed.Scheme == "https" {
		return nil
	}

	if parsed.Scheme == "http" {
		host := parsed.Hostname()

		if strings.EqualFold(host, "localhost") {
			return nil
		}

		if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
			return nil
		}
	}

	return newValidationError("baseURL", "must be an absolute HTTPS URL or an HTTP loopback URL")
}

func decodeAPIError(status int, body []byte) error {
	apiError := &APIError{HTTPStatus: status, RawBody: append([]byte(nil), body...)}

	if len(bytes.TrimSpace(body)) != 0 {
		_ = json.Unmarshal(body, apiError)

		if apiError.Message == "" {
			var fallback struct {
				Error json.RawMessage `json:"error"`
			}

			if json.Unmarshal(body, &fallback) == nil && len(fallback.Error) != 0 {
				var nested APIError
				if json.Unmarshal(fallback.Error, &nested) == nil && nested.Message != "" {
					apiError.Name, apiError.Message, apiError.Code, apiError.Type, apiError.Data = nested.Name, nested.Message, nested.Code, nested.Type, nested.Data
				} else {
					_ = json.Unmarshal(fallback.Error, &apiError.Message)
				}
			}
		}
	}

	return apiError
}

func errorCategory(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrValidation):
		return "validation"
	case errors.Is(err, ErrUnauthorized):
		return "unauthorized"
	case errors.Is(err, ErrBadRequest):
		return "bad_request"
	case errors.Is(err, ErrNotFound):
		return "not_found"
	case errors.Is(err, ErrUnprocessableEntity):
		return "unprocessable_entity"
	case errors.Is(err, ErrServer):
		return "server"
	case errors.Is(err, ErrTransport):
		return "transport"
	case errors.Is(err, ErrEncode):
		return "encode"
	case errors.Is(err, ErrDecode):
		return "decode"
	case errors.Is(err, ErrCallback):
		return "callback"
	case errors.Is(err, ErrCrypto):
		return "crypto"
	default:
		return "request_failed"
	}
}
