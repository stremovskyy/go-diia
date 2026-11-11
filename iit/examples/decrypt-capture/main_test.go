package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	diia "github.com/stremovskyy/go-diia"
)

func fixtureArchive(t *testing.T, names []string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)

	for _, name := range names {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := entry.Write([]byte("fixture")); err != nil {
			t.Fatal(err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	return buffer.Bytes()
}

func TestCaptureRedirectsStayOnOriginalHost(t *testing.T) {
	var foreignRequests atomic.Int32
	foreign := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		foreignRequests.Add(1)
		_, _ = io.WriteString(writer, "foreign")
	}))
	defer foreign.Close()

	source := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/same-host":
			http.Redirect(writer, request, "/download", http.StatusFound)
		case "/foreign-host":
			http.Redirect(writer, request, foreign.URL+"/download", http.StatusFound)
		case "/download":
			_, _ = io.WriteString(writer, "capture")
		default:
			http.NotFound(writer, request)
		}
	}))
	defer source.Close()

	client := source.Client()
	client.CheckRedirect = captureRedirectPolicy
	data, err := getLimited(context.Background(), client, source.URL+"/same-host", 32)

	if err != nil || string(data) != "capture" {
		t.Fatalf("same-host redirect failed: %q, %v", data, err)
	}

	response, err := client.Get(source.URL + "/foreign-host")
	if response != nil {
		_ = response.Body.Close()
	}

	if err == nil || !strings.Contains(err.Error(), "unsafe webhook.site download redirect") {
		t.Fatalf("foreign-host redirect was not rejected: %v", err)
	}

	if foreignRequests.Load() != 0 {
		t.Fatal("foreign host received the redirected request")
	}
}

func TestSummarizeArchiveCountsFiveVisiblePDFs(t *testing.T) {
	archive := fixtureArchive(t, []string{
		"metadata.json", "metadata.json.p7s",
		"taxpayer-card:1.pdf", "internal-passport:1.pdf", "driver-license:1.pdf",
		"vehicle-license:1.pdf", "vehicle-license:2.pdf",
		"taxpayer-card-1.pdf.p7s", "vehicle-license-1.pdf.p7s",
	})
	summary, err := summarizeArchive(archive)
	if err != nil {
		t.Fatal(err)
	}

	if summary.Entries != 9 || summary.PDFEntries != 5 || summary.UnknownPDFEntries != 0 ||
		summary.PDFTypes[diia.DocumentTypeInternalPassport] != 1 ||
		summary.PDFTypes[diia.DocumentTypeTaxpayerCard] != 1 ||
		summary.PDFTypes[diia.DocumentTypeDriverLicense] != 1 ||
		summary.PDFTypes[diia.DocumentTypeVehicleLicense] != 2 {
		t.Fatalf("unexpected visible PDF counts: %+v", summary)
	}
}

func TestSummarizeArchiveRejectsUnsafePathsAndEntryCount(t *testing.T) {
	for _, name := range []string{"../vehicle-license.pdf", "/internal-passport.pdf", "dir\\taxpayer-card.pdf"} {
		t.Run(name, func(t *testing.T) {
			if _, err := summarizeArchive(fixtureArchive(t, []string{name})); err == nil {
				t.Fatal("unsafe ZIP path accepted")
			}
		})
	}

	names := make([]string, maxArchiveEntries+1)

	for index := range names {
		names[index] = "metadata-" + string(rune('a'+index/26)) + string(rune('a'+index%26)) + ".json"
	}

	if _, err := summarizeArchive(fixtureArchive(t, names)); err == nil {
		t.Fatal("oversized ZIP entry count accepted")
	}
}

func TestLoadCaptureRetainsMultiSharingZIP(t *testing.T) {
	const requestID = "example-multisharing-zip"
	const fileID = "33333333-3333-3333-3333-333333333333"
	archive := fixtureArchive(t, []string{"vehicle-license-1.pdf", "vehicle-license-2.pdf"})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/capture":
			_ = json.NewEncoder(writer).Encode(capture{
				Method: http.MethodPost,
				Headers: map[string][]string{
					"x-document-request-trace-id": {requestID},
					"content-type":                {"multipart/form-data; boundary=probe"},
				},
				Request: map[string]string{"encodeData": base64.StdEncoding.EncodeToString([]byte("metadata"))},
				Files: map[string]capturedFile{
					"multi-sharing": {ID: fileID, Name: "multi-sharing", Filename: "multi-sharing.zip"},
				},
			})

		case "/capture/download/" + fileID:
			_, _ = io.WriteString(writer, base64.StdEncoding.EncodeToString(archive))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	payload, err := loadCaptureURL(context.Background(), server.Client(), server.URL+"/capture", requestID)
	if err != nil {
		t.Fatal(err)
	}

	if len(payload.Documents) != 0 || len(payload.UnknownParts) != 1 ||
		payload.UnknownParts[0].FieldName != "multi-sharing" || !bytes.Equal(payload.UnknownParts[0].Data, archive) {
		t.Fatal("multi-sharing ZIP was not retained as an archive part")
	}
}

func TestLoadCapturePreservesTwoIndexedVehicleDocuments(t *testing.T) {
	const requestID = "example-multisharing-123"
	fileIDs := []string{
		"11111111-1111-1111-1111-111111111111",
		"22222222-2222-2222-2222-222222222222",
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/capture" {
			_ = json.NewEncoder(writer).Encode(capture{
				Method: http.MethodPost,
				Headers: map[string][]string{
					"x-document-request-trace-id": {requestID},
					"content-type":                {"multipart/form-data; boundary=probe"},
				},
				Request: map[string]string{"encodeData": base64.StdEncoding.EncodeToString([]byte("metadata"))},
				Files: map[string]capturedFile{
					"vehicle-license.0": {ID: fileIDs[0], Name: "vehicle-license", Filename: "vehicle-one.pdf.p7s.p7e"},
					"vehicle-license.1": {ID: fileIDs[1], Name: "vehicle-license", Filename: "vehicle-two.pdf.p7s.p7e"},
				},
			})

			return
		}

		for index, id := range fileIDs {
			if request.URL.Path == "/capture/download/"+id {
				_, _ = io.WriteString(writer, base64.StdEncoding.EncodeToString([]byte{byte(index + 1)}))

				return
			}
		}

		http.NotFound(writer, request)
	}))
	defer server.Close()

	payload, err := loadCaptureURL(context.Background(), server.Client(), server.URL+"/capture", requestID)
	if err != nil {
		t.Fatal(err)
	}

	if len(payload.Documents) != 2 || string(payload.EncodeData) != "metadata" {
		t.Fatalf("capture documents = %d", len(payload.Documents))
	}

	for index, document := range payload.Documents {
		if document.Type != diia.DocumentTypeVehicleLicense || document.FieldName != "vehicle-license" ||
			!strings.Contains(document.FileName, "vehicle-") || len(document.Envelope) != 1 ||
			document.Envelope[0] != byte(index+1) {
			t.Fatalf("document %d was not preserved", index)
		}
	}
}

func TestParseReconstructedCallbackWithoutBearer(t *testing.T) {
	const requestID = "example-multisharing-123"
	captured := &diia.CallbackPayload{
		RequestID:  requestID,
		EncodeData: []byte("encrypted-metadata"),
		Documents: []diia.CallbackDocument{
			{
				Type:      diia.DocumentTypeVehicleLicense,
				FieldName: "vehicle-license",
				FileName:  "vehicle-one.p7e",
				Envelope:  []byte{1},
			},
			{
				Type:      diia.DocumentTypeVehicleLicense,
				FieldName: "vehicle-license",
				FileName:  "vehicle-two.p7e",
				Envelope:  []byte{2},
			},
		},
		Signatures: []diia.CallbackSignature{
			{FieldName: "signature-vehicle-license", FileName: "vehicle.sig", Signed: []byte{3}},
		},
		UnknownParts: []diia.CallbackPart{
			{FieldName: "multi-sharing", FileName: "multi-sharing.zip", Data: []byte{4, 5}},
		},
	}
	client := diia.NewClient("unused", diia.WithCallbackToken("configured-token"),
		diia.WithCallbackRequestIDAuthorizer(func(_ context.Context, got string) (bool, error) {
			return got == requestID, nil
		}))
	parsed, err := parseReconstructedCallback(context.Background(), client, captured)
	if err != nil {
		t.Fatal(err)
	}

	if !parsed.MissingBearerAuth || parsed.RequestID != requestID || !bytes.Equal(parsed.EncodeData, captured.EncodeData) ||
		len(parsed.Documents) != 2 ||
		len(parsed.Signatures) != 1 ||
		len(parsed.UnknownParts) != 1 {
		t.Fatalf("reconstructed callback was not parsed completely: %+v", parsed)
	}

	for index, document := range parsed.Documents {
		if document.Type != diia.DocumentTypeVehicleLicense ||
			document.FileName != captured.Documents[index].FileName ||
			!bytes.Equal(document.Envelope, captured.Documents[index].Envelope) {
			t.Fatalf("vehicle document %d was not preserved", index)
		}
	}

	if !bytes.Equal(parsed.UnknownParts[0].Data, captured.UnknownParts[0].Data) ||
		!bytes.Equal(parsed.Signatures[0].Signed, captured.Signatures[0].Signed) {
		t.Fatal("ZIP or signature bytes changed during callback parse")
	}
}

func TestMandatoryDocumentTypes(t *testing.T) {
	types, err := mandatoryDocumentTypes("internal-passport,vehicle-license")

	if err != nil || len(types) != 2 || types[0] != diia.DocumentTypeInternalPassport ||
		types[1] != diia.DocumentTypeVehicleLicense {
		t.Fatalf("valid mandatory types: %v %v", types, err)
	}

	for _, value := range []string{"internal-passport,", "unknown", "vehicle-license,vehicle-license"} {
		if _, err := mandatoryDocumentTypes(value); err == nil {
			t.Fatalf("invalid mandatory types accepted: %q", value)
		}
	}
}
