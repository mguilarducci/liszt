package main

import (
	"context"
	"os"

	"github.com/mguilarducci/liszt/internal/cli"
)

// coverage: binary entry point; exercised only via the testscript harness.
func main() {
	if err := cli.Execute(context.Background()); err != nil {
		os.Exit(1)
	}
}
