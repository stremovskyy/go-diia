package main

import (
	"context"
	"fmt"
	"os"
	"time"

	diia "github.com/stremovskyy/go-diia"
	"github.com/stremovskyy/go-diia/examples/internal/exampleutil"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, exampleutil.SafeError(err))
		os.Exit(1)
	}
}

func run() error {
	config, err := exampleutil.Load()
	if err != nil {
		return err
	}

	if err := config.AllowMutation(); err != nil {
		return err
	}

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

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	request := config.Client.Sharing().RequestOnline().
		WithBranchID(branchID).
		WithOfferID(offerID).
		WithRequestID(requestID)

	if returnLink := os.Getenv("DIIA_RETURN_LINK"); returnLink != "" {
		request = request.WithReturnLink(returnLink)
	}

	// Keep this ID even if the POST times out: the server may have accepted it.
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

	fmt.Println("online_request=created")

	if os.Getenv("DIIA_SHOW_DEEPLINK") == "1" {
		fmt.Printf("deeplink=%s\n", link.Deeplink)
	}

	return nil
}
