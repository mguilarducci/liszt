package cli

import (
	"fmt"
	"os"

	"github.com/mguilarducci/liszt/internal/gitx"
	"github.com/mguilarducci/liszt/internal/marketplace"
	"github.com/mguilarducci/liszt/internal/repos"
)

// PluginList handles `liszt plugin list`.
func PluginList(p Paths) error {
	cfg, err := repos.Load(p.Repos)
	if err != nil {
		return err
	}
	if len(cfg.Repos) == 0 {
		fmt.Println("no repos. add one with: liszt repo add <url>")
		return nil
	}
	for i, r := range cfg.Repos {
		owner, repo, err := gitx.ParseGitHubURL(r.URL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skip %s: %v\n", r.Name, err)
			continue
		}
		mp, _, err := marketplace.Read(gitx.RepoPath(p.Cache, owner, repo))
		if err != nil {
			fmt.Fprintf(os.Stderr, "skip %s: %v\n", r.Name, err)
			continue
		}
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("== %s (%d plugins) ==\n", r.Name, len(mp.Plugins))
		for _, plug := range mp.Plugins {
			fmt.Printf("- %s", plug.Name)
			if plug.Version != "" {
				fmt.Printf(" (v%s)", plug.Version)
			}
			fmt.Println()
			if plug.Description != "" {
				fmt.Printf("  %s\n", plug.Description)
			}
		}
	}
	return nil
}

// PluginInstall handles `liszt plugin install <slug> --flavor <flavor>`.
func PluginInstall(p Paths, slug, flavor string) error {
	cfg, err := repos.Load(p.Repos)
	if err != nil {
		return err
	}
	for _, r := range cfg.Repos {
		owner, repo, err := gitx.ParseGitHubURL(r.URL)
		if err != nil {
			continue
		}
		root := gitx.RepoPath(p.Cache, owner, repo)
		mp, _, err := marketplace.Read(root)
		if err != nil {
			continue
		}
		for _, plug := range mp.Plugins {
			if plug.Name != slug {
				continue
			}
			return recordInstall(p, match{
				kind:       "plugin",
				flavor:     flavor,
				slug:       plug.Name,
				pluginName: plug.Name,
				repoName:   r.Name,
				sha:        r.SHA,
				path:       mp.ResolvePluginPath(plug),
			}, slug)
		}
	}
	fmt.Fprintf(os.Stderr, "plugin %q not found\n", slug)
	os.Exit(1)
	return nil
}
