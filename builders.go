package go_diia

import (
	"context"
	"strings"
)

var (
	_ SessionTokenBuilder  = (*sessionTokenBuilder)(nil)
	_ BranchCreateBuilder  = (*branchCreateBuilder)(nil)
	_ BranchUpdateBuilder  = (*branchUpdateBuilder)(nil)
	_ BranchDeleteBuilder  = (*branchDeleteBuilder)(nil)
	_ BranchGetBuilder     = (*branchGetBuilder)(nil)
	_ BranchListBuilder    = (*branchListBuilder)(nil)
	_ OfferCreateBuilder   = (*offerCreateBuilder)(nil)
	_ OfferDeleteBuilder   = (*offerDeleteBuilder)(nil)
	_ OfferListBuilder     = (*offerListBuilder)(nil)
	_ OfflineShareBuilder  = (*offlineShareBuilder)(nil)
	_ OnlineShareBuilder   = (*onlineShareBuilder)(nil)
	_ OfflineStatusBuilder = (*offlineStatusBuilder)(nil)
	_ OnlineStatusBuilder  = (*onlineStatusBuilder)(nil)
)

func validateExecutionContext(ctx context.Context) error {
	if ctx == nil {
		return newValidationError("context", "is required")
	}

	return ctx.Err()
}

type sessionTokenBuilder struct{ service *authService }

func (s *authService) SessionToken() SessionTokenBuilder {
	return &sessionTokenBuilder{service: s}
}

func (b *sessionTokenBuilder) Do(ctx context.Context) (*SessionToken, error) {
	if err := validateExecutionContext(ctx); err != nil {
		return nil, err
	}

	return b.service.sessionToken(ctx)
}

type branchCreateBuilder struct {
	service *branchService
	request *BranchRequest
	options []RunOption
}

func (s *branchService) Create() BranchCreateBuilder {
	return &branchCreateBuilder{service: s, request: NewBranchRequest()}
}

func (b *branchCreateBuilder) WithName(value string) BranchCreateBuilder {
	b.request.WithName(value)

	return b
}

func (b *branchCreateBuilder) WithEmail(value string) BranchCreateBuilder {
	b.request.WithEmail(value)

	return b
}

func (b *branchCreateBuilder) WithRegion(value string) BranchCreateBuilder {
	b.request.WithRegion(value)

	return b
}

func (b *branchCreateBuilder) WithDistrict(value string) BranchCreateBuilder {
	b.request.WithDistrict(value)

	return b
}

func (b *branchCreateBuilder) WithLocation(value string) BranchCreateBuilder {
	b.request.WithLocation(value)

	return b
}

func (b *branchCreateBuilder) WithStreet(value string) BranchCreateBuilder {
	b.request.WithStreet(value)

	return b
}

func (b *branchCreateBuilder) WithHouse(value string) BranchCreateBuilder {
	b.request.WithHouse(value)

	return b
}

func (b *branchCreateBuilder) WithCustomFullName(value string) BranchCreateBuilder {
	b.request.WithCustomFullName(value)

	return b
}

func (b *branchCreateBuilder) WithCustomFullAddress(value string) BranchCreateBuilder {
	b.request.WithCustomFullAddress(value)

	return b
}

func (b *branchCreateBuilder) WithSharing(types ...DocumentType) BranchCreateBuilder {
	b.request.WithSharing(types...)

	return b
}

func (b *branchCreateBuilder) WithRunOptions(options ...RunOption) BranchCreateBuilder {
	b.options = append(b.options, options...)

	return b
}

func (b *branchCreateBuilder) Do(ctx context.Context) (*Branch, error) {
	if err := validateExecutionContext(ctx); err != nil {
		return nil, err
	}

	return b.service.create(ctx, b.request, b.options...)
}

type branchUpdateBuilder struct {
	service  *branchService
	branchID string
	request  *BranchRequest
	options  []RunOption
}

func (s *branchService) Update() BranchUpdateBuilder {
	return &branchUpdateBuilder{service: s, request: NewBranchRequest()}
}

func (b *branchUpdateBuilder) WithBranchID(value string) BranchUpdateBuilder {
	b.branchID = strings.TrimSpace(value)

	return b
}

func (b *branchUpdateBuilder) WithName(value string) BranchUpdateBuilder {
	b.request.WithName(value)

	return b
}

func (b *branchUpdateBuilder) WithEmail(value string) BranchUpdateBuilder {
	b.request.WithEmail(value)

	return b
}

func (b *branchUpdateBuilder) WithRegion(value string) BranchUpdateBuilder {
	b.request.WithRegion(value)

	return b
}

func (b *branchUpdateBuilder) WithDistrict(value string) BranchUpdateBuilder {
	b.request.WithDistrict(value)

	return b
}

func (b *branchUpdateBuilder) WithLocation(value string) BranchUpdateBuilder {
	b.request.WithLocation(value)

	return b
}

func (b *branchUpdateBuilder) WithStreet(value string) BranchUpdateBuilder {
	b.request.WithStreet(value)

	return b
}

func (b *branchUpdateBuilder) WithHouse(value string) BranchUpdateBuilder {
	b.request.WithHouse(value)

	return b
}

func (b *branchUpdateBuilder) WithCustomFullName(value string) BranchUpdateBuilder {
	b.request.WithCustomFullName(value)

	return b
}

func (b *branchUpdateBuilder) WithCustomFullAddress(value string) BranchUpdateBuilder {
	b.request.WithCustomFullAddress(value)

	return b
}

func (b *branchUpdateBuilder) WithSharing(types ...DocumentType) BranchUpdateBuilder {
	b.request.WithSharing(types...)

	return b
}

func (b *branchUpdateBuilder) WithRunOptions(options ...RunOption) BranchUpdateBuilder {
	b.options = append(b.options, options...)

	return b
}

func (b *branchUpdateBuilder) Do(ctx context.Context) (*Branch, error) {
	if err := validateExecutionContext(ctx); err != nil {
		return nil, err
	}

	return b.service.update(ctx, b.branchID, b.request, b.options...)
}

type branchDeleteBuilder struct {
	service  *branchService
	branchID string
	options  []RunOption
}

func (s *branchService) Delete() BranchDeleteBuilder { return &branchDeleteBuilder{service: s} }

func (b *branchDeleteBuilder) WithBranchID(value string) BranchDeleteBuilder {
	b.branchID = strings.TrimSpace(value)

	return b
}

func (b *branchDeleteBuilder) WithRunOptions(options ...RunOption) BranchDeleteBuilder {
	b.options = append(b.options, options...)

	return b
}

func (b *branchDeleteBuilder) Do(ctx context.Context) error {
	if err := validateExecutionContext(ctx); err != nil {
		return err
	}

	return b.service.delete(ctx, b.branchID, b.options...)
}

type branchGetBuilder struct {
	service  *branchService
	branchID string
}

func (s *branchService) Get() BranchGetBuilder { return &branchGetBuilder{service: s} }

func (b *branchGetBuilder) WithBranchID(value string) BranchGetBuilder {
	b.branchID = strings.TrimSpace(value)

	return b
}

func (b *branchGetBuilder) Do(ctx context.Context) (*Branch, error) {
	if err := validateExecutionContext(ctx); err != nil {
		return nil, err
	}

	return b.service.get(ctx, b.branchID)
}

type branchListBuilder struct {
	service *branchService
	page    *PageRequest
}

func (s *branchService) List() BranchListBuilder {
	return &branchListBuilder{service: s, page: NewPageRequest()}
}

func (b *branchListBuilder) WithSkip(value int) BranchListBuilder {
	b.page.WithSkip(value)

	return b
}

func (b *branchListBuilder) WithLimit(value int) BranchListBuilder {
	b.page.WithLimit(value)

	return b
}

func (b *branchListBuilder) Do(ctx context.Context) (*BranchList, error) {
	if err := validateExecutionContext(ctx); err != nil {
		return nil, err
	}

	return b.service.list(ctx, b.page)
}

type offerCreateBuilder struct {
	service  *offerService
	branchID string
	request  *OfferRequest
	options  []RunOption
}

func (s *offerService) Create() OfferCreateBuilder {
	return &offerCreateBuilder{service: s, request: NewOfferRequest()}
}

func (b *offerCreateBuilder) WithBranchID(value string) OfferCreateBuilder {
	b.branchID = strings.TrimSpace(value)

	return b
}

func (b *offerCreateBuilder) WithName(value string) OfferCreateBuilder {
	b.request.WithName(value)

	return b
}

func (b *offerCreateBuilder) WithSharing(types ...DocumentType) OfferCreateBuilder {
	b.request.WithSharing(types...)

	return b
}

func (b *offerCreateBuilder) WithMandatorySharing(types ...DocumentType) OfferCreateBuilder {
	b.request.WithMandatorySharing(types...)

	return b
}

func (b *offerCreateBuilder) WithReturnLink(value string) OfferCreateBuilder {
	b.request.WithReturnLink(value)

	return b
}

func (b *offerCreateBuilder) WithRunOptions(options ...RunOption) OfferCreateBuilder {
	b.options = append(b.options, options...)

	return b
}

func (b *offerCreateBuilder) Do(ctx context.Context) (*Offer, error) {
	if err := validateExecutionContext(ctx); err != nil {
		return nil, err
	}

	return b.service.create(ctx, b.branchID, b.request, b.options...)
}

type offerDeleteBuilder struct {
	service  *offerService
	branchID string
	offerID  string
	options  []RunOption
}

func (s *offerService) Delete() OfferDeleteBuilder { return &offerDeleteBuilder{service: s} }

func (b *offerDeleteBuilder) WithBranchID(value string) OfferDeleteBuilder {
	b.branchID = strings.TrimSpace(value)

	return b
}

func (b *offerDeleteBuilder) WithOfferID(value string) OfferDeleteBuilder {
	b.offerID = strings.TrimSpace(value)

	return b
}

func (b *offerDeleteBuilder) WithRunOptions(options ...RunOption) OfferDeleteBuilder {
	b.options = append(b.options, options...)

	return b
}

func (b *offerDeleteBuilder) Do(ctx context.Context) error {
	if err := validateExecutionContext(ctx); err != nil {
		return err
	}

	return b.service.delete(ctx, b.branchID, b.offerID, b.options...)
}

type offerListBuilder struct {
	service  *offerService
	branchID string
	page     *PageRequest
}

func (s *offerService) List() OfferListBuilder {
	return &offerListBuilder{service: s, page: NewPageRequest()}
}

func (b *offerListBuilder) WithBranchID(value string) OfferListBuilder {
	b.branchID = strings.TrimSpace(value)

	return b
}

func (b *offerListBuilder) WithSkip(value int) OfferListBuilder {
	b.page.WithSkip(value)

	return b
}

func (b *offerListBuilder) WithLimit(value int) OfferListBuilder {
	b.page.WithLimit(value)

	return b
}

func (b *offerListBuilder) Do(ctx context.Context) (*OfferList, error) {
	if err := validateExecutionContext(ctx); err != nil {
		return nil, err
	}

	return b.service.list(ctx, b.branchID, b.page)
}

type offlineShareBuilder struct {
	service *sharingService
	request *OfflineShareRequest
	options []RunOption
}

func (s *sharingService) RequestOffline() OfflineShareBuilder {
	return &offlineShareBuilder{service: s, request: NewOfflineShareRequest()}
}

func (b *offlineShareBuilder) WithBranchID(value string) OfflineShareBuilder {
	b.request.WithBranchID(value)

	return b
}

func (b *offlineShareBuilder) WithBarcode(value string) OfflineShareBuilder {
	b.request.WithBarcode(value)

	return b
}

func (b *offlineShareBuilder) WithRequestID(value string) OfflineShareBuilder {
	b.request.WithRequestID(value)

	return b
}

func (b *offlineShareBuilder) WithDiiaID(enabled bool) OfflineShareBuilder {
	b.request.WithDiiaID(enabled)

	return b
}

func (b *offlineShareBuilder) WithRunOptions(options ...RunOption) OfflineShareBuilder {
	b.options = append(b.options, options...)

	return b
}

func (b *offlineShareBuilder) Do(ctx context.Context) (*SuccessResponse, error) {
	if err := validateExecutionContext(ctx); err != nil {
		return nil, err
	}

	return b.service.requestOffline(ctx, b.request, b.options...)
}

type onlineShareBuilder struct {
	service *sharingService
	request *OnlineShareRequest
	options []RunOption
}

func (s *sharingService) RequestOnline() OnlineShareBuilder {
	return &onlineShareBuilder{service: s, request: NewOnlineShareRequest()}
}

func (b *onlineShareBuilder) WithBranchID(value string) OnlineShareBuilder {
	b.request.WithBranchID(value)

	return b
}

func (b *onlineShareBuilder) WithOfferID(value string) OnlineShareBuilder {
	b.request.WithOfferID(value)

	return b
}

func (b *onlineShareBuilder) WithRequestID(value string) OnlineShareBuilder {
	b.request.WithRequestID(value)

	return b
}

func (b *onlineShareBuilder) WithReturnLink(value string) OnlineShareBuilder {
	b.request.WithReturnLink(value)

	return b
}

func (b *onlineShareBuilder) WithDiiaID(enabled bool) OnlineShareBuilder {
	b.request.WithDiiaID(enabled)

	return b
}

func (b *onlineShareBuilder) WithSignAlgo(value string) OnlineShareBuilder {
	b.request.WithSignAlgo(value)

	return b
}

func (b *onlineShareBuilder) WithRunOptions(options ...RunOption) OnlineShareBuilder {
	b.options = append(b.options, options...)

	return b
}

func (b *onlineShareBuilder) Do(ctx context.Context) (*DeeplinkResponse, error) {
	if err := validateExecutionContext(ctx); err != nil {
		return nil, err
	}

	return b.service.requestOnline(ctx, b.request, b.options...)
}

type offlineStatusBuilder struct {
	service *sharingService
	request *OfflineStatusRequest
}

func (s *sharingService) OfflineStatus() OfflineStatusBuilder {
	return &offlineStatusBuilder{service: s, request: NewOfflineStatusRequest()}
}

func (b *offlineStatusBuilder) WithBarcode(value string) OfflineStatusBuilder {
	b.request.WithBarcode(value)

	return b
}

func (b *offlineStatusBuilder) WithRequestID(value string) OfflineStatusBuilder {
	b.request.WithRequestID(value)

	return b
}

func (b *offlineStatusBuilder) Do(ctx context.Context) (SharingStatus, error) {
	if err := validateExecutionContext(ctx); err != nil {
		return "", err
	}

	return b.service.offlineStatus(ctx, b.request)
}

type onlineStatusBuilder struct {
	service *sharingService
	request *OnlineStatusRequest
}

func (s *sharingService) OnlineStatus() OnlineStatusBuilder {
	return &onlineStatusBuilder{service: s, request: NewOnlineStatusRequest()}
}

func (b *onlineStatusBuilder) WithOTP(value string) OnlineStatusBuilder {
	b.request.WithOTP(value)

	return b
}

func (b *onlineStatusBuilder) WithRequestID(value string) OnlineStatusBuilder {
	b.request.WithRequestID(value)

	return b
}

func (b *onlineStatusBuilder) Do(ctx context.Context) (SharingStatus, error) {
	if err := validateExecutionContext(ctx); err != nil {
		return "", err
	}

	return b.service.onlineStatus(ctx, b.request)
}
