// Command branches demonstrates the Diia branch operations. It selects the
// sandbox unless DIIA_ENV=production is set explicitly.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	diia "github.com/stremovskyy/go-diia"
	"github.com/stremovskyy/go-diia/examples/internal/exampleutil"
)

var (
	errMissingBranchID             = errors.New("branch response did not contain an ID")
	errBranchHasOffers             = errors.New("delete this branch's offers before updating it")
	errUnsafeUpdate                = errors.New("branch configuration cannot be updated by this example")
	errBranchReplacementNotAllowed = errors.New("DIIA_ALLOW_BRANCH_REPLACEMENT=1 is required for branch update")
)

type branchFields struct {
	name     string
	email    string
	region   string
	district string
	location string
	street   string
	house    string
}

func main() {
	if len(os.Args) != 2 || !validAction(os.Args[1]) {
		fmt.Fprintln(os.Stderr, "usage: go run ./examples/branches create|list|get|update|delete")
		os.Exit(2)
	}

	if err := run(os.Args[1]); err != nil {
		switch {
		case errors.Is(err, errMissingBranchID):
			fmt.Fprintln(os.Stderr, errMissingBranchID)
		case errors.Is(err, errBranchHasOffers):
			fmt.Fprintln(os.Stderr, errBranchHasOffers)
		case errors.Is(err, errUnsafeUpdate):
			fmt.Fprintln(os.Stderr, errUnsafeUpdate)
		case errors.Is(err, errBranchReplacementNotAllowed):
			fmt.Fprintln(os.Stderr, errBranchReplacementNotAllowed)
		default:
			fmt.Fprintln(os.Stderr, exampleutil.SafeError(err))
		}

		os.Exit(1)
	}
}

func validAction(action string) bool {
	switch action {
	case "create", "list", "get", "update", "delete":
		return true
	default:
		return false
	}
}

func run(action string) error {
	cfg, err := exampleutil.Load()
	if err != nil {
		return err
	}

	if action == "create" || action == "update" || action == "delete" {
		if err := cfg.AllowMutation(); err != nil {
			return err
		}
	}

	if action == "update" && os.Getenv("DIIA_ALLOW_BRANCH_REPLACEMENT") != "1" {
		return errBranchReplacementNotAllowed
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	switch action {
	case "create":
		fields, err := loadBranchFields()
		if err != nil {
			return err
		}

		branch, err := cfg.Client.Branches().Create().
			WithName(fields.name).
			WithEmail(fields.email).
			WithRegion(fields.region).
			WithDistrict(fields.district).
			WithLocation(fields.location).
			WithStreet(fields.street).
			WithHouse(fields.house).
			WithSharing(diia.DocumentTypeInternalPassport).
			Do(ctx)
		if err != nil {
			return err
		}

		if branch == nil || branch.ID == "" {
			return errMissingBranchID
		}

		fmt.Printf("branch_id=%s\n", branch.ID)

	case "list":
		branches, err := cfg.Client.Branches().List().WithSkip(0).WithLimit(100).Do(ctx)
		if err != nil {
			return err
		}

		fmt.Printf("total=%d\n", branches.Total)

		for _, branch := range branches.Branches {
			fmt.Printf("branch_id=%s name=%q\n", branch.ID, branch.Name)
		}

	case "get":
		id, err := exampleutil.RequiredEnv("DIIA_BRANCH_ID")
		if err != nil {
			return err
		}

		branch, err := cfg.Client.Branches().Get().WithBranchID(id).Do(ctx)
		if err != nil {
			return err
		}

		fmt.Printf("branch_id=%s name=%q\n", branch.ID, branch.Name)

	case "update":
		id, err := exampleutil.RequiredEnv("DIIA_BRANCH_ID")
		if err != nil {
			return err
		}

		name, err := exampleutil.RequiredEnv("DIIA_BRANCH_NAME")
		if err != nil {
			return err
		}

		if err := updateBranch(ctx, cfg.Client, id, name); err != nil {
			return err
		}

		fmt.Printf("branch_updated=%s\n", id)

	case "delete":
		id, err := exampleutil.RequiredEnv("DIIA_BRANCH_ID")
		if err != nil {
			return err
		}

		if err := cfg.Client.Branches().Delete().WithBranchID(id).Do(ctx); err != nil {
			return err
		}

		fmt.Printf("branch_deleted=%s\n", id)
	}

	return nil
}

// updateBranch copies every branch field represented by this SDK into the PUT
// payload. Fields added by Diia that the SDK does not model cannot be preserved.
// The caller must explicitly opt in and use only a branch it owns for testing.
func updateBranch(ctx context.Context, client diia.Diia, id, name string) error {
	offers, err := client.Offers().List().WithBranchID(id).WithSkip(0).WithLimit(1).Do(ctx)
	if err != nil {
		return err
	}

	if offers == nil {
		return errUnsafeUpdate
	}

	if offers.Total > 0 || len(offers.Offers) > 0 {
		return errBranchHasOffers
	}

	branch, err := client.Branches().Get().WithBranchID(id).Do(ctx)
	if err != nil {
		return err
	}

	if branch == nil || branch.ID != id || len(branch.Scopes.Sharing) == 0 ||
		len(branch.DeliveryTypes) != 1 || branch.DeliveryTypes[0] != "api" ||
		branch.OfferRequestType != "dynamic" {
		return errUnsafeUpdate
	}

	_, err = client.Branches().Update().
		WithBranchID(id).
		WithName(name).
		WithEmail(branch.Email).
		WithRegion(branch.Region).
		WithDistrict(branch.District).
		WithLocation(branch.Location).
		WithStreet(branch.Street).
		WithHouse(branch.House).
		WithCustomFullName(branch.CustomFullName).
		WithCustomFullAddress(branch.CustomFullAddress).
		WithSharing(branch.Scopes.Sharing...).
		Do(ctx)

	return err
}

func loadBranchFields() (branchFields, error) {
	var fields branchFields
	inputs := []struct {
		name   string
		target *string
	}{
		{"DIIA_BRANCH_NAME", &fields.name},
		{"DIIA_BRANCH_EMAIL", &fields.email},
		{"DIIA_BRANCH_REGION", &fields.region},
		{"DIIA_BRANCH_DISTRICT", &fields.district},
		{"DIIA_BRANCH_LOCATION", &fields.location},
		{"DIIA_BRANCH_STREET", &fields.street},
		{"DIIA_BRANCH_HOUSE", &fields.house},
	}

	for _, input := range inputs {
		value, err := exampleutil.RequiredEnv(input.name)
		if err != nil {
			return branchFields{}, err
		}

		*input.target = value
	}

	return fields, nil
}
