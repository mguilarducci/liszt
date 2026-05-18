package cli

import (
	"fmt"
	"os"

	"github.com/mguilarducci/liszt/internal/gitx"
	"github.com/mguilarducci/liszt/internal/marketplace"
	"github.com/mguilarducci/liszt/internal/repos"
)

// Repo handles `liszt repo add <github-url>`.
func Repo(p Paths, args []string) error {
	if len(args) < 2 || args[0] != "add" {
		fmt.Fprintln(os.Stderr, "usage: liszt repo add <github-url>")
		os.Exit(2)
	}
	owner, repo, err := gitx.ParseGitHubURL(args[1])
	if err != nil {
		return err
	}

	dest := gitx.RepoPath(p.Cache, owner, repo)
	if err := gitx.EnsureClone(args[1], dest); err != nil {
		return err
	}

	sha, err := gitx.HeadSHA(dest)
	if err != nil {
		return err
	}
	if _, _, err := marketplace.Read(dest); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}

	cfg, err := repos.Load(p.Repos)
	if err != nil {
		return err
	}
	cfg.Upsert(repos.Entry{Name: owner + "/" + repo, URL: args[1], SHA: sha})
	if err := repos.Save(p.Repos, cfg); err != nil {
		return err
	}

	fmt.Printf("added %s/%s @ %s\n", owner, repo, sha[:12])
	return nil
}
