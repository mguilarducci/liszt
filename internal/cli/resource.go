package cli

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/mguilarducci/liszt/internal/gitx"
	"github.com/mguilarducci/liszt/internal/marketplace"
	"github.com/mguilarducci/liszt/internal/render"
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
		return fmt.Errorf("plugin %q not found (add repo with: liszt repo add <url>)", pluginName)
	}
	return nil
}

// ResourceInstall handles `liszt <kind> install <slug> --flavor <flavor>`.
func ResourceInstall(p Paths, kind, slug, flavor string) error {
	bar := newInstallBar(fmt.Sprintf("%s/%s", kind, slug))
	prev := gitx.SetOutput(io.Discard)
	defer gitx.SetOutput(prev)

	bar.StageResolve(slug)
	matches, err := resolveSlug(p, kind, slug)
	if err != nil {
		bar.Fail("resolve failed", "slug", slug, "err", err)
		return err
	}
	if len(matches) == 0 {
		err := fmt.Errorf("%s %q not found in cached repos", kind, slug)
		bar.Fail("not found", "kind", kind, "slug", slug)
		return err
	}
	bar.StageCloneEnd()

	m := matches[0]
	m.flavor = flavor
	if len(matches) > 1 {
		render.Warn(
			fmt.Sprintf("%d sources for %q; picking %s:%s", len(matches), slug, m.pluginName, m.slug),
			"repo", m.repoName,
		)
	}

	bar.StageMaterialize(slug)
	bar.StageManifest()
	if err := recordInstall(p, m, slug); err != nil {
		bar.Fail("manifest write failed", "slug", slug, "err", err)
		return err
	}
	bar.Done(slug, flavor)
	return nil
}
