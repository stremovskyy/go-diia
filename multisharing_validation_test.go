package go_diia

import (
	"encoding/json"
	"errors"
	"testing"
)

func multisharingPackage(t *testing.T, metadataJSON string, documents ...DocumentType) *DocumentPackage {
	t.Helper()
	var metadata Metadata

	if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
		t.Fatal(err)
	}

	pkg := &DocumentPackage{
		Metadata:             &metadata,
		RawMetadata:          json.RawMessage(metadataJSON),
		MetadataSealFileName: "metadata.json.p7s",
	}

	for _, documentType := range documents {
		pkg.Documents = append(pkg.Documents, DocumentFile{Type: documentType, PDF: []byte("%PDF-1.7")})
	}

	return pkg
}

func TestValidateMultisharingPackage(t *testing.T) {
	const allTypes = `{"documentTypes":["internal-passport","taxpayer-card","driver-license","vehicle-license"],"errorDocuments":[],"data":{}}`
	complete := multisharingPackage(t, allTypes,
		DocumentTypeInternalPassport, DocumentTypeTaxpayerCard, DocumentTypeDriverLicense,
		DocumentTypeVehicleLicense, DocumentTypeVehicleLicense)

	if err := ValidateMultisharingPackage(complete,
		DocumentTypeInternalPassport, DocumentTypeTaxpayerCard, DocumentTypeDriverLicense, DocumentTypeVehicleLicense); err != nil {
		t.Fatalf("five PDFs and four mandatory types: %v", err)
	}

	for _, test := range []struct {
		name        string
		metadata    string
		documents   []DocumentType
		mandatory   []DocumentType
		wantError   bool
		wantMessage string
	}{
		{
			name:      "optional document failed",
			metadata:  `{"documentTypes":["internal-passport","taxpayer-card"],"errorDocuments":[{"documentType":"taxpayer-card","result":404}],"data":{}}`,
			documents: []DocumentType{DocumentTypeInternalPassport}, mandatory: []DocumentType{DocumentTypeInternalPassport},
		},
		{
			name:      "Diia omits missing optional passport from document types",
			metadata:  `{"documentTypes":["taxpayer-card","driver-license","vehicle-license"],"errorDocuments":[{"documentType":"internal-passport","result":404}],"data":{}}`,
			documents: []DocumentType{DocumentTypeTaxpayerCard, DocumentTypeDriverLicense, DocumentTypeVehicleLicense, DocumentTypeVehicleLicense},
			mandatory: []DocumentType{DocumentTypeTaxpayerCard, DocumentTypeDriverLicense},
		},
		{
			name:      "missing passport is mandatory despite typed error",
			metadata:  `{"documentTypes":["taxpayer-card","driver-license","vehicle-license"],"errorDocuments":[{"documentType":"internal-passport","result":404}],"data":{}}`,
			documents: []DocumentType{DocumentTypeTaxpayerCard, DocumentTypeDriverLicense, DocumentTypeVehicleLicense},
			mandatory: []DocumentType{DocumentTypeInternalPassport}, wantError: true,
		},
		{
			name:      "mandatory document failed",
			metadata:  `{"documentTypes":["internal-passport","taxpayer-card"],"errorDocuments":[{"documentType":"taxpayer-card","result":404}],"data":{}}`,
			documents: []DocumentType{DocumentTypeInternalPassport}, mandatory: []DocumentType{DocumentTypeTaxpayerCard}, wantError: true,
		},
		{
			name:      "declared type without PDF or error",
			metadata:  allTypes,
			documents: []DocumentType{DocumentTypeInternalPassport}, wantError: true,
		},
		{
			name:      "PDF type absent from manifest",
			metadata:  `{"documentTypes":["internal-passport"],"errorDocuments":[],"data":{}}`,
			documents: []DocumentType{DocumentTypeInternalPassport, DocumentTypeVehicleLicense}, wantError: true,
		},
		{
			name:      "unknown error result",
			metadata:  `{"documentTypes":["internal-passport"],"errorDocuments":[{"documentType":"vehicle-license","result":418}],"data":{}}`,
			documents: []DocumentType{DocumentTypeInternalPassport}, wantError: true,
			wantMessage: "multi-sharing metadata has invalid document error result",
		},
		{
			name:      "unknown error document type",
			metadata:  `{"documentTypes":["internal-passport"],"errorDocuments":[{"documentType":"future-document","result":404}],"data":{}}`,
			documents: []DocumentType{DocumentTypeInternalPassport}, wantError: true,
			wantMessage: "multi-sharing metadata has an unsupported document error type",
		},
		{
			name:      "typed optional error absent from manifest",
			metadata:  `{"documentTypes":["internal-passport"],"errorDocuments":[{"documentType":"vehicle-license","result":404}],"data":{}}`,
			documents: []DocumentType{DocumentTypeInternalPassport},
		},
		{
			name:      "error conflicts with PDF",
			metadata:  `{"documentTypes":["internal-passport","taxpayer-card"],"errorDocuments":[{"documentType":"internal-passport","result":404}],"data":{}}`,
			documents: []DocumentType{DocumentTypeInternalPassport, DocumentTypeTaxpayerCard}, wantError: true,
			wantMessage: "multi-sharing metadata document error conflicts with PDF",
		},
		{
			name:      "duplicate typed errors",
			metadata:  `{"documentTypes":["taxpayer-card"],"errorDocuments":[{"documentType":"internal-passport","result":404},{"documentType":"internal-passport","result":404}],"data":{}}`,
			documents: []DocumentType{DocumentTypeTaxpayerCard}, wantError: true,
			wantMessage: "multi-sharing metadata has duplicate document error type",
		},
		{
			name:      "legacy error string",
			metadata:  `{"documentTypes":["internal-passport"],"errorDocuments":["vehicle-license"],"data":{}}`,
			documents: []DocumentType{DocumentTypeInternalPassport}, wantError: true,
		},
		{
			name:      "duplicate declaration",
			metadata:  `{"documentTypes":["internal-passport","internal-passport"],"errorDocuments":[],"data":{}}`,
			documents: []DocumentType{DocumentTypeInternalPassport}, wantError: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			pkg := multisharingPackage(t, test.metadata, test.documents...)
			err := ValidateMultisharingPackage(pkg, test.mandatory...)

			if test.wantError && !errors.Is(err, ErrCallback) {
				t.Fatalf("invalid package accepted: %v", err)
			}

			if !test.wantError && err != nil {
				t.Fatalf("valid package rejected: %v", err)
			}
			if test.wantMessage != "" && (err == nil || err.Error() != ErrCallback.Error()+": "+test.wantMessage) {
				t.Fatalf("wrong fixed validation reason: got %v, want %q", err, test.wantMessage)
			}
		})
	}
}
