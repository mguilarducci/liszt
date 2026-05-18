package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mguilarducci/liszt/internal/gitx"
	"github.com/mguilarducci/liszt/internal/marketplace"
	"github.com/mguilarducci/liszt/internal/repos"
	"github.com/mguilarducci/liszt/internal/resource"
)

// ResourceList handles `liszt <kind> list [--plugin <name>]`.
func ResourceList(p Paths, kind, pluginName string) error {
	def, ok := resource.Get(kind)
	if !ok {
		return fmt.Errorf("unknown kind %q", kind)
	}

	cfg, err := repos.Load(p.Repos)
	if err != nil {
		return err
	}

	matched := false
	first := true
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
			if pluginName != "" && plug.Name != pluginName {
				continue
			}
			pluginRoot := filepath.Join(root, mp.ResolvePluginPath(plug))
			items, err := def.List(pluginRoot)
			if err != nil || len(items) == 0 {
				if pluginName != "" && plug.Name == pluginName {
					matched = true
					printHeader(&first, r.Name, plug.Name, kind, len(items))
				}
				continue
			}
			matched = true
			printHeader(&first, r.Name, plug.Name, kind, len(items))
			for _, it := range items {
				if it.Extra != "" {
					fmt.Printf("- %s (%s)\n", it.Slug, it.Extra)
				} else {
					fmt.Printf("- %s\n", it.Slug)
				}
			}
		}
	}
	if pluginName != "" && !matched {
		fmt.Fprintf(os.Stderr, "plugin %q not found (add repo with: liszt repo add <url>)\n", pluginName)
		os.Exit(1)
	}
	return nil
}

// ResourceInstall handles `liszt <kind> install <slug> --flavor <flavor>`.
func ResourceInstall(p Paths, kind, slug, flavor string) error {
	matches, err := resolveSlug(p, kind, slug)
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		fmt.Fprintf(os.Stderr, "%s %q not found in cached repos\n", kind, slug)
		os.Exit(1)
	}
	m := matches[0]
	m.flavor = flavor
	if len(matches) > 1 {
		fmt.Fprintf(os.Stderr, "note: %d sources for %q, picking %s:%s (%s); qualify as <plugin>:%s to override\n",
			len(matches), slug, m.pluginName, m.slug, m.repoName, slug)
	}
	return recordInstall(p, m, slug)
}
