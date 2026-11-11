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

	barcode, err := exampleutil.RequiredEnv("DIIA_BARCODE")
	if err != nil {
		return err
	}

	requestID, err := exampleutil.RequestID()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Keep this ID even if the POST times out: the server may have accepted it.
	fmt.Printf("request_id=%s\n", requestID)

	accepted, err := config.Client.Sharing().RequestOffline().
		WithBranchID(branchID).
		WithBarcode(barcode).
		WithRequestID(requestID).
		Do(ctx)
	if err != nil {
		return err
	}

	if accepted == nil || !accepted.Success {
		return fmt.Errorf("%w: offline request was not accepted", diia.ErrDecode)
	}

	fmt.Println("offline_request=accepted")

	return nil
}
