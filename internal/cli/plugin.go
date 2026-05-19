package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mguilarducci/liszt/internal/claudehome"
	"github.com/mguilarducci/liszt/internal/claudestate"
	"github.com/mguilarducci/liszt/internal/gitx"
	"github.com/mguilarducci/liszt/internal/marketplace"
	"github.com/mguilarducci/liszt/internal/render"
	"github.com/mguilarducci/liszt/internal/repos"
)

// PluginList handles `liszt plugin list`.
func PluginList(p Paths) error {
	cfg, err := repos.Load(p.Repos)
	if err != nil {
		return err
	}
	if len(cfg.Repos) == 0 {
		render.Hint("no repos. add one with: liszt repo add <url>")
		return nil
	}
	for i, r := range cfg.Repos {
		owner, repo, err := gitx.ParseGitHubURL(r.URL)
		if err != nil {
			render.Warn("skip", "name", r.Name, "err", err)
			continue
		}
		mp, _, err := marketplace.Read(gitx.RepoPath(p.Cache, owner, repo))
		if err != nil {
			render.Warn("skip", "name", r.Name, "err", err)
			continue
		}
		if i > 0 {
			fmt.Println()
		}
		render.Header(fmt.Sprintf("%s (%d plugins)", r.Name, len(mp.Plugins)))
		for _, plug := range mp.Plugins {
			label := plug.Name
			if plug.Version != "" {
				label = fmt.Sprintf("%s (v%s)", plug.Name, plug.Version)
			}
			fmt.Printf("- %s\n", label)
			if plug.Description != "" {
				render.Hint("  " + plug.Description)
			}
		}
	}
	return nil
}

// PluginInstall handles `liszt plugin install <slug> --flavor <flavor>`.
func PluginInstall(p Paths, slug, flavor string) error {
	bar := newInstallBar(slug)
	prev := gitx.SetOutput(io.Discard)
	defer gitx.SetOutput(prev)

	bar.StageResolve(slug)
	cfg, err := repos.Load(p.Repos)
	if err != nil {
		bar.Fail("repos load failed", "path", p.Repos, "err", err)
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
			m := match{
				kind:       "plugin",
				flavor:     flavor,
				slug:       plug.Name,
				pluginName: plug.Name,
				repoName:   r.Name,
				sha:        r.SHA,
				path:       mp.ResolvePluginPath(plug),
			}
			if flavor == "claude" {
				if err := claudeInstallWithBar(p, r, owner, repo, root, mp, plug, bar); err != nil {
					bar.Fail("claude install failed", "slug", slug, "err", err)
					return err
				}
			} else {
				bar.StageCloneEnd()
				bar.StageMaterialize(slug)
			}
			bar.StageManifest()
			if err := recordInstall(p, m, slug); err != nil {
				bar.Fail("manifest write failed", "slug", slug, "err", err)
				return err
			}
			bar.Done(slug, flavor)
			return nil
		}
	}
	err = fmt.Errorf("plugin %q not found", slug)
	bar.Fail("plugin not found", "slug", slug)
	return err
}

// claudeInstallWithBar materializes plug into ~/.claude/plugins/ and enables
// it. Drives the install bar through clone (when source is external) and
// materialize stages.
func claudeInstallWithBar(p Paths, r repos.Entry, owner, repoName, mpClone string, mp *marketplace.Marketplace, plug marketplace.Plugin, bar *installBar) error {
	src, err := marketplace.ParseSource(plug.Source)
	if err != nil {
		return err
	}

	var srcDir, srcSha string
	switch {
	case src.External != nil:
		extOwner, extRepo, err := gitx.ParseGitHubURL(src.External.URL)
		if err != nil {
			return fmt.Errorf("parse git-subdir url %q: %w", src.External.URL, err)
		}
		bar.StageCloneBegin(plug.Name)
		extClone := gitx.RepoPath(p.Cache, extOwner, extRepo)
		if err := gitx.CloneAtSHA(src.External.URL, src.External.SHA, extClone); err != nil {
			return err
		}
		bar.StageCloneEnd()
		srcDir = filepath.Join(extClone, src.External.Path)
		srcSha = src.External.SHA
	default:
		bar.StageCloneEnd()
		srcDir = filepath.Join(mpClone, src.Subdir)
		srcSha = r.SHA
	}

	bar.StageMaterialize(plug.Name)

	version := marketplace.PluginManifestVersion(srcDir)
	if version == "" {
		version = plug.Version
	}
	if version == "" {
		version = "unknown"
	}

	home := claudehome.Dir()
	installedPath := filepath.Join(home, "plugins", "installed_plugins.json")
	knownPath := filepath.Join(home, "plugins", "known_marketplaces.json")
	settingsPath := filepath.Join(home, "settings.json")
	mpName := mp.Name
	key := plug.Name + "@" + mpName
	now := time.Now().UTC().Format(time.RFC3339)

	installed, err := claudestate.LoadInstalled(installedPath)
	if err != nil {
		return err
	}
	cur := installed.FindUserEntry(key)
	needCopy := cur == nil || cur.GitCommitSha != srcSha

	if needCopy {
		installPath, err := claudestate.MaterializePlugin(home, mpName, plug.Name, version, srcDir)
		if err != nil {
			return err
		}
		entry := claudestate.InstalledPlugin{
			InstallPath:  installPath,
			Version:      version,
			LastUpdated:  now,
			GitCommitSha: srcSha,
		}
		if cur == nil {
			entry.InstalledAt = now
		}
		installed.Upsert(key, entry)
		if err := claudestate.SaveInstalled(installedPath, installed); err != nil {
			return err
		}
	}

	known, err := claudestate.LoadKnown(knownPath)
	if err != nil {
		return err
	}
	mpInstallLoc := filepath.Join(home, "plugins", "marketplaces", mpName)
	src2 := claudestate.MarketplaceSource{Source: "github", Repo: owner + "/" + repoName}
	if err := known.UpsertMarketplace(mpName, src2, mpInstallLoc, now); err != nil {
		return err
	}
	if err := claudestate.SaveKnown(knownPath, known); err != nil {
		return err
	}
	if err := ensureMarketplaceSymlink(mpInstallLoc, mpClone); err != nil {
		return err
	}

	return claudestate.EnableSettingPlugin(settingsPath, key)
}

// ensureMarketplaceSymlink makes link point at target when nothing exists
// there yet. If link is already a symlink to a different path, returns an
// error. If link is an existing directory (e.g. Claude already cloned the
// marketplace itself), leaves it alone: Claude can keep updating its own
// clone, and liszt's plugin installPath does not depend on this link.
func ensureMarketplaceSymlink(link, target string) error {
	if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
		return err
	}
	if cur, err := os.Readlink(link); err == nil {
		if cur == target {
			return nil
		}
		return fmt.Errorf("symlink %s exists and points to %s, not %s; resolve manually", link, cur, target)
	}
	if _, err := os.Lstat(link); err == nil {
		// Pre-existing directory (likely Claude's own marketplace clone). Accept.
		return nil
	}
	return os.Symlink(target, link)
}
