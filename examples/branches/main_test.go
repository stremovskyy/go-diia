package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	diia "github.com/stremovskyy/go-diia"
)

func TestUpdateBranchPreservesModeledFields(t *testing.T) {
	putBody := make(chan diia.BranchRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/auth/acquirer/test-token":
			_, _ = w.Write([]byte(`{"token":"session-token"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/acquirers/branch/branch-1/offers":
			_, _ = w.Write([]byte(`{"total":0,"offers":[]}`))

		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/acquirers/branch/branch-1":
			_, _ = w.Write(
				[]byte(
					`{"_id":"branch-1","name":"Old name","email":"owner@example.com","region":"Kyiv region","district":"Kyiv district","location":"Kyiv","street":"Main Street","house":"12","customFullName":"Existing display name","customFullAddress":"Existing display address","deliveryTypes":["api"],"offerRequestType":"dynamic","scopes":{"sharing":["internal-passport","taxpayer-card"]}}`,
				),
			)

		case r.Method == http.MethodPut && r.URL.Path == "/api/v2/acquirers/branch/branch-1":
			var body diia.BranchRequest

			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode PUT: %v", err)
				w.WriteHeader(http.StatusBadRequest)

				return
			}

			putBody <- body
			_, _ = w.Write([]byte(`{"_id":"branch-1"}`))

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := diia.NewClient("test-token", diia.WithBaseURL(server.URL))

	if err := updateBranch(context.Background(), client, "branch-1", "New name"); err != nil {
		t.Fatalf("updateBranch: %v", err)
	}

	want := diia.BranchRequest{
		Name: "New name", Email: "owner@example.com", Region: "Kyiv region",
		District: "Kyiv district", Location: "Kyiv", Street: "Main Street", House: "12",
		CustomFullName: "Existing display name", CustomFullAddress: "Existing display address",
		DeliveryTypes: []string{"api"}, OfferRequestType: "dynamic",
		Scopes: diia.BranchScopes{Sharing: []diia.DocumentType{
			diia.DocumentTypeInternalPassport, diia.DocumentTypeTaxpayerCard,
		}},
	}

	select {
	case got := <-putBody:
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("PUT changed an existing modeled field:\ngot  %+v\nwant %+v", got, want)
		}
	default:
		t.Fatal("branch PUT was not sent")
	}
}

func TestUpdateBranchRefusesWithOffers(t *testing.T) {
	var branchRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/v1/auth/acquirer/test-token":
			_, _ = w.Write([]byte(`{"token":"session-token"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/acquirers/branch/branch-1/offers":
			_, _ = w.Write([]byte(`{"total":1,"offers":[{"_id":"offer-1"}]}`))

		default:
			branchRequests.Add(1)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := diia.NewClient("test-token", diia.WithBaseURL(server.URL))
	err := updateBranch(context.Background(), client, "branch-1", "New name")

	if !errors.Is(err, errBranchHasOffers) || branchRequests.Load() != 0 {
		t.Fatalf("expected refusal before branch request; err=%v branchRequests=%d", err, branchRequests.Load())
	}
}

func TestUpdateBranchRequiresExplicitOptIn(t *testing.T) {
	t.Setenv("DIIA_ENV", "sandbox")
	t.Setenv("DIIA_ACQUIRER_TOKEN", "test-token")
	t.Setenv("DIIA_AUTH_ACQUIRER_TOKEN", "test-auth-token")
	t.Setenv("DIIA_ALLOW_BRANCH_REPLACEMENT", "")

	if err := run("update"); !errors.Is(err, errBranchReplacementNotAllowed) {
		t.Fatalf("expected replacement opt-in error, got %v", err)
	}
}
