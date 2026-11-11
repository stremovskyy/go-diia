package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"

	diia "github.com/stremovskyy/go-diia"
)

const (
	demoRequestID = "demo-request-123"
	demoToken     = "local-demo-token"
)

// insecureDemoCrypto only strips markers from synthetic bytes. It does not
// decrypt, verify a signature, or authenticate any data. Never use it with a
// real Diia callback.
type insecureDemoCrypto struct{}

func (insecureDemoCrypto) DecryptAndVerify(ctx context.Context, envelope []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	const marker = "demo-encrypted:"

	if !bytes.HasPrefix(envelope, []byte(marker)) {
		return nil, errors.New("invalid local demo envelope")
	}

	return bytes.Clone(envelope[len(marker):]), nil
}

func (insecureDemoCrypto) Verify(ctx context.Context, signed []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	const marker = "demo-signed:"

	if !bytes.HasPrefix(signed, []byte(marker)) {
		return nil, errors.New("invalid local demo signature")
	}

	return bytes.Clone(signed[len(marker):]), nil
}

// pendingRequests models an atomic, single-process consume operation for the
// demo. A real callback handler needs a durable, shared store.
type pendingRequests struct {
	mu  sync.Mutex
	ids map[string]struct{}
}

func (p *pendingRequests) consume(requestID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.ids[requestID]; !ok {
		return false
	}

	delete(p.ids, requestID)

	return true
}

func localRequest(token string) (*http.Request, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields := []struct {
		name, filename string
		data           string
	}{
		{
			"encodeData",
			"",
			"demo-encrypted:{\"requestId\":\"demo-request-123\",\"barcode\":\"1234567890123\",\"documentTypes\":[\"internal-passport\"],\"data\":{}}",
		},
		{"internal-passport", "demo.pdf.p7s.p7e", "demo-encrypted:%PDF-local-demo"},
		{"signature-internal-passport-1", "demo.pdf.p7s", "demo-signed:local-demo-hash"},
	}

	for _, field := range fields {
		var partWriter io.Writer
		var err error
		if field.filename == "" {
			partWriter, err = writer.CreateFormField(field.name)
		} else {
			partWriter, err = writer.CreateFormFile(field.name, field.filename)
		}

		if err != nil {
			return nil, err
		}

		if _, err := io.WriteString(partWriter, base64.StdEncoding.EncodeToString([]byte(field.data))); err != nil {
			return nil, err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	request := httptest.NewRequest(http.MethodPost, "https://local.example/diia/callback", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("X-Document-Request-Trace-Id", demoRequestID)
	request.Header.Set("Authorization", "Bearer "+token)

	return request, nil
}

func handleLocalCallback(client diia.Diia, pending *pendingRequests, request *http.Request) (int, error) {
	response := httptest.NewRecorder()
	ctx := request.Context()

	payload, err := client.Callbacks().Parse().WithRequest(request).Do(ctx)

	if err == nil && !pending.consume(payload.RequestID) {
		err = errors.New("unknown or already consumed request ID")
	}

	if err == nil {
		var documentPackage *diia.DocumentPackage
		documentPackage, err = client.Callbacks().Process().WithPayload(payload).Do(ctx)

		if err == nil && (documentPackage.RequestID != demoRequestID ||
			len(documentPackage.Documents) != 1 || len(documentPackage.Signatures) != 1) {
			err = errors.New("unexpected local demo package")
		}
	}

	if err != nil {
		writeErr := client.Callbacks().WriteFailure().WithWriter(response).WithError(err).Do(ctx)

		return response.Code, writeErr
	}

	writeErr := client.Callbacks().WriteSuccess().WithWriter(response).Do(ctx)

	return response.Code, writeErr
}

func main() {
	// No credentials, socket, or Diia network request is needed for this demo.
	client := diia.NewClient("", diia.WithCallbackToken(demoToken), diia.WithCryptoProvider(insecureDemoCrypto{}))
	pending := &pendingRequests{ids: map[string]struct{}{demoRequestID: {}}}

	scenarios := []struct {
		name  string
		token string
		want  int
	}{
		{"valid local callback", demoToken, http.StatusOK},
		{"replayed callback", demoToken, http.StatusNotFound},
		{"invalid bearer token", "wrong-local-token", http.StatusNotFound},
	}

	for _, scenario := range scenarios {
		request, err := localRequest(scenario.token)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: could not build local request\n", scenario.name)
			os.Exit(1)
		}

		status, err := handleLocalCallback(client, pending, request)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: could not write local response\n", scenario.name)
			os.Exit(1)
		}

		if status != scenario.want {
			fmt.Fprintf(os.Stderr, "%s: unexpected HTTP status %d\n", scenario.name, status)
			os.Exit(1)
		}

		fmt.Printf("%s: HTTP %d\n", scenario.name, status)
	}
}
