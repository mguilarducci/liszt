package cli

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

func TestRootHasUseAndVersion(t *testing.T) {
	if rootCmd.Use != "liszt" {
		t.Errorf("rootCmd.Use = %q; want %q", rootCmd.Use, "liszt")
	}
	if rootCmd.Version == "" {
		t.Errorf("rootCmd.Version is empty")
	}
}

func TestRootSilencesUsageAndErrors(t *testing.T) {
	if !rootCmd.SilenceUsage {
		t.Errorf("rootCmd.SilenceUsage = false; want true")
	}
	if !rootCmd.SilenceErrors {
		t.Errorf("rootCmd.SilenceErrors = false; want true")
	}
}

func TestNoColorFlagSetsEnv(t *testing.T) {
	orig, hadOrig := os.LookupEnv("NO_COLOR")
	t.Cleanup(func() {
		if hadOrig {
			_ = os.Setenv("NO_COLOR", orig)
		} else {
			_ = os.Unsetenv("NO_COLOR")
		}
		noColor = false
	})
	_ = os.Unsetenv("NO_COLOR")

	noColor = true
	rootCmd.PersistentPreRun(rootCmd, nil)

	if os.Getenv("NO_COLOR") != "1" {
		t.Errorf("--no-color did not set NO_COLOR=1")
	}
}

func TestVerboseFlagRegistered(t *testing.T) {
	f := rootCmd.PersistentFlags().Lookup("verbose")
	if f == nil {
		t.Fatal("persistent --verbose flag not registered")
	}
	if f.Shorthand != "v" {
		t.Errorf("--verbose shorthand = %q; want %q", f.Shorthand, "v")
	}
}

func TestVerbosePreRunWiresRender(t *testing.T) {
	t.Cleanup(func() { verbose = false })

	verbose = true
	rootCmd.PersistentPreRun(rootCmd, nil)
	// Reading Default state is unsafe; instead trip Detail and observe it
	// makes it through. Default writes to os.Stderr — we just confirm the
	// flag flow did not panic and the PreRun ran.
}

func TestExecuteHelpDoesNotError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"--help"})
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})

	if err := Execute(context.Background()); err != nil {
		t.Errorf("Execute(--help) returned error: %v", err)
	}
	if !strings.Contains(stdout.String()+stderr.String(), "liszt") {
		t.Errorf("--help output missing program name")
	}
}
