package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mguilarducci/liszt/internal/gitx"
	"github.com/mguilarducci/liszt/internal/lock"
	"github.com/mguilarducci/liszt/internal/manifest"
	"github.com/mguilarducci/liszt/internal/marketplace"
	"github.com/mguilarducci/liszt/internal/render"
	"github.com/mguilarducci/liszt/internal/repos"
	"github.com/mguilarducci/liszt/internal/resource"
)

// installBar drives the multi-step install progress bar. Callers invoke
// each Stage* method between the corresponding work step; the clone stage
// flips to indeterminate mode since clone duration is opaque.
type installBar struct {
	bar *render.Bar
}

func newInstallBar(label string) *installBar {
	return &installBar{bar: render.Default.Bar("installing " + label)}
}

func (b *installBar) StageResolve(slug string) {
	b.bar.Update("resolving " + slug)
	b.bar.Set(0.0)
}

func (b *installBar) StageCloneBegin(slug string) {
	b.bar.Update("cloning " + slug)
	b.bar.SetIndeterminate(true)
	b.bar.Set(0.25)
}

func (b *installBar) StageCloneEnd() {
	b.bar.SetIndeterminate(false)
	b.bar.Set(0.50)
}

func (b *installBar) StageMaterialize(slug string) {
	b.bar.Update("materializing " + slug)
	b.bar.Set(0.75)
}

func (b *installBar) StageManifest() {
	b.bar.Update("writing manifest")
	b.bar.Set(1.0)
}

func (b *installBar) Done(slug, flavor string) {
	b.bar.Done("installed", "slug", slug, "flavor", flavor)
}

func (b *installBar) Fail(msg string, kv ...any) {
	b.bar.Fail(msg, kv...)
}

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

	return nil
}
