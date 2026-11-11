package go_diia

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
)

const (
	maxCallbackSize          = 256 << 20
	maxPartSize              = 96 << 20
	maxCallbackParts         = 1024
	maxEncodedArchivePart    = 24 << 20
	maxCallbackErrorRunes    = 512
	defaultCallbackErrorText = "callback processing failed"
)

// CryptoProvider isolates the SDK from a concrete qualified-signature vendor.
// A provider typically maps DecryptAndVerify to IIT CtxDevelopData followed by
// VerifyDataInternalSpecific, and Verify to VerifyDataInternalSpecific.
type CryptoProvider interface {
	DecryptAndVerify(ctx context.Context, envelope []byte) ([]byte, error)
	Verify(ctx context.Context, signed []byte) ([]byte, error)
}

// MultisharingArchiveProcessor opens a password-protected multi-sharing ZIP,
// verifies its exact metadata and PDF bytes against their signatures,
// and returns only a complete, authenticated document package.
// It lives in the optional IIT module so the core SDK has no ZIP or native
// cryptography dependency.
type MultisharingArchiveProcessor interface {
	ProcessArchive(
		ctx context.Context,
		archive []byte,
		password, requestID, offerRequestID string,
	) (*DocumentPackage, error)
}

type callbackService struct{ client *client }

func (s *callbackService) parse(ctx context.Context, request *http.Request) (*CallbackPayload, error) {
	if request == nil {
		return nil, callbackError("request is required", nil)
	}

	if request.Method != http.MethodPost {
		return nil, callbackError("callback method must be POST", nil)
	}

	if request.Body == nil {
		return nil, callbackError("callback body is required", nil)
	}

	traceValues, _ := callbackHeaderValues(request.Header, "X-Document-Request-Trace-Id")

	if len(traceValues) == 0 || strings.TrimSpace(traceValues[0]) == "" {
		return nil, callbackError("X-Document-Request-Trace-Id is required", nil)
	}

	if len(traceValues) != 1 {
		return nil, callbackError("X-Document-Request-Trace-Id must occur once", nil)
	}

	requestID := traceValues[0]

	if err := validateRequestID(requestID, false); err != nil {
		return nil, callbackError("invalid X-Document-Request-Trace-Id", err)
	}

	authorization, authorizationPresent := callbackHeaderValues(request.Header, "Authorization")

	if err := s.authorize(ctx, authorization, authorizationPresent, requestID); err != nil {
		return nil, err
	}

	if request.ContentLength > maxCallbackSize {
		return nil, callbackError("callback body is too large", nil)
	}

	mediaType, parameters, err := mime.ParseMediaType(request.Header.Get("Content-Type"))

	if err != nil || !strings.EqualFold(mediaType, "multipart/form-data") || parameters["boundary"] == "" {
		return nil, callbackError("Content-Type must be multipart/form-data with a boundary", err)
	}

	reader := multipart.NewReader(io.LimitReader(request.Body, maxCallbackSize+1), parameters["boundary"])
	payload := &CallbackPayload{RequestID: requestID, MissingBearerAuth: !authorizationPresent}
	var total int64
	partCount := 0

	for {
		if err := ctx.Err(); err != nil {
			return nil, callbackError("parse callback", err)
		}

		part, nextErr := reader.NextPart()

		if errors.Is(nextErr, io.EOF) {
			break
		}

		if nextErr != nil {
			return nil, callbackError("read multipart part", nextErr)
		}

		partCount++

		if partCount > maxCallbackParts {
			return nil, callbackError("too many multipart parts", nil)
		}

		fieldName := strings.TrimSpace(part.FormName())

		if fieldName == "" {
			return nil, callbackError("multipart field name is missing", nil)
		}

		partLimit := int64(maxPartSize)

		if isMultisharingField(fieldName) {
			partLimit = maxEncodedArchivePart
		}

		data, tooLarge, readErr := readCallbackPart(part, partLimit)

		if readErr != nil {
			return nil, callbackError("read multipart content", readErr)
		}

		if tooLarge {
			return nil, callbackError("multipart part is too large", nil)
		}

		if err := part.Close(); err != nil {
			return nil, callbackError("close multipart content", err)
		}

		total += int64(len(data))

		if total > maxCallbackSize {
			return nil, callbackError("callback body is too large", nil)
		}

		fileName := part.FileName()

		decoded, decodeErr := decodeCallbackBase64(data)
		if decodeErr != nil {
			return nil, callbackError("invalid base64 in multipart field "+fieldName, decodeErr)
		}

		switch {
		case fieldName == "encodeData":
			if len(payload.EncodeData) != 0 {
				return nil, callbackError("encodeData must occur once", nil)
			}

			payload.EncodeData = decoded

		case strings.HasPrefix(strings.ToLower(fieldName), "signature-"):
			payload.Signatures = append(
				payload.Signatures,
				CallbackSignature{FieldName: fieldName, FileName: fileName, Signed: decoded},
			)

		default:
			normalized := DocumentType(strings.TrimSuffix(fieldName, "[]"))
			if normalized.Valid() {
				payload.Documents = append(
					payload.Documents,
					CallbackDocument{Type: normalized, FieldName: fieldName, FileName: fileName, Envelope: decoded},
				)
			} else {
				payload.UnknownParts = append(payload.UnknownParts, CallbackPart{FieldName: fieldName, FileName: fileName, Data: decoded})
			}
		}
	}

	if len(payload.EncodeData) == 0 {
		return nil, callbackError("encodeData is required", nil)
	}

	return payload, nil
}

func readCallbackPart(part io.Reader, limit int64) ([]byte, bool, error) {
	data, err := io.ReadAll(io.LimitReader(part, limit+1))
	if err != nil {
		return nil, false, err
	}

	return data, int64(len(data)) > limit, nil
}

func isMultisharingField(name string) bool {
	return strings.EqualFold(strings.TrimSuffix(strings.TrimSpace(name), "[]"), "multi-sharing")
}

type multisharingReference struct {
	RequestID      string
	OfferRequestID string
}

func decodeMultisharingReference(data []byte) (multisharingReference, error) {
	var reference multisharingReference
	decoder := json.NewDecoder(bytes.NewReader(data))
	start, err := decoder.Token()

	if err != nil || start != json.Delim('{') {
		return reference, errors.New("expected a JSON object")
	}

	seen := make(map[string]bool)

	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return reference, err
		}

		key, ok := keyToken.(string)

		if !ok || seen[key] {
			return reference, errors.New("duplicate or invalid JSON key")
		}

		seen[key] = true
		var value json.RawMessage

		if err := decoder.Decode(&value); err != nil {
			return reference, err
		}

		if key == "requestId" || key == "offerRequestId" {
			if len(bytes.TrimSpace(value)) == 0 || bytes.TrimSpace(value)[0] != '"' {
				return reference, errors.New("request identifier is not a string")
			}

			var identifier string

			if err := json.Unmarshal(value, &identifier); err != nil {
				return reference, err
			}

			if key == "requestId" {
				reference.RequestID = identifier
			} else {
				reference.OfferRequestID = identifier
			}
		}
	}

	end, err := decoder.Token()

	if err != nil || end != json.Delim('}') {
		return reference, errors.New("invalid JSON object")
	}

	var extra any

	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return reference, errors.New("trailing JSON content")
	}

	return reference, nil
}

func (s *callbackService) process(ctx context.Context, payload *CallbackPayload) (*DocumentPackage, error) {
	if payload == nil {
		return nil, callbackError("payload is required", nil)
	}

	var multisharingArchive []byte

	for _, part := range payload.UnknownParts {
		if isMultisharingField(part.FieldName) {
			if multisharingArchive != nil {
				return nil, callbackError("multi-sharing ZIP must occur once", nil)
			}

			multisharingArchive = part.Data
		}
	}

	if multisharingArchive != nil {
		return s.processMultisharing(ctx, payload, multisharingArchive)
	}

	if payload.MissingBearerAuth {
		return nil, callbackError(
			"callback without bearer requires verified multi-sharing ZIP processing",
			ErrUnauthorized,
		)
	}

	if s.client.cfg.crypto == nil {
		return nil, fmt.Errorf("%w: crypto provider is not configured", ErrCrypto)
	}

	if len(payload.EncodeData) == 0 {
		return nil, callbackError("encodeData is required", nil)
	}

	rawMetadata, err := s.client.cfg.crypto.DecryptAndVerify(ctx, payload.EncodeData)
	if err != nil {
		return nil, fmt.Errorf("%w: decrypt metadata: %w", ErrCrypto, err)
	}

	var metadata Metadata

	if err := json.Unmarshal(rawMetadata, &metadata); err != nil {
		return nil, fmt.Errorf("%w: metadata JSON: %w", ErrDecode, err)
	}

	if metadata.RequestID != "" && payload.RequestID != "" && metadata.RequestID != payload.RequestID {
		return nil, callbackError("metadata requestId does not match trace header", nil)
	}

	requestID := payload.RequestID

	if requestID == "" {
		requestID = metadata.RequestID
	}

	result := &DocumentPackage{
		RequestID: requestID, OfferRequestID: metadata.OfferRequestID, Barcode: metadata.Barcode,
		Metadata: &metadata, RawMetadata: append(json.RawMessage(nil), rawMetadata...),
		Documents:  make([]DocumentFile, 0, len(payload.Documents)),
		Signatures: make([]VerifiedSignature, 0, len(payload.Signatures)),
	}

	for _, document := range payload.Documents {
		pdf, decryptErr := s.client.cfg.crypto.DecryptAndVerify(ctx, document.Envelope)
		if decryptErr != nil {
			return nil, fmt.Errorf("%w: decrypt document %s: %w", ErrCrypto, document.Type, decryptErr)
		}

		result.Documents = append(result.Documents, DocumentFile{
			Type: document.Type, FieldName: document.FieldName, FileName: document.FileName,
			PDF: append([]byte(nil), pdf...),
		})
	}

	for _, signature := range payload.Signatures {
		verified, verifyErr := s.client.cfg.crypto.Verify(ctx, signature.Signed)
		if verifyErr != nil {
			return nil, fmt.Errorf("%w: verify signature part: %w", ErrCrypto, verifyErr)
		}

		result.Signatures = append(result.Signatures, VerifiedSignature{
			FieldName: signature.FieldName, FileName: signature.FileName,
			Data: append([]byte(nil), verified...),
		})
	}

	return result, nil
}

func (s *callbackService) processMultisharing(
	ctx context.Context,
	payload *CallbackPayload,
	archive []byte,
) (*DocumentPackage, error) {
	processor := s.client.cfg.multisharingArchive
	if processor == nil {
		return nil, callbackError("multi-sharing ZIP callback is not supported without an archive processor", nil)
	}

	if len(payload.Documents) != 0 || len(payload.Signatures) != 0 || len(payload.UnknownParts) != 1 {
		return nil, callbackError("multi-sharing ZIP must be the only document part", nil)
	}

	if len(archive) == 0 {
		return nil, callbackError("multi-sharing ZIP is empty", nil)
	}

	if s.client.cfg.crypto == nil {
		return nil, fmt.Errorf("%w: crypto provider is not configured", ErrCrypto)
	}

	if len(payload.EncodeData) == 0 {
		return nil, callbackError("encodeData is required", nil)
	}

	decoded, err := s.client.cfg.crypto.DecryptAndVerify(ctx, payload.EncodeData)
	if err != nil {
		return nil, fmt.Errorf("%w: decrypt multi-sharing encodeData: %w", ErrCrypto, err)
	}

	reference, err := decodeMultisharingReference(decoded)
	if err != nil {
		return nil, fmt.Errorf("%w: multi-sharing encodeData JSON: %w", ErrDecode, err)
	}

	if reference.RequestID != "" && reference.RequestID != payload.RequestID {
		return nil, callbackError("multi-sharing requestId does not match trace header", nil)
	}

	if reference.OfferRequestID == "" {
		return nil, callbackError("multi-sharing offerRequestId is required", nil)
	}

	password, err := s.client.sharing.OfferRequestPassword(ctx, reference.OfferRequestID)
	if err != nil {
		return nil, fmt.Errorf("%w: fetch multi-sharing archive password: %w", ErrCallback, err)
	}

	result, err := processor.ProcessArchive(ctx, archive, password, payload.RequestID, reference.OfferRequestID)
	if err != nil {
		return nil, fmt.Errorf("%w: process multi-sharing ZIP: %w", ErrCrypto, err)
	}

	if result == nil || result.Metadata == nil || len(result.RawMetadata) == 0 || len(result.Documents) == 0 ||
		result.MetadataSealFileName == "" || len(result.Signatures) != 1+2*len(result.Documents) {
		return nil, callbackError("multi-sharing ZIP processor returned an incomplete package", nil)
	}

	if result.RequestID != payload.RequestID || result.OfferRequestID != reference.OfferRequestID {
		return nil, callbackError("multi-sharing ZIP processor returned mismatched identifiers", nil)
	}

	if result.Metadata.RequestID != "" && result.Metadata.RequestID != payload.RequestID {
		return nil, callbackError("multi-sharing ZIP metadata requestId does not match trace header", nil)
	}

	if result.Metadata.OfferRequestID != "" && result.Metadata.OfferRequestID != reference.OfferRequestID {
		return nil, callbackError("multi-sharing ZIP metadata offerRequestId does not match encodeData", nil)
	}

	if reference.RequestID == "" && result.Metadata.RequestID == "" {
		bind := s.client.cfg.callbackOfferRequestAuthorizer
		if bind == nil {
			return nil, callbackError("multi-sharing request ID cannot be bound to offerRequestId", ErrUnauthorized)
		}

		bound, bindErr := bind(ctx, payload.RequestID, reference.OfferRequestID)

		if bindErr != nil || !bound {
			return nil, callbackError("multi-sharing request ID is not bound to offerRequestId", ErrUnauthorized)
		}
	}

	type signatureLink struct{ role, document, file string }
	links := make(map[signatureLink]bool, len(result.Signatures))

	for _, signature := range result.Signatures {
		link := signatureLink{signature.Role, signature.DocumentFileName, signature.FileName}

		if links[link] {
			return nil, callbackError("multi-sharing ZIP processor returned duplicate signature links", nil)
		}

		links[link] = true
	}

	if !links[signatureLink{"diia-seal", "metadata.json", result.MetadataSealFileName}] {
		return nil, callbackError("multi-sharing ZIP processor returned no metadata seal link", nil)
	}

	documentNames := make(map[string]bool, len(result.Documents))

	for _, document := range result.Documents {
		if !document.Type.Valid() || len(document.PDF) == 0 || document.FileName == "" ||
			document.DiiaSealFileName == "" || document.UserSignatureFileName == "" {
			return nil, callbackError("multi-sharing ZIP processor returned an incomplete document", nil)
		}

		if documentNames[document.FileName] ||
			!links[signatureLink{"diia-seal", document.FileName, document.DiiaSealFileName}] ||
			!links[signatureLink{"user", document.FileName, document.UserSignatureFileName}] {
			return nil, callbackError("multi-sharing ZIP processor returned inconsistent signature links", nil)
		}

		documentNames[document.FileName] = true
	}

	return result, nil
}

func (s *callbackService) writeSuccess(writer http.ResponseWriter) error {
	return writeCallbackResponse(writer, http.StatusOK, map[string]any{"success": true})
}

func (s *callbackService) writeFailure(writer http.ResponseWriter, err error) error {
	message := defaultCallbackErrorText

	if mapper := s.client.cfg.callbackErrorMapper; mapper != nil {
		message = sanitizeCallbackError(mapper(err))
	}

	return writeCallbackResponse(writer, http.StatusNotFound, map[string]any{"success": false, "error": message})
}

func sanitizeCallbackError(message string) string {
	message = strings.TrimSpace(message)

	if message == "" {
		return defaultCallbackErrorText
	}

	runes := []rune(message)

	if len(runes) > maxCallbackErrorRunes {
		message = string(runes[:maxCallbackErrorRunes])
	}

	return message
}

func callbackHeaderValues(headers http.Header, target string) ([]string, bool) {
	var values []string
	present := false

	for name, entries := range headers {
		if strings.EqualFold(name, target) {
			present = true
			values = append(values, entries...)
		}
	}

	return values, present
}

func (s *callbackService) authorize(ctx context.Context, headers []string, present bool, requestID string) error {
	expected := s.client.cfg.callbackToken

	if !present {
		if authorizer := s.client.cfg.callbackRequestIDAuthorizer; authorizer != nil {
			allowed, err := authorizer(ctx, requestID)

			if err != nil || !allowed {
				return callbackError("callback request ID is not active", ErrUnauthorized)
			}

			return nil
		}
	}

	if len(headers) != 1 {
		return callbackError("callback bearer token mismatch", ErrUnauthorized)
	}

	if expected == "" {
		return callbackError("callback bearer token is not configured", ErrUnauthorized)
	}

	parts := strings.Fields(headers[0])

	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") ||
		subtle.ConstantTimeCompare([]byte(parts[1]), []byte(expected)) != 1 {
		return callbackError("callback bearer token mismatch", ErrUnauthorized)
	}

	return nil
}

func decodeCallbackBase64(data []byte) ([]byte, error) {
	// Compact in place: the encoded buffer is local to the parser and is not
	// retained after decoding.
	compact := data[:0]

	for _, value := range data {
		switch value {
		case ' ', '\n', '\r', '\t':
			continue
		default:
			compact = append(compact, value)
		}
	}

	if len(compact) == 0 {
		return nil, errors.New("empty value")
	}

	// Raw decoding may produce more bytes than padded DecodedLen for input
	// without trailing '='; its bound accommodates both formats.
	decoded := make([]byte, base64.RawStdEncoding.DecodedLen(len(compact)))
	n, err := base64.StdEncoding.Decode(decoded, compact)
	if err == nil {
		return decoded[:n], nil
	}

	n, rawErr := base64.RawStdEncoding.Decode(decoded, compact)
	if rawErr == nil {
		return decoded[:n], nil
	}

	return nil, err
}

func callbackError(message string, cause error) error {
	if cause == nil {
		return fmt.Errorf("%w: %s", ErrCallback, message)
	}

	return fmt.Errorf("%w: %s: %w", ErrCallback, message, cause)
}

func writeCallbackResponse(writer http.ResponseWriter, status int, payload any) error {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)

	if err := json.NewEncoder(writer).Encode(payload); err != nil {
		return callbackError("write callback response", err)
	}

	return nil
}
