package go_diia

import "github.com/stremovskyy/go-diia/internal/models/domain"

type Environment = domain.Environment

const (
	EnvironmentProduction = domain.EnvironmentProduction
	EnvironmentSandbox    = domain.EnvironmentSandbox
	Production            = domain.EnvironmentProduction
	Sandbox               = domain.EnvironmentSandbox
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

type SharingStatus = domain.SharingStatus

const (
	SharingStatusInitialized = domain.SharingStatusInitialized
	SharingStatusProcessing  = domain.SharingStatusProcessing
	SharingStatusDone        = domain.SharingStatusDone
	SharingStatusRefuse      = domain.SharingStatusRefuse
	SharingStatusNotUsed     = domain.SharingStatusNotUsed
)
