package go_diia

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	diialog "github.com/stremovskyy/go-diia/log"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestInternalErrorCausesRemainDiscoverable(t *testing.T) {
	transportClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, context.Canceled
	})}
	_, err := NewClient("acquirer", WithHTTPClient(transportClient)).Auth().SessionToken().Do(context.Background())

	if !errors.Is(err, ErrTransport) || !errors.Is(err, context.Canceled) {
		t.Fatalf("transport error = %v", err)
	}

	_, err = encodePayload(func() {})
	var unsupported *json.UnsupportedTypeError

	if !errors.Is(err, ErrEncode) || !errors.As(err, &unsupported) {
		t.Fatalf("encode error = %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{invalid`)
	}))
	defer server.Close()

	_, err = NewClient("acquirer", WithBaseURL(server.URL)).Auth().SessionToken().Do(context.Background())
	var syntax *json.SyntaxError

	if !errors.Is(err, ErrDecode) || !errors.As(err, &syntax) {
		t.Fatalf("decode error = %v", err)
	}
}

func TestEndpointContracts(t *testing.T) {
	type contract struct {
		name     string
		method   string
		path     string
		query    string
		body     map[string]any
		status   int
		response string
		call     func(Diia) error
	}
	branchRequest := func() *BranchRequest {
		return NewBranchRequest().
			WithName("Kyiv").
			WithEmail("test@example.com").
			WithRegion("Kyiv region").
			WithDistrict("Kyiv district").
			WithLocation("Kyiv").
			WithStreet("Khreshchatyk").
			WithHouse("1").
			WithSharing(DocumentTypeInternalPassport)
	}
	offerRequest := func() *OfferRequest {
		return NewOfferRequest().
			WithName("Account opening").
			WithSharing(DocumentTypeInternalPassport).
			WithReturnLink("https://example.com/done")
	}
	contracts := []contract{
		{name: "create branch", method: http.MethodPost, path: "/api/v2/acquirers/branch", body: map[string]any{
			"name": "Kyiv", "email": "test@example.com", "region": "Kyiv region", "district": "Kyiv district", "location": "Kyiv", "street": "Khreshchatyk", "house": "1",
			"deliveryTypes": []any{
				"api",
			}, "offerRequestType": "dynamic", "scopes": map[string]any{"sharing": []any{"internal-passport"}},
		}, status: 200, response: `{"_id":"branch"}`, call: func(c Diia) error {
			request := branchRequest()
			_, err := c.Branches().Create().
				WithName(request.Name).
				WithEmail(request.Email).
				WithRegion(request.Region).
				WithDistrict(request.District).
				WithLocation(request.Location).
				WithStreet(request.Street).
				WithHouse(request.House).
				WithSharing(request.Scopes.Sharing...).
				Do(context.Background())

			return err
		}},
		{name: "update branch", method: http.MethodPut, path: "/api/v2/acquirers/branch/branch", body: map[string]any{
			"name": "Kyiv", "email": "test@example.com", "region": "Kyiv region", "district": "Kyiv district", "location": "Kyiv", "street": "Khreshchatyk", "house": "1",
			"deliveryTypes": []any{
				"api",
			}, "offerRequestType": "dynamic", "scopes": map[string]any{"sharing": []any{"internal-passport"}},
		}, status: 200, response: `{"_id":"branch"}`, call: func(c Diia) error {
			request := branchRequest()
			_, err := c.Branches().Update().
				WithBranchID("branch").
				WithName(request.Name).
				WithEmail(request.Email).
				WithRegion(request.Region).
				WithDistrict(request.District).
				WithLocation(request.Location).
				WithStreet(request.Street).
				WithHouse(request.House).
				WithSharing(request.Scopes.Sharing...).
				Do(context.Background())

			return err
		}},
		{
			name:   "delete branch",
			method: http.MethodDelete,
			path:   "/api/v2/acquirers/branch/branch",
			status: 204,
			call: func(c Diia) error {
				return c.Branches().Delete().WithBranchID("branch").Do(context.Background())
			},
		},
		{
			name:     "get branch",
			method:   http.MethodGet,
			path:     "/api/v2/acquirers/branch/branch",
			status:   200,
			response: `{"_id":"branch"}`,

			call: func(c Diia) error {
				_, err := c.Branches().Get().WithBranchID("branch").Do(context.Background())

				return err
			},
		},
		{
			name:     "list branches",
			method:   http.MethodGet,
			path:     "/api/v2/acquirers/branches",
			query:    "limit=2&skip=1",
			status:   200,
			response: `{"total":0,"branches":[]}`,
			call: func(c Diia) error {
				_, err := c.Branches().List().WithSkip(1).WithLimit(2).Do(context.Background())

				return err
			},
		},
		{
			name:   "create offer",
			method: http.MethodPost,
			path:   "/api/v1/acquirers/branch/branch/offer",
			body: map[string]any{
				"name": "Account opening", "returnLink": "https://example.com/done", "scopes": map[string]any{"sharing": []any{"internal-passport"}},
			},
			status:   200,
			response: `{"_id":"offer"}`,
			call: func(c Diia) error {
				request := offerRequest()
				_, err := c.Offers().Create().
					WithBranchID("branch").
					WithName(request.Name).
					WithSharing(request.Scopes.Sharing...).
					WithReturnLink(request.ReturnLink).
					Do(context.Background())

				return err
			},
		},
		{
			name:   "delete offer uses documented v1",
			method: http.MethodDelete,
			path:   "/api/v1/acquirers/branch/branch/offer/offer",
			status: 204,
			call: func(c Diia) error {
				return c.Offers().Delete().
					WithBranchID("branch").
					WithOfferID("offer").
					Do(context.Background())
			},
		},
		{
			name:     "list offers",
			method:   http.MethodGet,
			path:     "/api/v1/acquirers/branch/branch/offers",
			query:    "limit=100&skip=0",
			status:   200,
			response: `{"total":0,"offers":[]}`,
			call: func(c Diia) error {
				_, err := c.Offers().List().
					WithBranchID("branch").
					WithSkip(0).
					WithLimit(100).
					Do(context.Background())

				return err
			},
		},
		{
			name:   "offline share",
			method: http.MethodPost,
			path:   "/api/v1/acquirers/document-request",
			body: map[string]any{
				"branchId": "branch", "barcode": "1234567890123", "requestId": "request-123", "useDiiaId": true,
			},
			status:   200,
			response: `{"success":true}`,
			call: func(c Diia) error {
				_, err := c.Sharing().RequestOffline().
					WithBranchID("branch").
					WithBarcode("1234567890123").
					WithRequestID("request-123").
					WithDiiaID(true).
					Do(context.Background())

				return err
			},
		},
		{
			name:   "online share sends actual offerId",
			method: http.MethodPost,
			path:   "/api/v2/acquirers/branch/branch/offer-request/dynamic",
			body: map[string]any{
				"offerId": "actual-offer", "requestId": "request-123", "returnLink": "https://example.com/done", "useDiiaId": false,
			},
			status:   200,
			response: `{"deeplink":"https://diia.app/acquirers/offer-request/dynamic/otp"}`,
			call: func(c Diia) error {
				_, err := c.Sharing().RequestOnline().
					WithBranchID("branch").
					WithOfferID("actual-offer").
					WithRequestID("request-123").
					WithReturnLink("https://example.com/done").
					WithDiiaID(false).
					Do(context.Background())

				return err
			},
		},
		{
			name:     "offline status",
			method:   http.MethodGet,
			path:     "/api/v1/acquirers/document-request/status",
			query:    "barcode=1234567890123&requestId=request-123",
			status:   200,
			response: `{"status":"done"}`,
			call: func(c Diia) error {
				_, err := c.Sharing().OfflineStatus().
					WithBarcode("1234567890123").
					WithRequestID("request-123").
					Do(context.Background())

				return err
			},
		},
		{
			name:     "online status uses otp query",
			method:   http.MethodGet,
			path:     "/api/v1/acquirers/offer-request/status",
			query:    "otp=otp-value&requestId=request-123",
			status:   200,
			response: `{"status":"processing"}`,
			call: func(c Diia) error {
				_, err := c.Sharing().OnlineStatus().
					WithOTP("otp-value").
					WithRequestID("request-123").
					Do(context.Background())

				return err
			},
		},
	}

	for _, test := range contracts {
		t.Run(test.name, func(t *testing.T) {
			var endpointCalls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/auth/acquirer/acquirer" {
					if r.Header.Get("Authorization") != "" {
						t.Errorf("production auth sent Authorization header")
					}

					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"token":"session"}`)

					return
				}

				endpointCalls.Add(1)

				if r.Method != test.method || r.URL.Path != test.path || r.URL.RawQuery != test.query {
					t.Errorf(
						"request = %s %s?%s, want %s %s?%s",
						r.Method,
						r.URL.Path,
						r.URL.RawQuery,
						test.method,
						test.path,
						test.query,
					)
				}

				if r.Header.Get("Authorization") != "Bearer session" || r.Header.Get("Accept") != "application/json" {
					t.Errorf("unexpected headers: %#v", r.Header)
				}

				if test.body != nil {
					if r.Header.Get("Content-Type") != "application/json" {
						t.Errorf("content type = %q", r.Header.Get("Content-Type"))
					}

					var got map[string]any

					if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
						t.Errorf("decode body: %v", err)
					}

					if !reflect.DeepEqual(got, test.body) {
						t.Errorf("body = %#v, want %#v", got, test.body)
					}
				}

				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, test.response)
			}))
			defer server.Close()

			client := NewClient("acquirer", WithBaseURL(server.URL))

			if err := test.call(client); err != nil {
				t.Fatalf("call: %v", err)
			}

			if endpointCalls.Load() != 1 {
				t.Fatalf("endpoint calls = %d", endpointCalls.Load())
			}
		})
	}
}

func TestSandboxBasicAuthAndProductionIsolation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Basic sandbox-secret" {
			t.Errorf("Authorization = %q", got)
		}

		_, _ = io.WriteString(w, `{"token":"session"}`)
	}))
	defer server.Close()

	client := NewSandboxClient("acquirer", "sandbox-secret", WithBaseURL(server.URL))

	if _, err := client.Auth().SessionToken().Do(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestTokenCacheConcurrentRefreshAnd401Retry(t *testing.T) {
	var authCalls atomic.Int32
	var branchCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/acquirer/acquirer":
			call := authCalls.Add(1)
			_, _ = fmt.Fprintf(w, `{"token":"session-%d"}`, call)

		case "/api/v2/acquirers/branch/branch":
			call := branchCalls.Add(1)

			if call == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = io.WriteString(w, `{"name":"Error","message":"expired","code":401,"data":{}}`)

				return
			}

			if r.Header.Get("Authorization") != "Bearer session-2" {
				t.Errorf("retry token = %q", r.Header.Get("Authorization"))
			}

			_, _ = io.WriteString(w, `{"_id":"branch"}`)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	firstSDK := NewClient("acquirer", WithBaseURL(server.URL))

	if _, err := firstSDK.Branches().Get().WithBranchID("branch").Do(context.Background()); err != nil {
		t.Fatal(err)
	}

	if authCalls.Load() != 2 || branchCalls.Load() != 2 {
		t.Fatalf("calls: auth=%d branch=%d", authCalls.Load(), branchCalls.Load())
	}

	// A fresh client performs exactly one auth request under concurrent load.
	authCalls.Store(0)
	sdk := NewClient("acquirer", WithBaseURL(server.URL))
	const workers = 32
	var wait sync.WaitGroup
	errorsSeen := make(chan error, workers)

	for range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()

			_, err := sdk.Auth().SessionToken().Do(context.Background())
			errorsSeen <- err
		}()
	}

	wait.Wait()
	close(errorsSeen)

	for err := range errorsSeen {
		if err != nil {
			t.Fatal(err)
		}
	}

	if authCalls.Load() != 1 {
		t.Fatalf("concurrent auth calls = %d, want 1", authCalls.Load())
	}

	internal := sdk.(*client)

	internal.tokenMu.Lock()
	internal.sessionEnds = time.Now().Add(4 * time.Minute)
	internal.tokenMu.Unlock()

	if _, err := sdk.Auth().SessionToken().Do(context.Background()); err != nil {
		t.Fatal(err)
	}

	if authCalls.Load() != 2 {
		t.Fatalf("early refresh auth calls = %d, want 2", authCalls.Load())
	}
}

func TestAPIErrorShapesAndSentinels(t *testing.T) {
	tests := []struct {
		status   int
		body     string
		sentinel error
	}{
		{
			400,
			`{"name":"Error","message":"bad","code":400,"type":"validation","data":{"field":["invalid"]}}`,
			ErrBadRequest,
		},
		{401, `{"error":{"name":"Error","message":"nope","code":401}}`, ErrUnauthorized},
		{404, `{"error":"missing"}`, ErrNotFound},
		{422, `{"message":"invalid","data":["x"]}`, ErrUnprocessableEntity},
		{500, `not-json`, ErrServer},
	}

	for _, test := range tests {
		t.Run(fmt.Sprint(test.status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/auth/acquirer/acquirer" {
					_, _ = io.WriteString(w, `{"token":"session"}`)

					return
				}

				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, test.body)
			}))
			defer server.Close()

			_, err := NewClient("acquirer", WithBaseURL(server.URL)).Branches().Get().
				WithBranchID("branch").
				Do(context.Background())

			if !errors.Is(err, test.sentinel) {
				t.Fatalf("errors.Is(%v) = false", test.sentinel)
			}

			var apiError *APIError

			if !errors.As(err, &apiError) || apiError.HTTPStatus != test.status ||
				string(apiError.RawBody) != test.body {
				t.Fatalf("APIError = %#v, %v", apiError, err)
			}
		})
	}
}

func TestRecorderNeverReceivesQueryHeadersOrBodies(t *testing.T) {
	var records []HTTPRecord
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/acquirer/acquirer-secret" {
			_, _ = io.WriteString(w, `{"token":"session-secret"}`)

			return
		}

		_, _ = io.WriteString(w, `{"status":"done"}`)
	}))
	defer server.Close()

	client := NewClient(
		"acquirer-secret",
		WithBaseURL(server.URL),
		WithRecorder(RecorderFunc(func(_ context.Context, record HTTPRecord) error {
			mu.Lock()
			defer mu.Unlock()

			records = append(records, record)

			return nil
		})),
	)
	_, err := client.Sharing().OfflineStatus().
		WithBarcode("1234567890123").
		WithRequestID("request-secret").
		Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	for _, record := range records {
		encoded, _ := json.Marshal(record)

		for _, secret := range []string{"acquirer-secret", "session-secret", "1234567890123", "request-secret"} {
			if stringContains(string(encoded), secret) {
				t.Fatalf("recorder leaked %q in %s", secret, encoded)
			}
		}
	}
}

func TestStructuredLogsRedactTokensQueriesAndBodies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/acquirer/acquirer-secret" {
			_, _ = io.WriteString(w, `{"token":"session-secret"}`)

			return
		}

		_, _ = io.WriteString(w, `{"status":"done"}`)
	}))
	defer server.Close()

	sdk := NewClient("acquirer-secret", WithBaseURL(server.URL))
	var output strings.Builder
	internal := sdk.(*client)
	internal.logger = diialog.NewLoggerWithWriter("test", &output)
	internal.SetLogLevel(diialog.LevelDebug)
	_, err := sdk.Sharing().OfflineStatus().
		WithBarcode("1234567890123").
		WithRequestID("request-secret").
		Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	for _, secret := range []string{"acquirer-secret", "session-secret", "1234567890123", "request-secret"} {
		if strings.Contains(output.String(), secret) {
			t.Fatalf("log leaked %q in %s", secret, output.String())
		}
	}

	if !strings.Contains(output.String(), "/api/v1/auth/acquirer/{acquirer_token}") {
		t.Fatalf("auth path was not structurally redacted: %s", output.String())
	}
}

func stringContains(value, fragment string) bool {
	for i := 0; i+len(fragment) <= len(value); i++ {
		if value[i:i+len(fragment)] == fragment {
			return true
		}
	}

	return false
}
