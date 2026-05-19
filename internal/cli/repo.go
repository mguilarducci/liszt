package cli

import (
	"io"

	"github.com/mguilarducci/liszt/internal/gitx"
	"github.com/mguilarducci/liszt/internal/marketplace"
	"github.com/mguilarducci/liszt/internal/render"
	"github.com/mguilarducci/liszt/internal/repos"
)

// RepoAdd clones url into p.Cache and upserts the entry into p.Repos.
func RepoAdd(p Paths, url string) error {
	bar := render.Default.Bar("cloning " + url)
	bar.SetIndeterminate(true)
	prev := gitx.SetOutput(io.Discard)
	defer gitx.SetOutput(prev)

	owner, repo, err := gitx.ParseGitHubURL(url)
	if err != nil {
		bar.Fail("parse url failed", "url", url, "err", err)
		return err
	}

	dest := gitx.RepoPath(p.Cache, owner, repo)
	if err := gitx.EnsureClone(url, dest); err != nil {
		bar.Fail("clone failed", "url", url, "err", err)
		return err
	}

	sha, err := gitx.HeadSHA(dest)
	if err != nil {
		bar.Fail("head-sha failed", "dest", dest, "err", err)
		return err
	}
	if _, _, err := marketplace.Read(dest); err != nil {
		render.Warn(err.Error())
	}

	cfg, err := repos.Load(p.Repos)
	if err != nil {
		bar.Fail("repos load failed", "path", p.Repos, "err", err)
		return err
	}
	cfg.Upsert(repos.Entry{Name: owner + "/" + repo, URL: url, SHA: sha})
	if err := repos.Save(p.Repos, cfg); err != nil {
		bar.Fail("repos save failed", "path", p.Repos, "err", err)
		return err
	}

	bar.Done("repo added", "name", owner+"/"+repo, "sha", sha[:12])
	return nil
}
