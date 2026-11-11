package callback

import (
	"encoding/json"

	"github.com/stremovskyy/go-diia/internal/models/domain"
	"github.com/stremovskyy/go-diia/internal/models/metadata"
)

type CallbackDocument struct {
	Type      domain.DocumentType
	FieldName string
	FileName  string
	Envelope  []byte
}

type CallbackSignature struct {
	FieldName string
	FileName  string
	Signed    []byte
}

type CallbackPart struct {
	FieldName string
	FileName  string
	Data      []byte
}

type CallbackPayload struct {
	RequestID         string
	MissingBearerAuth bool
	EncodeData        []byte
	Documents         []CallbackDocument
	Signatures        []CallbackSignature
	UnknownParts      []CallbackPart
}

type DocumentFile struct {
	Type                  domain.DocumentType
	FieldName             string
	FileName              string
	DiiaSealFileName      string
	UserSignatureFileName string
	PDF                   []byte
}

type VerifiedSignature struct {
	FieldName        string
	FileName         string
	Role             string
	DocumentFileName string
	Data             []byte
}

type DocumentPackage struct {
	RequestID            string
	OfferRequestID       string
	Barcode              string
	Metadata             *metadata.Metadata
	MetadataSealFileName string
	Documents            []DocumentFile
	Signatures           []VerifiedSignature
	RawMetadata          json.RawMessage
}
