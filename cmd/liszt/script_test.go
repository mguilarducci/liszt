package main_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/mguilarducci/liszt/internal/cli"
	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"liszt": func() {
			if err := cli.Execute(context.Background()); err != nil {
				_, _ = fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		},
	})
}

func TestScripts(t *testing.T) {
	t.Parallel()

	testscript.Run(t, testscript.Params{
		Dir: "testdata/script",
	})
}
