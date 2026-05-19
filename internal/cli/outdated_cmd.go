package cli

import (
	"github.com/spf13/cobra"
)

var outdatedCmd = &cobra.Command{
	Use:   "outdated",
	Short: "Compare local lock SHAs against remote HEAD",
	RunE: func(_ *cobra.Command, _ []string) error {
		return Outdated(defaultPaths())
	},
}

func init() {
	rootCmd.AddCommand(outdatedCmd)
}
