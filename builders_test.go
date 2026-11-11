package go_diia

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestExecutableBuildersValidateContextAndFieldsBeforeTransport(t *testing.T) {
	var calls atomic.Int32
	client := NewClient("token", WithHTTPClient(&http.Client{Transport: roundTripFunc(
		func(*http.Request) (*http.Response, error) {
			calls.Add(1)

			return nil, errors.New("must not execute")
		},
	)}))

	//nolint:staticcheck // The public builder contract must reject a nil context.
	if _, err := client.Auth().SessionToken().Do(nil); !errors.Is(err, ErrValidation) {
		t.Fatalf("nil context error = %v", err)
	}

	if _, err := client.Sharing().RequestOnline().Do(context.Background()); !errors.Is(err, ErrValidation) {
		t.Fatalf("missing fields error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := client.Branches().List().Do(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled context error = %v", err)
	}

	if calls.Load() != 0 {
		t.Fatalf("transport calls = %d, want 0", calls.Load())
	}
}

func TestExecutableBuilderCanExecuteMoreThanOnce(t *testing.T) {
	var branchCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/api/v1/auth/acquirer/acquirer" {
			_, _ = io.WriteString(writer, `{"token":"session"}`)

			return
		}

		branchCalls.Add(1)
		_, _ = io.WriteString(writer, `{"_id":"branch"}`)
	}))
	defer server.Close()

	builder := NewClient("acquirer", WithBaseURL(server.URL)).Branches().Get().WithBranchID(" branch ")

	for range 2 {
		branch, err := builder.Do(context.Background())

		if err != nil || branch.ID != "branch" {
			t.Fatalf("Do() = %#v, %v", branch, err)
		}
	}

	if branchCalls.Load() != 2 {
		t.Fatalf("branch calls = %d, want 2", branchCalls.Load())
	}
}

func TestZeroArgumentRequestModelsNormalizeFluentValues(t *testing.T) {
	request := NewOnlineShareRequest().
		WithBranchID(" branch ").
		WithOfferID(" offer ").
		WithRequestID(" request-123 ").
		WithReturnLink(" https://example.com/done ").
		WithDiiaID(true)

	if request.BranchID != "branch" || request.OfferID != "offer" || request.RequestID != "request-123" ||
		request.ReturnLink != "https://example.com/done" || request.UseDiiaID == nil || !*request.UseDiiaID {
		t.Fatalf("request = %#v", request)
	}
}

type failingResponseWriter struct {
	header http.Header
	err    error
}

func (w *failingResponseWriter) Header() http.Header { return w.header }
func (*failingResponseWriter) WriteHeader(int)       {}
func (w *failingResponseWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func TestCallbackWriterBuilderPreservesWriteCause(t *testing.T) {
	cause := errors.New("connection closed")
	writer := &failingResponseWriter{header: make(http.Header), err: cause}
	err := NewClient("unused").Callbacks().WriteSuccess().
		WithWriter(writer).
		Do(context.Background())

	if !errors.Is(err, ErrCallback) || !errors.Is(err, cause) || strings.Contains(err.Error(), "secret") {
		t.Fatalf("write error = %v", err)
	}
}
