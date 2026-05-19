package cli

import (
	"fmt"
	"io"

	"github.com/mguilarducci/liszt/internal/gitx"
	"github.com/mguilarducci/liszt/internal/lock"
	"github.com/mguilarducci/liszt/internal/render"
	"github.com/mguilarducci/liszt/internal/repos"
)

// Outdated handles `liszt outdated`.
func Outdated(p Paths) error {
	cfg, err := lock.Load(p.Lock)
	if err != nil {
		return err
	}
	reposCfg, err := repos.Load(p.Repos)
	if err != nil {
		return err
	}

	urlFor := map[string]string{}
	for _, r := range reposCfg.Repos {
		urlFor[r.Name] = r.URL
	}

	type drift struct {
		entry  lock.Entry
		latest string
	}

	bar := render.NewBar("checking remotes")
	prev := gitx.SetOutput(io.Discard)
	defer gitx.SetOutput(prev)

	remoteHead := map[string]string{}
	var drifts []drift
	upToDate := 0
	unknown := 0
	total := len(cfg.Locked)

	for i, e := range cfg.Locked {
		bar.Update(e.Repo)
		latest, ok := remoteHead[e.Repo]
		if !ok {
			url := urlFor[e.Repo]
			if url == "" {
				unknown++
				if total > 0 {
					bar.Set(float64(i+1) / float64(total))
				}
				continue
			}
			sha, err := gitx.LsRemoteHead(url)
			if err != nil {
				render.Warn("could not reach remote")
				render.Detail("ls-remote failed", "repo", e.Repo, "err", err)
				unknown++
				if total > 0 {
					bar.Set(float64(i+1) / float64(total))
				}
				continue
			}
			latest = sha
			remoteHead[e.Repo] = latest
		}
		if latest == e.SHA {
			upToDate++
		} else {
			drifts = append(drifts, drift{entry: e, latest: latest})
		}
		if total > 0 {
			bar.Set(float64(i+1) / float64(total))
		}
	}

	if len(drifts) == 0 {
		bar.Done("up to date", "entries", upToDate)
		return nil
	}
	bar.Done("outdated", "drifts", len(drifts), "up_to_date", upToDate, "unknown", unknown)
	for _, d := range drifts {
		render.Hint(fmt.Sprintf("- %s %s [%s]  %s..%s",
			d.entry.Kind, d.entry.Slug, d.entry.Flavor,
			d.entry.SHA[:12], d.latest[:12]))
		render.Detail("drift",
			"plugin", d.entry.Plugin,
			"repo", d.entry.Repo,
			"locked", d.entry.SHA[:12],
			"remote", d.latest[:12],
		)
	}
	return nil
}
