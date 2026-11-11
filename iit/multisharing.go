package iit

import (
	"bytes"
	"context"
	"encoding/asn1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"

	diia "github.com/stremovskyy/go-diia"
	implementation "github.com/stremovskyy/go-diia-iit"
	"go.foxforensics.dev/go-zip/pkg/zip"
)

const (
	// Bounds include every entry, including signatures and metadata. The ZIP is
	// kept in memory so a callback cannot create files or use its entry names as
	// paths on the application host.
	maxMultisharingArchiveBytes = 16 << 20
	maxMultisharingEntries      = 128
	maxMultisharingEntryBytes   = 32 << 20
	maxMultisharingTotalBytes   = 128 << 20
	winZipAESExtraID            = 0x9901
)

// DetachedSignatureVerifier verifies the exact bytes supplied by the ZIP
// processor. The native provider accepts attached Diia CMS only when its
// authenticated embedded content equals those bytes; user CMS is detached.
// Implementations must fail closed when signer trust or certificate status
// cannot be established.
type DetachedSignatureVerifier interface {
	VerifyDetached(context.Context, []byte, []byte, implementation.SignatureRole) error
}

// MultisharingArchiveProcessor processes the password-protected WinZip AES-256
// archive delivered in a Diia multi-sharing callback.
type MultisharingArchiveProcessor struct {
	verifier DetachedSignatureVerifier
}

var _ diia.MultisharingArchiveProcessor = (*MultisharingArchiveProcessor)(nil)

func NewMultisharingArchiveProcessor(verifier DetachedSignatureVerifier) *MultisharingArchiveProcessor {
	return &MultisharingArchiveProcessor{verifier: verifier}
}

func (processor *MultisharingArchiveProcessor) ProcessArchive(
	ctx context.Context, archive []byte, password, requestID, offerRequestID string,
) (*diia.DocumentPackage, error) {
	if processor == nil || isNilVerifier(processor.verifier) {
		return nil, errors.New("multi-sharing detached signature verifier is required")
	}

	if ctx == nil {
		return nil, errors.New("multi-sharing context is required")
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if len(archive) == 0 || len(archive) > maxMultisharingArchiveBytes {
		return nil, errors.New("multi-sharing archive exceeds size limit or is empty")
	}

	if password == "" || requestID == "" || offerRequestID == "" {
		return nil, errors.New("multi-sharing password and request identifiers are required")
	}

	entries, err := readAuthenticatedArchive(ctx, archive, password)
	if err != nil {
		return nil, err
	}

	rawMetadata, ok := entries["metadata.json"]

	if !ok {
		return nil, errors.New("multi-sharing metadata.json is missing")
	}

	metadataSignature, ok := entries["metadata.json.p7s"]

	if !ok {
		return nil, errors.New("multi-sharing metadata signature is missing")
	}

	metadataSignature, err = normalizeArchiveSignature(metadataSignature)
	if err != nil {
		return nil, fmt.Errorf("decode multi-sharing metadata signature: %w", err)
	}

	if err := processor.verifier.VerifyDetached(ctx, rawMetadata, metadataSignature, implementation.SignatureRoleDiiaSeal); err != nil {
		return nil, fmt.Errorf("verify multi-sharing metadata seal: %w", err)
	}

	var metadata diia.Metadata

	if err := json.Unmarshal(rawMetadata, &metadata); err != nil {
		return nil, fmt.Errorf("decode multi-sharing metadata: %w", err)
	}

	if metadata.RequestID != "" && metadata.RequestID != requestID {
		return nil, errors.New("multi-sharing metadata requestId mismatch")
	}

	if metadata.OfferRequestID != "" && metadata.OfferRequestID != offerRequestID {
		return nil, errors.New("multi-sharing metadata offerRequestId mismatch")
	}

	records, err := documentRecords(rawMetadata)
	if err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return nil, errors.New("multi-sharing metadata contains no documents")
	}

	if len(entries) != 2+3*len(records) {
		return nil, errors.New("multi-sharing archive has missing or extra entries")
	}

	used := map[string]bool{"metadata.json": true, "metadata.json.p7s": true}
	result := &diia.DocumentPackage{
		RequestID: requestID, OfferRequestID: offerRequestID,
		Barcode: metadata.Barcode, Metadata: &metadata,
		MetadataSealFileName: "metadata.json.p7s",
		RawMetadata:          json.RawMessage(rawMetadata),
		Documents:            make([]diia.DocumentFile, 0, len(records)),
		Signatures:           make([]diia.VerifiedSignature, 0, 1+2*len(records)),
	}
	result.Signatures = append(result.Signatures, diia.VerifiedSignature{
		FieldName: "multi-sharing", FileName: "metadata.json.p7s", DocumentFileName: "metadata.json", Role: "diia-seal",
	})

	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		sealName := record.sealFileName
		pdfName := strings.TrimSuffix(sealName, ".p7s")
		userName := "user_" + sealName
		pdf, pdfFound := entries[pdfName]
		seal, sealFound := entries[sealName]
		userSignature, userFound := entries[userName]

		if !pdfFound || !sealFound || !userFound || used[pdfName] || used[sealName] || used[userName] {
			return nil, errors.New("multi-sharing document and signatures do not match metadata")
		}

		if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
			return nil, errors.New("multi-sharing document is not a PDF")
		}

		seal, err = normalizeArchiveSignature(seal)
		if err != nil {
			return nil, fmt.Errorf("decode multi-sharing Diia seal: %w", err)
		}

		userSignature, err = normalizeArchiveSignature(userSignature)
		if err != nil {
			return nil, fmt.Errorf("decode multi-sharing user signature: %w", err)
		}

		if err := processor.verifier.VerifyDetached(ctx, pdf, seal, implementation.SignatureRoleDiiaSeal); err != nil {
			return nil, fmt.Errorf("verify multi-sharing Diia document seal: %w", err)
		}

		if err := processor.verifier.VerifyDetached(ctx, pdf, userSignature, implementation.SignatureRoleUser); err != nil {
			return nil, fmt.Errorf("verify multi-sharing user document signature: %w", err)
		}

		used[pdfName], used[sealName], used[userName] = true, true, true
		result.Documents = append(result.Documents, diia.DocumentFile{
			Type: record.documentType, FieldName: string(record.documentType), FileName: pdfName,
			PDF: pdf, DiiaSealFileName: sealName, UserSignatureFileName: userName,
		})
		result.Signatures = append(
			result.Signatures,
			diia.VerifiedSignature{
				FieldName:        string(record.documentType),
				FileName:         sealName,
				DocumentFileName: pdfName,
				Role:             "diia-seal",
			},
			diia.VerifiedSignature{
				FieldName:        string(record.documentType),
				FileName:         userName,
				DocumentFileName: pdfName,
				Role:             "user",
			},
		)
	}

	for name := range entries {
		if !used[name] {
			return nil, errors.New("multi-sharing archive has an unreferenced entry")
		}
	}

	if err := diia.ValidateMultisharingPackage(result); err != nil {
		return nil, err
	}

	return result, nil
}

func isNilVerifier(verifier DetachedSignatureVerifier) bool {
	if verifier == nil {
		return true
	}

	value := reflect.ValueOf(verifier)

	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

// readAuthenticatedArchive verifies the WinZip AES authentication code before
// exposing each entry's plaintext. go-zip's default buffered authentication
// reader does this before its first Read; DeferAuth must remain false.
func readAuthenticatedArchive(ctx context.Context, archive []byte, password string) (map[string][]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("open multi-sharing archive: %w", err)
	}

	if len(reader.File) == 0 || len(reader.File) > maxMultisharingEntries {
		return nil, errors.New("multi-sharing archive entry count exceeds limit or is empty")
	}

	entries := make(map[string][]byte, len(reader.File))
	var total uint64

	for _, file := range reader.File {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		name, err := safeArchiveEntryName(file.Name)
		if err != nil {
			return nil, err
		}

		if _, exists := entries[name]; exists {
			return nil, errors.New("multi-sharing archive has duplicate entry names")
		}

		if !file.FileInfo().Mode().IsRegular() || !isAES256(file.Extra) || !file.IsEncrypted() || file.DeferAuth ||
			file.CompressedSize64 < 28 || file.UncompressedSize64 > maxMultisharingEntryBytes ||
			file.CompressedSize64 > uint64(len(archive)) {
			return nil, errors.New("multi-sharing archive has unsupported or oversized entry")
		}

		if total+file.UncompressedSize64 > maxMultisharingTotalBytes {
			return nil, errors.New("multi-sharing archive uncompressed size exceeds limit")
		}

		file.SetPassword(password)
		opened, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("decrypt multi-sharing archive entry: %w", err)
		}

		value, readErr := io.ReadAll(io.LimitReader(opened, maxMultisharingEntryBytes+1))
		closeErr := opened.Close()

		if readErr != nil {
			return nil, fmt.Errorf("read multi-sharing archive entry: %w", readErr)
		}

		if closeErr != nil {
			return nil, fmt.Errorf("close multi-sharing archive entry: %w", closeErr)
		}

		if len(value) > maxMultisharingEntryBytes || uint64(len(value)) != file.UncompressedSize64 {
			return nil, errors.New("multi-sharing archive entry size mismatch")
		}

		total += uint64(len(value))
		entries[name] = value
	}

	return entries, nil
}

func safeArchiveEntryName(name string) (string, error) {
	if name == "" || name == "." || name == ".." || strings.TrimSpace(name) != name ||
		strings.ContainsAny(name, "/\\\x00") || strings.Contains(name, "..") {
		return "", errors.New("multi-sharing archive has an unsafe entry name")
	}

	for _, char := range name {
		if char < 0x20 || char == 0x7f {
			return "", errors.New("multi-sharing archive has an unsafe entry name")
		}
	}

	return name, nil
}

// isAES256 parses the public ZIP extra-field bytes; go-zip keeps the parsed
// strength private. The 0x9901 record declares the WinZip AES strength.
func isAES256(extra []byte) bool {
	found := false

	for len(extra) >= 4 {
		kind := binary.LittleEndian.Uint16(extra[:2])
		length := int(binary.LittleEndian.Uint16(extra[2:4]))
		extra = extra[4:]

		if length > len(extra) {
			return false
		}

		if kind == winZipAESExtraID {
			if found || length != 7 {
				return false
			}

			version := binary.LittleEndian.Uint16(extra[:2])

			if (version != 1 && version != 2) || string(extra[2:4]) != "AE" || extra[4] != 3 {
				return false
			}

			method := binary.LittleEndian.Uint16(extra[5:7])

			if method != zip.Deflate && method != zip.Store {
				return false
			}

			found = true
		}

		extra = extra[length:]
	}

	return found && len(extra) == 0
}

type metadataDocumentRecord struct {
	documentType diia.DocumentType
	sealFileName string
}

func documentRecords(rawMetadata []byte) ([]metadataDocumentRecord, error) {
	envelope, err := decodeUniqueObject(rawMetadata)

	if err != nil || len(envelope["data"]) == 0 {
		return nil, errors.New("multi-sharing metadata data is missing")
	}

	byType, err := decodeUniqueObject(envelope["data"])
	if err != nil {
		return nil, errors.New("multi-sharing metadata data must be an object")
	}

	var records []metadataDocumentRecord
	seenNames := make(map[string]bool)
	typeNames := make([]string, 0, len(byType))

	for typeName := range byType {
		typeNames = append(typeNames, typeName)
	}

	sort.Strings(typeNames)

	for _, typeName := range typeNames {
		rawDocuments := byType[typeName]
		documentType := diia.DocumentType(typeName)

		if !documentType.Valid() {
			return nil, errors.New("multi-sharing metadata has unsupported document type")
		}

		var documents []json.RawMessage
		trimmed := bytes.TrimSpace(rawDocuments)

		if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
			return nil, errors.New("multi-sharing metadata document list is empty")
		}

		if trimmed[0] == '[' {
			if err := json.Unmarshal(trimmed, &documents); err != nil || len(documents) == 0 {
				return nil, errors.New("multi-sharing metadata document list is invalid")
			}
		} else {
			documents = []json.RawMessage{trimmed}
		}

		for _, rawDocument := range documents {
			document, err := decodeUniqueObject(rawDocument)
			if err != nil {
				return nil, errors.New("multi-sharing metadata document is invalid")
			}

			var fileName string

			if err := json.Unmarshal(document["fileName"], &fileName); err != nil || fileName == "" {
				return nil, errors.New("multi-sharing metadata document has no fileName")
			}

			name, err := safeArchiveEntryName(fileName)

			if err != nil || !strings.HasSuffix(name, ".pdf.p7s") || seenNames[name] {
				return nil, errors.New("multi-sharing metadata document fileName is invalid or duplicated")
			}

			seenNames[name] = true
			records = append(records, metadataDocumentRecord{documentType: documentType, sealFileName: name})
		}
	}

	return records, nil
}

func decodeUniqueObject(data []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()

	if err != nil || token != json.Delim('{') {
		return nil, errors.New("JSON object is required")
	}

	fields := make(map[string]json.RawMessage)

	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}

		key, ok := token.(string)

		if !ok {
			return nil, errors.New("invalid JSON object key")
		}

		if _, duplicate := fields[key]; duplicate {
			return nil, errors.New("duplicate JSON object key")
		}

		var value json.RawMessage

		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}

		fields[key] = value
	}

	if _, err := decoder.Token(); err != nil {
		return nil, err
	}

	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("trailing JSON data")
	}

	return fields, nil
}

// Diia currently places the metadata seal as Base64 text inside the AES ZIP,
// while PDF seals and user signatures are binary DER. Accept both
// representations, but hand only one complete DER object to IIT.
func normalizeArchiveSignature(signature []byte) ([]byte, error) {
	if len(signature) == 0 {
		return nil, errors.New("empty detached signature")
	}

	if signature[0] != 0x30 {
		compact := make([]byte, 0, len(signature))

		for _, char := range signature {
			if char != ' ' && char != '\n' && char != '\r' && char != '\t' {
				compact = append(compact, char)
			}
		}

		var err error
		signature, err = base64.StdEncoding.DecodeString(string(compact))
		if err != nil {
			signature, err = base64.RawStdEncoding.DecodeString(string(compact))
			if err != nil {
				return nil, errors.New("detached signature is neither DER nor Base64 DER")
			}
		}
	}

	var object asn1.RawValue
	rest, err := asn1.Unmarshal(signature, &object)

	if err != nil || len(rest) != 0 || object.Class != 0 || object.Tag != asn1.TagSequence || !object.IsCompound {
		return nil, errors.New("detached signature is not one DER sequence")
	}

	return signature, nil
}
