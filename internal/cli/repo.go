package cli

import (
	"errors"
	"io"

	"github.com/mguilarducci/liszt/internal/gitx"
	"github.com/mguilarducci/liszt/internal/marketplace"
	"github.com/mguilarducci/liszt/internal/render"
	"github.com/mguilarducci/liszt/internal/repos"
)

// ErrAlreadyAdded is returned by RepoAdd when the registry already contains
// an entry for the resolved repo name. Callers (CLI / tests) match on this
// sentinel via errors.Is.
var ErrAlreadyAdded = errors.New("repo already added")

// RepoAdd clones url into p.Cache and appends the entry to p.Repos.
// Preflight: if the registry already lists the resolved name, RepoAdd
// returns ErrAlreadyAdded without cloning. Refresh is the future
// `liszt repo update` command's responsibility.
func RepoAdd(p Paths, url string) error {
	prev := gitx.SetOutput(io.Discard)
	defer gitx.SetOutput(prev)

	render.Info("resolving repo", "url", url)
	owner, repo, err := gitx.ParseGitHubURL(url)
	if err != nil {
		render.Fail("parse url failed", "url", url, "err", err)
		return err
	}
	name := owner + "/" + repo
	dest := gitx.RepoPath(p.Cache, owner, repo)

	cfg, err := repos.Load(p.Repos)
	if err != nil {
		render.Fail("repos load failed", "path", p.Repos, "err", err)
		return err
	}
	if _, ok := cfg.Find(name); ok {
		render.Fail(name+" already added",
			"hint", "Run `liszt repo update "+name+"` to refresh")
		return ErrAlreadyAdded
	}

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
		render.Fail("head-sha failed", "dest", dest, "err", err)
		return err
	}
	render.Info("cloned", "sha", sha[:12])

	render.Info("reading marketplace.json")
	mp, flavor, mpErr := marketplace.Read(dest)
	if mpErr != nil {
		render.Warn("marketplace.json failed to read")
	} else {
		render.Info("marketplace", "name", mp.Name, "flavor", flavor, "plugins", len(mp.Plugins))
	}

	render.Info("saving repos.toml", "path", p.Repos)
	cfg.Repos = append(cfg.Repos, repos.Entry{Name: name, URL: url, SHA: sha})
	if err := repos.Save(p.Repos, cfg); err != nil {
		render.Fail("repos save failed", "path", p.Repos, "err", err)
		return err
	}

	render.Done("repo added", "name", name)
	return nil
}
