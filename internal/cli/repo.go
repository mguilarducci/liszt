package cli

import (
	"fmt"
	"io"

	"github.com/mguilarducci/liszt/internal/gitx"
	"github.com/mguilarducci/liszt/internal/marketplace"
	"github.com/mguilarducci/liszt/internal/render"
	"github.com/mguilarducci/liszt/internal/repos"
)

// RepoAdd clones url into p.Cache and upserts the entry into p.Repos. Drives
// the progress bar through the resolve/clone/inspect/save stages and prints
// a multi-line summary after the bar closes.
func RepoAdd(p Paths, url string) error {
	bar := render.NewBar("resolving " + url)
	bar.SetIndeterminate(true)
	prev := gitx.SetOutput(io.Discard)
	defer gitx.SetOutput(prev)

	owner, repo, err := gitx.ParseGitHubURL(url)
	if err != nil {
		bar.Fail("parse url failed", "url", url, "err", err)
		return err
	}
	name := owner + "/" + repo

	bar.Update("cloning " + name)
	dest := gitx.RepoPath(p.Cache, owner, repo)
	if err := gitx.EnsureClone(url, dest); err != nil {
		bar.Fail("clone failed", "url", url, "err", err)
		return err
	}

	bar.Update("reading head sha")
	sha, err := gitx.HeadSHA(dest)
	if err != nil {
		bar.Fail("head-sha failed", "dest", dest, "err", err)
		return err
	}

	bar.Update("reading marketplace.json")
	mp, flavor, mpErr := marketplace.Read(dest)

	bar.Update("saving repos.toml")
	cfg, err := repos.Load(p.Repos)
	if err != nil {
		bar.Fail("repos load failed", "path", p.Repos, "err", err)
		return err
	}
	cfg.Upsert(repos.Entry{Name: name, URL: url, SHA: sha})
	if err := repos.Save(p.Repos, cfg); err != nil {
		bar.Fail("repos save failed", "path", p.Repos, "err", err)
		return err
	}

	bar.Done("repo added", "name", name)
	render.Subheader(name)
	render.Hint(fmt.Sprintf("  url         %s", url))
	render.Hint(fmt.Sprintf("  cache       %s", dest))
	render.Hint(fmt.Sprintf("  sha         %s", sha[:12]))
	if mpErr != nil {
		render.Warn("marketplace.json", "err", mpErr)
		return nil
	}
	render.Hint(fmt.Sprintf("  marketplace %s (%s)", mp.Name, flavor))
	render.Hint(fmt.Sprintf("  plugins     %d", len(mp.Plugins)))
	return nil
}
