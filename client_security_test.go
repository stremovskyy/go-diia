package go_diia

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type observedContext struct {
	context.Context

	entered chan struct{}
	once    sync.Once
}

func (c *observedContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.entered) })

	return c.Context.Done()
}

func TestTransportErrorRedactsURLAndPreservesCause(t *testing.T) {
	cause := errors.New("dial failed")
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, cause
	})}
	_, err := NewSandboxClient("secret-acquirer", "secret-basic", WithHTTPClient(httpClient)).
		Auth().SessionToken().Do(context.Background())

	if err == nil || !errors.Is(err, ErrTransport) || !errors.Is(err, cause) {
		t.Fatalf("transport causes lost: %T", err)
	}

	var urlErr *url.Error

	if !errors.As(err, &urlErr) {
		t.Fatalf("url.Error cause lost: %T", err)
	}

	if strings.Contains(urlErr.URL, "secret-acquirer") || strings.Contains(urlErr.Error(), "secret-acquirer") ||
		strings.Contains(urlErr.Error(), "secret-basic") {
		t.Fatalf("typed transport error exposed credentials: %s", urlErr)
	}

	if strings.Contains(err.Error(), "secret-acquirer") || strings.Contains(err.Error(), "secret-basic") ||
		strings.Contains(err.Error(), "/api/v1/auth/") {
		t.Fatalf("transport error exposed credentials: %s", err)
	}
}

func TestEarlyRefreshFailureKeepsValidSessionForConcurrentCallers(t *testing.T) {
	var authCalls, listCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/acquirer/acquirer":
			if authCalls.Add(1) == 1 {
				_, _ = io.WriteString(w, `{"token":"session-1"}`)

				return
			}

			w.WriteHeader(http.StatusServiceUnavailable)

		case "/api/v2/acquirers/branches":
			if r.Header.Get("Authorization") != "Bearer session-1" {
				t.Errorf("branch list used unexpected session")
			}

			listCalls.Add(1)
			_, _ = io.WriteString(w, `{"total":0,"branches":[]}`)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	sdk := NewClient("acquirer", WithBaseURL(server.URL))

	if _, err := sdk.Auth().SessionToken().Do(context.Background()); err != nil {
		t.Fatal(err)
	}

	internal := sdk.(*client)

	internal.tokenMu.Lock()
	internal.sessionEnds = time.Now().Add(4 * time.Minute)
	internal.tokenMu.Unlock()

	const callers = 32
	results := make(chan error, callers)

	for range callers {
		go func() {
			_, err := sdk.Branches().List().Do(context.Background())
			results <- err
		}()
	}

	for range callers {
		if err := <-results; err != nil {
			t.Fatalf("valid cached session was not used after refresh failure: %v", err)
		}
	}

	if got := authCalls.Load(); got != 2 {
		t.Fatalf("concurrent refresh attempts=%d, want one failed refresh", got-1)
	}

	if got := listCalls.Load(); got != callers {
		t.Fatalf("successful branch lists=%d, want %d", got, callers)
	}

	// The refresh backoff must never extend the token's actual lifetime.
	internal.tokenMu.Lock()
	internal.sessionEnds = time.Now().Add(-time.Second)
	internal.tokenMu.Unlock()

	if _, err := sdk.Auth().SessionToken().Do(context.Background()); !errors.Is(err, ErrServer) {
		t.Fatalf("expired cached session was reused: %v", err)
	}

	if got := authCalls.Load(); got != 3 {
		t.Fatalf("expired token did not trigger auth: calls=%d", got)
	}
}

func TestAPIErrorStringDoesNotEchoUpstreamText(t *testing.T) {
	err := &APIError{
		HTTPStatus: http.StatusBadRequest,
		Name:       "secret-name",
		Message:    "secret-message",
		RawBody:    []byte("secret-body"),
	}

	if got := err.Error(); got != "diia: API request failed (HTTP 400)" {
		t.Fatalf("unsafe API error: %q", got)
	}

	if err.Name != "secret-name" || err.Message != "secret-message" || string(err.RawBody) != "secret-body" {
		t.Fatal("structured API error data was changed")
	}
}

func TestExpectedResponseRejectsEmptySuccess(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusNoContent} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/api/v1/auth/acquirer/") {
					_, _ = io.WriteString(w, `{"token":"session"}`)

					return
				}

				w.WriteHeader(status)

				if status == http.StatusOK {
					_, _ = io.WriteString(w, "  \n")
				}
			}))
			defer server.Close()

			_, err := NewClient("acquirer", WithBaseURL(server.URL)).Branches().Get().
				WithBranchID("branch").Do(context.Background())

			if !errors.Is(err, ErrDecode) {
				t.Fatalf("empty HTTP %d result was accepted: %v", status, err)
			}
		})
	}
}

func TestMutating401InvalidatesSessionWithoutReplay(t *testing.T) {
	var authCalls, postCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/auth/acquirer/"):
			call := authCalls.Add(1)
			_, _ = fmt.Fprintf(w, `{"token":"session-%d"}`, call)

		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/acquirers/branch":
			postCalls.Add(1)
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"message":"expired"}`)

		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/acquirers/branches":
			_, _ = io.WriteString(w, `{"total":0,"branches":[]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	sdk := NewClient("acquirer", WithBaseURL(server.URL))
	_, err := sdk.Branches().Create().
		WithName("Kyiv").WithEmail("test@example.com").
		WithRegion("Kyiv region").WithDistrict("Kyiv district").
		WithLocation("Kyiv").WithStreet("Khreshchatyk").WithHouse("1").
		WithSharing(DocumentTypeInternalPassport).
		Do(context.Background())

	if !errors.Is(err, ErrUnauthorized) || postCalls.Load() != 1 || authCalls.Load() != 1 {
		t.Fatalf(
			"mutating 401 replayed or changed error: post=%d auth=%d err=%v",
			postCalls.Load(),
			authCalls.Load(),
			err,
		)
	}

	if _, err := sdk.Branches().List().Do(context.Background()); err != nil {
		t.Fatal(err)
	}

	if authCalls.Load() != 2 {
		t.Fatalf("session was not invalidated: auth calls=%d", authCalls.Load())
	}
}

func TestCanceledAuthWaiterReturnsBeforeLeader(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()

	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		close(entered)
		<-release

		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"token":"session"}`)),
			Header:     make(http.Header),
			Request:    r,
		}, nil
	})}
	sdk := NewClient("acquirer", WithHTTPClient(httpClient), WithTimeout(0))
	leaderDone := make(chan error, 1)
	go func() {
		_, err := sdk.Auth().SessionToken().Do(context.Background())
		leaderDone <- err
	}()
	<-entered

	base, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx := &observedContext{Context: base, entered: make(chan struct{})}
	waiterDone := make(chan error, 1)
	go func() {
		_, err := sdk.Auth().SessionToken().Do(ctx)
		waiterDone <- err
	}()

	select {
	case <-ctx.entered:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("waiter did not reach context-aware auth wait")
	}

	cancel()

	select {
	case err := <-waiterDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("waiter returned wrong error: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("canceled waiter remained blocked by auth refresh")
	}

	close(release)
	released = true

	if err := <-leaderDone; err != nil {
		t.Fatal(err)
	}
}

func TestBaseURLAndRedirectSecurity(t *testing.T) {
	for _, raw := range []string{"http://example.com", "ftp://example.com", "https://user:pass@example.com", "https://example.com?"} {
		if err := validateBaseURL(raw); !errors.Is(err, ErrValidation) {
			t.Fatalf("insecure base URL %q was accepted", raw)
		}
	}

	for _, raw := range []string{"https://example.com/prefix", "http://localhost:8080", "http://127.0.0.1:8080", "http://[::1]:8080"} {
		if err := validateBaseURL(raw); err != nil {
			t.Fatalf("safe base URL %q was rejected", raw)
		}
	}

	var insecureCalls atomic.Int32
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		insecureCalls.Add(1)

		return nil, errors.New("unexpected transport call")
	})}
	_, err := NewClient("secret-acquirer", WithBaseURL("http://example.com"), WithHTTPClient(httpClient)).
		Auth().SessionToken().Do(context.Background())

	if !errors.Is(err, ErrValidation) || insecureCalls.Load() != 0 || strings.Contains(err.Error(), "secret-acquirer") {
		t.Fatalf("insecure request reached transport or leaked credential: calls=%d err=%v", insecureCalls.Load(), err)
	}

	var targetCalls atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		targetCalls.Add(1)
	}))
	defer target.Close()

	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", target.URL)
		w.WriteHeader(http.StatusFound)
	}))
	defer source.Close()

	_, err = NewSandboxClient("acquirer", "basic-secret", WithBaseURL(source.URL), WithHTTPClient(source.Client())).
		Auth().SessionToken().Do(context.Background())
	var apiErr *APIError

	if !errors.As(err, &apiErr) || apiErr.HTTPStatus != http.StatusFound || targetCalls.Load() != 0 {
		t.Fatalf("credentialed redirect followed: target calls=%d err=%v", targetCalls.Load(), err)
	}
}

func TestAuthPathRedactedWithGatewayPrefix(t *testing.T) {
	got := sanitizedPath("/gateway/api/v1/auth/acquirer/secret-acquirer")

	if got != "/gateway/api/v1/auth/acquirer/{acquirer_token}" {
		t.Fatalf("gateway auth path leaked: %q", got)
	}
}
