package go_diia

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type authService struct{ client *client }

func (s *authService) sessionToken(ctx context.Context) (*SessionToken, error) {
	return s.client.sessionToken(ctx)
}

type branchService struct{ client *client }

func (s *branchService) create(ctx context.Context, request *BranchRequest, options ...RunOption) (*Branch, error) {
	if err := validateBranchRequest(request); err != nil {
		return nil, err
	}

	path := "/api/v2/acquirers/branch"

	if collectRunOptions(options).handle(path, request) {
		return branchFromRequest(request), nil
	}

	var result Branch

	if err := s.client.do(ctx, http.MethodPost, path, request, nil, true, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (s *branchService) update(
	ctx context.Context,
	branchID string,
	request *BranchRequest,
	options ...RunOption,
) (*Branch, error) {
	if err := validateIdentifier("branchId", branchID); err != nil {
		return nil, err
	}

	if err := validateBranchRequest(request); err != nil {
		return nil, err
	}

	path := "/api/v2/acquirers/branch/" + url.PathEscape(branchID)

	if collectRunOptions(options).handle(path, request) {
		result := branchFromRequest(request)
		result.ID = branchID

		return result, nil
	}

	var result Branch

	if err := s.client.do(ctx, http.MethodPut, path, request, nil, true, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (s *branchService) delete(ctx context.Context, branchID string, options ...RunOption) error {
	if err := validateIdentifier("branchId", branchID); err != nil {
		return err
	}

	path := "/api/v2/acquirers/branch/" + url.PathEscape(branchID)

	if collectRunOptions(options).handle(path, nil) {
		return nil
	}

	return s.client.do(ctx, http.MethodDelete, path, nil, nil, true, nil)
}

func (s *branchService) get(ctx context.Context, branchID string) (*Branch, error) {
	if err := validateIdentifier("branchId", branchID); err != nil {
		return nil, err
	}

	var result Branch
	path := "/api/v2/acquirers/branch/" + url.PathEscape(branchID)

	if err := s.client.do(ctx, http.MethodGet, path, nil, nil, true, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (s *branchService) list(ctx context.Context, page *PageRequest) (*BranchList, error) {
	if err := validatePageRequest(page); err != nil {
		return nil, err
	}

	path := withPage("/api/v2/acquirers/branches", page)
	var result BranchList

	if err := s.client.do(ctx, http.MethodGet, path, nil, nil, true, &result); err != nil {
		return nil, err
	}

	if result.Branches == nil {
		result.Branches = []Branch{}
	}

	return &result, nil
}

func branchFromRequest(request *BranchRequest) *Branch {
	return &Branch{
		Name: request.Name, Email: request.Email, Region: request.Region, District: request.District,
		Location: request.Location, Street: request.Street, House: request.House,
		CustomFullName: request.CustomFullName, CustomFullAddress: request.CustomFullAddress,
		DeliveryTypes: append([]string(nil), request.DeliveryTypes...), OfferRequestType: request.OfferRequestType,
		Scopes: BranchScopes{Sharing: append([]DocumentType(nil), request.Scopes.Sharing...)},
	}
}

type offerService struct{ client *client }

func (s *offerService) create(
	ctx context.Context,
	branchID string,
	request *OfferRequest,
	options ...RunOption,
) (*Offer, error) {
	if err := validateIdentifier("branchId", branchID); err != nil {
		return nil, err
	}

	if err := validateOfferRequest(request); err != nil {
		return nil, err
	}

	path := "/api/v1/acquirers/branch/" + url.PathEscape(branchID) + "/offer"

	if collectRunOptions(options).handle(path, request) {
		return &Offer{Name: request.Name, ReturnLink: request.ReturnLink, Scopes: request.Scopes}, nil
	}

	var result Offer

	if err := s.client.do(ctx, http.MethodPost, path, request, nil, true, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (s *offerService) delete(ctx context.Context, branchID, offerID string, options ...RunOption) error {
	if err := validateIdentifier("branchId", branchID); err != nil {
		return err
	}

	if err := validateIdentifier("offerId", offerID); err != nil {
		return err
	}

	path := "/api/v1/acquirers/branch/" + url.PathEscape(branchID) + "/offer/" + url.PathEscape(offerID)

	if collectRunOptions(options).handle(path, nil) {
		return nil
	}

	return s.client.do(ctx, http.MethodDelete, path, nil, nil, true, nil)
}

func (s *offerService) list(ctx context.Context, branchID string, page *PageRequest) (*OfferList, error) {
	if err := validateIdentifier("branchId", branchID); err != nil {
		return nil, err
	}

	if err := validatePageRequest(page); err != nil {
		return nil, err
	}

	path := withPage("/api/v1/acquirers/branch/"+url.PathEscape(branchID)+"/offers", page)
	var result OfferList

	if err := s.client.do(ctx, http.MethodGet, path, nil, nil, true, &result); err != nil {
		return nil, err
	}

	if result.Offers == nil {
		result.Offers = []Offer{}
	}

	return &result, nil
}

type sharingService struct{ client *client }

func (s *sharingService) OfferRequestPassword(ctx context.Context, offerRequestID string) (string, error) {
	if err := validateExecutionContext(ctx); err != nil {
		return "", err
	}

	if err := validateIdentifier("offerRequestId", offerRequestID); err != nil {
		return "", err
	}

	path := "/api/v1/acquirers/offer-request/" + url.PathEscape(offerRequestID) + "/password"
	var response struct {
		Password string `json:"password"`
	}

	if err := s.client.do(ctx, http.MethodGet, path, nil, nil, true, &response); err != nil {
		return "", err
	}

	if strings.TrimSpace(response.Password) == "" {
		return "", fmt.Errorf("%w: Diia returned an empty offer-request password", ErrDecode)
	}

	return response.Password, nil
}

func (s *sharingService) requestOffline(
	ctx context.Context,
	request *OfflineShareRequest,
	options ...RunOption,
) (*SuccessResponse, error) {
	if err := validateOfflineShareRequest(request); err != nil {
		return nil, err
	}

	path := "/api/v1/acquirers/document-request"

	if collectRunOptions(options).handle(path, request) {
		return &SuccessResponse{Success: true}, nil
	}

	var result SuccessResponse

	if err := s.client.do(ctx, http.MethodPost, path, request, nil, true, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (s *sharingService) requestOnline(
	ctx context.Context,
	request *OnlineShareRequest,
	options ...RunOption,
) (*DeeplinkResponse, error) {
	if err := validateOnlineShareRequest(request); err != nil {
		return nil, err
	}

	path := "/api/v2/acquirers/branch/" + url.PathEscape(request.BranchID) + "/offer-request/dynamic"

	if collectRunOptions(options).handle(path, request) {
		return &DeeplinkResponse{}, nil
	}

	var result DeeplinkResponse

	if err := s.client.do(ctx, http.MethodPost, path, request, nil, true, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (s *sharingService) offlineStatus(ctx context.Context, request *OfflineStatusRequest) (SharingStatus, error) {
	if err := validateOfflineStatusRequest(request); err != nil {
		return "", err
	}

	query := make(url.Values)

	if request.Barcode != "" {
		query.Set("barcode", request.Barcode)
	}

	if request.RequestID != "" {
		query.Set("requestId", request.RequestID)
	}

	return s.status(ctx, "/api/v1/acquirers/document-request/status?"+query.Encode())
}

func (s *sharingService) onlineStatus(ctx context.Context, request *OnlineStatusRequest) (SharingStatus, error) {
	if err := validateOnlineStatusRequest(request); err != nil {
		return "", err
	}

	query := make(url.Values)

	if request.OTP != "" {
		query.Set("otp", request.OTP)
	}

	if request.RequestID != "" {
		query.Set("requestId", request.RequestID)
	}

	return s.status(ctx, "/api/v1/acquirers/offer-request/status?"+query.Encode())
}

func (s *sharingService) status(ctx context.Context, path string) (SharingStatus, error) {
	var response statusResponse

	if err := s.client.do(ctx, http.MethodGet, path, nil, nil, true, &response); err != nil {
		return "", err
	}

	if !response.Status.Valid() {
		return "", fmt.Errorf("%w: Diia returned unknown sharing status %q", ErrDecode, response.Status)
	}

	return response.Status, nil
}

func withPage(path string, page *PageRequest) string {
	if page == nil {
		return path
	}

	query := make(url.Values)

	if page.Skip != nil {
		query.Set("skip", strconv.Itoa(*page.Skip))
	}

	if page.Limit != nil {
		query.Set("limit", strconv.Itoa(*page.Limit))
	}

	if len(query) == 0 {
		return path
	}

	return path + "?" + query.Encode()
}
