package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRelayForwardsRawCallback(t *testing.T) {
	const boundary = "sandbox-boundary"
	body := []byte("--" + boundary + "\r\nContent-Disposition: form-data; name=\"encodeData\"\r\n\r\n")
	body = append(body, 0, 1, 2, 255)
	body = append(body, []byte("\r\n--"+boundary+"--\r\n")...)

	var calls atomic.Int32
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodPost || r.URL.Path != "/capture" || r.URL.RawQuery != "source=diia" {
			t.Errorf("forwarded method or URL: %s %s", r.Method, r.URL)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" ||
			r.Header.Get("X-Document-Request-Trace-Id") != "example-test-request" ||
			r.Header.Get("Content-Type") != "multipart/form-data; boundary="+boundary ||
			r.Header.Get("X-Custom-Header") != "unchanged" {
			t.Error("required end-to-end headers were not preserved")
		}
		if r.Header.Get("X-Hop-Header") != "" {
			t.Error("Connection-nominated hop header was forwarded")
		}
		got, err := io.ReadAll(r.Body)
		if err != nil || !bytes.Equal(got, body) {
			t.Error("multipart bytes changed in transit")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	target, err := url.Parse(upstream.URL + "/capture")
	if err != nil {
		t.Fatal(err)
	}
	handler := routes(&relay{target: target, client: upstream.Client()})
	request := httptest.NewRequest(http.MethodPost, "https://ms.utaxcloud.net"+callbackPath+"?source=diia", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer test-token")
	request.Header.Set("X-Document-Request-Trace-Id", "example-test-request")
	request.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	request.Header.Set("X-Custom-Header", "unchanged")
	request.Header.Set("Connection", "X-Hop-Header")
	request.Header.Set("X-Hop-Header", "must-not-forward")

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != `{"success":true}` || calls.Load() != 1 {
		t.Fatalf("callback result: status=%d body=%q upstream_calls=%d", response.Code, response.Body.String(), calls.Load())
	}
}

func TestRelayRejectsUnrelatedOrInvalidRequests(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()
	target, _ := url.Parse(upstream.URL)
	handler := routes(&relay{target: target, client: upstream.Client()})

	tests := []struct {
		name        string
		method      string
		path        string
		traceID     string
		contentType string
		want        int
	}{
		{"unrelated path", http.MethodPost, "/other", "request-123", "multipart/form-data; boundary=x", http.StatusNotFound},
		{"wrong method", http.MethodGet, callbackPath, "request-123", "multipart/form-data; boundary=x", http.StatusMethodNotAllowed},
		{"missing trace ID", http.MethodPost, callbackPath, "", "multipart/form-data; boundary=x", http.StatusBadRequest},
		{"wrong content type", http.MethodPost, callbackPath, "request-123", "application/json", http.StatusBadRequest},
		{"missing boundary", http.MethodPost, callbackPath, "request-123", "multipart/form-data", http.StatusBadRequest},
		{"health", http.MethodGet, "/healthz", "", "", http.StatusOK},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, "https://ms.utaxcloud.net"+test.path, strings.NewReader("raw"))
			request.Header.Set("X-Document-Request-Trace-Id", test.traceID)
			request.Header.Set("Content-Type", test.contentType)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d", response.Code, test.want)
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatalf("unexpected upstream calls: %d", calls.Load())
	}
}

func TestRelayDoesNotAcknowledgeUpstreamFailure(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, "upstream-secret")
	}))
	defer upstream.Close()
	target, _ := url.Parse(upstream.URL)
	handler := routes(&relay{target: target, client: upstream.Client()})
	request := httptest.NewRequest(http.MethodPost, "https://ms.utaxcloud.net"+callbackPath, strings.NewReader("raw"))
	request.Header.Set("X-Document-Request-Trace-Id", "request-123")
	request.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadGateway || strings.Contains(response.Body.String(), "upstream-secret") ||
		strings.Contains(response.Body.String(), `"success":true`) {
		t.Fatalf("unsafe failure response: status=%d body=%q", response.Code, response.Body.String())
	}
}

func TestRelayDoesNotFollowUpstreamRedirect(t *testing.T) {
	var redirectedCalls atomic.Int32
	redirected := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		redirectedCalls.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer redirected.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirected.URL, http.StatusFound)
	}))
	defer upstream.Close()
	target, _ := url.Parse(upstream.URL)
	handler := routes(&relay{target: target, client: &http.Client{CheckRedirect: noRedirect}})
	request := httptest.NewRequest(http.MethodPost, "https://ms.utaxcloud.net"+callbackPath, strings.NewReader("raw"))
	request.Header.Set("X-Document-Request-Trace-Id", "request-123")
	request.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadGateway || redirectedCalls.Load() != 0 {
		t.Fatalf("redirect was followed or acknowledged: status=%d redirected_calls=%d", response.Code, redirectedCalls.Load())
	}
}

func TestWebhookURLValidation(t *testing.T) {
	const valid = "https://webhook.site/00000000-0000-4000-8000-000000000001"
	if _, err := parseWebhookURL(valid); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		"http://webhook.site/00000000-0000-4000-8000-000000000001",
		"https://webhook.site.evil.example/00000000-0000-4000-8000-000000000001",
		"https://user:password@webhook.site/00000000-0000-4000-8000-000000000001",
		"https://webhook.site/other",
		"https://webhook.site/00000000-0000-4000-8000-000000000001?copy=1",
	} {
		if _, err := parseWebhookURL(raw); err == nil {
			t.Errorf("accepted invalid webhook URL")
		}
	}
}
