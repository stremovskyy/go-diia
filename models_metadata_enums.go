package go_diia

import "github.com/stremovskyy/go-diia/internal/models/metadata"

type (
	PassportGenderUA            = metadata.PassportGenderUA
	PassportGenderEN            = metadata.PassportGenderEN
	IDPDocumentType             = metadata.IDPDocumentType
	IDPGender                   = metadata.IDPGender
	PensionGender               = metadata.PensionGender
	ActRecordGender             = metadata.ActRecordGender
	ActRecordRestoreFlag        = metadata.ActRecordRestoreFlag
	ActRecordVerificationStatus = metadata.ActRecordVerificationStatus
	ActRecordCertificateStatus  = metadata.ActRecordCertificateStatus
	IncomeResultCode            = metadata.IncomeResultCode
	BirthLocalityType           = metadata.BirthLocalityType
)

const (
	PassportGenderUAFemale = metadata.PassportGenderUAFemale
	PassportGenderUAMale   = metadata.PassportGenderUAMale
	PassportGenderENFemale = metadata.PassportGenderENFemale
	PassportGenderENMale   = metadata.PassportGenderENMale

	IDPDocumentTypePerson      = metadata.IDPDocumentTypePerson
	IDPDocumentTypeAccompanied = metadata.IDPDocumentTypeAccompanied
	IDPGenderFemale            = metadata.IDPGenderFemale
	IDPGenderMale              = metadata.IDPGenderMale

	PensionGenderFemale = metadata.PensionGenderFemale
	PensionGenderMale   = metadata.PensionGenderMale

	ActRecordGenderFemale = metadata.ActRecordGenderFemale
	ActRecordGenderMale   = metadata.ActRecordGenderMale

	ActRecordRestoreFlagZero = metadata.ActRecordRestoreFlagZero
	ActRecordRestoreFlagOne  = metadata.ActRecordRestoreFlagOne

	ActRecordVerificationStatusZero = metadata.ActRecordVerificationStatusZero
	ActRecordVerificationStatusOne  = metadata.ActRecordVerificationStatusOne

	ActRecordCertificateStatusZero = metadata.ActRecordCertificateStatusZero
	ActRecordCertificateStatusOne  = metadata.ActRecordCertificateStatusOne

	IncomeResultCode1  = metadata.IncomeResultCode1
	IncomeResultCode2  = metadata.IncomeResultCode2
	IncomeResultCode3  = metadata.IncomeResultCode3
	IncomeResultCode14 = metadata.IncomeResultCode14

	BirthLocalityTypeCity              = metadata.BirthLocalityTypeCity
	BirthLocalityTypeDistrict          = metadata.BirthLocalityTypeDistrict
	BirthLocalityTypeUrbanSettlement   = metadata.BirthLocalityTypeUrbanSettlement
	BirthLocalityTypeVillage           = metadata.BirthLocalityTypeVillage
	BirthLocalityTypeSettlement        = metadata.BirthLocalityTypeSettlement
	BirthLocalityTypeVillageCouncil    = metadata.BirthLocalityTypeVillageCouncil
	BirthLocalityTypeSettlementCouncil = metadata.BirthLocalityTypeSettlementCouncil
)
