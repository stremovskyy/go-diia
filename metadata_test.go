package go_diia

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSanitizedMetadataCatalogSnapshot(t *testing.T) {
	wire, err := os.ReadFile(filepath.Join("testdata", "metadata", "catalog-2026-11-09.json"))
	if err != nil {
		t.Fatal(err)
	}

	var metadata Metadata

	if err := json.Unmarshal(wire, &metadata); err != nil {
		t.Fatal(err)
	}

	checks := map[string]bool{
		"internal passport": metadata.Data.InternalPassports[0].Registration != nil &&
			metadata.Data.InternalPassports[0].Registration.Region != nil &&
			*metadata.Data.InternalPassports[0].Registration.Region == "Example region",
		"foreign passport": metadata.Data.ForeignPassports[0].CountryCode == "UKR",
		"taxpayer card":    metadata.Data.TaxpayerCards[0].DocNumber == "0000000001",
		"IDP": metadata.Data.InternallyDisplaced[0].Address != nil &&
			metadata.Data.InternallyDisplaced[0].Address.Actual == "Example actual",
		"user birth record": metadata.Data.UserBirthRecords[0].Child.FirstName == "Example",
		"birth certificate": metadata.Data.BirthCertificates[0].Act.Number == "ACT0001",
		"child taxpayer":    metadata.Data.ChildTaxpayerCards[0].DocStatus == "1",
		"driver license":    metadata.Data.DriverLicenses[0].UA.DocumentNumber.Value == "DL0001",
		"vehicle license": metadata.Data.VehicleLicenses[0].UA.PowerKWT != nil &&
			metadata.Data.VehicleLicenses[0].UA.PowerKWT.Value == "100",
		"student card":        metadata.Data.StudentIDCards[0].CollegeName == "Example University",
		"permanent residence": metadata.Data.PermanentResidencePermits[0].DocNumber == "RP0001",
		"temporary residence": metadata.Data.TemporaryResidencePermits[0].Registration != nil &&
			*metadata.Data.TemporaryResidencePermits[0].Registration == "Example city",
		"pension card": metadata.Data.PensionCards[0].PensionType == "Example pension",
		"education variants": len(metadata.Data.EducationDocuments) == 4 &&
			metadata.Data.EducationDocuments[3].SubType == "diplomaOfJuniorSpecialist",
		"veteran certificate": metadata.Data.VeteranCertificates[0].LawArticle == "Example article",
		"name act":            metadata.Data.NameChangeActRecords[0].OldLastName != nil,
		"marriage act":        metadata.Data.MarriageActRecords[0].PartnerSecond.FirstName == "Second",
		"divorce act":         metadata.Data.DivorceActRecords[0].ReasonDivorce != nil,
		"income report":       metadata.Data.IncomeReports[0].Info.SourcesOfIncome[0].IncomeTaxMilitary.IncomeAccrued == "100.50",
		"ok5":                 metadata.Data.OK5[0].InsuredData[0].MonthlyData[0].Sum2 == "200",
		"ok7":                 metadata.Data.OK7[0].InsuredData[0].Year == "2026",
	}

	for name, valid := range checks {
		if !valid {
			t.Errorf("catalog fixture did not populate %s", name)
		}
	}

	if !reflect.DeepEqual(metadata.ErrorDocuments, StringList{"future-document", "7"}) {
		t.Fatalf("errorDocuments = %#v", metadata.ErrorDocuments)
	}

	if _, ok := metadata.Data.Unknown["future-document-2027"]; !ok {
		t.Fatal("unknown metadata was not preserved")
	}
}

func TestMetadataDecodesEveryCurrentDocumentWireKey(t *testing.T) {
	fixtures := []struct {
		key     DocumentType
		payload string
		count   func(MetadataData) int
	}{
		{DocumentTypeInternalPassport, `{}`, func(d MetadataData) int { return len(d.InternalPassports) }},
		{DocumentTypeForeignPassport, `{}`, func(d MetadataData) int { return len(d.ForeignPassports) }},
		{DocumentTypeTaxpayerCard, `{}`, func(d MetadataData) int { return len(d.TaxpayerCards) }},
		{
			DocumentTypeReferenceInternallyDisplaced,
			`{}`,
			func(d MetadataData) int { return len(d.InternallyDisplaced) },
		},
		{DocumentTypeUserBirthRecord, `{}`, func(d MetadataData) int { return len(d.UserBirthRecords) }},
		{DocumentTypeBirthCertificate, `{}`, func(d MetadataData) int { return len(d.BirthCertificates) }},
		{DocumentTypeChildTaxpayerCard, `{}`, func(d MetadataData) int { return len(d.ChildTaxpayerCards) }},
		{DocumentTypeDriverLicense, `{}`, func(d MetadataData) int { return len(d.DriverLicenses) }},
		{DocumentTypeVehicleLicense, `{}`, func(d MetadataData) int { return len(d.VehicleLicenses) }},
		{DocumentTypeStudentIDCard, `{}`, func(d MetadataData) int { return len(d.StudentIDCards) }},
		{
			DocumentTypeResidencePermitPermanent,
			`{}`,
			func(d MetadataData) int { return len(d.PermanentResidencePermits) },
		},
		{
			DocumentTypeResidencePermitTemporary,
			`{}`,
			func(d MetadataData) int { return len(d.TemporaryResidencePermits) },
		},
		{DocumentTypePensionCard, `{}`, func(d MetadataData) int { return len(d.PensionCards) }},
		{
			DocumentTypeEducationDocument,
			`[{"subType":"diplomaOfBachelor"},{"subType":"attestationOfCompleteGeneralSecondaryEducation"},{"subType":"certificateOfBasicGeneralSecondaryEducation"},{"subType":"diplomaOfJuniorSpecialist"}]`,
			func(d MetadataData) int { return len(d.EducationDocuments) },
		},
		{DocumentTypeVeteranCertificate, `{}`, func(d MetadataData) int { return len(d.VeteranCertificates) }},
		{DocumentTypeNameChangeActRecord, `{}`, func(d MetadataData) int { return len(d.NameChangeActRecords) }},
		{DocumentTypeMarriageActRecord, `{}`, func(d MetadataData) int { return len(d.MarriageActRecords) }},
		{DocumentTypeDivorceActRecord, `{}`, func(d MetadataData) int { return len(d.DivorceActRecords) }},
		{DocumentTypeIncomeReport, `{}`, func(d MetadataData) int { return len(d.IncomeReports) }},
		{DocumentTypeOK5, `{}`, func(d MetadataData) int { return len(d.OK5) }},
		{DocumentTypeOK7, `{}`, func(d MetadataData) int { return len(d.OK7) }},
	}

	for _, fixture := range fixtures {
		t.Run(string(fixture.key), func(t *testing.T) {
			wire := []byte(`{"data":{"` + string(fixture.key) + `":` + fixture.payload + `}}`)
			var metadata Metadata

			if err := json.Unmarshal(wire, &metadata); err != nil {
				t.Fatal(err)
			}

			want := 1

			if fixture.key == DocumentTypeEducationDocument {
				want = 4
			}

			if got := fixture.count(metadata.Data); got != want {
				t.Fatalf("normalized count = %d, want %d", got, want)
			}
		})
	}
}

func TestMetadataCommonNestedTypesAndTolerances(t *testing.T) {
	wire := []byte(`{
		"requestId":"request-123",
		"errorDocuments":"foreign-passport",
		"data":{
			"internal-passport":{"registration":{"region":"Kyiv","cancelregistration_date":null}},
			"income-report":{"Info":{"SourcesOfIncome":[{"IncomeTaxMilitary":{"IncomeAccrued":123.45,"IncomePaid":"100.00"}}]}},
			"ok5":{"insuredData":[{"year":"2026","monthlyData":[{"sum1":10,"sum2":"20","monthExp":"3"}]}]},
			"unknown-2027":{"new":true}
		}
	}`)
	var metadata Metadata

	if err := json.Unmarshal(wire, &metadata); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(metadata.ErrorDocuments, StringList{"foreign-passport"}) {
		t.Fatalf("errorDocuments = %#v", metadata.ErrorDocuments)
	}

	registration := metadata.Data.InternalPassports[0].Registration

	if registration == nil || registration.Region == nil || *registration.Region != "Kyiv" ||
		registration.CancelRegistrationDate != nil {
		t.Fatalf("registration = %#v", registration)
	}

	income := metadata.Data.IncomeReports[0].Info.SourcesOfIncome[0].IncomeTaxMilitary

	if income.IncomeAccrued != "123.45" || income.IncomePaid == nil || *income.IncomePaid != "100.00" {
		t.Fatalf("income = %#v", income)
	}

	monthly := metadata.Data.OK5[0].InsuredData[0].MonthlyData[0]

	if monthly.Sum1 != "10" || monthly.Sum2 != "20" || monthly.MonthExp != "3" {
		t.Fatalf("monthly = %#v", monthly)
	}

	if _, ok := metadata.Data.Unknown["unknown-2027"]; !ok {
		t.Fatal("unknown document metadata not preserved")
	}
}

func TestRegistrationCountryIDAcceptsNumberAndString(t *testing.T) {
	cases := []struct {
		name string
		wire string
		want *string
	}{
		{"number", `{"country_id":804,"region":"Example region"}`, ptr("804")},
		{"string", `{"country_id":"804","region":"Example region"}`, ptr("804")},
		{"null", `{"country_id":null,"region":"Example region"}`, nil},
		{"absent", `{"region":"Example region"}`, nil},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var registration Registration
			if err := json.Unmarshal([]byte(test.wire), &registration); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(registration.CountryID, test.want) {
				t.Fatalf("country ID = %v, want %v", registration.CountryID, test.want)
			}
			if registration.Region == nil || *registration.Region != "Example region" {
				t.Fatalf("region was not decoded: %#v", registration.Region)
			}
		})
	}
}

func ptr(value string) *string { return &value }

func TestStringListAcceptsListAndMixedDocumentedValues(t *testing.T) {
	var values StringList

	if err := json.Unmarshal([]byte(`["one",2,null,{"code":"x"}]`), &values); err != nil {
		t.Fatal(err)
	}

	want := StringList{"one", "2", `{"code":"x"}`}

	if !reflect.DeepEqual(values, want) {
		t.Fatalf("values = %#v, want %#v", values, want)
	}
}

func TestFlexibleMetadataValuesIgnoreOuterJSONWhitespace(t *testing.T) {
	var number FlexibleNumber

	if err := json.Unmarshal([]byte(" \n 123.40\t"), &number); err != nil {
		t.Fatal(err)
	}

	if number != "123.40" {
		t.Fatalf("number = %q", number)
	}

	marshaled, err := json.Marshal(number)
	if err != nil {
		t.Fatal(err)
	}

	if string(marshaled) != `"123.40"` {
		t.Fatalf("marshaled number = %s", marshaled)
	}

	var values StringList

	if err := json.Unmarshal([]byte(" \n [\"one\", 2] \t"), &values); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(values, StringList{"one", "2"}) {
		t.Fatalf("values = %#v", values)
	}
}

func TestMetadataDecodesNumericStatusesAndActRecordWireLayout(t *testing.T) {
	wire := []byte(`{
		"data":{
			"name-change-act-record":{
				"baseData":{"status":1,"issuedAt":"2026-01-01"},
				"docStatus":"2","isRestore":"true","gender":"male",
				"firstName":"Olena","oldLastName":"Old",
				"addressDetails":{"state":"Ukraine","disctrict":"Kyiv"},
				"certificates":[{"status":"valid","serie":"AA","serieNumber":"42"}]
			},
			"education-document":{"docStatus":3,"subType":"master"},
			"residence-permit-temporary":{"docNumber":"TT1","registration":"Kyiv"},
			"income-report":{
				"Info":{"RNOKPP":"0000000000"},
				"SourcesOfIncome":[{"IncomeTaxMilitary":{"period_year":2026}}],
				"period":{"period_begin_month":1,"period_begin_year":"2025","period_end_month":12,"period_end_year":2025}
			}
		}
	}`)
	var metadata Metadata

	if err := json.Unmarshal(wire, &metadata); err != nil {
		t.Fatal(err)
	}

	act := metadata.Data.NameChangeActRecords[0]

	if act.BaseData == nil || act.BaseData.Status != "1" || act.DocStatus != "2" || act.FirstName != "Olena" ||
		act.AddressDetails == nil ||
		act.AddressDetails.MisspelledDistrict == nil {
		t.Fatalf("act record = %#v", act)
	}

	if metadata.Data.EducationDocuments[0].DocStatus != "3" {
		t.Fatalf("education = %#v", metadata.Data.EducationDocuments[0])
	}

	permit := metadata.Data.TemporaryResidencePermits[0]

	if permit.Registration == nil || *permit.Registration != "Kyiv" {
		t.Fatalf("temporary residence permit = %#v", permit)
	}

	income := metadata.Data.IncomeReports[0]

	if len(income.SourcesOfIncome) != 1 || income.SourcesOfIncome[0].IncomeTaxMilitary.PeriodYear == nil ||
		*income.SourcesOfIncome[0].IncomeTaxMilitary.PeriodYear != "2026" ||
		income.Period.BeginMonth != "1" ||
		income.Period.EndYear != "2025" {
		t.Fatalf("income = %#v", income)
	}
}
