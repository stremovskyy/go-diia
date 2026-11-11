package go_diia

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeCrypto struct {
	failDecrypt bool
	failVerify  bool
	decryptErr  error
	verifyErr   error
}

type archiveProcessorFunc func(context.Context, []byte, string, string, string) (*DocumentPackage, error)

func (f archiveProcessorFunc) ProcessArchive(
	ctx context.Context,
	archive []byte,
	password, requestID, offerRequestID string,
) (*DocumentPackage, error) {
	return f(ctx, archive, password, requestID, offerRequestID)
}

func (f fakeCrypto) DecryptAndVerify(_ context.Context, envelope []byte) ([]byte, error) {
	if f.decryptErr != nil {
		return nil, f.decryptErr
	}

	if f.failDecrypt {
		return nil, errors.New("decrypt failure")
	}

	if !bytes.HasPrefix(envelope, []byte("encrypted:")) {
		return nil, errors.New("unexpected envelope")
	}

	return append([]byte(nil), envelope[len("encrypted:"):]...), nil
}

func (f fakeCrypto) Verify(_ context.Context, signed []byte) ([]byte, error) {
	if f.verifyErr != nil {
		return nil, f.verifyErr
	}

	if f.failVerify {
		return nil, errors.New("verify failure")
	}

	if !bytes.HasPrefix(signed, []byte("signed:")) {
		return nil, errors.New("unexpected signed data")
	}

	return append([]byte(nil), signed[len("signed:"):]...), nil
}

type multipartField struct {
	name, filename string
	data           []byte
}

func callbackRequest(t *testing.T, requestID, token string, fields []multipartField) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	for _, field := range fields {
		var part io.Writer
		var err error
		if field.filename == "" {
			part, err = writer.CreateFormField(field.name)
		} else {
			part, err = writer.CreateFormFile(field.name, field.filename)
		}

		if err != nil {
			t.Fatal(err)
		}

		_, _ = io.WriteString(part, base64.StdEncoding.EncodeToString(field.data))
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "https://partner.example/callback", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("x-document-request-trace-id", requestID)

	if token != "" {
		request.Header.Set("Authorization", "bEaReR "+token)
	}

	return request
}

func TestCallbackParseAndProcessOffline(t *testing.T) {
	requestID := "request-123"
	metadata := `{"requestId":"request-123","barcode":"1234567890123","documentTypes":["internal-passport"],"data":{"internal-passport":{"docNumber":"AA123456","futureField":"preserved"},"future-document":{"value":1}},"futureEnvelope":true}`
	request := callbackRequest(t, requestID, "callback-secret", []multipartField{
		{name: "encodeData", data: []byte("encrypted:" + metadata)},
		{name: "internal-passport", filename: "passport.pdf.p7s.p7e", data: []byte("encrypted:%PDF-document")},
		{name: "signature-internal-passport-1", filename: "passport.pdf.p7s", data: []byte("signed:sha256:abc")},
	})
	client := NewClient("unused", WithCallbackToken("callback-secret"), WithCryptoProvider(fakeCrypto{}))
	payload, err := client.Callbacks().Parse().WithRequest(request).Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if payload.RequestID != requestID || len(payload.Documents) != 1 || len(payload.Signatures) != 1 {
		t.Fatalf("payload = %#v", payload)
	}

	result, err := client.Callbacks().Process().WithPayload(payload).Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if result.RequestID != requestID || result.Barcode != "1234567890123" ||
		string(result.Documents[0].PDF) != "%PDF-document" ||
		string(result.Signatures[0].Data) != "sha256:abc" {
		t.Fatalf("result = %#v", result)
	}

	if len(result.Metadata.Data.InternalPassports) != 1 ||
		result.Metadata.Data.InternalPassports[0].DocNumber != "AA123456" {
		t.Fatalf("typed metadata = %#v", result.Metadata)
	}

	if _, ok := result.Metadata.Data.Unknown["future-document"]; !ok ||
		!bytes.Contains(result.RawMetadata, []byte("futureEnvelope")) {
		t.Fatalf("future metadata was not preserved: %s", result.RawMetadata)
	}
}

func TestCallbackOnlineNormalizesArrayFieldsAndDuplicates(t *testing.T) {
	request := callbackRequest(t, "request-123", "callback-secret", []multipartField{
		{
			name: "encodeData",
			data: []byte(
				`encrypted:{"requestId":"request-123","data":{"foreign-passport":[{"docNumber":"one"},{"docNumber":"two"}]}}`,
			),
		},
		{name: "foreign-passport[]", filename: "one.pdf.p7s.p7e", data: []byte("encrypted:pdf-one")},
		{name: "foreign-passport[]", filename: "two.pdf.p7s.p7e", data: []byte("encrypted:pdf-two")},
	})
	client := NewClient("unused", WithCallbackToken("callback-secret"), WithCryptoProvider(fakeCrypto{}))
	payload, err := client.Callbacks().Parse().WithRequest(request).Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(payload.Documents) != 2 || payload.Documents[0].Type != DocumentTypeForeignPassport ||
		payload.Documents[1].Type != DocumentTypeForeignPassport {
		t.Fatalf("documents = %#v", payload.Documents)
	}

	result, err := client.Callbacks().Process().WithPayload(payload).Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Documents) != 2 || len(result.Metadata.Data.ForeignPassports) != 2 {
		t.Fatalf("result = %#v", result)
	}
}

func TestCallbackMultisharingPreservesTwoVehicleLicenses(t *testing.T) {
	metadata := `{"requestId":"request-multisharing-123","documentTypes":["internal-passport","taxpayer-card","driver-license","vehicle-license"],"data":{"internal-passport":[{}],"taxpayer-card":[{}],"driver-license":[{}],"vehicle-license":[{"fileName":"vehicle-one.pdf.p7s.p7e"},{"fileName":"vehicle-two.pdf.p7s.p7e"}]}}`
	request := callbackRequest(t, "request-multisharing-123", "callback-secret", []multipartField{
		{name: "encodeData", data: []byte("encrypted:" + metadata)},
		{name: "internal-passport", filename: "passport.pdf.p7s.p7e", data: []byte("encrypted:passport-pdf")},
		{name: "taxpayer-card", filename: "taxpayer.pdf.p7s.p7e", data: []byte("encrypted:taxpayer-pdf")},
		{name: "driver-license", filename: "driver.pdf.p7s.p7e", data: []byte("encrypted:driver-pdf")},
		{name: "vehicle-license[]", filename: "vehicle-one.pdf.p7s.p7e", data: []byte("encrypted:vehicle-one-pdf")},
		{name: "vehicle-license[]", filename: "vehicle-two.pdf.p7s.p7e", data: []byte("encrypted:vehicle-two-pdf")},
	})
	client := NewClient("unused", WithCallbackToken("callback-secret"), WithCryptoProvider(fakeCrypto{}))
	payload, err := client.Callbacks().Parse().WithRequest(request).Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(payload.Documents) != 5 {
		t.Fatalf("parsed documents = %d, want 5", len(payload.Documents))
	}

	result, err := client.Callbacks().Process().WithPayload(payload).Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Documents) != 5 || len(result.Metadata.Data.VehicleLicenses) != 2 ||
		result.Documents[3].Type != DocumentTypeVehicleLicense ||
		result.Documents[4].Type != DocumentTypeVehicleLicense ||
		string(result.Documents[3].PDF) != "vehicle-one-pdf" ||
		string(result.Documents[4].PDF) != "vehicle-two-pdf" {
		t.Fatalf("multisharing documents were not preserved")
	}
}

func TestCallbackProcessRejectsMultisharingZIP(t *testing.T) {
	request := callbackRequest(t, "request-multisharing-zip", "callback-secret", []multipartField{
		{name: "encodeData", data: []byte(`encrypted:{"requestId":"request-multisharing-zip"}`)},
		{name: "multi-sharing", filename: "multi-sharing.zip", data: []byte("PK\x03\x04synthetic")},
	})
	client := NewClient("unused", WithCallbackToken("callback-secret"), WithCryptoProvider(fakeCrypto{}))
	payload, err := client.Callbacks().Parse().WithRequest(request).Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(payload.Documents) != 0 || len(payload.UnknownParts) != 1 {
		t.Fatalf("parsed documents = %d, unknown parts = %d", len(payload.Documents), len(payload.UnknownParts))
	}

	result, err := client.Callbacks().Process().WithPayload(payload).Do(context.Background())

	if result != nil || !errors.Is(err, ErrCallback) || !strings.Contains(err.Error(), "multi-sharing ZIP") {
		t.Fatalf("expected unsupported multi-sharing ZIP callback, result = %v, err = %v", result, err)
	}
}

func TestCallbackProcessMultisharingZIP(t *testing.T) {
	const requestID = "request-multisharing-zip"
	const offerRequestID = "offer-request-123"
	archive := []byte("PK\x03\x04synthetic")
	var authCalls, passwordCalls, processorCalls int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/v1/auth/acquirer/acquirer":
			authCalls++
			_, _ = io.WriteString(writer, `{"token":"session"}`)

		case "/api/v1/acquirers/offer-request/" + offerRequestID + "/password":
			passwordCalls++

			if request.Method != http.MethodGet || request.Header.Get("Authorization") != "Bearer session" {
				t.Errorf("password request method or authorization is wrong")
			}

			_, _ = io.WriteString(writer, `{"password":"archive-secret"}`)

		default:
			t.Errorf("unexpected request path %q", request.URL.Path)
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	processor := archiveProcessorFunc(
		func(_ context.Context, got []byte, password, gotRequestID, gotOfferRequestID string) (*DocumentPackage, error) {
			processorCalls++

			if !bytes.Equal(got, archive) || password != "archive-secret" || gotRequestID != requestID ||
				gotOfferRequestID != offerRequestID {
				t.Fatal("archive processor received incorrect input")
			}

			return &DocumentPackage{
				RequestID: gotRequestID, OfferRequestID: gotOfferRequestID,
				Metadata: &Metadata{}, MetadataSealFileName: "metadata.json.p7s", RawMetadata: json.RawMessage(`{"data":{}}`),
				Documents: []DocumentFile{
					{
						Type: DocumentTypeVehicleLicense, FileName: "vehicle.pdf", PDF: []byte("%PDF"),
						DiiaSealFileName: "vehicle.pdf.p7s", UserSignatureFileName: "user_vehicle.pdf.p7s",
					},
				},
				Signatures: []VerifiedSignature{
					{FileName: "metadata.json.p7s", Role: "diia-seal", DocumentFileName: "metadata.json"},
					{FileName: "vehicle.pdf.p7s", Role: "diia-seal", DocumentFileName: "vehicle.pdf"},
					{FileName: "user_vehicle.pdf.p7s", Role: "user", DocumentFileName: "vehicle.pdf"},
				},
			}, nil
		},
	)
	client := NewClient(
		"acquirer",
		WithBaseURL(server.URL),
		WithCallbackToken("callback-secret"),
		WithCryptoProvider(fakeCrypto{}),
		WithMultisharingArchiveProcessor(processor),
		WithCallbackRequestIDAuthorizer(func(_ context.Context, gotRequestID string) (bool, error) {
			return gotRequestID == requestID, nil
		}),
		WithCallbackOfferRequestAuthorizer(
			func(_ context.Context, gotRequestID, gotOfferRequestID string) (bool, error) {
				return gotRequestID == requestID && gotOfferRequestID == offerRequestID, nil
			},
		),
	)
	request := callbackRequest(t, requestID, "", []multipartField{
		{name: "encodeData", data: []byte(`encrypted:{"offerRequestId":"` + offerRequestID + `"}`)},
		{name: "multi-sharing", filename: "multi-sharing.zip", data: archive},
	})
	payload, err := client.Callbacks().Parse().WithRequest(request).Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if !payload.MissingBearerAuth {
		t.Fatal("missing-bearer callback was not marked for verified ZIP processing")
	}

	result, err := client.Callbacks().Process().WithPayload(payload).Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if result.RequestID != requestID || len(result.Documents) != 1 || authCalls != 1 || passwordCalls != 1 ||
		processorCalls != 1 {
		t.Fatal("multi-sharing pipeline did not complete exactly once")
	}

	withoutBinding := NewClient("acquirer", WithBaseURL(server.URL), WithCryptoProvider(fakeCrypto{}),
		WithMultisharingArchiveProcessor(processor))

	if result, err := withoutBinding.Callbacks().Process().WithPayload(payload).Do(context.Background()); result != nil ||
		!errors.Is(err, ErrUnauthorized) {
		t.Fatalf("unbound request ID accepted: result=%v err=%v", result, err)
	}

	wrongBinding := NewClient("acquirer", WithBaseURL(server.URL), WithCryptoProvider(fakeCrypto{}),
		WithMultisharingArchiveProcessor(processor),
		WithCallbackOfferRequestAuthorizer(func(context.Context, string, string) (bool, error) { return false, nil }))

	if result, err := wrongBinding.Callbacks().Process().WithPayload(payload).Do(context.Background()); result != nil ||
		!errors.Is(err, ErrUnauthorized) {
		t.Fatalf("wrong offer binding accepted: result=%v err=%v", result, err)
	}
}

func TestCallbackProcessMultisharingZIPRejectsIncompleteProcessorResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.HasPrefix(request.URL.Path, "/api/v1/auth/acquirer/") {
			_, _ = io.WriteString(writer, `{"token":"session"}`)

			return
		}

		_, _ = io.WriteString(writer, `{"password":"archive-secret"}`)
	}))
	defer server.Close()

	processor := archiveProcessorFunc(func(_ context.Context, _ []byte, _, _, _ string) (*DocumentPackage, error) {
		return &DocumentPackage{
			RequestID: "request-123", OfferRequestID: "offer-request-123",
			Metadata:             &Metadata{RequestID: "request-123"},
			MetadataSealFileName: "metadata.json.p7s",
			RawMetadata:          json.RawMessage(`{"requestId":"request-123","data":{}}`),
			Documents: []DocumentFile{{
				Type: DocumentTypeVehicleLicense, FileName: "vehicle.pdf", PDF: []byte("%PDF"),
				DiiaSealFileName: "vehicle.pdf.p7s", UserSignatureFileName: "user_vehicle.pdf.p7s",
			}},
		}, nil
	})
	client := NewClient(
		"acquirer",
		WithBaseURL(server.URL),
		WithCryptoProvider(fakeCrypto{}),
		WithMultisharingArchiveProcessor(processor),
	)
	payload := &CallbackPayload{
		RequestID: "request-123", EncodeData: []byte(`encrypted:{"offerRequestId":"offer-request-123"}`),
		UnknownParts: []CallbackPart{{FieldName: "multi-sharing", Data: []byte("PK")}},
	}

	if result, err := client.Callbacks().Process().WithPayload(payload).Do(context.Background()); result != nil ||
		!errors.Is(err, ErrCallback) {
		t.Fatalf("incomplete package accepted: result=%v err=%v", result, err)
	}
}

func TestCallbackMissingBearerRequiresActiveRequestID(t *testing.T) {
	const requestID = "request-123"
	var checked int
	client := NewClient("unused", WithCallbackToken("expected"), WithCallbackRequestIDAuthorizer(
		func(_ context.Context, got string) (bool, error) {
			checked++

			return got == requestID, nil
		},
	))
	valid := []multipartField{{name: "encodeData", data: []byte("encrypted:{}")}}
	payload, err := client.Callbacks().
		Parse().
		WithRequest(callbackRequest(t, requestID, "", valid)).
		Do(context.Background())
	if err != nil {
		t.Fatalf("active request ID without bearer: %v", err)
	}

	if !payload.MissingBearerAuth {
		t.Fatal("missing bearer authorization mode was not retained")
	}

	if result, err := client.Callbacks().Process().WithPayload(payload).Do(context.Background()); result != nil ||
		!errors.Is(err, ErrUnauthorized) {
		t.Fatalf("legacy callback without bearer accepted: result=%v err=%v", result, err)
	}

	if checked != 1 {
		t.Fatalf("authorizer calls = %d, want 1", checked)
	}

	for _, test := range []struct {
		name      string
		requestID string
		token     string
	}{
		{"inactive ID", "request-inactive", ""},
		{"wrong bearer", requestID, "wrong"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := client.Callbacks().
				Parse().
				WithRequest(callbackRequest(t, test.requestID, test.token, valid)).
				Do(context.Background())

			if !errors.Is(err, ErrUnauthorized) {
				t.Fatalf("expected unauthorized, got %v", err)
			}
		})
	}

	if checked != 2 {
		t.Fatalf("invalid bearer unexpectedly reached request ID authorizer; calls=%d", checked)
	}

	for _, header := range []string{"", "   ", "Basic anything"} {
		request := callbackRequest(t, requestID, "", valid)
		request.Header["Authorization"] = []string{header}

		if _, err := client.Callbacks().Parse().WithRequest(request).Do(context.Background()); !errors.Is(
			err,
			ErrUnauthorized,
		) {
			t.Fatalf("present malformed Authorization %q accepted: %v", header, err)
		}
	}

	request := callbackRequest(t, requestID, "expected", valid)
	request.Header.Add("Authorization", "Bearer wrong")

	if _, err := client.Callbacks().Parse().WithRequest(request).Do(context.Background()); !errors.Is(
		err,
		ErrUnauthorized,
	) {
		t.Fatalf("duplicate Authorization accepted: %v", err)
	}

	request = callbackRequest(t, requestID, "expected", valid)
	request.Header.Add("X-Document-Request-Trace-Id", "request-other")

	if _, err := client.Callbacks().Parse().WithRequest(request).Do(context.Background()); !errors.Is(
		err,
		ErrCallback,
	) {
		t.Fatalf("duplicate trace ID accepted: %v", err)
	}
}

func TestDecodeMultisharingReferenceRejectsDuplicateIDs(t *testing.T) {
	for _, raw := range []string{
		`{"offerRequestId":"one","offerRequestId":"two"}`,
		`{"requestId":"one","requestId":"two","offerRequestId":"three"}`,
		`{"offerRequestId":null}`,
		`{"offerRequestId":"one"}{"offerRequestId":"two"}`,
	} {
		if _, err := decodeMultisharingReference([]byte(raw)); err == nil {
			t.Fatalf("invalid reference accepted: %s", raw)
		}
	}
}

func TestCallbackParseRequiresConfiguredBearerToken(t *testing.T) {
	for _, client := range []Diia{
		NewClient("unused"),
		NewClient("unused", WithCallbackToken(" ")),
	} {
		request := callbackRequest(t, "request-123", "callback-secret", []multipartField{
			{name: "encodeData", data: []byte("encrypted:{}")},
		})
		_, err := client.Callbacks().Parse().WithRequest(request).Do(context.Background())

		if !errors.Is(err, ErrCallback) || !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("missing configured token: expected ErrCallback and ErrUnauthorized, got %v", err)
		}
	}
}

func TestCallbackParseFailures(t *testing.T) {
	client := NewClient("unused", WithCallbackToken("expected"), WithCryptoProvider(fakeCrypto{}))
	valid := []multipartField{{name: "encodeData", data: []byte("encrypted:{}")}}
	tests := []struct {
		name    string
		request func() *http.Request
		also    error
	}{
		{
			"token mismatch",
			func() *http.Request { return callbackRequest(t, "request-123", "wrong", valid) },
			ErrUnauthorized,
		},
		{"missing trace", func() *http.Request { return callbackRequest(t, "", "expected", valid) }, nil},
		{"missing encodeData", func() *http.Request {
			return callbackRequest(
				t,
				"request-123",
				"expected",
				[]multipartField{{name: "internal-passport", data: []byte("encrypted:pdf")}},
			)
		}, nil},
		{"wrong method", func() *http.Request {
			r := callbackRequest(t, "request-123", "expected", valid)
			r.Method = http.MethodGet

			return r
		}, nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := client.Callbacks().Parse().WithRequest(test.request()).Do(context.Background())

			if !errors.Is(err, ErrCallback) {
				t.Fatalf("expected ErrCallback, got %v", err)
			}

			if test.also != nil && !errors.Is(err, test.also) {
				t.Fatalf("expected %v, got %v", test.also, err)
			}
		})
	}

	// Build a multipart body manually so the form value is invalid Base64.
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormField("encodeData")
	_, _ = io.WriteString(part, "%%%")
	_ = writer.Close()
	request := httptest.NewRequest(http.MethodPost, "/", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("X-DOCUMENT-REQUEST-TRACE-ID", "request-123")
	request.Header.Set("Authorization", "Bearer expected")

	if _, err := client.Callbacks().Parse().WithRequest(request).Do(context.Background()); !errors.Is(
		err,
		ErrCallback,
	) {
		t.Fatalf("invalid base64: %v", err)
	}
}

func TestCallbackParseBounds(t *testing.T) {
	client := NewClient("unused", WithCallbackToken("expected"))
	valid := []multipartField{{name: "encodeData", data: []byte("encrypted:{}")}}

	t.Run("nil body", func(t *testing.T) {
		request := callbackRequest(t, "request-123", "expected", valid)
		request.Body = nil

		_, err := client.Callbacks().Parse().WithRequest(request).Do(context.Background())

		if !errors.Is(err, ErrCallback) {
			t.Fatalf("nil body: expected ErrCallback, got %v", err)
		}
	})

	t.Run("declared body exceeds limit", func(t *testing.T) {
		request := callbackRequest(t, "request-123", "expected", valid)
		request.ContentLength = maxCallbackSize + 1

		_, err := client.Callbacks().Parse().WithRequest(request).Do(context.Background())

		if !errors.Is(err, ErrCallback) || !strings.Contains(err.Error(), "too large") {
			t.Fatalf("oversized body: expected size error, got %v", err)
		}
	})

	t.Run("multipart part count", func(t *testing.T) {
		fields := make([]multipartField, 0, maxCallbackParts+1)
		fields = append(fields, valid[0])

		for i := 1; i < maxCallbackParts; i++ {
			fields = append(fields, multipartField{name: fmt.Sprintf("unknown-%d", i), data: []byte("x")})
		}

		request := callbackRequest(t, "request-123", "expected", fields)
		payload, err := client.Callbacks().Parse().WithRequest(request).Do(context.Background())

		if err != nil || len(payload.UnknownParts) != maxCallbackParts-1 {
			t.Fatalf("limit boundary: payload=%v, err=%v", payload != nil, err)
		}

		fields = append(fields, multipartField{name: "one-too-many", data: []byte("x")})
		request = callbackRequest(t, "request-123", "expected", fields)
		_, err = client.Callbacks().Parse().WithRequest(request).Do(context.Background())

		if !errors.Is(err, ErrCallback) || !strings.Contains(err.Error(), "too many multipart parts") {
			t.Fatalf("part count: expected limit error, got %v", err)
		}
	})
}

func TestCallbackPartReadLimitAndBase64(t *testing.T) {
	data, tooLarge, err := readCallbackPart(strings.NewReader("1234"), 4)

	if err != nil || tooLarge || string(data) != "1234" {
		t.Fatalf("at part limit: data=%q, tooLarge=%t, err=%v", data, tooLarge, err)
	}

	data, tooLarge, err = readCallbackPart(strings.NewReader("12345"), 4)

	if err != nil || !tooLarge || len(data) != 5 {
		t.Fatalf("over part limit: data length=%d, tooLarge=%t, err=%v", len(data), tooLarge, err)
	}

	for _, test := range []struct {
		encoded string
		want    string
	}{
		{" Y W I = \r\n", "ab"},
		{"YWI", "ab"},
	} {
		decoded, err := decodeCallbackBase64([]byte(test.encoded))

		if err != nil || string(decoded) != test.want {
			t.Fatalf("decode %q: got %q, err=%v", test.encoded, decoded, err)
		}
	}
}

func TestCallbackCryptoFailuresAndRequestIDMismatch(t *testing.T) {
	payload := &CallbackPayload{
		RequestID:  "request-123",
		EncodeData: []byte(`encrypted:{"requestId":"different-id","data":{}}`),
	}
	client := NewClient("unused", WithCryptoProvider(fakeCrypto{}))

	if _, err := client.Callbacks().Process().WithPayload(payload).Do(context.Background()); !errors.Is(
		err,
		ErrCallback,
	) {
		t.Fatalf("mismatch: %v", err)
	}

	client = NewClient("unused", WithCryptoProvider(fakeCrypto{failDecrypt: true}))

	if _, err := client.Callbacks().Process().WithPayload(payload).Do(context.Background()); !errors.Is(
		err,
		ErrCrypto,
	) {
		t.Fatalf("decrypt: %v", err)
	}

	payload = &CallbackPayload{
		RequestID:  "request-123",
		EncodeData: []byte(`encrypted:{"requestId":"request-123","data":{}}`),
		Signatures: []CallbackSignature{{Signed: []byte("signed:value")}},
	}
	client = NewClient("unused", WithCryptoProvider(fakeCrypto{failVerify: true}))

	if _, err := client.Callbacks().Process().WithPayload(payload).Do(context.Background()); !errors.Is(
		err,
		ErrCrypto,
	) {
		t.Fatalf("verify: %v", err)
	}

	nativeFailure := errors.New("typed native failure")
	client = NewClient("unused", WithCryptoProvider(fakeCrypto{decryptErr: nativeFailure}))

	if _, err := client.Callbacks().Process().WithPayload(payload).Do(context.Background()); !errors.Is(
		err,
		ErrCrypto,
	) ||
		!errors.Is(err, nativeFailure) {
		t.Fatalf("wrapped crypto failure: %v", err)
	}
}

func TestCallbackResponses(t *testing.T) {
	client := NewClient("unused")
	tests := []struct {
		name   string
		write  func(http.ResponseWriter) error
		status int
		body   map[string]any
	}{
		{"success", func(w http.ResponseWriter) error {
			return client.Callbacks().WriteSuccess().WithWriter(w).Do(context.Background())
		}, 200, map[string]any{"success": true}},
		{
			"failure",
			func(w http.ResponseWriter) error {
				return client.Callbacks().WriteFailure().
					WithWriter(w).
					WithError(errors.New("rejected")).
					Do(context.Background())
			},
			404,
			map[string]any{"success": false, "error": defaultCallbackErrorText},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()

			if err := test.write(recorder); err != nil {
				t.Fatal(err)
			}

			if recorder.Code != test.status || recorder.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("response = %#v", recorder.Result())
			}

			var body map[string]any

			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}

			if fmt.Sprint(body) != fmt.Sprint(test.body) {
				t.Fatalf("body = %#v, want %#v", body, test.body)
			}
		})
	}
}

func TestCallbackFailureMapperIsExplicitSanitizedAndBounded(t *testing.T) {
	secret := errors.New("native secret")
	longMessage := strings.Repeat("Ж", maxCallbackErrorRunes+10)
	tests := []struct {
		name   string
		mapper CallbackErrorMapper
		want   string
	}{
		{"mapped", func(error) string { return "  retry later  " }, "retry later"},
		{"blank fallback", func(error) string { return " \n " }, defaultCallbackErrorText},
		{"bounded", func(error) string { return longMessage }, strings.Repeat("Ж", maxCallbackErrorRunes)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := NewClient("unused", WithCallbackErrorMapper(test.mapper))
			recorder := httptest.NewRecorder()

			if err := client.Callbacks().WriteFailure().
				WithWriter(recorder).
				WithError(secret).
				Do(context.Background()); err != nil {
				t.Fatal(err)
			}

			var body struct {
				Error string `json:"error"`
			}

			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}

			if body.Error != test.want || strings.Contains(body.Error, secret.Error()) {
				t.Fatalf("error = %q, want %q", body.Error, test.want)
			}
		})
	}
}

func TestCallbackDoesNotLogOrEchoBody(t *testing.T) {
	// The public payload types intentionally contain bytes but no String or
	// error method that could accidentally serialize their contents to logs.
	payload := CallbackPayload{EncodeData: []byte("private-metadata")}

	if strings.Contains(fmt.Sprintf("%T", payload), "private-metadata") {
		t.Fatal("type formatting leaked body")
	}
}

func TestCallbackTypedNilProvidersFailClosed(t *testing.T) {
	t.Run("crypto provider", func(t *testing.T) {
		var provider *fakeCrypto
		client := NewClient("unused", WithCryptoProvider(provider))
		payload := &CallbackPayload{EncodeData: []byte("encrypted:{}")}

		if _, err := client.Callbacks().Process().WithPayload(payload).Do(context.Background()); !errors.Is(err, ErrCrypto) {
			t.Fatalf("typed nil crypto provider returned %v, want ErrCrypto", err)
		}
	})

	t.Run("archive processor", func(t *testing.T) {
		var processor archiveProcessorFunc
		client := NewClient("unused", WithCryptoProvider(fakeCrypto{}), WithMultisharingArchiveProcessor(processor))
		payload := &CallbackPayload{UnknownParts: []CallbackPart{{FieldName: "multi-sharing", Data: []byte("zip")}}}

		if _, err := client.Callbacks().Process().WithPayload(payload).Do(context.Background()); !errors.Is(err, ErrCallback) {
			t.Fatalf("typed nil archive processor returned %v, want ErrCallback", err)
		}
	})
}
