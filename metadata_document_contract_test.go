package go_diia

import (
	"encoding/json"
	"fmt"
	"testing"
)

// These small payloads exercise the catalog's field names without copying
// personal data from its examples into the repository.
func TestMetadataCatalogFieldNameVariants(t *testing.T) {
	tests := []struct {
		name string
		wire string
		got  func(Metadata) string
	}{
		{
			name: "birth record example serie Original",
			wire: `{"data":{"user-birth-record":{"document":{"serie Original":"AB"}}}}`,
			got:  func(m Metadata) string { return m.Data.UserBirthRecords[0].Document.SerieOriginal },
		},
		{
			name: "birth record table serieOriginal",
			wire: `{"data":{"user-birth-record":{"document":{"serieOriginal":"AB"}}}}`,
			got:  func(m Metadata) string { return m.Data.UserBirthRecords[0].Document.SerieOriginal },
		},
		{
			name: "birth certificate example rnokpp question mark",
			wire: `{"data":{"birth-certificate":{"child":{"rnokpp?":"AB"}}}}`,
			got:  func(m Metadata) string { return m.Data.BirthCertificates[0].Child.RNOKPP },
		},
		{
			name: "birth certificate table rnokpp",
			wire: `{"data":{"birth-certificate":{"child":{"rnokpp":"AB"}}}}`,
			got:  func(m Metadata) string { return m.Data.BirthCertificates[0].Child.RNOKPP },
		},
		{
			name: "vehicle example powerKWT question mark",
			wire: `{"data":{"vehicle-license":{"eng":{"powerKWT?":{"value":"AB"}}}}}`,
			got:  func(m Metadata) string { return catalogLocalizedValue(m.Data.VehicleLicenses[0].ENG.PowerKWT) },
		},
		{
			name: "vehicle table powerKWT",
			wire: `{"data":{"vehicle-license":{"eng":{"powerKWT":{"value":"AB"}}}}}`,
			got:  func(m Metadata) string { return catalogLocalizedValue(m.Data.VehicleLicenses[0].ENG.PowerKWT) },
		},
		{
			name: "student example middleNameUA",
			wire: `{"data":{"student-id-card":{"middleNameUA":"AB"}}}`,
			got:  func(m Metadata) string { return m.Data.StudentIDCards[0].MiddleNameUA },
		},
		{
			name: "student table middNameUA",
			wire: `{"data":{"student-id-card":{"middNameUA":"AB"}}}`,
			got:  func(m Metadata) string { return m.Data.StudentIDCards[0].MiddleNameUA },
		},
		{
			name: "education example issuedByUniversityNameEn",
			wire: `{"data":{"education-document":{"issuedByUniversityNameEn":"AB"}}}`,
			got: func(m Metadata) string {
				return catalogOptionalString(m.Data.EducationDocuments[0].IssuedByUniversityNameEN)
			},
		},
		{
			name: "education table issueByUniversityNameEn",
			wire: `{"data":{"education-document":{"issueByUniversityNameEn":"AB"}}}`,
			got: func(m Metadata) string {
				return catalogOptionalString(m.Data.EducationDocuments[0].IssuedByUniversityNameEN)
			},
		},
		{
			name: "education example specialityNameEn",
			wire: `{"data":{"education-document":{"specialityNameEn":"AB"}}}`,
			got:  func(m Metadata) string { return catalogOptionalString(m.Data.EducationDocuments[0].SpecialtyNameEN) },
		},
		{
			name: "education table specialtyNameEn",
			wire: `{"data":{"education-document":{"specialtyNameEn":"AB"}}}`,
			got:  func(m Metadata) string { return catalogOptionalString(m.Data.EducationDocuments[0].SpecialtyNameEN) },
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var metadata Metadata
			if err := json.Unmarshal([]byte(test.wire), &metadata); err != nil {
				t.Fatal(err)
			}
			if got := test.got(metadata); got != "AB" {
				t.Fatalf("decoded field = %q, want AB", got)
			}
		})
	}
}

func catalogOptionalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func catalogLocalizedValue(value *LocalizedField) string {
	if value == nil {
		return ""
	}
	return value.Value
}

func TestMetadataCatalogIncomeNestedEmploymentDates(t *testing.T) {
	const wire = `{"data":{"income-report":{"SourcesOfIncome":[{"IncomeTaxMilitary":{"DateOfDismissal":"2026-01-02","DateOfEmployment":"2025-01-02"}}]}}}`
	var metadata Metadata
	if err := json.Unmarshal([]byte(wire), &metadata); err != nil {
		t.Fatal(err)
	}
	income := metadata.Data.IncomeReports[0].SourcesOfIncome[0].IncomeTaxMilitary
	if income.DateOfDismissal == nil || *income.DateOfDismissal != "2026-01-02" {
		t.Fatalf("nested DateOfDismissal = %v", income.DateOfDismissal)
	}
	if income.DateOfEmployment == nil || *income.DateOfEmployment != "2025-01-02" {
		t.Fatalf("nested DateOfEmployment = %v", income.DateOfEmployment)
	}
}

func TestMetadataCatalogEducationSubtypes(t *testing.T) {
	const wire = `{"data":{"education-document":[{"subType":"diplomaOfBachelor"},{"subType":"attestationOfCompleteGeneralSecondaryEducation"},{"subType":"certificateOfBasicGeneralSecondaryEducation"},{"subType":"diplomaOfJuniorSpecialist"}]}}`
	var metadata Metadata
	if err := json.Unmarshal([]byte(wire), &metadata); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"diplomaOfBachelor",
		"attestationOfCompleteGeneralSecondaryEducation",
		"certificateOfBasicGeneralSecondaryEducation",
		"diplomaOfJuniorSpecialist",
	}
	if len(metadata.Data.EducationDocuments) != len(want) {
		t.Fatalf("education documents = %d, want %d", len(metadata.Data.EducationDocuments), len(want))
	}
	for i, expected := range want {
		if got := metadata.Data.EducationDocuments[i].SubType; got != expected {
			t.Errorf("subType[%d] = %q, want %q", i, got, expected)
		}
	}
}

func TestMetadataCatalogMultiSharingErrorsStayStructured(t *testing.T) {
	const wire = `{"errorDocuments":[{"documentType":"internal-passport","result":404}],"data":{}}`
	var metadata Metadata
	if err := json.Unmarshal([]byte(wire), &metadata); err != nil {
		t.Fatal(err)
	}
	if len(metadata.ErrorDocumentDetails) != 1 {
		t.Fatalf("error details = %d, want 1", len(metadata.ErrorDocumentDetails))
	}
	detail := metadata.ErrorDocumentDetails[0]
	if detail.DocumentType != DocumentTypeInternalPassport || detail.Result != 404 {
		t.Fatalf("error detail = %#v", detail)
	}
	if len(metadata.ErrorDocuments) != 1 || metadata.ErrorDocuments[0] == "" {
		t.Fatal("legacy errorDocuments view is unavailable")
	}

	encoded, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	var output struct {
		ErrorDocuments []struct {
			DocumentType DocumentType `json:"documentType"`
			Result       int          `json:"result"`
		} `json:"errorDocuments"`
	}
	if err := json.Unmarshal(encoded, &output); err != nil {
		t.Fatalf("errorDocuments was not an object array: %v", err)
	}
	if len(output.ErrorDocuments) != 1 || output.ErrorDocuments[0].DocumentType != DocumentTypeInternalPassport || output.ErrorDocuments[0].Result != 404 {
		t.Fatalf("marshaled errorDocuments = %#v", output.ErrorDocuments)
	}
}

func TestMetadataLegacyScalarErrorDocumentsStillDecode(t *testing.T) {
	const wire = `{"errorDocuments":["taxpayer-card",7,null],"data":{}}`
	var metadata Metadata
	if err := json.Unmarshal([]byte(wire), &metadata); err != nil {
		t.Fatal(err)
	}
	if len(metadata.ErrorDocuments) != 2 || metadata.ErrorDocuments[0] != "taxpayer-card" || metadata.ErrorDocuments[1] != "7" {
		t.Fatalf("legacy errorDocuments = %#v", metadata.ErrorDocuments)
	}
	if len(metadata.ErrorDocumentDetails) != 0 {
		t.Fatalf("legacy scalar values should not create object details: %#v", metadata.ErrorDocumentDetails)
	}
}

func TestMetadataCatalogCategoricalValues(t *testing.T) {
	const wire = `{"data":{"internal-passport":{"genderUA":"Ж","genderEN":"F"},"reference-internally-displaced-person":{"docType":"person","gender":"Жінка"},"pension-card":{"gender":"Ч"},"marriage-act-record":{"isRestore":"1","arVerificationSt":"0","partnerFirst":{"gender":"male"},"certificates":[{"status":"1"}]},"income-report":[{"SourcesOfIncome":[{"IncomeTaxMilitary":{"result_income":14}}]},{"SourcesOfIncome":[{"IncomeTaxMilitary":{"result_income":"2"}}]}]}}`
	var metadata Metadata
	if err := json.Unmarshal([]byte(wire), &metadata); err != nil {
		t.Fatal(err)
	}
	passport := metadata.Data.InternalPassports[0]
	if passport.GenderUA != PassportGenderUAFemale || passport.GenderEN != PassportGenderENFemale {
		t.Fatalf("passport gender codes = %q / %q", passport.GenderUA, passport.GenderEN)
	}
	idp := metadata.Data.InternallyDisplaced[0]
	if idp.DocType != IDPDocumentTypePerson || idp.Gender != IDPGenderFemale {
		t.Fatalf("IDP codes = %q / %q", idp.DocType, idp.Gender)
	}
	pension := metadata.Data.PensionCards[0]
	if pension.Gender == nil || *pension.Gender != PensionGenderMale {
		t.Fatalf("pension gender = %v", pension.Gender)
	}
	act := metadata.Data.MarriageActRecords[0]
	if act.IsRestore == nil || *act.IsRestore != ActRecordRestoreFlagOne ||
		act.ARVerification == nil || *act.ARVerification != ActRecordVerificationStatusZero ||
		act.PartnerFirst.Gender == nil || *act.PartnerFirst.Gender != ActRecordGenderMale ||
		len(act.Certificates) != 1 || act.Certificates[0].Status != ActRecordCertificateStatusOne {
		t.Fatalf("act record codes were not decoded")
	}
	if len(metadata.Data.IncomeReports) != 2 {
		t.Fatalf("income reports = %d, want 2", len(metadata.Data.IncomeReports))
	}
	for i, want := range []IncomeResultCode{IncomeResultCode14, IncomeResultCode2} {
		got := metadata.Data.IncomeReports[i].SourcesOfIncome[0].IncomeTaxMilitary.ResultIncome
		if got == nil || *got != want || !got.Valid() {
			t.Fatalf("result_income[%d] = %v, want %q", i, got, want)
		}
	}
}

func TestMetadataCatalogEnumsKeepUnknownValuesReadable(t *testing.T) {
	const wire = `{"data":{"reference-internally-displaced-person":{"docType":"future-type"},"income-report":{"SourcesOfIncome":[{"IncomeTaxMilitary":{"result_income":"99"}}]}}}`
	var metadata Metadata
	if err := json.Unmarshal([]byte(wire), &metadata); err != nil {
		t.Fatal(err)
	}
	docType := metadata.Data.InternallyDisplaced[0].DocType
	if docType != IDPDocumentType("future-type") || docType.Valid() {
		t.Fatalf("unknown docType = %q", docType)
	}
	result := metadata.Data.IncomeReports[0].SourcesOfIncome[0].IncomeTaxMilitary.ResultIncome
	if result == nil || *result != IncomeResultCode("99") || result.Valid() {
		t.Fatalf("unknown result_income = %v", result)
	}
}

func TestMetadataCatalogIncomeIDRequestExample(t *testing.T) {
	const wire = `{"data":{"income-report":{"IDrequest":"example-request"}}}`
	var metadata Metadata
	if err := json.Unmarshal([]byte(wire), &metadata); err != nil {
		t.Fatal(err)
	}
	if got := metadata.Data.IncomeReports[0].IDRequest; got == nil || *got != "example-request" {
		t.Fatalf("IDrequest = %v", got)
	}
}

func TestMetadataCatalogEnumWireValuesAndValidation(t *testing.T) {
	tests := []struct {
		name  string
		value interface{ Valid() bool }
		wire  string
		valid bool
	}{
		{"passport Ukrainian", PassportGenderUAFemale, "Ж", true},
		{"passport Latin", PassportGenderENMale, "M", true},
		{"IDP document owner", IDPDocumentTypeAccompanied, "accompanied", true},
		{"IDP gender", IDPGenderMale, "Чоловік", true},
		{"pension gender", PensionGenderFemale, "Ж", true},
		{"civil record gender", ActRecordGenderFemale, "female", true},
		{"restoration flag", ActRecordRestoreFlagOne, "1", true},
		{"verification status", ActRecordVerificationStatusZero, "0", true},
		{"certificate status", ActRecordCertificateStatusOne, "1", true},
		{"income result", IncomeResultCode14, "14", true},
		{"future income result", IncomeResultCode("99"), "99", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := fmt.Sprint(test.value); got != test.wire {
				t.Fatalf("wire value = %q, want %q", got, test.wire)
			}
			if got := test.value.Valid(); got != test.valid {
				t.Fatalf("Valid() = %t, want %t", got, test.valid)
			}
		})
	}
}
