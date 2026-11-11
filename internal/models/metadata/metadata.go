package metadata

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/stremovskyy/go-diia/internal/models/domain"
)

type DocumentType = domain.DocumentType

const (
	DocumentTypeInternalPassport             = domain.DocumentTypeInternalPassport
	DocumentTypeForeignPassport              = domain.DocumentTypeForeignPassport
	DocumentTypeTaxpayerCard                 = domain.DocumentTypeTaxpayerCard
	DocumentTypeReferenceInternallyDisplaced = domain.DocumentTypeReferenceInternallyDisplaced
	DocumentTypeUserBirthRecord              = domain.DocumentTypeUserBirthRecord
	DocumentTypeBirthCertificate             = domain.DocumentTypeBirthCertificate
	DocumentTypeChildTaxpayerCard            = domain.DocumentTypeChildTaxpayerCard
	DocumentTypeDriverLicense                = domain.DocumentTypeDriverLicense
	DocumentTypeVehicleLicense               = domain.DocumentTypeVehicleLicense
	DocumentTypeStudentIDCard                = domain.DocumentTypeStudentIDCard
	DocumentTypeResidencePermitPermanent     = domain.DocumentTypeResidencePermitPermanent
	DocumentTypeResidencePermitTemporary     = domain.DocumentTypeResidencePermitTemporary
	DocumentTypePensionCard                  = domain.DocumentTypePensionCard
	DocumentTypeEducationDocument            = domain.DocumentTypeEducationDocument
	DocumentTypeVeteranCertificate           = domain.DocumentTypeVeteranCertificate
	DocumentTypeNameChangeActRecord          = domain.DocumentTypeNameChangeActRecord
	DocumentTypeMarriageActRecord            = domain.DocumentTypeMarriageActRecord
	DocumentTypeDivorceActRecord             = domain.DocumentTypeDivorceActRecord
	DocumentTypeIncomeReport                 = domain.DocumentTypeIncomeReport
	DocumentTypeOK5                          = domain.DocumentTypeOK5
	DocumentTypeOK7                          = domain.DocumentTypeOK7
)

// Metadata is the normalized metadata envelope decrypted from encodeData.
// Every document collection is a slice: offline singleton objects and online
// arrays therefore have one stable representation.
type Metadata struct {
	RequestID      string     `json:"requestId,omitempty"`
	OfferRequestID string     `json:"offerRequestId,omitempty"`
	Barcode        string     `json:"barcode,omitempty"`
	ErrorDocuments StringList `json:"errorDocuments,omitempty"`
	// ErrorDocumentDetails provides typed access to object entries in errorDocuments.
	ErrorDocumentDetails []ErrorDocument `json:"-"`
	DocumentTypes        []DocumentType  `json:"documentTypes,omitempty"`
	Data                 MetadataData    `json:"data"`
	rawJSON              json.RawMessage
	normalizedJSON       []byte
	originalLegacyErrors StringList
	originalErrorDetails []ErrorDocument
}

// MetadataData contains supported Document Sharing wire keys.
type MetadataData struct {
	InternalPassports         []InternalPassport         `json:"internal-passport,omitempty"`
	ForeignPassports          []ForeignPassport          `json:"foreign-passport,omitempty"`
	TaxpayerCards             []TaxpayerCard             `json:"taxpayer-card,omitempty"`
	InternallyDisplaced       []InternallyDisplaced      `json:"reference-internally-displaced-person,omitempty"`
	UserBirthRecords          []BirthRecord              `json:"user-birth-record,omitempty"`
	BirthCertificates         []BirthRecord              `json:"birth-certificate,omitempty"`
	ChildTaxpayerCards        []ChildTaxpayerCard        `json:"child-taxpayer-card,omitempty"`
	DriverLicenses            []DriverLicense            `json:"driver-license,omitempty"`
	VehicleLicenses           []VehicleLicense           `json:"vehicle-license,omitempty"`
	StudentIDCards            []StudentIDCard            `json:"student-id-card,omitempty"`
	PermanentResidencePermits []ResidencePermit          `json:"residence-permit-permanent,omitempty"`
	TemporaryResidencePermits []ResidencePermit          `json:"residence-permit-temporary,omitempty"`
	PensionCards              []PensionCard              `json:"pension-card,omitempty"`
	EducationDocuments        []EducationDocument        `json:"education-document,omitempty"`
	VeteranCertificates       []VeteranCertificate       `json:"veteran-certificate,omitempty"`
	NameChangeActRecords      []NameChangeActRecord      `json:"name-change-act-record,omitempty"`
	MarriageActRecords        []MarriageActRecord        `json:"marriage-act-record,omitempty"`
	DivorceActRecords         []DivorceActRecord         `json:"divorce-act-record,omitempty"`
	IncomeReports             []IncomeReport             `json:"income-report,omitempty"`
	OK5                       []InsuranceStatement       `json:"ok5,omitempty"`
	OK7                       []InsuranceStatement       `json:"ok7,omitempty"`
	Unknown                   map[string]json.RawMessage `json:"-"`
}

func (d *MetadataData) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage

	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("metadata data: %w", err)
	}

	known := map[string]func(json.RawMessage) error{
		string(DocumentTypeInternalPassport):             func(v json.RawMessage) error { return decodeOneOrMany(v, &d.InternalPassports) },
		string(DocumentTypeForeignPassport):              func(v json.RawMessage) error { return decodeOneOrMany(v, &d.ForeignPassports) },
		string(DocumentTypeTaxpayerCard):                 func(v json.RawMessage) error { return decodeOneOrMany(v, &d.TaxpayerCards) },
		string(DocumentTypeReferenceInternallyDisplaced): func(v json.RawMessage) error { return decodeOneOrMany(v, &d.InternallyDisplaced) },
		string(DocumentTypeUserBirthRecord):              func(v json.RawMessage) error { return decodeOneOrMany(v, &d.UserBirthRecords) },
		string(DocumentTypeBirthCertificate):             func(v json.RawMessage) error { return decodeOneOrMany(v, &d.BirthCertificates) },
		string(DocumentTypeChildTaxpayerCard):            func(v json.RawMessage) error { return decodeOneOrMany(v, &d.ChildTaxpayerCards) },
		string(DocumentTypeDriverLicense):                func(v json.RawMessage) error { return decodeOneOrMany(v, &d.DriverLicenses) },
		string(DocumentTypeVehicleLicense):               func(v json.RawMessage) error { return decodeOneOrMany(v, &d.VehicleLicenses) },
		string(DocumentTypeStudentIDCard):                func(v json.RawMessage) error { return decodeOneOrMany(v, &d.StudentIDCards) },
		string(DocumentTypeResidencePermitPermanent):     func(v json.RawMessage) error { return decodeOneOrMany(v, &d.PermanentResidencePermits) },
		string(DocumentTypeResidencePermitTemporary):     func(v json.RawMessage) error { return decodeOneOrMany(v, &d.TemporaryResidencePermits) },
		string(DocumentTypePensionCard):                  func(v json.RawMessage) error { return decodeOneOrMany(v, &d.PensionCards) },
		string(DocumentTypeEducationDocument):            func(v json.RawMessage) error { return decodeOneOrMany(v, &d.EducationDocuments) },
		string(DocumentTypeVeteranCertificate):           func(v json.RawMessage) error { return decodeOneOrMany(v, &d.VeteranCertificates) },
		string(DocumentTypeNameChangeActRecord):          func(v json.RawMessage) error { return decodeOneOrMany(v, &d.NameChangeActRecords) },
		string(DocumentTypeMarriageActRecord):            func(v json.RawMessage) error { return decodeOneOrMany(v, &d.MarriageActRecords) },
		string(DocumentTypeDivorceActRecord):             func(v json.RawMessage) error { return decodeOneOrMany(v, &d.DivorceActRecords) },
		string(DocumentTypeIncomeReport):                 func(v json.RawMessage) error { return decodeOneOrMany(v, &d.IncomeReports) },
		string(DocumentTypeOK5):                          func(v json.RawMessage) error { return decodeOneOrMany(v, &d.OK5) },
		string(DocumentTypeOK7):                          func(v json.RawMessage) error { return decodeOneOrMany(v, &d.OK7) },
	}

	for key, value := range raw {
		decode, ok := known[key]

		if !ok {
			if d.Unknown == nil {
				d.Unknown = make(map[string]json.RawMessage)
			}

			d.Unknown[key] = append(json.RawMessage(nil), value...)
			continue
		}

		if err := decode(value); err != nil {
			return fmt.Errorf("metadata %s: %w", key, err)
		}
	}

	return nil
}

func decodeOneOrMany[T any](data json.RawMessage, target *[]T) error {
	trimmed := bytes.TrimSpace(data)

	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		*target = nil

		return nil
	}

	if trimmed[0] == '[' {
		return json.Unmarshal(trimmed, target)
	}

	var single T

	if err := json.Unmarshal(trimmed, &single); err != nil {
		return err
	}

	*target = []T{single}

	return nil
}

type jsonFieldAlias struct {
	alternate string
	canonical string
}

// unmarshalWithAliases accepts alternate field spellings. The canonical
// spelling wins if both are present.
func unmarshalWithAliases(data []byte, target any, aliases ...jsonFieldAlias) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, alias := range aliases {
		if _, present := fields[alias.canonical]; present {
			continue
		}
		if value, present := fields[alias.alternate]; present {
			fields[alias.canonical] = value
		}
	}
	canonical, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	return json.Unmarshal(canonical, target)
}

// Registration is the common address/registration block used by passports
// and residence permits. Dates remain strings because Diia publishes multiple
// wire formats across document types.
type Registration struct {
	CBCCountry                *string `json:"cbc_country,omitempty"`
	CountryID                 *string `json:"country_id,omitempty"`
	Postbox                   *string `json:"postbox,omitempty"`
	AddressKOATUU             *string `json:"address_koatuu,omitempty"`
	AddressKATOTTH            *string `json:"address_katotth,omitempty"`
	Region                    *string `json:"region,omitempty"`
	RegionDistrict            *string `json:"region_district,omitempty"`
	CityDistrict              *string `json:"city_district,omitempty"`
	SettlementName            *string `json:"settlement_name,omitempty"`
	SettlementType            *string `json:"settlement_type,omitempty"`
	StreetName                *string `json:"street_name,omitempty"`
	StreetType                *string `json:"street_type,omitempty"`
	BuildingNumber            *string `json:"building_number,omitempty"`
	BuildingPart              *string `json:"building_part,omitempty"`
	Apartment                 *string `json:"apartment,omitempty"`
	RegistrationDate          *string `json:"registration_date,omitempty"`
	CancelRegistrationDate    *string `json:"cancelregistration_date,omitempty"`
	AddressCommunityKATOTTH   *string `json:"address_grom_katottg,omitempty"`
	SettlementDistrictKATOTTH *string `json:"settlement_district_katottg,omitempty"`
}

// Diia sends country_id as either a JSON number or a string. Keep the public
// field as *string while accepting both wire formats.
func (registration *Registration) UnmarshalJSON(data []byte) error {
	type plain Registration
	var fields plain
	value := struct {
		*plain
		CountryID json.RawMessage `json:"country_id"`
	}{plain: &fields}

	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if countryID := bytes.TrimSpace(value.CountryID); len(countryID) != 0 && !bytes.Equal(countryID, []byte("null")) {
		var normalized FlexibleNumber
		if err := json.Unmarshal(countryID, &normalized); err != nil {
			return fmt.Errorf("registration country_id: %w", err)
		}
		text := string(normalized)
		fields.CountryID = &text
	}
	*registration = Registration(fields)
	return nil
}

type InternalPassport struct {
	TaxpayerNumber string           `json:"taxpayerNumber,omitempty"`
	ResidenceUA    *string          `json:"residenceUA,omitempty"`
	DocNumber      string           `json:"docNumber,omitempty"`
	GenderUA       PassportGenderUA `json:"genderUA,omitempty"`
	NationalityUA  string           `json:"nationalityUA,omitempty"`
	LastNameUA     string           `json:"lastNameUA,omitempty"`
	FirstNameUA    string           `json:"firstNameUA,omitempty"`
	MiddleNameUA   string           `json:"middleNameUA,omitempty"`
	Birthday       string           `json:"birthday,omitempty"`
	BirthPlaceUA   string           `json:"birthPlaceUA,omitempty"`
	IssueDate      string           `json:"issueDate,omitempty"`
	ExpirationDate string           `json:"expirationDate,omitempty"`
	RecordNumber   string           `json:"recordNumber,omitempty"`
	Department     string           `json:"department,omitempty"`
	GenderEN       PassportGenderEN `json:"genderEN,omitempty"`
	ID             string           `json:"id,omitempty"`
	LastNameEN     string           `json:"lastNameEN,omitempty"`
	FirstNameEN    string           `json:"firstNameEN,omitempty"`
	Registration   *Registration    `json:"registration,omitempty"`
	FileName       string           `json:"fileName,omitempty"`
}

type ForeignPassport struct {
	TaxpayerNumber string           `json:"taxpayerNumber,omitempty"`
	ResidenceUA    *string          `json:"residenceUA,omitempty"`
	DocNumber      string           `json:"docNumber,omitempty"`
	GenderUA       PassportGenderUA `json:"genderUA,omitempty"`
	NationalityUA  string           `json:"nationalityUA,omitempty"`
	LastNameUA     string           `json:"lastNameUA,omitempty"`
	FirstNameUA    string           `json:"firstNameUA,omitempty"`
	MiddleNameUA   string           `json:"middleNameUA,omitempty"`
	Birthday       string           `json:"birthday,omitempty"`
	BirthPlaceUA   string           `json:"birthPlaceUA,omitempty"`
	IssueDate      string           `json:"issueDate,omitempty"`
	ExpirationDate string           `json:"expirationDate,omitempty"`
	RecordNumber   string           `json:"recordNumber,omitempty"`
	Department     string           `json:"department,omitempty"`
	DepartmentUA   string           `json:"departmentUA,omitempty"`
	DepartmentEN   string           `json:"departmentEN,omitempty"`
	CountryCode    string           `json:"countryCode,omitempty"`
	GenderEN       PassportGenderEN `json:"genderEN,omitempty"`
	ID             string           `json:"id,omitempty"`
	LastNameEN     string           `json:"lastNameEN,omitempty"`
	FirstNameEN    string           `json:"firstNameEN,omitempty"`
	Registration   *Registration    `json:"registration,omitempty"`
	FileName       string           `json:"fileName,omitempty"`
}

type TaxpayerCard struct {
	CreationDate string `json:"creationDate,omitempty"`
	DocNumber    string `json:"docNumber,omitempty"`
	LastNameUA   string `json:"lastNameUA,omitempty"`
	FirstNameUA  string `json:"firstNameUA,omitempty"`
	MiddleNameUA string `json:"middleNameUA,omitempty"`
	Birthday     string `json:"birthday,omitempty"`
	FileName     string `json:"fileName,omitempty"`
}

type IdentityDocument struct {
	Number     string `json:"number,omitempty"`
	IssueDate  string `json:"issueDate,omitempty"`
	Department string `json:"department,omitempty"`
}

type IDPAddress struct {
	Birth        *string `json:"birth,omitempty"`
	Registration string  `json:"registration,omitempty"`
	Actual       string  `json:"actual,omitempty"`
}

type InternallyDisplaced struct {
	DocType             IDPDocumentType   `json:"docType,omitempty"`
	DocNumber           string            `json:"docNumber,omitempty"`
	Department          *string           `json:"department,omitempty"`
	LastName            string            `json:"lastName,omitempty"`
	FirstName           string            `json:"firstName,omitempty"`
	MiddleName          *string           `json:"middleName,omitempty"`
	IssueDate           *string           `json:"issueDate,omitempty"`
	BirthDate           string            `json:"birthDate,omitempty"`
	Gender              IDPGender         `json:"gender,omitempty"`
	LegalRepresentative *string           `json:"legalRepresentative,omitempty"`
	DocIdentity         *IdentityDocument `json:"docIdentity,omitempty"`
	Address             *IDPAddress       `json:"address,omitempty"`
	FileName            string            `json:"fileName,omitempty"`
}

type BirthRecordDocument struct {
	ID            string  `json:"id,omitempty"`
	Serie         string  `json:"serie,omitempty"`
	SerieOriginal string  `json:"serieOriginal,omitempty"`
	Number        string  `json:"number,omitempty"`
	SerialNumber  *string `json:"serialNumber,omitempty"`
	SerieNumber   *string `json:"serieNumber,omitempty"`
	Department    string  `json:"department,omitempty"`
	IssueDate     string  `json:"issueDate,omitempty"`
}

func (document *BirthRecordDocument) UnmarshalJSON(data []byte) error {
	type plain BirthRecordDocument
	var decoded plain
	if err := unmarshalWithAliases(data, &decoded, jsonFieldAlias{"serie Original", "serieOriginal"}); err != nil {
		return err
	}
	*document = BirthRecordDocument(decoded)
	return nil
}

type BirthRecordChild struct {
	LastName     string             `json:"lastName,omitempty"`
	FirstName    string             `json:"firstName,omitempty"`
	MiddleName   string             `json:"middleName,omitempty"`
	Gender       string             `json:"gender,omitempty"`
	RNOKPP       string             `json:"rnokpp,omitempty"`
	BirthDate    string             `json:"birthDate,omitempty"`
	BirthPlace   *string            `json:"birthPlace,omitempty"`
	BirthState   *string            `json:"birthState,omitempty"`
	BirthRegion  *string            `json:"birthRegion,omitempty"`
	LocalityType *BirthLocalityType `json:"birthLocalityType,omitempty"`
	Locality     *string            `json:"birthLocality,omitempty"`
}

func (child *BirthRecordChild) UnmarshalJSON(data []byte) error {
	type plain BirthRecordChild
	var decoded plain
	if err := unmarshalWithAliases(data, &decoded, jsonFieldAlias{"rnokpp?", "rnokpp"}); err != nil {
		return err
	}
	*child = BirthRecordChild(decoded)
	return nil
}

type BirthRecordParent struct {
	FullName               string `json:"fullName,omitempty"`
	Citizenship            string `json:"citizenship,omitempty"`
	RNOKPP                 string `json:"rnokpp,omitempty"`
	BirthDate              string `json:"birthDate,omitempty"`
	DeprivedOfParentRights bool   `json:"deprivedOfParentRights,omitempty"`
}

type BirthRecordParents struct {
	Father *BirthRecordParent `json:"father,omitempty"`
	Mother *BirthRecordParent `json:"mother,omitempty"`
}

type BirthRecordAct struct {
	Name              string  `json:"name,omitempty"`
	ARRegistrationNo  *string `json:"arRegNumber,omitempty"`
	Number            string  `json:"number,omitempty"`
	RegistrationPlace string  `json:"registrationPlace,omitempty"`
	Date              string  `json:"date,omitempty"`
	Parenthood        string  `json:"parenthood,omitempty"`
}

type BirthRecord struct {
	ID       string              `json:"id,omitempty"`
	Document BirthRecordDocument `json:"document,omitempty"`
	Child    BirthRecordChild    `json:"child,omitempty"`
	Parents  BirthRecordParents  `json:"parents,omitempty"`
	Act      BirthRecordAct      `json:"act,omitempty"`
	FileName string              `json:"fileName,omitempty"`
}

type BaseData struct {
	ID                    string         `json:"id,omitempty"`
	Number                string         `json:"number,omitempty"`
	Status                FlexibleNumber `json:"status,omitempty"`
	Name                  string         `json:"name,omitempty"`
	ListName              string         `json:"listName,omitempty"`
	ListDescription       string         `json:"listDescription,omitempty"`
	DocumentType          string         `json:"documentType,omitempty"`
	DocumentTypeCamelCase string         `json:"documentTypeCamelCase,omitempty"`
	OwnerType             string         `json:"ownerType,omitempty"`
	Subtype               string         `json:"subtype,omitempty"`
	IssuedAt              string         `json:"issuedAt,omitempty"`
}

type ChildTaxpayerPerson struct {
	ID         string `json:"id,omitempty"`
	LastName   string `json:"lastName,omitempty"`
	FirstName  string `json:"firstName,omitempty"`
	MiddleName string `json:"middleName,omitempty"`
	Birthday   string `json:"birthday,omitempty"`
	ITN        string `json:"itn,omitempty"`
	Identifier string `json:"identifier,omitempty"`
}

type ChildTaxpayerCard struct {
	BaseData  *BaseData            `json:"baseData,omitempty"`
	Child     *ChildTaxpayerPerson `json:"child,omitempty"`
	ID        string               `json:"id,omitempty"`
	DocNumber string               `json:"docNumber,omitempty"`
	DocStatus FlexibleNumber       `json:"docStatus,omitempty"`
	FileName  string               `json:"fileName,omitempty"`
}

type LocalizedField struct {
	Code  string `json:"code,omitempty"`
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

type DriverCategory struct {
	Category    LocalizedField `json:"category,omitempty"`
	OpeningDate LocalizedField `json:"openingDate,omitempty"`
}

type DriverLicenseSide struct {
	LastName       LocalizedField   `json:"lastName,omitempty"`
	FirstName      LocalizedField   `json:"firstName,omitempty"`
	Birth          LocalizedField   `json:"birth,omitempty"`
	IssueDate      LocalizedField   `json:"issueDate,omitempty"`
	ExpiryDate     LocalizedField   `json:"expiryDate,omitempty"`
	Identifier     LocalizedField   `json:"identifier,omitempty"`
	DocumentNumber LocalizedField   `json:"documentNumber,omitempty"`
	Department     *LocalizedField  `json:"department,omitempty"`
	Categories     []DriverCategory `json:"categories,omitempty"`
}

type DriverLicense struct {
	UA       DriverLicenseSide `json:"ua,omitempty"`
	ENG      DriverLicenseSide `json:"eng,omitempty"`
	FileName string            `json:"fileName,omitempty"`
}

type NameValue struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

type VehicleLicenseSide struct {
	LicensePlate          LocalizedField  `json:"licensePlate,omitempty"`
	FirstRegistrationDate LocalizedField  `json:"firstRegistrationDate,omitempty"`
	MakeYear              LocalizedField  `json:"makeYear,omitempty"`
	RegistrationDate      LocalizedField  `json:"registrationDate,omitempty"`
	DocumentNumber        LocalizedField  `json:"documentNumber,omitempty"`
	Department            LocalizedField  `json:"department,omitempty"`
	LastName              LocalizedField  `json:"lastName,omitempty"`
	FirstName             LocalizedField  `json:"firstName,omitempty"`
	Address               LocalizedField  `json:"address,omitempty"`
	Ownership             LocalizedField  `json:"ownership,omitempty"`
	Brand                 LocalizedField  `json:"brand,omitempty"`
	Model                 LocalizedField  `json:"model,omitempty"`
	VIN                   LocalizedField  `json:"vin,omitempty"`
	TotalWeight           LocalizedField  `json:"totalWeight,omitempty"`
	OwnWeight             LocalizedField  `json:"ownWeight,omitempty"`
	RankCategory          LocalizedField  `json:"rankCategory,omitempty"`
	Capacity              LocalizedField  `json:"capacity,omitempty"`
	Fuel                  LocalizedField  `json:"fuel,omitempty"`
	Color                 LocalizedField  `json:"color,omitempty"`
	NSeating              LocalizedField  `json:"nseating,omitempty"`
	NStandup              LocalizedField  `json:"nstandup,omitempty"`
	KindBody              NameValue       `json:"kindBody,omitempty"`
	PowerKWT              *LocalizedField `json:"powerKWT,omitempty"`
}

func (side *VehicleLicenseSide) UnmarshalJSON(data []byte) error {
	type plain VehicleLicenseSide
	var decoded plain
	if err := unmarshalWithAliases(data, &decoded, jsonFieldAlias{"powerKWT?", "powerKWT"}); err != nil {
		return err
	}
	*side = VehicleLicenseSide(decoded)
	return nil
}

type VehicleLicense struct {
	UA       VehicleLicenseSide `json:"ua,omitempty"`
	ENG      VehicleLicenseSide `json:"eng,omitempty"`
	FileName string             `json:"fileName,omitempty"`
}

type StudentIDCard struct {
	DocNumber      string  `json:"docNumber,omitempty"`
	LastNameUA     string  `json:"lastNameUA,omitempty"`
	FirstNameUA    *string `json:"firstNameUA,omitempty"`
	MiddleNameUA   string  `json:"middleNameUA,omitempty"`
	CollegeName    string  `json:"collegeName,omitempty"`
	FacultyName    *string `json:"facultyName,omitempty"`
	IssueDate      *string `json:"issueDate,omitempty"`
	ExpirationDate string  `json:"expirationDate,omitempty"`
	EducationType  string  `json:"educationType,omitempty"`
	FileName       string  `json:"fileName,omitempty"`
}

func (card *StudentIDCard) UnmarshalJSON(data []byte) error {
	type plain StudentIDCard
	var decoded plain
	if err := unmarshalWithAliases(data, &decoded, jsonFieldAlias{"middNameUA", "middleNameUA"}); err != nil {
		return err
	}
	*card = StudentIDCard(decoded)
	return nil
}

type ResidencePermit struct {
	DocNumber      string   `json:"docNumber,omitempty"`
	ID             string   `json:"id,omitempty"`
	TaxpayerNumber string   `json:"taxpayerNumber,omitempty"`
	FirstNameUA    string   `json:"firstNameUA,omitempty"`
	LastNameUA     string   `json:"lastNameUA,omitempty"`
	FirstNameEN    string   `json:"firstNameEN,omitempty"`
	LastNameEN     string   `json:"lastNameEN,omitempty"`
	RecordNumber   string   `json:"recordNumber,omitempty"`
	Gender         string   `json:"gender,omitempty"`
	Birthday       string   `json:"birthday,omitempty"`
	IssueDate      string   `json:"issueDate,omitempty"`
	ExpirationDate string   `json:"expirationDate,omitempty"`
	Authority      string   `json:"authority,omitempty"`
	IssueReason    string   `json:"issueReason,omitempty"`
	Nationality    []string `json:"nationality,omitempty"`
	BirthCountry   *string  `json:"birthCountry,omitempty"`
	Registration   *string  `json:"registration,omitempty"`
	FileName       string   `json:"fileName,omitempty"`
}

type PensionCard struct {
	DocNumber         string         `json:"docNumber,omitempty"`
	ID                string         `json:"id,omitempty"`
	FirstNameUA       string         `json:"firstNameUA,omitempty"`
	LastNameUA        string         `json:"lastNameUA,omitempty"`
	MiddleNameUA      string         `json:"middleNameUA,omitempty"`
	Gender            *PensionGender `json:"gender,omitempty"`
	Birthday          string         `json:"birthday,omitempty"`
	IssueDate         string         `json:"issueDate,omitempty"`
	ExpirationDate    string         `json:"expirationDate,omitempty"`
	PensionCaseNumber string         `json:"pensionCaseNumber,omitempty"`
	PensionType       string         `json:"pensionType,omitempty"`
	FileName          string         `json:"fileName,omitempty"`
}

// EducationDocument covers the supported education variants. Fields that vary
// by subType remain optional pointers.
type EducationDocument struct {
	ID                           string         `json:"id,omitempty"`
	DocStatus                    FlexibleNumber `json:"docStatus,omitempty"`
	DocNumber                    string         `json:"docNumber,omitempty"`
	Number                       string         `json:"number,omitempty"`
	Series                       *string        `json:"series,omitempty"`
	SubType                      string         `json:"subType,omitempty"`
	SubTypeName                  string         `json:"subTypeName,omitempty"`
	SubTypeNameEN                *string        `json:"subTypeNameEn,omitempty"`
	AccreditationInstitutionName *string        `json:"accreditationInstitutionName,omitempty"`
	AccreditationInstitutionEN   *string        `json:"accreditationInstitutionNameEn,omitempty"`
	AdditionalAwardInfo          *string        `json:"additionalAwardInfo,omitempty"`
	AdditionalAwardInfoEN        *string        `json:"additionalAwardInfoEn,omitempty"`
	BirthDate                    *string        `json:"birthDate,omitempty"`
	BossFullName                 *string        `json:"bossFullName,omitempty"`
	BossFullNameEN               *string        `json:"bossFullNameEn,omitempty"`
	BossPositionTitle            *string        `json:"bossPositionTitle,omitempty"`
	BossPositionTitleEN          *string        `json:"bossPositionTitleEn,omitempty"`
	IssueDate                    string         `json:"issueDate,omitempty"`
	EducationEndDate             *string        `json:"educationEndDate,omitempty"`
	EducationUniversityName      *string        `json:"educationUniversityName,omitempty"`
	EducationUniversityNameEN    *string        `json:"educationUniversityNameEn,omitempty"`
	LastName                     string         `json:"lastName,omitempty"`
	LastNameEN                   *string        `json:"lastNameEn,omitempty"`
	FirstName                    string         `json:"firstName,omitempty"`
	FirstNameEN                  *string        `json:"firstNameEn,omitempty"`
	MiddleName                   *string        `json:"middleName,omitempty"`
	IsDuplicate                  *bool          `json:"isDuplicate,omitempty"`
	IssuedByUniversityName       *string        `json:"issuedByUniversityName,omitempty"`
	IssuedByUniversityNameEN     *string        `json:"issuedByUniversityNameEn,omitempty"`
	SpecialityName               *string        `json:"specialityName,omitempty"`
	SpecialtyNameEN              *string        `json:"specialityNameEn,omitempty"`
	SpecializationName           *string        `json:"specializationName,omitempty"`
	StudyProgramName             *string        `json:"studyProgramName,omitempty"`
	StudyProgramNameEN           *string        `json:"studyProgramNameEn,omitempty"`
	QualificationName            *string        `json:"qualificationName,omitempty"`
	FileName                     string         `json:"fileName,omitempty"`
}

func (document *EducationDocument) UnmarshalJSON(data []byte) error {
	type plain EducationDocument
	var decoded plain
	if err := unmarshalWithAliases(data, &decoded,
		jsonFieldAlias{"issueByUniversityNameEn", "issuedByUniversityNameEn"},
		jsonFieldAlias{"specialtyNameEn", "specialityNameEn"},
	); err != nil {
		return err
	}
	*document = EducationDocument(decoded)
	return nil
}

type VeteranCertificate struct {
	DocNumber       string  `json:"docNumber,omitempty"`
	ID              string  `json:"id,omitempty"`
	DocType         string  `json:"docType,omitempty"`
	Law             string  `json:"law,omitempty"`
	LawArticle      string  `json:"lawArticle,omitempty"`
	DisabilityGroup *string `json:"disabilityGroup,omitempty"`
	LastName        string  `json:"lastName,omitempty"`
	FirstName       string  `json:"firstName,omitempty"`
	MiddleName      string  `json:"middleName,omitempty"`
	Department      string  `json:"department,omitempty"`
	PersonID        string  `json:"personId,omitempty"`
	IssueDate       string  `json:"issueDate,omitempty"`
	ExpirationDate  string  `json:"expirationDate,omitempty"`
	FileName        string  `json:"fileName,omitempty"`
}

type ActRecordCertificate struct {
	Status      ActRecordCertificateStatus `json:"status,omitempty"`
	Serie       string                     `json:"serie,omitempty"`
	Series      string                     `json:"series,omitempty"`
	Number      string                     `json:"number,omitempty"`
	SerieNumber string                     `json:"serieNumber,omitempty"`
	IssueDate   string                     `json:"issueDate,omitempty"`
	Department  string                     `json:"department,omitempty"`
}

type ActRecordAddress struct {
	Country            *string `json:"country,omitempty"`
	State              *string `json:"state,omitempty"`
	Region             *string `json:"region,omitempty"`
	District           *string `json:"district,omitempty"`
	MisspelledDistrict *string `json:"disctrict,omitempty"`
	City               *string `json:"city,omitempty"`
	Street             *string `json:"street,omitempty"`
	House              *string `json:"house,omitempty"`
	Apartment          *string `json:"apartment,omitempty"`
	FullAddress        *string `json:"fullAddress,omitempty"`
}

type ActRecordPerson struct {
	Gender         *ActRecordGender  `json:"gender,omitempty"`
	FirstName      string            `json:"firstName,omitempty"`
	LastName       string            `json:"lastName,omitempty"`
	MiddleName     *string           `json:"middleName,omitempty"`
	OldFirstName   *string           `json:"oldFirstName,omitempty"`
	OldLastName    *string           `json:"oldLastName,omitempty"`
	OldMiddleName  *string           `json:"oldMiddleName,omitempty"`
	RNOKPP         *string           `json:"rnokpp,omitempty"`
	BirthDate      *string           `json:"birthDate,omitempty"`
	Citizenship    *string           `json:"citizenship,omitempty"`
	Status         *string           `json:"status,omitempty"`
	FamilyStatus   *string           `json:"familyStatus,omitempty"`
	DocType        *string           `json:"docType,omitempty"`
	SeriesNumber   *string           `json:"seriesNumber,omitempty"`
	DocDate        *string           `json:"docDate,omitempty"`
	DocOrgName     *string           `json:"docOrgName,omitempty"`
	BirthState     *string           `json:"birthState,omitempty"`
	BirthRegion    *string           `json:"birthRegion,omitempty"`
	BirthDistrict  *string           `json:"birthDistrict,omitempty"`
	Region         *string           `json:"region,omitempty"`
	District       *string           `json:"district,omitempty"`
	Document       *IdentityDocument `json:"document,omitempty"`
	AddressDetails *ActRecordAddress `json:"addressDetails,omitempty"`
}

type ActRecordCommon struct {
	BaseData           *BaseData                    `json:"baseData,omitempty"`
	DocStatus          FlexibleNumber               `json:"docStatus,omitempty"`
	ID                 string                       `json:"id,omitempty"`
	DocNumber          string                       `json:"docNumber,omitempty"`
	SubType            string                       `json:"subType,omitempty"`
	ARRegistrationDate string                       `json:"arRegDate,omitempty"`
	OperationDate      string                       `json:"opDate,omitempty"`
	OperationName      string                       `json:"arOpName,omitempty"`
	Name               string                       `json:"name,omitempty"`
	Number             string                       `json:"number,omitempty"`
	Date               string                       `json:"date,omitempty"`
	RegistrationPlace  string                       `json:"registrationPlace,omitempty"`
	IsRestore          *ActRecordRestoreFlag        `json:"isRestore,omitempty"`
	ARVerification     *ActRecordVerificationStatus `json:"arVerificationSt,omitempty"`
	NumberLink         *string                      `json:"numberLink,omitempty"`
	DateLink           *string                      `json:"dateLink,omitempty"`
	ComposeOrgLink     *string                      `json:"composeOrgLink,omitempty"`
	Certificates       []ActRecordCertificate       `json:"certificates,omitempty"`
	FileName           string                       `json:"fileName,omitempty"`
}

type NameChangeActRecord struct {
	ActRecordCommon

	Gender         *ActRecordGender  `json:"gender,omitempty"`
	FirstName      string            `json:"firstName,omitempty"`
	LastName       string            `json:"lastName,omitempty"`
	MiddleName     *string           `json:"middleName,omitempty"`
	OldFirstName   *string           `json:"oldFirstName,omitempty"`
	OldLastName    *string           `json:"oldLastName,omitempty"`
	OldMiddleName  *string           `json:"oldMiddleName,omitempty"`
	RNOKPP         *string           `json:"rnokpp,omitempty"`
	BirthDate      *string           `json:"birthDate,omitempty"`
	Citizenship    *string           `json:"citizenship,omitempty"`
	AddressDetails *ActRecordAddress `json:"addressDetails,omitempty"`
}

type MarriageActRecord struct {
	ActRecordCommon

	PartnerFirst  ActRecordPerson `json:"partnerFirst,omitempty"`
	PartnerSecond ActRecordPerson `json:"partnerSecond,omitempty"`
}

type DivorceActRecord struct {
	ActRecordCommon

	PartnerFirst  ActRecordPerson `json:"partnerFirst,omitempty"`
	PartnerSecond ActRecordPerson `json:"partnerSecond,omitempty"`
	ReasonDivorce *string         `json:"reasonDivorce,omitempty"`
}

type IncomeTaxMilitary struct {
	IncomeAccrued          FlexibleNumber    `json:"IncomeAccrued,omitempty"`
	IncomePaid             *FlexibleNumber   `json:"IncomePaid,omitempty"`
	TaxCharged             FlexibleNumber    `json:"TaxCharged,omitempty"`
	TaxTransferred         *FlexibleNumber   `json:"TaxTransferred,omitempty"`
	TaxMilitaryCharged     *FlexibleNumber   `json:"TaxMilitaryCharged,omitempty"`
	TaxMilitaryTransferred *FlexibleNumber   `json:"TaxMilitaryTransferred,omitempty"`
	SignOfIncomePrivilege  *string           `json:"SignOfIncomePrivilege,omitempty"`
	PeriodQuarterMonth     *string           `json:"period_quarter_month,omitempty"`
	PeriodYear             *FlexibleNumber   `json:"period_year,omitempty"`
	ResultIncome           *IncomeResultCode `json:"result_income,omitempty"`
	DateOfDismissal        *string           `json:"DateOfDismissal,omitempty"`
	DateOfEmployment       *string           `json:"DateOfEmployment,omitempty"`
}

type IncomeSource struct {
	TaxAgent          *string           `json:"TaxAgent,omitempty"`
	NameTaxAgent      *string           `json:"NameTaxAgent,omitempty"`
	IncomeTaxMilitary IncomeTaxMilitary `json:"IncomeTaxMilitary,omitempty"`
	DateOfDismissal   *string           `json:"DateOfDismissal,omitempty"`
	DateOfEmployment  *string           `json:"DateOfEmployment,omitempty"`
}

type IncomeInfo struct {
	RNOKPP          string         `json:"RNOKPP,omitempty"`
	SourcesOfIncome []IncomeSource `json:"SourcesOfIncome,omitempty"`
}

type IncomePerson struct {
	RNOKPP     string `json:"RNOKPP,omitempty"`
	LastName   string `json:"last_name,omitempty"`
	FirstName  string `json:"first_name,omitempty"`
	MiddleName string `json:"middle_name,omitempty"`
	BirthDate  string `json:"date_birth,omitempty"`
}

type IncomePeriod struct {
	BeginMonth FlexibleNumber `json:"period_begin_month,omitempty"`
	BeginYear  FlexibleNumber `json:"period_begin_year,omitempty"`
	EndMonth   FlexibleNumber `json:"period_end_month,omitempty"`
	EndYear    FlexibleNumber `json:"period_end_year,omitempty"`
}

type IncomeReport struct {
	IDRequest         *string        `json:"IDrequest,omitempty"`
	ApplicationNumber string         `json:"applicationNumber,omitempty"`
	ApplicationDate   string         `json:"applicationDate,omitempty"`
	Error             *string        `json:"error,omitempty"`
	Info              IncomeInfo     `json:"Info,omitempty"`
	SourcesOfIncome   []IncomeSource `json:"SourcesOfIncome,omitempty"`
	Person            IncomePerson   `json:"person,omitempty"`
	Period            IncomePeriod   `json:"period,omitempty"`
	FileName          string         `json:"fileName,omitempty"`
}

type InsuranceMonthlyData struct {
	Sum1     FlexibleNumber `json:"sum1,omitempty"`
	Sum2     FlexibleNumber `json:"sum2,omitempty"`
	Sum3     FlexibleNumber `json:"sum3,omitempty"`
	Sum4     FlexibleNumber `json:"sum4,omitempty"`
	Sum5     FlexibleNumber `json:"sum5,omitempty"`
	Sum6     FlexibleNumber `json:"sum6,omitempty"`
	Sum7     FlexibleNumber `json:"sum7,omitempty"`
	Sum8     FlexibleNumber `json:"sum8,omitempty"`
	Sum9     FlexibleNumber `json:"sum9,omitempty"`
	Sum10    FlexibleNumber `json:"sum10,omitempty"`
	Sum11    FlexibleNumber `json:"sum11,omitempty"`
	Sum12    FlexibleNumber `json:"sum12,omitempty"`
	MonthExp FlexibleNumber `json:"monthExp,omitempty"`
}

type InsuranceEmployer struct {
	EDRPO       string                 `json:"edrpo,omitempty"`
	CompanyName string                 `json:"companyName,omitempty"`
	Year        FlexibleNumber         `json:"year,omitempty"`
	MonthlyData []InsuranceMonthlyData `json:"monthlyData,omitempty"`
}

type InsuranceStatement struct {
	LastName    string              `json:"lastName,omitempty"`
	Name        string              `json:"name,omitempty"`
	Patronymic  string              `json:"patronymic,omitempty"`
	KSS         string              `json:"kss,omitempty"`
	Numident    *string             `json:"numident,omitempty"`
	Passport    *string             `json:"passport,omitempty"`
	InsuredData []InsuranceEmployer `json:"insuredData,omitempty"`
	FileName    string              `json:"fileName,omitempty"`
}
