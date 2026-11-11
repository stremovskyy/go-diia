// decrypt-capture inspects one saved Diia sandbox callback from webhook.site.
// It is an offline diagnostic, not an HTTP callback receiver.
package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	diia "github.com/stremovskyy/go-diia"
	diiaiit "github.com/stremovskyy/go-diia/iit"
)

const (
	maxCaptureJSON       = 4 << 20
	maxEncodedPart       = 96 << 20
	maxArchiveBytes      = 16 << 20
	maxArchiveEntries    = 128
	maxArchiveEntryBytes = 32 << 20
	maxArchiveTotalBytes = 128 << 20
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type capturedFile struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Filename string `json:"filename"`
}

type capture struct {
	Method  string                  `json:"method"`
	Headers map[string][]string     `json:"headers"`
	Request map[string]string       `json:"request"`
	Files   map[string]capturedFile `json:"files"`
}

type archiveSummary struct {
	Entries           int
	EncryptedEntries  int
	PDFEntries        int
	UnknownPDFEntries int
	PDFTypes          map[diia.DocumentType]int
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "decrypt-capture:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 2 || (os.Args[1] != "inspect" && os.Args[1] != "decrypt") {
		return errors.New("usage: go run ./examples/decrypt-capture inspect|decrypt")
	}

	tokenID := strings.TrimSpace(os.Getenv("WEBHOOK_SITE_TOKEN"))
	captureID := strings.TrimSpace(os.Getenv("WEBHOOK_SITE_REQUEST"))
	expectedRequestID := strings.TrimSpace(os.Getenv("DIIA_REQUEST_ID"))

	if !uuidPattern.MatchString(tokenID) || !uuidPattern.MatchString(captureID) || expectedRequestID == "" {
		return errors.New("set WEBHOOK_SITE_TOKEN, WEBHOOK_SITE_REQUEST, and DIIA_REQUEST_ID")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	httpClient := &http.Client{
		Timeout:       30 * time.Second,
		CheckRedirect: captureRedirectPolicy,
	}
	payload, err := loadCapture(ctx, httpClient, tokenID, captureID, expectedRequestID)
	if err != nil {
		return err
	}

	fmt.Printf("request_id=%s\n", payload.RequestID)
	fmt.Printf("encrypted_metadata_bytes=%d\n", len(payload.EncodeData))
	fmt.Printf("encrypted_documents=%d\n", len(payload.Documents))

	for _, document := range payload.Documents {
		fmt.Printf("document_type=%s encrypted_bytes=%d\n", document.Type, len(document.Envelope))
	}

	fmt.Printf("signature_parts=%d\n", len(payload.Signatures))
	var archive []byte

	for _, part := range payload.UnknownParts {
		if isMultiSharingField(part.FieldName) {
			archive = part.Data
		}
	}

	if archive != nil {
		summary, err := summarizeArchive(archive)
		if err != nil {
			return err
		}

		fmt.Printf("multi_sharing_archive_bytes=%d\n", len(archive))
		fmt.Printf("zip_visible_entries=%d\n", summary.Entries)
		fmt.Printf("zip_encrypted_entries=%d\n", summary.EncryptedEntries)
		fmt.Printf("zip_visible_pdf_entries=%d\n", summary.PDFEntries)

		for _, documentType := range []diia.DocumentType{
			diia.DocumentTypeInternalPassport,
			diia.DocumentTypeTaxpayerCard,
			diia.DocumentTypeDriverLicense,
			diia.DocumentTypeVehicleLicense,
		} {
			fmt.Printf("zip_visible_pdf_type=%s count=%d\n", documentType, summary.PDFTypes[documentType])
		}

		fmt.Printf("zip_visible_unknown_pdf_entries=%d\n", summary.UnknownPDFEntries)
	}

	if os.Args[1] == "inspect" {
		if archive != nil {
			fmt.Println("zip_contents_verified=false")
		}

		return nil
	}

	return decrypt(ctx, payload)
}

func captureRedirectPolicy(request *http.Request, via []*http.Request) error {
	if len(via) == 0 || len(via) > 3 || request.URL.Scheme != "https" ||
		request.URL.User != nil || !strings.EqualFold(request.URL.Host, via[0].URL.Host) {
		return errors.New("unsafe webhook.site download redirect")
	}

	return nil
}

func loadCapture(
	ctx context.Context,
	client *http.Client,
	tokenID, captureID, expectedRequestID string,
) (*diia.CallbackPayload, error) {
	baseURL := fmt.Sprintf("https://webhook.site/token/%s/request/%s", tokenID, captureID)

	return loadCaptureURL(ctx, client, baseURL, expectedRequestID)
}

func loadCaptureURL(
	ctx context.Context,
	client *http.Client,
	baseURL, expectedRequestID string,
) (*diia.CallbackPayload, error) {
	data, err := getLimited(ctx, client, baseURL, maxCaptureJSON)
	if err != nil {
		return nil, fmt.Errorf("fetch capture metadata: %w", err)
	}

	var saved capture

	if err := json.Unmarshal(data, &saved); err != nil {
		return nil, errors.New("decode webhook.site capture metadata")
	}

	if saved.Method != http.MethodPost {
		return nil, errors.New("captured method is not POST")
	}

	if len(saved.Headers["x-document-request-trace-id"]) != 1 ||
		saved.Headers["x-document-request-trace-id"][0] != expectedRequestID {
		return nil, errors.New("captured trace ID does not match DIIA_REQUEST_ID")
	}

	if len(saved.Headers["content-type"]) != 1 {
		return nil, errors.New("capture has no unique Content-Type")
	}

	contentType, _, err := mime.ParseMediaType(saved.Headers["content-type"][0])

	if err != nil || !strings.EqualFold(contentType, "multipart/form-data") {
		return nil, errors.New("capture is not multipart/form-data")
	}

	encodedMetadata, ok := saved.Request["encodeData"]

	if !ok {
		return nil, errors.New("capture has no encodeData field")
	}

	metadata, err := decodePart([]byte(encodedMetadata))
	if err != nil {
		return nil, fmt.Errorf("decode encodeData: %w", err)
	}

	payload := &diia.CallbackPayload{RequestID: expectedRequestID, EncodeData: metadata}

	fieldNames := make([]string, 0, len(saved.Request)+len(saved.Files))

	for name := range saved.Request {
		if name != "encodeData" {
			fieldNames = append(fieldNames, name)
		}
	}

	for name := range saved.Files {
		if _, duplicate := saved.Request[name]; duplicate {
			return nil, errors.New("capture has duplicate form and file field")
		}

		fieldNames = append(fieldNames, name)
	}

	sort.Strings(fieldNames)

	for _, storageKey := range fieldNames {
		var encoded []byte
		filename := ""
		fieldName := storageKey
		if file, ok := saved.Files[storageKey]; ok {
			if !uuidPattern.MatchString(file.ID) {
				return nil, errors.New("capture has invalid file ID")
			}

			if strings.TrimSpace(file.Name) != "" {
				// webhook.site indexes repeated file fields as e.g.
				// vehicle-license.0 and vehicle-license.1. The original
				// document type is in each file object's name.
				fieldName = strings.TrimSpace(file.Name)
			}

			filename = file.Filename
			encoded, err = getLimited(ctx, client, baseURL+"/download/"+file.ID, maxEncodedPart)
			if err != nil {
				return nil, fmt.Errorf("download multipart field %s: %w", storageKey, err)
			}
		} else {
			encoded = []byte(saved.Request[storageKey])
		}
		decoded, err := decodePart(encoded)
		if err != nil {
			return nil, fmt.Errorf("decode multipart field %s: %w", storageKey, err)
		}

		if strings.HasPrefix(strings.ToLower(fieldName), "signature-") {
			payload.Signatures = append(payload.Signatures, diia.CallbackSignature{
				FieldName: fieldName, FileName: filename, Signed: decoded,
			})
			continue
		}

		if isMultiSharingField(fieldName) {
			if len(decoded) > maxArchiveBytes {
				return nil, errors.New("multi-sharing ZIP exceeds size limit")
			}

			if len(payload.UnknownParts) != 0 {
				return nil, errors.New("duplicate multi-sharing ZIP part")
			}

			payload.UnknownParts = append(payload.UnknownParts, diia.CallbackPart{
				FieldName: fieldName, FileName: filename, Data: decoded,
			})
			continue
		}

		documentType := diia.DocumentType(strings.TrimSuffix(fieldName, "[]"))

		if !documentType.Valid() {
			return nil, fmt.Errorf("unsupported multipart field %q", fieldName)
		}

		payload.Documents = append(payload.Documents, diia.CallbackDocument{
			Type: documentType, FieldName: fieldName, FileName: filename, Envelope: decoded,
		})
	}

	return payload, nil
}

func isMultiSharingField(name string) bool {
	return strings.EqualFold(strings.TrimSuffix(strings.TrimSpace(name), "[]"), "multi-sharing")
}

// summarizeArchive reads only the ZIP directory. Entry names and sizes are
// untrusted until the archive has been decrypted and its signatures verified.
func summarizeArchive(data []byte) (archiveSummary, error) {
	summary := archiveSummary{PDFTypes: make(map[diia.DocumentType]int)}

	if len(data) == 0 || len(data) > maxArchiveBytes {
		return summary, errors.New("empty or oversized multi-sharing ZIP")
	}

	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return summary, errors.New("invalid multi-sharing ZIP directory")
	}

	if len(archive.File) == 0 || len(archive.File) > maxArchiveEntries {
		return summary, errors.New("multi-sharing ZIP has invalid entry count")
	}

	summary.Entries = len(archive.File)
	var total uint64

	for _, entry := range archive.File {
		name := strings.TrimSuffix(entry.Name, "/")

		if name == "" || len(name) > 1024 || strings.HasPrefix(name, "/") ||
			strings.ContainsAny(name, "\\\x00") || name == ".." ||
			strings.HasPrefix(name, "../") || path.Clean(name) != name {
			return archiveSummary{}, errors.New("multi-sharing ZIP has unsafe entry path")
		}

		if entry.UncompressedSize64 > maxArchiveEntryBytes ||
			total+entry.UncompressedSize64 > maxArchiveTotalBytes {
			return archiveSummary{}, errors.New("multi-sharing ZIP exceeds uncompressed size limit")
		}

		total += entry.UncompressedSize64

		if entry.Flags&1 != 0 {
			summary.EncryptedEntries++
		}

		if entry.FileInfo().IsDir() || !strings.EqualFold(path.Ext(name), ".pdf") {
			continue
		}

		summary.PDFEntries++
		lowerName := strings.ToLower(name)
		var matched diia.DocumentType

		for _, documentType := range []diia.DocumentType{
			diia.DocumentTypeInternalPassport,
			diia.DocumentTypeTaxpayerCard,
			diia.DocumentTypeDriverLicense,
			diia.DocumentTypeVehicleLicense,
		} {
			if strings.Contains(lowerName, string(documentType)) {
				if matched != "" {
					return archiveSummary{}, errors.New("multi-sharing ZIP has ambiguous PDF name")
				}

				matched = documentType
			}
		}

		if matched == "" {
			summary.UnknownPDFEntries++
		} else {
			summary.PDFTypes[matched]++
		}
	}

	return summary, nil
}

func getLimited(ctx context.Context, client *http.Client, url string, limit int64) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.New("build webhook.site request")
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, errors.New("webhook.site request failed")
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("webhook.site returned HTTP %d", response.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, errors.New("read webhook.site response")
	}

	if int64(len(data)) > limit {
		return nil, errors.New("webhook.site response exceeds size limit")
	}

	return data, nil
}

func decodePart(encoded []byte) ([]byte, error) {
	if len(encoded) == 0 || len(encoded) > maxEncodedPart {
		return nil, errors.New("empty or oversized Base64 field")
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))

	if err != nil || len(decoded) == 0 {
		return nil, errors.New("invalid Base64 field")
	}

	return decoded, nil
}

func decrypt(ctx context.Context, payload *diia.CallbackPayload) error {
	password, err := privateKeyPassword()
	if err != nil {
		return err
	}

	if os.Getenv("PARTNER_KEY_PATH") == "" || password == "" ||
		os.Getenv("IIT_CA_SETTINGS_PATH") == "" || os.Getenv("IIT_CA_CERTIFICATES_PATH") == "" ||
		os.Getenv("DIIA_ENCRYPTION_CERTIFICATE_PATHS") == "" || os.Getenv("DIIA_SIGNING_CERTIFICATE_PATHS") == "" {
		return errors.New("set partner key/password, CA settings/bundle, and Diia encryption/signing certificate paths")
	}

	config, err := diiaiit.LoadConfig().
		WithPrivateKeyFile(os.Getenv("PARTNER_KEY_PATH"), password).
		WithCASettingsFile(os.Getenv("IIT_CA_SETTINGS_PATH")).
		WithCACertificatesFile(os.Getenv("IIT_CA_CERTIFICATES_PATH")).
		WithPartnerCertificateFiles(paths(os.Getenv("PARTNER_CERTIFICATE_PATHS"))...).
		WithDiiaEncryptionCertificateFiles(paths(os.Getenv("DIIA_ENCRYPTION_CERTIFICATE_PATHS"))...).
		WithDiiaSigningCertificateFiles(paths(os.Getenv("DIIA_SIGNING_CERTIFICATE_PATHS"))...).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("load IIT configuration: %w", err)
	}

	provider, err := diiaiit.NewProvider().WithConfig(config).Do(ctx)
	config.Destroy()

	if err != nil {
		return fmt.Errorf("initialize IIT provider: %w", err)
	}

	defer func() { _ = provider.Close().Do(context.Background()) }()

	options := []diia.Option{diia.WithCryptoProvider(provider)}
	acquirerToken := ""
	authAcquirerToken := ""

	if len(payload.UnknownParts) != 0 {
		acquirerToken = strings.TrimSpace(os.Getenv("DIIA_ACQUIRER_TOKEN"))
		authAcquirerToken = strings.TrimSpace(os.Getenv("DIIA_AUTH_ACQUIRER_TOKEN"))

		if acquirerToken == "" || authAcquirerToken == "" {
			return errors.New("set DIIA_ACQUIRER_TOKEN and DIIA_AUTH_ACQUIRER_TOKEN to retrieve the ZIP password")
		}

		options = append(
			options,
			diia.WithMultisharingArchiveProcessor(diiaiit.NewMultisharingArchiveProcessor(provider)),
		)
		options = append(
			options,
			diia.WithCallbackRequestIDAuthorizer(func(_ context.Context, requestID string) (bool, error) {
				// The capture was already matched against DIIA_REQUEST_ID by loadCapture.
				return requestID == payload.RequestID, nil
			}),
		)
	}

	client := diia.NewSandboxClient(acquirerToken, authAcquirerToken, options...)

	if len(payload.UnknownParts) != 0 {
		payload, err = parseReconstructedCallback(ctx, client, payload)
		if err != nil {
			return fmt.Errorf("parse reconstructed callback: %w", err)
		}

		fmt.Println("parsed_missing_bearer_callback=true")
	}

	result, err := client.Callbacks().Process().WithPayload(payload).Do(ctx)
	if err != nil {
		return fmt.Errorf("decrypt and verify callback: %w", err)
	}

	if result.RequestID != payload.RequestID {
		return errors.New("decrypted request ID does not match capture")
	}

	if len(payload.UnknownParts) != 0 {
		mandatory, err := mandatoryDocumentTypes(os.Getenv("DIIA_MANDATORY_SHARING"))
		if err != nil {
			return err
		}

		if err := diia.ValidateMultisharingPackage(result, mandatory...); err != nil {
			return fmt.Errorf("validate mandatory sharing: %w", err)
		}

		fmt.Println("zip_verified_against_configured_certificates=true")
		fmt.Printf("verified_mandatory_document_types=%d\n", len(mandatory))
		fmt.Printf("verified_vehicle_license_metadata_records=%d\n", len(result.Metadata.Data.VehicleLicenses))
		fmt.Printf("verified_document_signature_links=%d\n", len(result.Documents)*2)
	}

	fmt.Printf("verified_documents=%d\n", len(result.Documents))
	fmt.Printf("verified_signatures=%d\n", len(result.Signatures))

	for _, document := range result.Documents {
		fmt.Printf("verified_document_type=%s pdf_bytes=%d\n", document.Type, len(document.PDF))
	}

	outputDir := strings.TrimSpace(os.Getenv("DIIA_OUTPUT_DIR"))

	if outputDir != "" {
		if err := saveVerified(outputDir, result); err != nil {
			return err
		}

		fmt.Println("verified_files_saved=true")
	}

	return nil
}

func mandatoryDocumentTypes(value string) ([]diia.DocumentType, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}

	var mandatory []diia.DocumentType
	seen := make(map[diia.DocumentType]bool)

	for _, item := range strings.Split(value, ",") {
		documentType := diia.DocumentType(strings.TrimSpace(item))

		if !documentType.Valid() || seen[documentType] {
			return nil, errors.New("DIIA_MANDATORY_SHARING contains an invalid or repeated type")
		}

		seen[documentType] = true
		mandatory = append(mandatory, documentType)
	}

	return mandatory, nil
}

// webhook.site does not retain the original MIME body for this capture. Rebuild
// it from the already decoded fields to exercise the SDK's HTTP callback parser.
func parseReconstructedCallback(
	ctx context.Context,
	client diia.Diia,
	captured *diia.CallbackPayload,
) (*diia.CallbackPayload, error) {
	if captured == nil || captured.RequestID == "" || len(captured.EncodeData) == 0 {
		return nil, errors.New("incomplete captured callback")
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	writePart := func(fieldName, fileName string, data []byte) error {
		var part io.Writer
		var err error
		if fileName == "" {
			part, err = writer.CreateFormField(fieldName)
		} else {
			part, err = writer.CreateFormFile(fieldName, fileName)
		}

		if err != nil {
			return err
		}

		_, err = io.WriteString(part, base64.StdEncoding.EncodeToString(data))

		return err
	}

	if err := writePart("encodeData", "", captured.EncodeData); err != nil {
		return nil, err
	}

	for _, document := range captured.Documents {
		if err := writePart(document.FieldName, document.FileName, document.Envelope); err != nil {
			return nil, err
		}
	}

	for _, signature := range captured.Signatures {
		if err := writePart(signature.FieldName, signature.FileName, signature.Signed); err != nil {
			return nil, err
		}
	}

	for _, part := range captured.UnknownParts {
		if err := writePart(part.FieldName, part.FileName, part.Data); err != nil {
			return nil, err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://callback.invalid/diia/callback", &body)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("X-Document-Request-Trace-Id", captured.RequestID)

	return client.Callbacks().Parse().WithRequest(request).Do(ctx)
}

func privateKeyPassword() (string, error) {
	value := os.Getenv("PARTNER_KEY_PASSWORD")
	path := strings.TrimSpace(os.Getenv("PARTNER_KEY_PASSWORD_FILE"))

	if value != "" && path != "" {
		return "", errors.New("set either PARTNER_KEY_PASSWORD or PARTNER_KEY_PASSWORD_FILE")
	}

	if path == "" {
		return value, nil
	}

	info, err := os.Stat(path)

	if err != nil || !info.Mode().IsRegular() || info.Size() > 4096 || info.Mode().Perm()&0077 != 0 {
		return "", errors.New("password file must be a readable regular file of at most 4 KiB with mode 0600")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return "", errors.New("read password file")
	}

	password := strings.TrimRight(string(raw), "\r\n")

	for index := range raw {
		raw[index] = 0
	}

	return password, nil
}

func paths(value string) []string {
	return strings.FieldsFunc(value, func(character rune) bool {
		return character == ',' || character == os.PathListSeparator
	})
}

func saveVerified(outputDir string, result *diia.DocumentPackage) error {
	if err := os.Mkdir(outputDir, 0700); err != nil {
		return fmt.Errorf("create new private output directory: %w", err)
	}

	write := func(name string, data []byte) error {
		file, err := os.OpenFile(filepath.Join(outputDir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}

		_, writeErr := file.Write(data)
		closeErr := file.Close()

		if writeErr != nil {
			return writeErr
		}

		return closeErr
	}

	if err := write("metadata.json", result.RawMetadata); err != nil {
		return fmt.Errorf("save verified metadata: %w", err)
	}

	for index, document := range result.Documents {
		name := fmt.Sprintf("document-%02d-%s.pdf", index+1, document.Type)

		if err := write(name, document.PDF); err != nil {
			return fmt.Errorf("save verified document: %w", err)
		}
	}

	return nil
}
