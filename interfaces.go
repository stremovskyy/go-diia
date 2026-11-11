package go_diia

import (
	"context"
	"net/http"

	"github.com/stremovskyy/go-diia/log"
)

// Diia is the root Document Sharing SDK interface.
type Diia interface {
	Auth() AuthService
	Branches() BranchService
	Offers() OfferService
	Sharing() SharingService
	Callbacks() CallbackService
	SetLogLevel(level log.Level)
}

type AuthService interface {
	SessionToken() SessionTokenBuilder
}

type BranchService interface {
	Create() BranchCreateBuilder
	Update() BranchUpdateBuilder
	Delete() BranchDeleteBuilder
	Get() BranchGetBuilder
	List() BranchListBuilder
}

type OfferService interface {
	Create() OfferCreateBuilder
	Delete() OfferDeleteBuilder
	List() OfferListBuilder
}

type SharingService interface {
	RequestOffline() OfflineShareBuilder
	RequestOnline() OnlineShareBuilder
	OfflineStatus() OfflineStatusBuilder
	OnlineStatus() OnlineStatusBuilder
	// OfferRequestPassword retrieves the password for an encrypted multi-sharing archive.
	OfferRequestPassword(ctx context.Context, offerRequestID string) (string, error)
}

type CallbackService interface {
	Parse() CallbackParseBuilder
	Process() CallbackProcessBuilder
	WriteSuccess() CallbackWriteSuccessBuilder
	WriteFailure() CallbackWriteFailureBuilder
}

// SessionTokenBuilder executes a session-token request.
type SessionTokenBuilder interface {
	Do(ctx context.Context) (*SessionToken, error)
}

type BranchCreateBuilder interface {
	WithName(value string) BranchCreateBuilder
	WithEmail(value string) BranchCreateBuilder
	WithRegion(value string) BranchCreateBuilder
	WithDistrict(value string) BranchCreateBuilder
	WithLocation(value string) BranchCreateBuilder
	WithStreet(value string) BranchCreateBuilder
	WithHouse(value string) BranchCreateBuilder
	WithCustomFullName(value string) BranchCreateBuilder
	WithCustomFullAddress(value string) BranchCreateBuilder
	WithSharing(types ...DocumentType) BranchCreateBuilder
	WithRunOptions(options ...RunOption) BranchCreateBuilder
	Do(ctx context.Context) (*Branch, error)
}

type BranchUpdateBuilder interface {
	WithBranchID(value string) BranchUpdateBuilder
	WithName(value string) BranchUpdateBuilder
	WithEmail(value string) BranchUpdateBuilder
	WithRegion(value string) BranchUpdateBuilder
	WithDistrict(value string) BranchUpdateBuilder
	WithLocation(value string) BranchUpdateBuilder
	WithStreet(value string) BranchUpdateBuilder
	WithHouse(value string) BranchUpdateBuilder
	WithCustomFullName(value string) BranchUpdateBuilder
	WithCustomFullAddress(value string) BranchUpdateBuilder
	WithSharing(types ...DocumentType) BranchUpdateBuilder
	WithRunOptions(options ...RunOption) BranchUpdateBuilder
	Do(ctx context.Context) (*Branch, error)
}

type BranchDeleteBuilder interface {
	WithBranchID(value string) BranchDeleteBuilder
	WithRunOptions(options ...RunOption) BranchDeleteBuilder
	Do(ctx context.Context) error
}

type BranchGetBuilder interface {
	WithBranchID(value string) BranchGetBuilder
	Do(ctx context.Context) (*Branch, error)
}

type BranchListBuilder interface {
	WithSkip(value int) BranchListBuilder
	WithLimit(value int) BranchListBuilder
	Do(ctx context.Context) (*BranchList, error)
}

type OfferCreateBuilder interface {
	WithBranchID(value string) OfferCreateBuilder
	WithName(value string) OfferCreateBuilder
	WithSharing(types ...DocumentType) OfferCreateBuilder
	WithMandatorySharing(types ...DocumentType) OfferCreateBuilder
	WithReturnLink(value string) OfferCreateBuilder
	WithRunOptions(options ...RunOption) OfferCreateBuilder
	Do(ctx context.Context) (*Offer, error)
}

type OfferDeleteBuilder interface {
	WithBranchID(value string) OfferDeleteBuilder
	WithOfferID(value string) OfferDeleteBuilder
	WithRunOptions(options ...RunOption) OfferDeleteBuilder
	Do(ctx context.Context) error
}

type OfferListBuilder interface {
	WithBranchID(value string) OfferListBuilder
	WithSkip(value int) OfferListBuilder
	WithLimit(value int) OfferListBuilder
	Do(ctx context.Context) (*OfferList, error)
}

type OfflineShareBuilder interface {
	WithBranchID(value string) OfflineShareBuilder
	WithBarcode(value string) OfflineShareBuilder
	WithRequestID(value string) OfflineShareBuilder
	WithDiiaID(enabled bool) OfflineShareBuilder
	WithRunOptions(options ...RunOption) OfflineShareBuilder
	Do(ctx context.Context) (*SuccessResponse, error)
}

type OnlineShareBuilder interface {
	WithBranchID(value string) OnlineShareBuilder
	WithOfferID(value string) OnlineShareBuilder
	WithRequestID(value string) OnlineShareBuilder
	WithReturnLink(value string) OnlineShareBuilder
	WithDiiaID(enabled bool) OnlineShareBuilder
	WithSignAlgo(value string) OnlineShareBuilder
	WithRunOptions(options ...RunOption) OnlineShareBuilder
	Do(ctx context.Context) (*DeeplinkResponse, error)
}

type OfflineStatusBuilder interface {
	WithBarcode(value string) OfflineStatusBuilder
	WithRequestID(value string) OfflineStatusBuilder
	Do(ctx context.Context) (SharingStatus, error)
}

type OnlineStatusBuilder interface {
	WithOTP(value string) OnlineStatusBuilder
	WithRequestID(value string) OnlineStatusBuilder
	Do(ctx context.Context) (SharingStatus, error)
}

type CallbackParseBuilder interface {
	WithRequest(request *http.Request) CallbackParseBuilder
	Do(ctx context.Context) (*CallbackPayload, error)
}

type CallbackProcessBuilder interface {
	WithPayload(payload *CallbackPayload) CallbackProcessBuilder
	Do(ctx context.Context) (*DocumentPackage, error)
}

type CallbackWriteSuccessBuilder interface {
	WithWriter(writer http.ResponseWriter) CallbackWriteSuccessBuilder
	Do(ctx context.Context) error
}

type CallbackWriteFailureBuilder interface {
	WithWriter(writer http.ResponseWriter) CallbackWriteFailureBuilder
	WithError(err error) CallbackWriteFailureBuilder
	Do(ctx context.Context) error
}
