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
	bar := render.NewBar("cloning " + url)
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
	mp, flavor, mpErr := marketplace.Read(dest)
	if mpErr != nil {
		render.Warn(mpErr.Error())
	}

	cfg, err := repos.Load(p.Repos)
	if err != nil {
		bar.Fail("repos load failed", "path", p.Repos, "err", err)
		return err
	}
	existing := false
	for _, e := range cfg.Repos {
		if e.Name == owner+"/"+repo {
			existing = true
			break
		}
	}
	cfg.Upsert(repos.Entry{Name: owner + "/" + repo, URL: url, SHA: sha})
	if err := repos.Save(p.Repos, cfg); err != nil {
		bar.Fail("repos save failed", "path", p.Repos, "err", err)
		return err
	}

	verb := "repo added"
	if existing {
		verb = "repo updated"
	}
	kv := []any{
		"name", owner + "/" + repo,
		"url", url,
		"sha", sha[:12],
		"cache", dest,
	}
	if mp != nil {
		kv = append(kv,
			"marketplace", mp.Name,
			"flavor", flavor,
			"plugins", len(mp.Plugins),
		)
	}
	bar.Done(verb, kv...)
	return nil
}
