package cli

import (
	"fmt"
	"os"

	"github.com/mguilarducci/liszt/internal/gitx"
	"github.com/mguilarducci/liszt/internal/lock"
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

	remoteHead := map[string]string{}
	urlFor := map[string]string{}
	for _, r := range reposCfg.Repos {
		urlFor[r.Name] = r.URL
	}

	type drift struct {
		entry  lock.Entry
		latest string
	}
	var drifts []drift
	upToDate := 0
	unknown := 0

	for _, e := range cfg.Locked {
		latest, ok := remoteHead[e.Repo]
		if !ok {
			url := urlFor[e.Repo]
			if url == "" {
				unknown++
				continue
			}
			sha, err := gitx.LsRemoteHead(url)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warn: ls-remote %s: %v\n", e.Repo, err)
				unknown++
				continue
			}
			latest = sha
			remoteHead[e.Repo] = latest
		}
		if latest == e.SHA {
			upToDate++
			continue
		}
		drifts = append(drifts, drift{entry: e, latest: latest})
	}

	if len(drifts) == 0 {
		fmt.Printf("up to date: %d locked entries\n", upToDate)
		return nil
	}
	fmt.Printf("outdated: %d (up to date: %d, unknown: %d)\n\n", len(drifts), upToDate, unknown)
	for _, d := range drifts {
		fmt.Printf("- %s %s [%s] (plugin: %s)\n", d.entry.Kind, d.entry.Slug, d.entry.Flavor, d.entry.Plugin)
		fmt.Printf("    repo:    %s\n", d.entry.Repo)
		fmt.Printf("    locked:  %s\n", d.entry.SHA[:12])
		fmt.Printf("    remote:  %s\n", d.latest[:12])
	}
	return nil
}
