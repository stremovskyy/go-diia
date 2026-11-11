package go_diia

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	diialog "github.com/stremovskyy/go-diia/log"
)

func TestMultisharingOfferAndOnlineWireFields(t *testing.T) {
	var offerCalls, onlineCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/auth/acquirer/") {
			_, _ = io.WriteString(w, `{"token":"session"}`)

			return
		}

		if got := r.Header.Get("Authorization"); got != "Bearer session" {
			t.Errorf("Authorization = %q", got)
		}

		var body map[string]any

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request body: %v", err)
		}

		switch r.URL.Path {
		case "/api/v1/acquirers/branch/branch/offer":
			offerCalls.Add(1)

			if r.Method != http.MethodPost {
				t.Errorf("offer method = %s", r.Method)
			}

			got, ok := body["scopes"].(map[string]any)
			want := map[string]any{
				"sharing":          []any{"internal-passport", "vehicle-license"},
				"mandatorySharing": []any{"internal-passport"},
			}

			if !ok || !reflect.DeepEqual(got, want) {
				t.Errorf("offer scopes = %#v", body["scopes"])
			}

			_, _ = io.WriteString(w, `{"_id":"offer"}`)

		case "/api/v2/acquirers/branch/branch/offer-request/dynamic":
			onlineCalls.Add(1)

			if r.Method != http.MethodPost || body["signAlgo"] != "DSTU4145-DSTU7564" {
				t.Errorf("online request = %s %#v", r.Method, body)
			}

			_, _ = io.WriteString(w, `{"deeplink":"https://diia.app/request/otp"}`)

		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	sdk := NewClient("acquirer", WithBaseURL(server.URL))

	if _, err := sdk.Offers().Create().
		WithBranchID("branch").WithName("Package").
		WithSharing(DocumentTypeInternalPassport, DocumentTypeVehicleLicense).
		WithMandatorySharing(DocumentTypeInternalPassport).
		Do(context.Background()); err != nil {
		t.Fatal(err)
	}

	if _, err := sdk.Sharing().RequestOnline().
		WithBranchID("branch").WithOfferID("offer").WithRequestID("request-123").
		WithSignAlgo("DSTU4145-DSTU7564").Do(context.Background()); err != nil {
		t.Fatal(err)
	}

	if offerCalls.Load() != 1 || onlineCalls.Load() != 1 {
		t.Fatalf("offer calls = %d, online calls = %d", offerCalls.Load(), onlineCalls.Load())
	}
}

func TestOptionalMultisharingFieldsOmitted(t *testing.T) {
	offer := NewOfferRequest().WithName("Package").WithSharing(DocumentTypeInternalPassport)
	online := NewOnlineShareRequest().WithBranchID("branch").WithOfferID("offer").WithRequestID("request-123")

	for name, request := range map[string]any{"offer": offer, "online": online} {
		encoded, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}

		if bytes.Contains(encoded, []byte(`"mandatorySharing"`)) || bytes.Contains(encoded, []byte(`"signAlgo"`)) {
			t.Fatalf("%s unexpectedly sent optional field", name)
		}
	}
}

func TestMultisharingRequestValidation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		request *OfferRequest
	}{
		{"sharing type is duplicated", NewOfferRequest().WithName("Package").WithSharing(DocumentTypeInternalPassport, DocumentTypeInternalPassport)},
		{"mandatory type is not shared", NewOfferRequest().WithName("Package").WithSharing(DocumentTypeInternalPassport).WithMandatorySharing(DocumentTypeVehicleLicense)},
		{"mandatory type is duplicated", NewOfferRequest().WithName("Package").WithSharing(DocumentTypeInternalPassport).WithMandatorySharing(DocumentTypeInternalPassport, DocumentTypeInternalPassport)},
		{"mandatory type is unknown", NewOfferRequest().WithName("Package").WithSharing(DocumentTypeInternalPassport).WithMandatorySharing(DocumentType("unknown"))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateOfferRequest(tc.request); !errors.Is(err, ErrValidation) {
				t.Fatalf("validation = %v", err)
			}
		})
	}

	for _, algo := range []string{"DSTU4145", "unknown", "DSTU4145-DSTU7564\n"} {
		request := NewOnlineShareRequest().WithBranchID("branch").WithOfferID("offer").WithRequestID("request-123")
		request.SignAlgo = algo

		if err := validateOnlineShareRequest(request); !errors.Is(err, ErrValidation) {
			t.Fatalf("invalid signAlgo accepted: %v", err)
		}
	}
}

func TestOfferRequestPasswordSessionRefreshAndRedaction(t *testing.T) {
	const requestID = "offer-request-secret-123"
	const password = "archive-password-secret"
	var authCalls, passwordCalls atomic.Int32
	var records []HTTPRecord
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/auth/acquirer/") {
			n := authCalls.Add(1)
			_, _ = io.WriteString(w, `{"token":"session-`+string(rune('0'+n))+`"}`)

			return
		}

		if r.URL.Path != "/api/v1/acquirers/offer-request/"+requestID+"/password" || r.Method != http.MethodGet {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}

		if r.Header.Get("Accept") != "application/json" || r.Header.Get("Content-Type") != "" {
			t.Errorf("unexpected password GET headers")
		}

		n := passwordCalls.Add(1)

		if got := r.Header.Get("Authorization"); got != "Bearer session-"+string(rune('0'+n)) {
			t.Errorf("Authorization = %q", got)
		}

		if n == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"message":"expired"}`)

			return
		}

		_, _ = io.WriteString(w, `{"password":"`+password+`"}`)
	}))
	defer server.Close()

	sdk := NewClient(
		"acquirer",
		WithBaseURL(server.URL),
		WithRecorder(RecorderFunc(func(_ context.Context, record HTTPRecord) error {
			records = append(records, record)

			return nil
		})),
	)
	var logs bytes.Buffer
	internal := sdk.(*client)
	internal.logger = diialog.NewLoggerWithWriter("test", &logs)
	internal.SetLogLevel(diialog.LevelDebug)
	got, err := sdk.Sharing().OfferRequestPassword(context.Background(), requestID)

	if err != nil || got != password {
		t.Fatalf("password GET failed: %v", err)
	}

	if authCalls.Load() != 2 || passwordCalls.Load() != 2 {
		t.Fatalf("auth calls = %d, password calls = %d", authCalls.Load(), passwordCalls.Load())
	}

	for _, record := range records {
		if strings.Contains(record.Path, requestID) || strings.Contains(record.Path, password) || record.Path == "" {
			t.Fatal("recorder exposed a secret or empty path")
		}
	}

	if strings.Contains(logs.String(), requestID) || strings.Contains(logs.String(), password) {
		t.Fatal("debug log exposed an offer request ID or password")
	}

	if got := sanitizedPath("/api/v1/acquirers/offer-request/" + requestID + "/password"); got != "/api/v1/acquirers/offer-request/{offer_request_id}/password" {
		t.Fatalf("sanitized path = %q", got)
	}
}

func TestOfferRequestPasswordErrors(t *testing.T) {
	for _, tc := range []struct {
		name     string
		status   int
		response string
		want     error
	}{
		{"missing password", 200, `{}`, ErrDecode},
		{"blank password", 200, `{"password":"   "}`, ErrDecode},
		{"invalid JSON", 200, `{`, ErrDecode},
		{"not found", 404, `{"message":"private upstream detail"}`, ErrNotFound},
		{"server error", 503, `{"message":"private upstream detail"}`, ErrServer},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/api/v1/auth/acquirer/") {
					_, _ = io.WriteString(w, `{"token":"session"}`)

					return
				}

				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.response)
			}))
			defer server.Close()

			_, err := NewClient(
				"acquirer",
				WithBaseURL(server.URL),
			).Sharing().
				OfferRequestPassword(context.Background(), "offer-request-123")

			if !errors.Is(err, tc.want) || strings.Contains(err.Error(), "private upstream detail") {
				t.Fatalf("error = %v", err)
			}
		})
	}

	var calls atomic.Int32
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls.Add(1); w.WriteHeader(200) }),
	)
	defer server.Close()

	sdk := NewClient("acquirer", WithBaseURL(server.URL))

	for _, id := range []string{"", "bad/id", " bad "} {
		if _, err := sdk.Sharing().OfferRequestPassword(context.Background(), id); !errors.Is(err, ErrValidation) {
			t.Fatalf("invalid identifier accepted: %v", err)
		}
	}

	//nolint:staticcheck // The public password API must reject a nil context.
	if _, err := sdk.Sharing().OfferRequestPassword(nil, "offer-request-123"); !errors.Is(err, ErrValidation) {
		t.Fatalf("nil context accepted: %v", err)
	}

	if calls.Load() != 0 {
		t.Fatalf("invalid requests reached the network: %d", calls.Load())
	}
}
