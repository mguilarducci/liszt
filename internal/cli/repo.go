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
// Five steps drive a single Progress bar: resolve URL, preflight registry,
// clone, read manifest, persist. Technical payload (URL, dest, SHA, path)
// rides on Detail and is gated by --verbose.
func RepoAdd(p Paths, url string) error {
	prev := gitx.SetOutput(io.Discard)
	defer gitx.SetOutput(prev)

	render.Detail("url=" + url)

	progress := render.NewProgress(5)

	progress.Step("Resolving " + url)
	owner, repo, err := gitx.ParseGitHubURL(url)
	if err != nil {
		progress.StepFail(err)
		return err
	}
	name := owner + "/" + repo
	dest := gitx.RepoPath(p.Cache, owner, repo)
	render.Detail("resolved", "name", name, "dest", dest)
	// Replace the in-flight step label with the resolved name so the ✓
	// line committed by the next Step reads `✓ Resolved <name>`.
	progress.SetLabel("Resolved " + name)

	progress.Step("Checking registry")
	cfg, err := repos.Load(p.Repos)
	if err != nil {
		progress.StepFail(err)
		render.Detail("repos load failed", "path", p.Repos, "err", err)
		return err
	}
	if _, ok := cfg.Find(name); ok {
		progress.StepFail(ErrAlreadyAdded)
		render.Fail(name + " already added")
		render.Hint("→ Run `liszt repo update " + name + "` to refresh")
		return ErrAlreadyAdded
	}
	progress.SetLabel("Not yet registered")

	progress.Step("Cloning " + name)
	if err := gitx.EnsureClone(url, dest); err != nil {
		progress.StepFail(err)
		render.Detail("clone failed", "url", url, "err", err)
		return err
	}
	sha, err := gitx.HeadSHA(dest)
	if err != nil {
		progress.StepFail(err)
		render.Detail("head-sha failed", "dest", dest, "err", err)
		return err
	}
	render.Detail("cloned", "sha", sha[:12])
	progress.SetLabel("Cloned " + name)

	progress.Step("Reading marketplace.json")
	mp, flavor, mpErr := marketplace.Read(dest)
	if mpErr != nil {
		render.Warn("marketplace.json missing or invalid")
		render.Detail("marketplace.json", "err", mpErr)
	} else {
		render.Detail("marketplace", "name", mp.Name, "flavor", flavor, "plugins", len(mp.Plugins))
		progress.SetLabel("Read marketplace.json")
	}

	progress.Step("Saving to repos.toml")
	cfg.Repos = append(cfg.Repos, repos.Entry{Name: name, URL: url, SHA: sha})
	if err := repos.Save(p.Repos, cfg); err != nil {
		progress.StepFail(err)
		render.Detail("repos save failed", "path", p.Repos, "err", err)
		return err
	}
	render.Detail("repos.toml", "path", p.Repos)
	progress.SetLabel("Saved to repos.toml")

	if mpErr != nil {
		progress.Done("Added " + name)
	} else {
		progress.Done("Added "+name,
			"marketplace", mp.Name,
			"plugins", len(mp.Plugins))
	}
	render.Hint("→ Run `liszt plugin list` to see available plugins")
	render.Hint("→ Run `liszt plugin install <name>` to install")
	return nil
}
