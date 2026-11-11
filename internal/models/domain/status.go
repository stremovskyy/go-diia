package domain

type SharingStatus string

const (
	SharingStatusInitialized SharingStatus = "initialized"
	SharingStatusProcessing  SharingStatus = "processing"
	SharingStatusDone        SharingStatus = "done"
	SharingStatusRefuse      SharingStatus = "refuse"
	SharingStatusNotUsed     SharingStatus = "notUsed"
)

func (s SharingStatus) Valid() bool {
	switch s {
	case SharingStatusInitialized,
		SharingStatusProcessing,
		SharingStatusDone,
		SharingStatusRefuse,
		SharingStatusNotUsed:
		return true
	default:
		return false
	}
}
