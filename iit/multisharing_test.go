package iit

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	diia "github.com/stremovskyy/go-diia"
	implementation "github.com/stremovskyy/go-diia-iit"
	"go.foxforensics.dev/go-zip/pkg/zip"
)

const (
	testPassword       = "archive-test-password"
	testRequestID      = "request-test-1"
	testOfferRequestID = "offer-test-1"
)

type testDetachedVerifier struct{}

func (testDetachedVerifier) VerifyDetached(
	_ context.Context,
	data, signature []byte,
	role implementation.SignatureRole,
) error {
	if !hmac.Equal(signature, testSignature(data, role)) {
		return errors.New("detached signature mismatch")
	}

	return nil
}

func testSignature(data []byte, role implementation.SignatureRole) []byte {
	mac := hmac.New(sha256.New, []byte("synthetic-fixture-key"))
	_, _ = mac.Write([]byte(role))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write(data)
	signature, err := asn1.Marshal(struct{ Digest []byte }{mac.Sum(nil)})
	if err != nil {
		panic(err)
	}

	return signature
}

func testArchiveEntries() map[string][]byte {
	metadata := []byte(
		`{"requestId":"` + testRequestID + `","offerRequestId":"` + testOfferRequestID + `","documentTypes":["internal-passport","taxpayer-card","driver-license","vehicle-license"],"data":{"internal-passport":[{"fileName":"internal-passport:1.pdf.p7s"}],"taxpayer-card":[{"fileName":"taxpayer-card:1.pdf.p7s"}],"driver-license":[{"fileName":"driver-license:1.pdf.p7s"}],"vehicle-license":[{"fileName":"vehicle-license:1.pdf.p7s"},{"fileName":"vehicle-license:2.pdf.p7s"}]}}`,
	)
	entries := map[string][]byte{
		"metadata.json": metadata,
		"metadata.json.p7s": []byte(
			base64.StdEncoding.EncodeToString(testSignature(metadata, implementation.SignatureRoleDiiaSeal)),
		),
	}

	for _, pdfName := range []string{
		"internal-passport:1.pdf", "taxpayer-card:1.pdf", "driver-license:1.pdf",
		"vehicle-license:1.pdf", "vehicle-license:2.pdf",
	} {
		pdf := []byte("%PDF-1.7\n" + pdfName + "\n%%EOF")
		sealName := pdfName + ".p7s"
		entries[pdfName] = pdf
		entries[sealName] = testSignature(pdf, implementation.SignatureRoleDiiaSeal)
		entries["user_"+sealName] = testSignature(pdf, implementation.SignatureRoleUser)
	}

	return entries
}

func encryptTestArchive(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)

	for name, content := range entries {
		entry, err := writer.Encrypt(name, testPassword, zip.AES256Encryption)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := entry.Write(content); err != nil {
			t.Fatal(err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	return buffer.Bytes()
}

func processTestArchive(archive []byte, password string) (*diia.DocumentPackage, error) {
	processor := NewMultisharingArchiveProcessor(testDetachedVerifier{})

	return processor.ProcessArchive(context.Background(), archive, password, testRequestID, testOfferRequestID)
}

func TestMultisharingArchiveRejectsTypedNilVerifier(t *testing.T) {
	var missingProvider *implementation.Provider
	processor := NewMultisharingArchiveProcessor(missingProvider)

	if _, err := processor.ProcessArchive(context.Background(), []byte("zip"), testPassword, testRequestID, testOfferRequestID); err == nil {
		t.Fatal("typed nil signature verifier was accepted")
	}
}

func TestMultisharingArchiveRejectsNilContext(t *testing.T) {
	processor := NewMultisharingArchiveProcessor(testDetachedVerifier{})

	if _, err := processor.ProcessArchive(nil, []byte("zip"), testPassword, testRequestID, testOfferRequestID); err == nil {
		t.Fatal("nil context was accepted")
	}
}

func TestMultisharingArchiveFiveDocumentsAndDistinctVehicles(t *testing.T) {
	result, err := processTestArchive(encryptTestArchive(t, testArchiveEntries()), testPassword)
	if err != nil {
		t.Fatal(err)
	}

	if result.RequestID != testRequestID || result.OfferRequestID != testOfferRequestID ||
		result.Metadata == nil || result.MetadataSealFileName != "metadata.json.p7s" ||
		len(result.Documents) != 5 || len(result.Signatures) != 11 {
		t.Fatalf(
			"invalid multi-sharing result: documents=%d signatures=%d",
			len(result.Documents),
			len(result.Signatures),
		)
	}

	vehicles := make(map[string]bool)

	for _, document := range result.Documents {
		if document.DiiaSealFileName != document.FileName+".p7s" ||
			document.UserSignatureFileName != "user_"+document.DiiaSealFileName ||
			!bytes.HasPrefix(document.PDF, []byte("%PDF-")) {
			t.Fatalf("document link invalid: %s", document.FileName)
		}

		if document.Type == diia.DocumentTypeVehicleLicense {
			vehicles[document.FileName] = true
		}
	}

	if len(vehicles) != 2 || !vehicles["vehicle-license:1.pdf"] || !vehicles["vehicle-license:2.pdf"] {
		t.Fatalf("two distinct vehicle documents not retained: %v", vehicles)
	}
}

func TestMultisharingArchiveWrongPasswordAndBadMAC(t *testing.T) {
	archive := encryptTestArchive(t, testArchiveEntries())

	if _, err := processTestArchive(archive, "wrong-password"); err == nil {
		t.Fatal("wrong password was accepted")
	}

	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatal(err)
	}

	mutated := append([]byte(nil), archive...)
	file := reader.File[0]
	offset, err := file.DataOffset()
	if err != nil {
		t.Fatal(err)
	}

	// The final ten bytes are the WinZip AES authentication code.
	mutated[int(offset+int64(file.CompressedSize64)-1)] ^= 1

	if _, err := processTestArchive(mutated, testPassword); !errors.Is(err, zip.ErrAuthentication) {
		t.Fatalf("bad AES MAC should return authentication error, got %v", err)
	}
}

func TestMultisharingArchiveRejectsAlteredPDFAndSignature(t *testing.T) {
	for _, change := range []struct {
		name string
		file string
	}{
		{"pdf", "vehicle-license:1.pdf"},
		{"diia-seal", "vehicle-license:1.pdf.p7s"},
		{"user-signature", "user_vehicle-license:1.pdf.p7s"},
		{"metadata-seal", "metadata.json.p7s"},
	} {
		t.Run(change.name, func(t *testing.T) {
			entries := testArchiveEntries()
			entries[change.file] = append([]byte(nil), entries[change.file]...)
			entries[change.file][len(entries[change.file])-1] ^= 1

			if _, err := processTestArchive(encryptTestArchive(t, entries), testPassword); err == nil {
				t.Fatal("altered signed data was accepted")
			}
		})
	}
}

func TestMultisharingArchiveRejectsMissingExtraAndDuplicateLinks(t *testing.T) {
	for _, change := range []struct {
		name   string
		modify func(map[string][]byte)
	}{
		{"missing-user-signature", func(entries map[string][]byte) { delete(entries, "user_vehicle-license:2.pdf.p7s") }},
		{"orphan-file", func(entries map[string][]byte) { entries["orphan.pdf"] = []byte("%PDF-1.7") }},
		{"duplicate-metadata-link", func(entries map[string][]byte) {
			metadata := bytes.Replace(entries["metadata.json"], []byte("vehicle-license:2.pdf.p7s"), []byte("vehicle-license:1.pdf.p7s"), 1)
			entries["metadata.json"] = metadata
			entries["metadata.json.p7s"] = []byte(base64.StdEncoding.EncodeToString(testSignature(metadata, implementation.SignatureRoleDiiaSeal)))
		}},
		{"missing-metadata-link", func(entries map[string][]byte) {
			metadata := bytes.Replace(entries["metadata.json"], []byte("vehicle-license:2.pdf.p7s"), []byte("vehicle-license:3.pdf.p7s"), 1)
			entries["metadata.json"] = metadata
			entries["metadata.json.p7s"] = []byte(base64.StdEncoding.EncodeToString(testSignature(metadata, implementation.SignatureRoleDiiaSeal)))
		}},
	} {
		t.Run(change.name, func(t *testing.T) {
			entries := testArchiveEntries()
			change.modify(entries)

			if _, err := processTestArchive(encryptTestArchive(t, entries), testPassword); err == nil {
				t.Fatal("invalid document set was accepted")
			}
		})
	}
}

func TestMultisharingArchiveRejectsDeclaredDocumentWithoutPDFOrError(t *testing.T) {
	metadata := []byte(
		`{"requestId":"` + testRequestID + `","offerRequestId":"` + testOfferRequestID + `","documentTypes":["internal-passport","taxpayer-card"],"errorDocuments":[],"data":{"internal-passport":[{"fileName":"internal-passport:1.pdf.p7s"}]}}`,
	)
	pdf := []byte("%PDF-1.7\ninternal-passport\n%%EOF")
	entries := map[string][]byte{
		"metadata.json": metadata,
		"metadata.json.p7s": []byte(
			base64.StdEncoding.EncodeToString(testSignature(metadata, implementation.SignatureRoleDiiaSeal)),
		),
		"internal-passport:1.pdf":          pdf,
		"internal-passport:1.pdf.p7s":      testSignature(pdf, implementation.SignatureRoleDiiaSeal),
		"user_internal-passport:1.pdf.p7s": testSignature(pdf, implementation.SignatureRoleUser),
	}

	if _, err := processTestArchive(encryptTestArchive(t, entries), testPassword); !errors.Is(err, diia.ErrCallback) {
		t.Fatalf("declared document without PDF or error was accepted: %v", err)
	}
}

func TestMultisharingArchiveRejectsDuplicateZIPEntryNames(t *testing.T) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entries := testArchiveEntries()

	for name, content := range entries {
		entry, err := writer.Encrypt(name, testPassword, zip.AES256Encryption)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := entry.Write(content); err != nil {
			t.Fatal(err)
		}
	}

	duplicate, err := writer.Encrypt("metadata.json", testPassword, zip.AES256Encryption)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := duplicate.Write(entries["metadata.json"]); err != nil {
		t.Fatal(err)
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := processTestArchive(buffer.Bytes(), testPassword); err == nil {
		t.Fatal("duplicate ZIP entry name was accepted")
	}
}

func TestMultisharingArchiveRejectsDuplicateMetadataKeys(t *testing.T) {
	for _, replacement := range []string{
		`"data":{"vehicle-license":[{"fileName":"vehicle-license:1.pdf.p7s"}],"vehicle-license":[{"fileName":"vehicle-license:2.pdf.p7s"}]}`,
		`"data":{"vehicle-license":[{"fileName":"vehicle-license:1.pdf.p7s","fileName":"vehicle-license:2.pdf.p7s"}]}`,
	} {
		entries := testArchiveEntries()
		metadata := []byte(
			`{"requestId":"` + testRequestID + `","offerRequestId":"` + testOfferRequestID + `",` + replacement + `}`,
		)
		entries["metadata.json"] = metadata
		entries["metadata.json.p7s"] = []byte(
			base64.StdEncoding.EncodeToString(testSignature(metadata, implementation.SignatureRoleDiiaSeal)),
		)

		if _, err := processTestArchive(encryptTestArchive(t, entries), testPassword); err == nil {
			t.Fatal("duplicate metadata JSON key was accepted")
		}
	}
}

func TestMultisharingArchiveRejectsUnsafeNamesAndBombs(t *testing.T) {
	for _, name := range []string{"../evil.pdf", "/absolute.pdf", "subdir/file.pdf", "dir\\file.pdf", "x\x00.pdf"} {
		t.Run(fmt.Sprintf("unsafe-%q", name), func(t *testing.T) {
			entries := testArchiveEntries()
			entries[name] = []byte("x")

			if _, err := processTestArchive(encryptTestArchive(t, entries), testPassword); err == nil {
				t.Fatal("unsafe archive name was accepted")
			}
		})
	}

	entries := testArchiveEntries()
	entries["bomb.pdf"] = []byte(strings.Repeat("0", maxMultisharingEntryBytes+1))

	if _, err := processTestArchive(encryptTestArchive(t, entries), testPassword); err == nil {
		t.Fatal("oversized uncompressed entry was accepted")
	}

	entries = testArchiveEntries()

	for n := 0; n < maxMultisharingEntries; n++ {
		entries[fmt.Sprintf("extra-%03d", n)] = []byte("x")
	}

	if _, err := processTestArchive(encryptTestArchive(t, entries), testPassword); err == nil {
		t.Fatal("oversized entry count was accepted")
	}
}

func TestMultisharingArchiveRejectsUnencryptedZIP(t *testing.T) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entry, err := writer.Create("metadata.json")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := io.WriteString(entry, `{}`); err != nil {
		t.Fatal(err)
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := processTestArchive(buffer.Bytes(), testPassword); err == nil {
		t.Fatal("unencrypted ZIP was accepted")
	}
}

func TestMultisharingArchiveRejectsUnsupportedSignatureEncoding(t *testing.T) {
	for _, signature := range [][]byte{nil, []byte("unknown"), []byte("MII invalid base64"), append(testSignature([]byte("x"), implementation.SignatureRoleDiiaSeal), 0)} {
		if _, err := normalizeArchiveSignature(signature); err == nil {
			t.Fatal("unsupported signature encoding was accepted")
		}
	}
}

// This optional format test reads Diia's published passworded sample. It uses
// a deliberately permissive verifier, so it proves ZIP decryption and metadata
// binding only; it makes no cryptographic signer/trust assertion.
func TestPasswordedSampleZIPFormat(t *testing.T) {
	path := os.Getenv("DIIA_MULTISHARING_SAMPLE_ZIP")
	password := os.Getenv("DIIA_MULTISHARING_SAMPLE_PASSWORD")

	if path == "" || password == "" {
		t.Skip("set DIIA_MULTISHARING_SAMPLE_ZIP and DIIA_MULTISHARING_SAMPLE_PASSWORD")
	}

	archive, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	entries, err := readAuthenticatedArchive(context.Background(), archive, password)
	if err != nil {
		t.Fatal(err)
	}

	var metadata diia.Metadata

	if err := json.Unmarshal(entries["metadata.json"], &metadata); err != nil {
		t.Fatal(err)
	}

	requestID := metadata.RequestID

	if requestID == "" {
		requestID = "official-sample-request"
	}

	offerRequestID := metadata.OfferRequestID

	if offerRequestID == "" {
		offerRequestID = "official-sample-offer"
	}

	processor := NewMultisharingArchiveProcessor(nonemptyDetachedVerifier{})
	result, err := processor.ProcessArchive(context.Background(), archive, password, requestID, offerRequestID)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Documents) == 0 || len(result.Signatures) != 1+2*len(result.Documents) {
		t.Fatalf("sample mapping invalid: documents=%d signatures=%d", len(result.Documents), len(result.Signatures))
	}

	for _, kind := range []struct {
		label      string
		selectName func(string) bool
	}{
		{"metadata", func(name string) bool { return name == "metadata.json.p7s" }},
		{"diia-pdf", func(name string) bool {
			return strings.HasSuffix(name, ".pdf.p7s") && !strings.HasPrefix(name, "user_")
		}},
		{"user-pdf", func(name string) bool { return strings.HasPrefix(name, "user_") && strings.HasSuffix(name, ".pdf.p7s") }},
	} {
		for name, raw := range entries {
			if !kind.selectName(name) {
				continue
			}

			signature, err := normalizeArchiveSignature(raw)
			if err != nil {
				t.Fatal(err)
			}

			content, present, err := cmsEContent(signature)
			if err != nil {
				t.Fatal(err)
			}

			signedName := "metadata.json"

			if kind.label != "metadata" {
				signedName = strings.TrimSuffix(strings.TrimPrefix(name, "user_"), ".p7s")
			}

			if kind.label == "user-pdf" {
				if present {
					t.Fatal("official sample user signature unexpectedly contains content")
				}
			} else if !present || !bytes.Equal(content, entries[signedName]) {
				t.Fatal("official sample Diia seal content differs from exact file bytes")
			}
			t.Logf(
				"official sample %s CMS eContent present=%t bytes=%d matches_file=%t",
				kind.label,
				present,
				len(content),
				present && bytes.Equal(content, entries[signedName]),
			)
			break
		}
	}

	t.Logf("official sample ZIP mechanics: %d PDFs, %d signatures", len(result.Documents), len(result.Signatures))
}

func cmsEContent(signature []byte) ([]byte, bool, error) {
	var outer asn1.RawValue

	if _, err := asn1.Unmarshal(signature, &outer); err != nil {
		return nil, false, err
	}

	var contentType asn1.ObjectIdentifier
	rest, err := asn1.Unmarshal(outer.Bytes, &contentType)
	if err != nil {
		return nil, false, err
	}

	var wrapped asn1.RawValue

	if _, err := asn1.Unmarshal(rest, &wrapped); err != nil {
		return nil, false, err
	}

	var signedData asn1.RawValue

	if _, err := asn1.Unmarshal(wrapped.Bytes, &signedData); err != nil {
		return nil, false, err
	}

	var version asn1.RawValue
	rest, err = asn1.Unmarshal(signedData.Bytes, &version)
	if err != nil {
		return nil, false, err
	}

	var digests asn1.RawValue
	rest, err = asn1.Unmarshal(rest, &digests)
	if err != nil {
		return nil, false, err
	}

	var encap asn1.RawValue

	if _, err := asn1.Unmarshal(rest, &encap); err != nil {
		return nil, false, err
	}

	var eContentType asn1.ObjectIdentifier
	rest, err = asn1.Unmarshal(encap.Bytes, &eContentType)
	if err != nil {
		return nil, false, err
	}

	if len(rest) == 0 {
		return nil, false, nil
	}

	var explicit asn1.RawValue

	if _, err := asn1.Unmarshal(rest, &explicit); err != nil {
		return nil, false, err
	}

	var octets []byte

	if _, err := asn1.Unmarshal(explicit.Bytes, &octets); err != nil {
		return nil, false, err
	}

	return octets, true, nil
}

type nonemptyDetachedVerifier struct{}

func (nonemptyDetachedVerifier) VerifyDetached(
	_ context.Context,
	data, signature []byte,
	_ implementation.SignatureRole,
) error {
	if len(data) == 0 || len(signature) == 0 {
		return errors.New("empty signed data or signature")
	}

	return nil
}
