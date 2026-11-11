//go:build iit_integration

package iit_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	diia "github.com/stremovskyy/go-diia"
	diiaiit "github.com/stremovskyy/go-diia/iit"
)

func TestOfficialIITCallbackThroughGoDiia(t *testing.T) {
	fixtureDir := os.Getenv("DIIA_IIT_FIXTURES_DIR")

	if fixtureDir == "" {
		t.Fatal("DIIA_IIT_FIXTURES_DIR must point to the installed official fixtures")
	}

	read := func(name string) []byte {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(fixtureDir, name))
		if err != nil {
			t.Fatal(err)
		}

		return data
	}
	decode := func(name string) []byte {
		t.Helper()
		data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(read(name))))
		if err != nil {
			t.Fatal(err)
		}

		return data
	}

	password := strings.TrimSpace(strings.TrimPrefix(string(read("Password.txt")), "\ufeff"))
	config, err := diiaiit.LoadConfig().
		WithPrivateKeyFile(filepath.Join(fixtureDir, "Key-6.dat"), password).
		WithCASettingsFile(filepath.Join(fixtureDir, "CAs.Test.json")).
		WithCACertificatesFile(filepath.Join(fixtureDir, "CACertificates.Test.p7b")).
		WithPartnerCertificateFiles(filepath.Join(fixtureDir, "TestUser.cer"), filepath.Join(fixtureDir, "TestUser-KEP.cer")).
		WithDiiaEncryptionCertificateFiles(filepath.Join(fixtureDir, "TestUser-KEP.cer")).
		WithDiiaSigningCertificateFiles(filepath.Join(fixtureDir, "TestUser.cer")).
		Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	provider, err := diiaiit.NewProvider().WithConfig(config).Do(context.Background())
	config.Destroy()

	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := provider.Close().Do(context.Background()); err != nil {
			t.Errorf("close provider: %v", err)
		}
	}()

	callbackRequest := func(withSignature bool) *http.Request {
		t.Helper()
		type callbackField struct {
			name string
			data []byte
		}
		fields := []callbackField{
			{"encodeData", decode("encodedData.json.p7s.p7e")},
			{"internal-passport", decode("document.pdf.p7s.p7e")},
		}

		if withSignature {
			fields = append(fields, callbackField{"signature-internal-passport", read("document.pdf.p7s")})
		}

		var body bytes.Buffer
		writer := multipart.NewWriter(&body)

		for _, field := range fields {
			part, err := writer.CreateFormField(field.name)
			if err != nil {
				t.Fatal(err)
			}

			if _, err := io.WriteString(part, base64.StdEncoding.EncodeToString(field.data)); err != nil {
				t.Fatal(err)
			}
		}

		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}

		request := httptest.NewRequest(http.MethodPost, "https://partner.example/diia/callback", &body)
		request.Header.Set("Content-Type", writer.FormDataContentType())
		request.Header.Set("Authorization", "Bearer test-callback-token")
		request.Header.Set("X-Document-Request-Trace-Id", "official-request-123")

		return request
	}

	client := diia.NewClient(
		"unused",
		diia.WithCallbackToken("test-callback-token"),
		diia.WithCryptoProvider(provider),
	)
	payload, err := client.Callbacks().Parse().WithRequest(callbackRequest(false)).Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.Callbacks().Process().WithPayload(payload).Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if result.RequestID != "official-request-123" || result.Barcode != "2858364170864" ||
		!bytes.Equal(result.RawMetadata, read("encodedData.json")) ||
		len(result.Documents) != 1 || !bytes.Equal(result.Documents[0].PDF, read("document.pdf")) {
		t.Fatalf(
			"unexpected callback package: request=%q barcode=%q documents=%d",
			result.RequestID,
			result.Barcode,
			len(result.Documents),
		)
	}

	// The standalone PDF fixture is signed by a distinct user certificate. It
	// must reach Verify and fail the configured Diia signer policy.
	payload, err = client.Callbacks().Parse().WithRequest(callbackRequest(true)).Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Callbacks().Process().WithPayload(payload).Do(context.Background())

	if !errors.Is(err, diia.ErrCrypto) || !errors.Is(err, diiaiit.ErrCertificate) {
		t.Fatalf("untrusted signature error = %v", err)
	}
}
