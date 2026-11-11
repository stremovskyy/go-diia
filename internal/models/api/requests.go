package api

import (
	"strings"

	"github.com/stremovskyy/go-diia/internal/models/domain"
)

type BranchRequest struct {
	CustomFullName    string       `json:"customFullName,omitempty"`
	CustomFullAddress string       `json:"customFullAddress,omitempty"`
	Name              string       `json:"name"`
	Email             string       `json:"email"`
	Region            string       `json:"region"`
	District          string       `json:"district"`
	Location          string       `json:"location"`
	Street            string       `json:"street"`
	House             string       `json:"house"`
	DeliveryTypes     []string     `json:"deliveryTypes"`
	OfferRequestType  string       `json:"offerRequestType"`
	Scopes            BranchScopes `json:"scopes"`
}

func NewBranchRequest() *BranchRequest {
	return &BranchRequest{
		DeliveryTypes: []string{"api"}, OfferRequestType: "dynamic",
		Scopes: BranchScopes{Sharing: []domain.DocumentType{}},
	}
}

func (r *BranchRequest) WithName(value string) *BranchRequest {
	r.Name = strings.TrimSpace(value)

	return r
}

func (r *BranchRequest) WithEmail(value string) *BranchRequest {
	r.Email = strings.TrimSpace(value)

	return r
}

func (r *BranchRequest) WithRegion(value string) *BranchRequest {
	r.Region = strings.TrimSpace(value)

	return r
}

func (r *BranchRequest) WithDistrict(value string) *BranchRequest {
	r.District = strings.TrimSpace(value)

	return r
}

func (r *BranchRequest) WithLocation(value string) *BranchRequest {
	r.Location = strings.TrimSpace(value)

	return r
}

func (r *BranchRequest) WithStreet(value string) *BranchRequest {
	r.Street = strings.TrimSpace(value)

	return r
}

func (r *BranchRequest) WithHouse(value string) *BranchRequest {
	r.House = strings.TrimSpace(value)

	return r
}

func (r *BranchRequest) WithCustomFullName(value string) *BranchRequest {
	r.CustomFullName = strings.TrimSpace(value)

	return r
}

func (r *BranchRequest) WithCustomFullAddress(value string) *BranchRequest {
	r.CustomFullAddress = strings.TrimSpace(value)

	return r
}

func (r *BranchRequest) WithSharing(types ...domain.DocumentType) *BranchRequest {
	r.Scopes.Sharing = append([]domain.DocumentType(nil), types...)

	return r
}

type OfferRequest struct {
	Name       string      `json:"name"`
	ReturnLink string      `json:"returnLink,omitempty"`
	Scopes     OfferScopes `json:"scopes"`
}

func NewOfferRequest() *OfferRequest {
	return &OfferRequest{Scopes: OfferScopes{Sharing: []domain.DocumentType{}}}
}

func (r *OfferRequest) WithName(value string) *OfferRequest {
	r.Name = strings.TrimSpace(value)

	return r
}

func (r *OfferRequest) WithSharing(types ...domain.DocumentType) *OfferRequest {
	r.Scopes.Sharing = append([]domain.DocumentType(nil), types...)

	return r
}

func (r *OfferRequest) WithMandatorySharing(types ...domain.DocumentType) *OfferRequest {
	r.Scopes.MandatorySharing = append([]domain.DocumentType(nil), types...)

	return r
}

func (r *OfferRequest) WithReturnLink(link string) *OfferRequest {
	r.ReturnLink = strings.TrimSpace(link)

	return r
}

type OfflineShareRequest struct {
	BranchID  string `json:"branchId"`
	Barcode   string `json:"barcode"`
	RequestID string `json:"requestId"`
	UseDiiaID *bool  `json:"useDiiaId,omitempty"`
}

func NewOfflineShareRequest() *OfflineShareRequest {
	return &OfflineShareRequest{}
}

func (r *OfflineShareRequest) WithBranchID(value string) *OfflineShareRequest {
	r.BranchID = strings.TrimSpace(value)

	return r
}

func (r *OfflineShareRequest) WithBarcode(value string) *OfflineShareRequest {
	r.Barcode = strings.TrimSpace(value)

	return r
}

func (r *OfflineShareRequest) WithRequestID(value string) *OfflineShareRequest {
	r.RequestID = strings.TrimSpace(value)

	return r
}

func (r *OfflineShareRequest) WithDiiaID(enabled bool) *OfflineShareRequest {
	r.UseDiiaID = &enabled

	return r
}

type OnlineShareRequest struct {
	BranchID   string `json:"-"`
	OfferID    string `json:"offerId"`
	ReturnLink string `json:"returnLink,omitempty"`
	RequestID  string `json:"requestId"`
	UseDiiaID  *bool  `json:"useDiiaId,omitempty"`
	SignAlgo   string `json:"signAlgo,omitempty"`
}

func NewOnlineShareRequest() *OnlineShareRequest {
	return &OnlineShareRequest{}
}

func (r *OnlineShareRequest) WithBranchID(value string) *OnlineShareRequest {
	r.BranchID = strings.TrimSpace(value)

	return r
}

func (r *OnlineShareRequest) WithOfferID(value string) *OnlineShareRequest {
	r.OfferID = strings.TrimSpace(value)

	return r
}

func (r *OnlineShareRequest) WithRequestID(value string) *OnlineShareRequest {
	r.RequestID = strings.TrimSpace(value)

	return r
}

func (r *OnlineShareRequest) WithReturnLink(link string) *OnlineShareRequest {
	r.ReturnLink = strings.TrimSpace(link)

	return r
}

func (r *OnlineShareRequest) WithDiiaID(enabled bool) *OnlineShareRequest {
	r.UseDiiaID = &enabled

	return r
}

func (r *OnlineShareRequest) WithSignAlgo(value string) *OnlineShareRequest {
	r.SignAlgo = strings.TrimSpace(value)

	return r
}

type OfflineStatusRequest struct {
	Barcode   string
	RequestID string
}

func NewOfflineStatusRequest() *OfflineStatusRequest { return &OfflineStatusRequest{} }

func (r *OfflineStatusRequest) WithBarcode(value string) *OfflineStatusRequest {
	r.Barcode = strings.TrimSpace(value)

	return r
}

func (r *OfflineStatusRequest) WithRequestID(value string) *OfflineStatusRequest {
	r.RequestID = strings.TrimSpace(value)

	return r
}

type OnlineStatusRequest struct {
	OTP       string
	RequestID string
}

func NewOnlineStatusRequest() *OnlineStatusRequest { return &OnlineStatusRequest{} }

func (r *OnlineStatusRequest) WithOTP(value string) *OnlineStatusRequest {
	r.OTP = strings.TrimSpace(value)

	return r
}

func (r *OnlineStatusRequest) WithRequestID(value string) *OnlineStatusRequest {
	r.RequestID = strings.TrimSpace(value)

	return r
}

type PageRequest struct {
	Skip  *int
	Limit *int
}

func NewPageRequest() *PageRequest { return &PageRequest{} }

func (r *PageRequest) WithSkip(skip int) *PageRequest { r.Skip = &skip; return r }

func (r *PageRequest) WithLimit(limit int) *PageRequest { r.Limit = &limit; return r }
