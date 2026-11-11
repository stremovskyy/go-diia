package go_diia

import (
	"github.com/stremovskyy/go-diia/internal/models/api"
)

type (
	SessionToken         = api.SessionToken
	BranchScopes         = api.BranchScopes
	Branch               = api.Branch
	BranchList           = api.BranchList
	OfferScopes          = api.OfferScopes
	Offer                = api.Offer
	OfferList            = api.OfferList
	SuccessResponse      = api.SuccessResponse
	DeeplinkResponse     = api.DeeplinkResponse
	BranchRequest        = api.BranchRequest
	OfferRequest         = api.OfferRequest
	OfflineShareRequest  = api.OfflineShareRequest
	OnlineShareRequest   = api.OnlineShareRequest
	OfflineStatusRequest = api.OfflineStatusRequest
	OnlineStatusRequest  = api.OnlineStatusRequest
	PageRequest          = api.PageRequest
)

func NewBranchRequest() *BranchRequest { return api.NewBranchRequest() }

func NewOfferRequest() *OfferRequest { return api.NewOfferRequest() }

func NewOfflineShareRequest() *OfflineShareRequest { return api.NewOfflineShareRequest() }

func NewOnlineShareRequest() *OnlineShareRequest { return api.NewOnlineShareRequest() }

func NewOfflineStatusRequest() *OfflineStatusRequest { return api.NewOfflineStatusRequest() }

func NewOnlineStatusRequest() *OnlineStatusRequest { return api.NewOnlineStatusRequest() }

func NewPageRequest() *PageRequest { return api.NewPageRequest() }

type statusResponse = api.StatusResponse
