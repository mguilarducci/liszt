package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mguilarducci/liszt/internal/gitx"
	"github.com/mguilarducci/liszt/internal/lock"
	"github.com/mguilarducci/liszt/internal/manifest"
	"github.com/mguilarducci/liszt/internal/marketplace"
	"github.com/mguilarducci/liszt/internal/repos"
	"github.com/mguilarducci/liszt/internal/resource"
)

// match holds a resolved artifact location.
type match struct {
	kind       string
	flavor     string
	slug       string
	pluginName string
	repoName   string
	sha        string
	path       string
}

// ParseInstallArgs reads "<slug> [--flavor X]" from positional args.
// Exits with code 2 on malformed input (preserves current behavior).
func ParseInstallArgs(args []string) (slug, flavor string) {
	if len(args) > 0 {
		slug = args[0]
	}
	for i := 1; i < len(args); i++ {
		if args[i] == "--flavor" && i+1 < len(args) {
			flavor = args[i+1]
			i++
		}
	}
	if flavor != "claude" && flavor != "copilot" {
		fmt.Fprintln(os.Stderr, "error: --flavor required, must be 'claude' or 'copilot'")
		os.Exit(2)
	}
	return
}

// resolveSlug scans repos for an artifact of the given kind matching slug.
// slug may be qualified as "<plugin>:<artifact>" to disambiguate.
func resolveSlug(p Paths, kind, raw string) ([]match, error) {
	var wantPlugin, wantSlug string
	if i := strings.Index(raw, ":"); i > 0 {
		wantPlugin = raw[:i]
		wantSlug = raw[i+1:]
	} else {
		wantSlug = raw
	}

	cfg, err := repos.Load(p.Repos)
	if err != nil {
		return nil, err
	}
	def, ok := resource.Get(kind)
	if !ok {
		return nil, fmt.Errorf("unknown kind %q", kind)
	}

	var out []match
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
			if wantPlugin != "" && plug.Name != wantPlugin {
				continue
			}
			rel := mp.ResolvePluginPath(plug)
			pluginRoot := filepath.Join(root, rel)
			items, err := def.List(pluginRoot)
			if err != nil {
				continue
			}
			for _, it := range items {
				if it.Slug != wantSlug {
					continue
				}
				out = append(out, match{
					kind:       kind,
					slug:       wantSlug,
					pluginName: plug.Name,
					repoName:   r.Name,
					sha:        r.SHA,
					path:       filepath.ToSlash(filepath.Join(rel, it.Path)),
				})
			}
		}
	}
	return out, nil
}

func recordInstall(p Paths, m match, requestedSlug string) error {
	man, err := manifest.Load(p.Manifest)
	if err != nil {
		return err
	}
	man.Upsert(manifest.Entry{Kind: m.kind, Slug: requestedSlug, Flavor: m.flavor})
	if err := manifest.Save(p.Manifest, man); err != nil {
		return err
	}

	lk, err := lock.Load(p.Lock)
	if err != nil {
		return err
	}
	lk.Upsert(lock.Entry{
		Kind: m.kind, Flavor: m.flavor, Slug: m.slug, Plugin: m.pluginName,
		Repo: m.repoName, SHA: m.sha, Path: m.path,
	})
	if err := lock.Save(p.Lock, lk); err != nil {
		return err
	}

	fmt.Printf("installed %s %s [%s] (from %s @ %s)\n", m.kind, m.slug, m.flavor, m.repoName, m.sha[:12])
	return nil
}
