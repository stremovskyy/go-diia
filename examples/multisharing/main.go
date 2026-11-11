// Command multisharing creates a sandbox branch and offer for four document
// types, then starts an online request. The test user chooses both vehicle
// registrations in Diia; repeating vehicle-license in scopes cannot force it.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	diia "github.com/stremovskyy/go-diia"
	"github.com/stremovskyy/go-diia/examples/internal/exampleutil"
)

var sharingScopes = []diia.DocumentType{
	diia.DocumentTypeInternalPassport,
	diia.DocumentTypeTaxpayerCard,
	diia.DocumentTypeDriverLicense,
	diia.DocumentTypeVehicleLicense,
}

var errMultisharingNotEnabled = errors.New(
	"acquirer does not support multisharing; ask Diia to enable it for this account",
)

func main() {
	if len(os.Args) != 2 || !validAction(os.Args[1]) {
		fmt.Fprintln(
			os.Stderr,
			"usage: go run ./examples/multisharing inspect-branch|create-branch|create-offer|request|status",
		)
		os.Exit(2)
	}

	if err := run(os.Args[1]); err != nil {
		if errors.Is(err, errMultisharingNotEnabled) {
			fmt.Fprintln(os.Stderr, errMultisharingNotEnabled)
		} else {
			fmt.Fprintln(os.Stderr, exampleutil.SafeError(err))
		}
		os.Exit(1)
	}
}

func validAction(action string) bool {
	switch action {
	case "inspect-branch", "create-branch", "create-offer", "request", "status":
		return true
	default:
		return false
	}
}

func run(action string) error {
	config, err := exampleutil.Load()
	if err != nil {
		return err
	}

	if action != "status" && action != "inspect-branch" {
		if err := config.AllowMutation(); err != nil {
			return err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	switch action {
	case "inspect-branch":
		branchID, err := exampleutil.RequiredEnv("DIIA_BRANCH_ID")
		if err != nil {
			return err
		}

		branch, err := config.Client.Branches().Get().WithBranchID(branchID).Do(ctx)
		if err != nil {
			return err
		}

		if branch == nil {
			return fmt.Errorf("%w: branch response is empty", diia.ErrDecode)
		}

		fmt.Printf(
			"branch_id=%s\nsharing_scopes=%v\nmultisharing_ready=%t\n",
			branch.ID,
			branch.Scopes.Sharing,
			allowsAll(branch.Scopes.Sharing, sharingScopes),
		)

		return nil

	case "create-branch":
		return createBranch(ctx, config.Client)
	case "create-offer":
		return createOffer(ctx, config.Client)
	case "request":
		return requestOnline(ctx, config.Client)

	case "status":
		requestID, err := exampleutil.RequiredEnv("DIIA_REQUEST_ID")
		if err != nil {
			return err
		}

		status, err := config.Client.Sharing().OnlineStatus().WithRequestID(requestID).Do(ctx)
		if err != nil {
			return err
		}

		fmt.Println(status)

		return nil
	}

	return errors.New("unsupported action")
}

func createBranch(ctx context.Context, client diia.Diia) error {
	fieldNames := []string{
		"DIIA_BRANCH_NAME", "DIIA_BRANCH_EMAIL", "DIIA_BRANCH_REGION",
		"DIIA_BRANCH_DISTRICT", "DIIA_BRANCH_LOCATION", "DIIA_BRANCH_STREET", "DIIA_BRANCH_HOUSE",
	}
	fields := make([]string, len(fieldNames))

	for index, name := range fieldNames {
		value, err := exampleutil.RequiredEnv(name)
		if err != nil {
			return err
		}

		fields[index] = value
	}

	branch, err := client.Branches().Create().
		WithName(fields[0]).WithEmail(fields[1]).
		WithRegion(fields[2]).WithDistrict(fields[3]).
		WithLocation(fields[4]).WithStreet(fields[5]).WithHouse(fields[6]).
		WithSharing(sharingScopes...).Do(ctx)
	if err != nil {
		return err
	}

	if branch == nil || branch.ID == "" {
		return fmt.Errorf("%w: branch response has no ID", diia.ErrDecode)
	}

	fmt.Printf("branch_id=%s\n", branch.ID)

	return nil
}

func createOffer(ctx context.Context, client diia.Diia) error {
	branchID, err := exampleutil.RequiredEnv("DIIA_BRANCH_ID")
	if err != nil {
		return err
	}

	name, err := exampleutil.RequiredEnv("DIIA_OFFER_NAME")
	if err != nil {
		return err
	}

	branch, err := client.Branches().Get().WithBranchID(branchID).Do(ctx)
	if err != nil {
		return err
	}

	if branch == nil || !allowsAll(branch.Scopes.Sharing, sharingScopes) {
		return errors.New("branch does not allow all four multisharing document types")
	}

	offer, err := client.Offers().Create().
		WithBranchID(branchID).WithName(name).
		WithSharing(sharingScopes...).WithMandatorySharing(sharingScopes...).
		WithReturnLink(os.Getenv("DIIA_RETURN_LINK")).Do(ctx)
	if err != nil {
		var apiErr *diia.APIError

		if errors.As(err, &apiErr) && apiErr.HTTPStatus == http.StatusForbidden &&
			strings.Contains(strings.ToLower(apiErr.Message), "doesn't support multi sharing") {
			return errMultisharingNotEnabled
		}

		return err
	}

	if offer == nil || offer.ID == "" {
		return fmt.Errorf("%w: offer response has no ID", diia.ErrDecode)
	}

	fmt.Printf("offer_id=%s\n", offer.ID)

	return nil
}

func requestOnline(ctx context.Context, client diia.Diia) error {
	branchID, err := exampleutil.RequiredEnv("DIIA_BRANCH_ID")
	if err != nil {
		return err
	}

	offerID, err := exampleutil.RequiredEnv("DIIA_OFFER_ID")
	if err != nil {
		return err
	}

	requestID, err := exampleutil.RequestID()
	if err != nil {
		return err
	}

	request := client.Sharing().RequestOnline().
		WithBranchID(branchID).WithOfferID(offerID).WithRequestID(requestID).
		WithSignAlgo("DSTU4145-DSTU7564")

	if returnLink := os.Getenv("DIIA_RETURN_LINK"); returnLink != "" {
		request = request.WithReturnLink(returnLink)
	}

	// A timeout is ambiguous; check status using this ID before retrying.
	fmt.Printf("request_id=%s\n", requestID)
	link, err := request.Do(ctx)
	if err != nil {
		return err
	}

	if link == nil {
		return fmt.Errorf("%w: online response is empty", diia.ErrDecode)
	}

	if _, err := link.OTP(); err != nil {
		return err
	}

	fmt.Println("multisharing_request=created")

	if os.Getenv("DIIA_SHOW_DEEPLINK") == "1" {
		fmt.Printf("deeplink=%s\n", link.Deeplink)
	}

	return nil
}

func allowsAll(allowed, requested []diia.DocumentType) bool {
	set := make(map[diia.DocumentType]bool, len(allowed))

	for _, documentType := range allowed {
		set[documentType] = true
	}

	for _, documentType := range requested {
		if !set[documentType] {
			return false
		}
	}

	return true
}
