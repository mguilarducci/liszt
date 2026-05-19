package cli

import (
	"io"

	"github.com/mguilarducci/liszt/internal/gitx"
	"github.com/mguilarducci/liszt/internal/marketplace"
	"github.com/mguilarducci/liszt/internal/render"
	"github.com/mguilarducci/liszt/internal/repos"
)

// RepoAdd clones url into p.Cache and upserts the entry into p.Repos. Each
// stage emits its own info line so the user sees the operation as a
// sequence of persistent steps; the clone step (the only slow opaque op)
// is wrapped in an indeterminate bar that disappears once it finishes.
func RepoAdd(p Paths, url string) error {
	prev := gitx.SetOutput(io.Discard)
	defer gitx.SetOutput(prev)

	render.Info("resolving repo", "url", url)
	owner, repo, err := gitx.ParseGitHubURL(url)
	if err != nil {
		render.Error("parse url failed", "url", url, "err", err)
		return err
	}
	name := owner + "/" + repo
	dest := gitx.RepoPath(p.Cache, owner, repo)

	render.Info("cloning", "name", name, "dest", dest)
	bar := render.NewBar("cloning " + name)
	bar.SetIndeterminate(true)
	if err := gitx.EnsureClone(url, dest); err != nil {
		bar.Fail("clone failed", "url", url, "err", err)
		return err
	}
	bar.Stop()

	sha, err := gitx.HeadSHA(dest)
	if err != nil {
		render.Error("head-sha failed", "dest", dest, "err", err)
		return err
	}
	render.Info("cloned", "sha", sha[:12])

	render.Info("reading marketplace.json")
	mp, flavor, mpErr := marketplace.Read(dest)
	if mpErr != nil {
		render.Warn("marketplace.json", "err", mpErr)
	} else {
		render.Info("marketplace", "name", mp.Name, "flavor", flavor, "plugins", len(mp.Plugins))
	}

	render.Info("saving repos.toml", "path", p.Repos)
	cfg, err := repos.Load(p.Repos)
	if err != nil {
		render.Error("repos load failed", "path", p.Repos, "err", err)
		return err
	}
	cfg.Upsert(repos.Entry{Name: name, URL: url, SHA: sha})
	if err := repos.Save(p.Repos, cfg); err != nil {
		render.Error("repos save failed", "path", p.Repos, "err", err)
		return err
	}

	render.Done("repo added", "name", name)
	return nil
}
