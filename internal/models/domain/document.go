package domain

// DocumentType is a current Diia Document Sharing wire key.
type DocumentType string

const (
	DocumentTypeInternalPassport             DocumentType = "internal-passport"
	DocumentTypeForeignPassport              DocumentType = "foreign-passport"
	DocumentTypeTaxpayerCard                 DocumentType = "taxpayer-card"
	DocumentTypeReferenceInternallyDisplaced DocumentType = "reference-internally-displaced-person"
	DocumentTypeUserBirthRecord              DocumentType = "user-birth-record"
	DocumentTypeBirthCertificate             DocumentType = "birth-certificate"
	DocumentTypeChildTaxpayerCard            DocumentType = "child-taxpayer-card"
	DocumentTypeDriverLicense                DocumentType = "driver-license"
	DocumentTypeVehicleLicense               DocumentType = "vehicle-license"
	DocumentTypeStudentIDCard                DocumentType = "student-id-card"
	DocumentTypeResidencePermitPermanent     DocumentType = "residence-permit-permanent"
	DocumentTypeResidencePermitTemporary     DocumentType = "residence-permit-temporary"
	DocumentTypePensionCard                  DocumentType = "pension-card"
	DocumentTypeEducationDocument            DocumentType = "education-document"
	DocumentTypeVeteranCertificate           DocumentType = "veteran-certificate"
	DocumentTypeNameChangeActRecord          DocumentType = "name-change-act-record"
	DocumentTypeMarriageActRecord            DocumentType = "marriage-act-record"
	DocumentTypeDivorceActRecord             DocumentType = "divorce-act-record"
	DocumentTypeIncomeReport                 DocumentType = "income-report"
	DocumentTypeOK5                          DocumentType = "ok5"
	DocumentTypeOK7                          DocumentType = "ok7"
)

var knownDocumentTypes = map[DocumentType]struct{}{
	DocumentTypeInternalPassport: {}, DocumentTypeForeignPassport: {},
	DocumentTypeTaxpayerCard: {}, DocumentTypeReferenceInternallyDisplaced: {},
	DocumentTypeUserBirthRecord: {}, DocumentTypeBirthCertificate: {},
	DocumentTypeChildTaxpayerCard: {}, DocumentTypeDriverLicense: {},
	DocumentTypeVehicleLicense: {}, DocumentTypeStudentIDCard: {},
	DocumentTypeResidencePermitPermanent: {}, DocumentTypeResidencePermitTemporary: {},
	DocumentTypePensionCard: {}, DocumentTypeEducationDocument: {},
	DocumentTypeVeteranCertificate: {}, DocumentTypeNameChangeActRecord: {},
	DocumentTypeMarriageActRecord: {}, DocumentTypeDivorceActRecord: {},
	DocumentTypeIncomeReport: {}, DocumentTypeOK5: {}, DocumentTypeOK7: {},
}

func (d DocumentType) Valid() bool { _, ok := knownDocumentTypes[d]; return ok }
