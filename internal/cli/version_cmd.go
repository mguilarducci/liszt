package cli

import (
	"github.com/spf13/cobra"

	"github.com/mguilarducci/liszt/internal/render"
	"github.com/mguilarducci/liszt/internal/version"
)

var versionCmd = NewVersionCmdWithRenderer(nil)

// NewVersionCmdWithRenderer constructs the `liszt version` subcommand using
// the given Renderer. Pass nil to delegate to render.Default; tests inject
// a Renderer wrapping a buffer.
func NewVersionCmdWithRenderer(r *render.Renderer) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		RunE: func(_ *cobra.Command, _ []string) error {
			if r == nil {
				render.Info(version.Full())
			} else {
				r.Info(version.Full())
			}
			return nil
		},
	}
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
