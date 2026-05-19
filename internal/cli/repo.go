package cli

import (
	"github.com/mguilarducci/liszt/internal/gitx"
	"github.com/mguilarducci/liszt/internal/marketplace"
	"github.com/mguilarducci/liszt/internal/render"
	"github.com/mguilarducci/liszt/internal/repos"
)

// RepoAdd clones url into p.Cache and upserts the entry into p.Repos.
func RepoAdd(p Paths, url string) error {
	owner, repo, err := gitx.ParseGitHubURL(url)
	if err != nil {
		return err
	}

	dest := gitx.RepoPath(p.Cache, owner, repo)
	if err := gitx.EnsureClone(url, dest); err != nil {
		return err
	}

	sha, err := gitx.HeadSHA(dest)
	if err != nil {
		return err
	}
	if _, _, err := marketplace.Read(dest); err != nil {
		render.Warn(err.Error())
	}

	cfg, err := repos.Load(p.Repos)
	if err != nil {
		return err
	}
	cfg.Upsert(repos.Entry{Name: owner + "/" + repo, URL: url, SHA: sha})
	if err := repos.Save(p.Repos, cfg); err != nil {
		return err
	}

	render.Done("repo added", "name", owner+"/"+repo, "sha", sha[:12])
	return nil
}
