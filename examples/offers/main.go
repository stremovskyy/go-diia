// Command offers demonstrates the Diia offer operations. It selects the
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

var errMissingOfferID = errors.New("offer response did not contain an ID")

func main() {
	if len(os.Args) != 2 || !validAction(os.Args[1]) {
		fmt.Fprintln(os.Stderr, "usage: go run ./examples/offers create|list|delete")
		os.Exit(2)
	}

	if err := run(os.Args[1]); err != nil {
		if errors.Is(err, errMissingOfferID) {
			fmt.Fprintln(os.Stderr, errMissingOfferID)
		} else {
			fmt.Fprintln(os.Stderr, exampleutil.SafeError(err))
		}
		os.Exit(1)
	}
}

func validAction(action string) bool {
	switch action {
	case "create", "list", "delete":
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

	if action == "create" || action == "delete" {
		if err := cfg.AllowMutation(); err != nil {
			return err
		}
	}

	branchID, err := exampleutil.RequiredEnv("DIIA_BRANCH_ID")
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	switch action {
	case "create":
		name, err := exampleutil.RequiredEnv("DIIA_OFFER_NAME")
		if err != nil {
			return err
		}

		offer, err := cfg.Client.Offers().Create().
			WithBranchID(branchID).
			WithName(name).
			WithSharing(diia.DocumentTypeInternalPassport).
			WithReturnLink(os.Getenv("DIIA_RETURN_LINK")).
			Do(ctx)
		if err != nil {
			return err
		}

		if offer == nil || offer.ID == "" {
			return errMissingOfferID
		}

		fmt.Printf("offer_id=%s\n", offer.ID)

	case "list":
		offers, err := cfg.Client.Offers().List().WithBranchID(branchID).WithSkip(0).WithLimit(100).Do(ctx)
		if err != nil {
			return err
		}

		fmt.Printf("total=%d\n", offers.Total)

		for _, offer := range offers.Offers {
			fmt.Printf("offer_id=%s name=%q\n", offer.ID, offer.Name)
		}

	case "delete":
		offerID, err := exampleutil.RequiredEnv("DIIA_OFFER_ID")
		if err != nil {
			return err
		}

		if err := cfg.Client.Offers().Delete().WithBranchID(branchID).WithOfferID(offerID).Do(ctx); err != nil {
			return err
		}

		fmt.Printf("offer_deleted=%s\n", offerID)
	}

	return nil
}
