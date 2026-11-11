package main

import (
	"context"
	"fmt"
	"os"

	"github.com/stremovskyy/go-diia/examples/internal/exampleutil"
)

func main() {
	if len(os.Args) != 2 || (os.Args[1] != "online" && os.Args[1] != "offline") {
		fmt.Fprintln(os.Stderr, "usage: go run ./examples/status online|offline")
		os.Exit(2)
	}

	config, err := exampleutil.Load()
	if err != nil {
		fail(err)
	}

	requestID, err := exampleutil.RequiredEnv("DIIA_REQUEST_ID")
	if err != nil {
		fail(err)
	}

	ctx := context.Background()

	if os.Args[1] == "online" {
		status, err := config.Client.Sharing().OnlineStatus().WithRequestID(requestID).Do(ctx)
		if err != nil {
			fail(err)
		}

		fmt.Println(status)

		return
	}

	status, err := config.Client.Sharing().OfflineStatus().WithRequestID(requestID).Do(ctx)
	if err != nil {
		fail(err)
	}

	fmt.Println(status)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, exampleutil.SafeError(err))
	os.Exit(1)
}
