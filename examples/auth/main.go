package main

import (
	"context"
	"fmt"
	"os"

	"github.com/stremovskyy/go-diia/examples/internal/exampleutil"
)

func main() {
	config, err := exampleutil.Load()
	if err != nil {
		fail(err)
	}

	if _, err := config.Client.Auth().SessionToken().Do(context.Background()); err != nil {
		fail(err)
	}

	fmt.Printf("Authentication succeeded (%s).\n", config.Environment)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, exampleutil.SafeError(err))
	os.Exit(1)
}
