package api

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/stremovskyy/go-diia/internal/faults"
	"github.com/stremovskyy/go-diia/internal/models/domain"
)

type SessionToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"-"`
}

type BranchScopes struct {
	Sharing []domain.DocumentType `json:"sharing"`
}

type Branch struct {
	ID                string       `json:"_id"`
	Name              string       `json:"name,omitempty"`
	Email             string       `json:"email,omitempty"`
	Region            string       `json:"region,omitempty"`
	District          string       `json:"district,omitempty"`
	Location          string       `json:"location,omitempty"`
	Street            string       `json:"street,omitempty"`
	House             string       `json:"house,omitempty"`
	CustomFullName    string       `json:"customFullName,omitempty"`
	CustomFullAddress string       `json:"customFullAddress,omitempty"`
	DeliveryTypes     []string     `json:"deliveryTypes,omitempty"`
	OfferRequestType  string       `json:"offerRequestType,omitempty"`
	Scopes            BranchScopes `json:"scopes,omitempty"`
}

type BranchList struct {
	Total    int      `json:"total"`
	Branches []Branch `json:"branches"`
}

type OfferScopes struct {
	Sharing          []domain.DocumentType `json:"sharing"`
	MandatorySharing []domain.DocumentType `json:"mandatorySharing,omitempty"`
}

type Offer struct {
	ID         string      `json:"_id"`
	Name       string      `json:"name,omitempty"`
	ReturnLink string      `json:"returnLink,omitempty"`
	Scopes     OfferScopes `json:"scopes,omitempty"`
}

type OfferList struct {
	Total  int     `json:"total"`
	Offers []Offer `json:"offers"`
}

type SuccessResponse struct {
	Success bool `json:"success"`
}

type StatusResponse struct {
	Status domain.SharingStatus `json:"status"`
}

type DeeplinkResponse struct {
	Deeplink string `json:"deeplink"`
}

func (r DeeplinkResponse) ParsedURL() (*url.URL, error) {
	parsed, err := url.Parse(r.Deeplink)

	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return nil, fmt.Errorf("%w: invalid deeplink", faults.ErrDecode)
	}

	return parsed, nil
}

func (r DeeplinkResponse) OTP() (string, error) {
	parsed, err := r.ParsedURL()
	if err != nil {
		return "", err
	}

	otp := strings.Trim(strings.TrimSpace(parsed.Path), "/")

	if index := strings.LastIndexByte(otp, '/'); index >= 0 {
		otp = otp[index+1:]
	}

	if otp == "" {
		return "", fmt.Errorf("%w: deeplink has no OTP", faults.ErrDecode)
	}

	return otp, nil
}
