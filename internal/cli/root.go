package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"

	"github.com/mguilarducci/liszt/internal/version"
)

var rootCmd = &cobra.Command{
	Use:           "liszt",
	Short:         "liszt — agent-agnostic plugin package manager",
	Version:       version.Full(),
	SilenceUsage:  true,
	SilenceErrors: true,
}

var noColor bool

func init() {
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable color output")
	rootCmd.PersistentPreRun = func(_ *cobra.Command, _ []string) {
		if noColor {
			// Setenv only fails on platforms without env support; failure
			// here means colors stay on, which is harmless.
			_ = os.Setenv("NO_COLOR", "1")
		}
	}
}

// Execute runs the root command through charmbracelet/fang. fang styles
// --help, --version, and error output. Callers should pass a
// context.Background() unless they need cancellation semantics.
func Execute(ctx context.Context) error {
	if err := fang.Execute(ctx, rootCmd); err != nil {
		return fmt.Errorf("execute: %w", err)
	}
	return nil
}
