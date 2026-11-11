package go_diia_test

import (
	"encoding/json"
	"errors"
	"testing"

	diia "github.com/stremovskyy/go-diia"
)

func TestPublicModelsRemainAvailableFromRootPackage(t *testing.T) {
	var mapper diia.CallbackErrorMapper = func(error) string { return "safe" }
	client := diia.NewClient("token", diia.WithCallbackErrorMapper(mapper))

	if client.Callbacks() == nil {
		t.Fatal("callback service is unavailable")
	}

	branch := diia.NewBranchRequest().
		WithName("Kyiv").
		WithEmail("test@example.com").
		WithRegion("Kyiv region").
		WithDistrict("Kyiv district").
		WithLocation("Kyiv").
		WithStreet("Khreshchatyk").
		WithHouse("1").
		WithSharing(diia.DocumentTypeInternalPassport)

	if branch.Scopes.Sharing[0] != diia.DocumentTypeInternalPassport {
		t.Fatalf("branch request = %#v", branch)
	}

	link := diia.DeeplinkResponse{Deeplink: "invalid"}

	if _, err := link.ParsedURL(); !errors.Is(err, diia.ErrDecode) {
		t.Fatalf("ParsedURL error = %v", err)
	}

	var metadata diia.Metadata

	if err := json.Unmarshal([]byte(`{"data":{"internal-passport":{"docNumber":"AA1"}}}`), &metadata); err != nil {
		t.Fatal(err)
	}

	if metadata.Data.InternalPassports[0].DocNumber != "AA1" {
		t.Fatalf("metadata = %#v", metadata)
	}

	payload := diia.CallbackPayload{
		RequestID: "request-123",
		Documents: []diia.CallbackDocument{{Type: diia.DocumentTypeInternalPassport}},
	}

	if payload.Documents[0].Type != diia.DocumentTypeInternalPassport {
		t.Fatalf("callback payload = %#v", payload)
	}
}
