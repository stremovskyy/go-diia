package metadata

import "encoding/json"

// PassportGenderUA is the Ukrainian gender code in passport metadata.
type PassportGenderUA string

const (
	PassportGenderUAFemale PassportGenderUA = "Ж"
	PassportGenderUAMale   PassportGenderUA = "Ч"
)

func (v PassportGenderUA) Valid() bool {
	return v == PassportGenderUAFemale || v == PassportGenderUAMale
}

// PassportGenderEN is the Latin gender code in passport metadata.
type PassportGenderEN string

const (
	PassportGenderENFemale PassportGenderEN = "F"
	PassportGenderENMale   PassportGenderEN = "M"
)

func (v PassportGenderEN) Valid() bool {
	return v == PassportGenderENFemale || v == PassportGenderENMale
}

// IDPDocumentType describes whose internally displaced person certificate is shared.
type IDPDocumentType string

const (
	IDPDocumentTypePerson      IDPDocumentType = "person"
	IDPDocumentTypeAccompanied IDPDocumentType = "accompanied"
)

func (v IDPDocumentType) Valid() bool {
	return v == IDPDocumentTypePerson || v == IDPDocumentTypeAccompanied
}

// IDPGender is the full Ukrainian gender value in IDP metadata.
type IDPGender string

const (
	IDPGenderFemale IDPGender = "Жінка"
	IDPGenderMale   IDPGender = "Чоловік"
)

func (v IDPGender) Valid() bool { return v == IDPGenderFemale || v == IDPGenderMale }

// PensionGender is the abbreviated gender value on a pension card.
type PensionGender string

const (
	PensionGenderFemale PensionGender = "Ж"
	PensionGenderMale   PensionGender = "Ч"
)

func (v PensionGender) Valid() bool { return v == PensionGenderFemale || v == PensionGenderMale }

// ActRecordGender is the English gender value in civil act records.
type ActRecordGender string

const (
	ActRecordGenderFemale ActRecordGender = "female"
	ActRecordGenderMale   ActRecordGender = "male"
)

func (v ActRecordGender) Valid() bool {
	return v == ActRecordGenderFemale || v == ActRecordGenderMale
}

// ActRecordRestoreFlag is a string-valued restoration flag.
type ActRecordRestoreFlag string

const (
	// ActRecordRestoreFlagZero means the document was not restored.
	ActRecordRestoreFlagZero ActRecordRestoreFlag = "0"
	// ActRecordRestoreFlagOne means the document was restored.
	ActRecordRestoreFlagOne ActRecordRestoreFlag = "1"
)

func (v ActRecordRestoreFlag) Valid() bool {
	return v == ActRecordRestoreFlagZero || v == ActRecordRestoreFlagOne
}

// ActRecordVerificationStatus is a string-valued verification status.
type ActRecordVerificationStatus string

const (
	// ActRecordVerificationStatusZero means the document was not verified.
	ActRecordVerificationStatusZero ActRecordVerificationStatus = "0"
	// ActRecordVerificationStatusOne means the document was verified.
	ActRecordVerificationStatusOne ActRecordVerificationStatus = "1"
)

func (v ActRecordVerificationStatus) Valid() bool {
	return v == ActRecordVerificationStatusZero || v == ActRecordVerificationStatusOne
}

// ActRecordCertificateStatus is a string-valued certificate status.
type ActRecordCertificateStatus string

const (
	// ActRecordCertificateStatusZero means the certificate is inactive.
	ActRecordCertificateStatusZero ActRecordCertificateStatus = "0"
	// ActRecordCertificateStatusOne means the certificate is active.
	ActRecordCertificateStatusOne ActRecordCertificateStatus = "1"
)

func (v ActRecordCertificateStatus) Valid() bool {
	return v == ActRecordCertificateStatusZero || v == ActRecordCertificateStatusOne
}

// IncomeResultCode is a result_income code in an income report. Unknown codes remain readable.
type IncomeResultCode string

const (
	// IncomeResultCode1 identifies a response based on annex 4DF tax reporting.
	IncomeResultCode1 IncomeResultCode = "1"
	// IncomeResultCode2 identifies a response based on tax declarations.
	IncomeResultCode2 IncomeResultCode = "2"
	// IncomeResultCode3 identifies a response based on a property and income declaration.
	IncomeResultCode3 IncomeResultCode = "3"
	// IncomeResultCode14 identifies a response based on the 2023 property and income declaration form.
	IncomeResultCode14 IncomeResultCode = "14"
)

func (v IncomeResultCode) Valid() bool {
	switch v {
	case IncomeResultCode1, IncomeResultCode2, IncomeResultCode3, IncomeResultCode14:
		return true
	default:
		return false
	}
}

// UnmarshalJSON also accepts numeric codes observed in Diia metadata.
func (v *IncomeResultCode) UnmarshalJSON(data []byte) error {
	var value FlexibleNumber
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*v = IncomeResultCode(value)
	return nil
}

// BirthLocalityType names supported locality kinds. Unknown and differently
// capitalized values remain valid wire data.
type BirthLocalityType string

const (
	BirthLocalityTypeCity              BirthLocalityType = "місто"
	BirthLocalityTypeDistrict          BirthLocalityType = "район"
	BirthLocalityTypeUrbanSettlement   BirthLocalityType = "селище міського типу"
	BirthLocalityTypeVillage           BirthLocalityType = "село"
	BirthLocalityTypeSettlement        BirthLocalityType = "селище"
	BirthLocalityTypeVillageCouncil    BirthLocalityType = "сільрада"
	BirthLocalityTypeSettlementCouncil BirthLocalityType = "селищна рада"
)
