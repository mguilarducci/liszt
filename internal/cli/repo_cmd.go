package cli

import (
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mguilarducci/liszt/internal/xdg"
)

var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Manage marketplace repositories",
}

var repoAddCmd = &cobra.Command{
	Use:   "add <url>",
	Short: "Clone a marketplace repository",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return RepoAdd(defaultPaths(), args[0])
	},
}

func init() {
	repoCmd.AddCommand(repoAddCmd)
	rootCmd.AddCommand(repoCmd)
}

// defaultPaths returns the production filesystem layout. Tests inject custom
// Paths through the underlying helpers; this wrapper exists for the cobra
// hot path.
func defaultPaths() Paths {
	return Paths{
		Repos:    filepath.Join(xdg.DataDir(), "repos.toml"),
		Manifest: "liszt.toml",
		Lock:     "liszt.lock",
		Cache:    filepath.Join(xdg.CacheDir(), "repos"),
	}
}
