package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mguilarducci/liszt/internal/version"
)

var versionCmd = newVersionCmd()

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), version.Full())
			return nil
		},
	}
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
