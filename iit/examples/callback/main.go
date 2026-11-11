package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	diia "github.com/stremovskyy/go-diia"
	diiaiit "github.com/stremovskyy/go-diia/iit"
)

// This integration template rejects callbacks until the application supplies
// durable pending-request claims and storage in the functions below.
func main() {
	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelStartup()

	config, err := diiaiit.LoadConfig().
		WithPrivateKeyFile(os.Getenv("PARTNER_KEY_PATH"), os.Getenv("PARTNER_KEY_PASSWORD")).
		WithCASettingsFile(os.Getenv("IIT_CA_SETTINGS_PATH")).
		WithCACertificatesFile(os.Getenv("IIT_CA_CERTIFICATES_PATH")).
		WithPartnerCertificateFiles(paths(os.Getenv("PARTNER_CERTIFICATE_PATHS"))...).
		WithDiiaEncryptionCertificateFiles(paths(os.Getenv("DIIA_ENCRYPTION_CERTIFICATE_PATHS"))...).
		WithDiiaSigningCertificateFiles(paths(os.Getenv("DIIA_SIGNING_CERTIFICATE_PATHS"))...).
		Do(startupCtx)
	if err != nil {
		log.Fatal(err)
	}

	provider, err := diiaiit.NewProvider().WithConfig(config).Do(startupCtx)
	config.Destroy()

	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		if closeErr := provider.Close().Do(context.Background()); closeErr != nil {
			log.Printf("go-diia-iit provider close failed: %T", closeErr)
		}
	}()

	options := []diia.Option{
		diia.WithCallbackToken(os.Getenv("DIIA_CALLBACK_TOKEN")),
		diia.WithCryptoProvider(provider),
		diia.WithMultisharingArchiveProcessor(diiaiit.NewMultisharingArchiveProcessor(provider)),
		diia.WithCallbackOfferRequestAuthorizer(isPendingOfferRequest),
	}

	if os.Getenv("DIIA_ALLOW_MISSING_CALLBACK_BEARER") == "1" {
		options = append(options, diia.WithCallbackRequestIDAuthorizer(isPendingRequestID))
	}

	var client diia.Diia
	if strings.EqualFold(os.Getenv("DIIA_ENV"), "sandbox") {
		client = diia.NewSandboxClient(
			os.Getenv("DIIA_ACQUIRER_TOKEN"),
			os.Getenv("DIIA_AUTH_ACQUIRER_TOKEN"),
			options...,
		)
	} else {
		client = diia.NewClient(os.Getenv("DIIA_ACQUIRER_TOKEN"), options...)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /diia/callback", func(writer http.ResponseWriter, request *http.Request) {
		payload, callbackErr := client.Callbacks().Parse().WithRequest(request).Do(request.Context())
		var pending *pendingRequest

		if callbackErr == nil {
			pending, callbackErr = claimPendingRequestID(request.Context(), payload.RequestID)

			if callbackErr == nil && pending == nil {
				callbackErr = errors.New("pending request state is missing")
			}
		}

		if callbackErr == nil {
			var documents *diia.DocumentPackage
			documents, callbackErr = client.Callbacks().Process().WithPayload(payload).Do(request.Context())

			if callbackErr == nil && documents.MetadataSealFileName != "" {
				callbackErr = diia.ValidateMultisharingPackage(documents, pending.MandatorySharing...)
			}

			if callbackErr == nil {
				callbackErr = persistDocumentPackage(request.Context(), documents)
			}
		}

		if callbackErr != nil {
			_ = client.Callbacks().WriteFailure().
				WithWriter(writer).
				WithError(callbackErr).
				Do(request.Context())

			return
		}

		_ = client.Callbacks().WriteSuccess().WithWriter(writer).Do(request.Context())
	})

	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       time.Minute,
	}

	log.Fatal(server.ListenAndServe())
}

func paths(value string) []string {
	return strings.FieldsFunc(value, func(character rune) bool {
		return character == ',' || character == os.PathListSeparator
	})
}

type pendingRequest struct {
	MandatorySharing []diia.DocumentType
}

func claimPendingRequestID(_ context.Context, _ string) (*pendingRequest, error) {
	// Atomically claim an active ID and return its stored mandatory offer scopes.
	return nil, errors.New("pending request store is not configured")
}

func isPendingRequestID(_ context.Context, _ string) (bool, error) {
	// Implement a read-only lookup in the durable pending-request store.
	return false, nil
}

func isPendingOfferRequest(_ context.Context, _, _ string) (bool, error) {
	// Bind request ID and offer request ID to the same pending request.
	return false, nil
}

func persistDocumentPackage(_ context.Context, _ *diia.DocumentPackage) error {
	// Store verified output durably before acknowledging the callback.
	return errors.New("document persistence is not configured")
}
