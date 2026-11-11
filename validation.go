package go_diia

import (
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

func validateBranchRequest(r *BranchRequest) error {
	if r == nil {
		return newValidationError("request", "is required")
	}

	checks := []struct {
		field, value string
		min, max     int
	}{
		{"name", r.Name, 1, 32},
		{"email", r.Email, 6, 100},
		{"region", r.Region, 4, 100},
		{"district", r.District, 4, 100},
		{"location", r.Location, 2, 100},
		{"street", r.Street, 4, 100},
		{"house", r.House, 1, 20},
	}

	for _, check := range checks {
		if err := validateLength(check.field, check.value, check.min, check.max, false); err != nil {
			return err
		}
	}

	if err := validateLength("customFullName", r.CustomFullName, 1, 100, true); err != nil {
		return err
	}

	if err := validateLength("customFullAddress", r.CustomFullAddress, 1, 200, true); err != nil {
		return err
	}

	address, err := mail.ParseAddress(r.Email)

	if err != nil || address.Address != r.Email {
		return newValidationError("email", "must be a valid email address")
	}

	if err := validateDocumentTypes(r.Scopes.Sharing); err != nil {
		return err
	}

	r.DeliveryTypes = []string{"api"}
	r.OfferRequestType = "dynamic"

	return nil
}

func validateOfferRequest(r *OfferRequest) error {
	if r == nil {
		return newValidationError("request", "is required")
	}

	if strings.TrimSpace(r.Name) == "" {
		return newValidationError("name", "is required")
	}

	if err := validateDocumentTypes(r.Scopes.Sharing); err != nil {
		return err
	}

	sharing := make(map[DocumentType]struct{}, len(r.Scopes.Sharing))

	for _, documentType := range r.Scopes.Sharing {
		sharing[documentType] = struct{}{}
	}

	mandatory := make(map[DocumentType]struct{}, len(r.Scopes.MandatorySharing))

	for _, documentType := range r.Scopes.MandatorySharing {
		if !documentType.Valid() {
			return newValidationError("scopes.mandatorySharing", "contains an unsupported document type")
		}

		if _, ok := sharing[documentType]; !ok {
			return newValidationError("scopes.mandatorySharing", "must be a subset of scopes.sharing")
		}

		if _, duplicate := mandatory[documentType]; duplicate {
			return newValidationError("scopes.mandatorySharing", "contains a duplicate document type")
		}

		mandatory[documentType] = struct{}{}
	}

	return validateHTTPS("returnLink", r.ReturnLink, true)
}

func validateOfflineShareRequest(r *OfflineShareRequest) error {
	if r == nil {
		return newValidationError("request", "is required")
	}

	if err := validateIdentifier("branchId", r.BranchID); err != nil {
		return err
	}

	if err := validateBarcode(r.Barcode); err != nil {
		return err
	}

	return validateRequestID(r.RequestID, false)
}

func validateOnlineShareRequest(r *OnlineShareRequest) error {
	if r == nil {
		return newValidationError("request", "is required")
	}

	if err := validateIdentifier("branchId", r.BranchID); err != nil {
		return err
	}

	if err := validateIdentifier("offerId", r.OfferID); err != nil {
		return err
	}

	if err := validateRequestID(r.RequestID, false); err != nil {
		return err
	}

	if r.SignAlgo != "" && r.SignAlgo != "DSTU4145-DSTU7564" {
		return newValidationError("signAlgo", "unsupported signing algorithm")
	}

	return validateHTTPS("returnLink", r.ReturnLink, true)
}

func validateOfflineStatusRequest(r *OfflineStatusRequest) error {
	if r == nil {
		return newValidationError("request", "is required")
	}

	if r.Barcode == "" && r.RequestID == "" {
		return newValidationError("request", "barcode or requestId is required")
	}

	if r.Barcode != "" {
		if err := validateBarcode(r.Barcode); err != nil {
			return err
		}
	}

	return validateRequestID(r.RequestID, true)
}

func validateOnlineStatusRequest(r *OnlineStatusRequest) error {
	if r == nil {
		return newValidationError("request", "is required")
	}

	if r.OTP == "" && r.RequestID == "" {
		return newValidationError("request", "otp or requestId is required")
	}

	if r.OTP != "" {
		if err := validateIdentifier("otp", r.OTP); err != nil {
			return err
		}
	}

	return validateRequestID(r.RequestID, true)
}

func validatePageRequest(r *PageRequest) error {
	if r == nil {
		return nil
	}

	if r.Skip != nil && *r.Skip < 0 {
		return newValidationError("skip", "must be zero or greater")
	}

	if r.Limit != nil && *r.Limit < 1 {
		return newValidationError("limit", "must be greater than zero")
	}

	return nil
}

func validateLength(field, value string, min, max int, optional bool) error {
	length := utf8.RuneCountInString(value)

	if optional && length == 0 {
		return nil
	}

	if length < min || length > max {
		return newValidationError(field, fmt.Sprintf("length must be between %d and %d", min, max))
	}

	return nil
}

var barcodePattern = regexp.MustCompile(`^[0-9]{13}$`)

func validateBarcode(value string) error {
	if !barcodePattern.MatchString(value) {
		return newValidationError("barcode", "must contain exactly 13 digits")
	}

	return nil
}

func validateRequestID(value string, optional bool) error {
	if optional && value == "" {
		return nil
	}

	if strings.TrimSpace(value) != value {
		return newValidationError("requestId", "must not contain surrounding whitespace")
	}

	if strings.ContainsAny(value, "/?#&=") {
		return newValidationError("requestId", "contains reserved URL characters")
	}

	return validateLength("requestId", value, 10, 255, false)
}

func validateHTTPS(field, value string, optional bool) error {
	if optional && value == "" {
		return nil
	}

	parsed, err := url.Parse(value)

	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return newValidationError(field, "must be an absolute HTTPS URL")
	}

	return nil
}

func validateDocumentTypes(types []DocumentType) error {
	if len(types) == 0 {
		return newValidationError("scopes.sharing", "must contain at least one document type")
	}

	seen := make(map[DocumentType]bool, len(types))
	for _, documentType := range types {
		if !documentType.Valid() {
			return newValidationError("scopes.sharing", fmt.Sprintf("unsupported document type %q", documentType))
		}
		if seen[documentType] {
			return newValidationError("scopes.sharing", "contains a duplicate document type")
		}
		seen[documentType] = true
	}

	return nil
}

func validateIdentifier(field, value string) error {
	trimmed := strings.TrimSpace(value)

	if trimmed == "" {
		return newValidationError(field, "is required")
	}

	if trimmed != value {
		return newValidationError(field, "must not contain surrounding whitespace")
	}

	if strings.ContainsAny(value, "/?#&=") {
		return newValidationError(field, "contains reserved URL characters")
	}

	return nil
}
