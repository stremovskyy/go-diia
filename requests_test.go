package go_diia

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestBranchBuilderDefaultsAndValidation(t *testing.T) {
	request := NewBranchRequest().
		WithName("Kyiv").
		WithEmail("test@example.com").
		WithRegion("Kyiv region").
		WithDistrict("Kyiv district").
		WithLocation("Kyiv").
		WithStreet("Khreshchatyk").
		WithHouse("1").
		WithCustomFullName("Partner branch").
		WithCustomFullAddress("Kyiv, 1 Khreshchatyk").
		WithSharing(DocumentTypeInternalPassport, DocumentTypeTaxpayerCard)

	if err := validateBranchRequest(request); err != nil {
		t.Fatalf("validate: %v", err)
	}

	if !reflect.DeepEqual(request.DeliveryTypes, []string{"api"}) || request.OfferRequestType != "dynamic" {
		t.Fatalf("documented defaults were not applied: %#v", request)
	}

	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}

	var wire map[string]any

	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(wire["deliveryTypes"], []any{"api"}) || wire["offerRequestType"] != "dynamic" {
		t.Fatalf("unexpected wire payload: %s", data)
	}

	invalid := NewBranchRequest().
		WithEmail("bad").
		WithRegion("x").
		WithDistrict("x").
		WithLocation("x").
		WithStreet("x")

	if err := validateBranchRequest(invalid); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestSharingBuildersValidateDocumentedConstraints(t *testing.T) {
	tests := []struct {
		name     string
		validate func() error
		valid    bool
	}{
		{"offline valid", func() error {
			return validateOfflineShareRequest(NewOfflineShareRequest().
				WithBranchID("branch").
				WithBarcode("1234567890123").
				WithRequestID("request-123"))
		}, true},
		{"barcode length", func() error {
			return validateOfflineShareRequest(NewOfflineShareRequest().
				WithBranchID("branch").
				WithBarcode("123").
				WithRequestID("request-123"))
		}, false},
		{"request id short", func() error {
			return validateOfflineShareRequest(NewOfflineShareRequest().
				WithBranchID("branch").
				WithBarcode("1234567890123").
				WithRequestID("short"))
		}, false},
		{
			"online valid",
			func() error {
				return validateOnlineShareRequest(
					NewOnlineShareRequest().
						WithBranchID("branch").
						WithOfferID("offer").
						WithRequestID("request-123").
						WithReturnLink("https://example.com/done"),
				)
			},
			true,
		},
		{
			"online non-https",
			func() error {
				return validateOnlineShareRequest(
					NewOnlineShareRequest().
						WithBranchID("branch").
						WithOfferID("offer").
						WithRequestID("request-123").
						WithReturnLink("http://example.com"),
				)
			},
			false,
		},
		{"offline status barcode", func() error {
			return validateOfflineStatusRequest(NewOfflineStatusRequest().WithBarcode("1234567890123"))
		}, true},
		{
			"offline status empty",
			func() error { return validateOfflineStatusRequest(NewOfflineStatusRequest()) },
			false,
		},
		{
			"online status otp",
			func() error { return validateOnlineStatusRequest(NewOnlineStatusRequest().WithOTP("otp-value")) },
			true,
		},
		{"online status empty", func() error { return validateOnlineStatusRequest(NewOnlineStatusRequest()) }, false},
		{"page negative", func() error { return validatePageRequest(NewPageRequest().WithSkip(-1)) }, false},
		{"page valid", func() error { return validatePageRequest(NewPageRequest().WithSkip(0).WithLimit(100)) }, true},
		{"raw branch id whitespace", func() error {
			return validateOfflineShareRequest(&OfflineShareRequest{
				BranchID: " branch ", Barcode: "1234567890123", RequestID: "request-123",
			})
		}, false},
		{"raw offer id reserved", func() error {
			return validateOnlineShareRequest(&OnlineShareRequest{
				BranchID: "branch", OfferID: "offer/child", RequestID: "request-123",
			})
		}, false},
		{"request id surrounding whitespace", func() error {
			return validateOfflineStatusRequest(&OfflineStatusRequest{RequestID: " request-123 "})
		}, false},
		{"otp reserved query separator", func() error {
			return validateOnlineStatusRequest(&OnlineStatusRequest{OTP: "otp&other=value"})
		}, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.validate()

			if test.valid && err != nil {
				t.Fatalf("expected valid request: %v", err)
			}

			if !test.valid && !errors.Is(err, ErrValidation) {
				t.Fatalf("expected validation error, got %v", err)
			}
		})
	}
}

func TestDeeplinkHelpersAndStatuses(t *testing.T) {
	response := DeeplinkResponse{
		Deeplink: "https://diia.app/acquirers/offer-request/dynamic/38abd418-f786-4a4d-a4d6-24fc842dacfc",
	}
	parsed, err := response.ParsedURL()

	if err != nil || parsed.Host != "diia.app" {
		t.Fatalf("ParsedURL: %v, %v", parsed, err)
	}

	otp, err := response.OTP()

	if err != nil || otp != "38abd418-f786-4a4d-a4d6-24fc842dacfc" {
		t.Fatalf("OTP: %q, %v", otp, err)
	}

	for _, status := range []SharingStatus{
		SharingStatusInitialized, SharingStatusProcessing, SharingStatusDone, SharingStatusRefuse, SharingStatusNotUsed,
	} {
		if !status.Valid() {
			t.Fatalf("documented status %q is invalid", status)
		}
	}

	if SharingStatus("future").Valid() {
		t.Fatal("unknown status must be invalid")
	}
}

func TestDryRunSkipsNetworkAndExposesTypedPayload(t *testing.T) {
	client := NewClient("token", WithBaseURL("http://127.0.0.1:1"))
	var endpoint string
	var payload any
	branch, err := client.Branches().Create().
		WithName("Kyiv").
		WithEmail("test@example.com").
		WithRegion("Kyiv region").
		WithDistrict("Kyiv district").
		WithLocation("Kyiv").
		WithStreet("Khreshchatyk").
		WithHouse("1").
		WithSharing(DocumentTypeInternalPassport).
		WithRunOptions(DryRun(func(gotEndpoint string, gotPayload any) {
			endpoint, payload = gotEndpoint, gotPayload
		})).
		Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	request, ok := payload.(*BranchRequest)

	if endpoint != "/api/v2/acquirers/branch" || !ok || branch.Name != request.Name {
		t.Fatalf("unexpected dry run: endpoint=%q payload=%T branch=%#v", endpoint, payload, branch)
	}
}
