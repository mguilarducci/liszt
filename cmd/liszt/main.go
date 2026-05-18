package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mguilarducci/liszt/internal/gitx"
	"github.com/mguilarducci/liszt/internal/lock"
	"github.com/mguilarducci/liszt/internal/manifest"
	"github.com/mguilarducci/liszt/internal/marketplace"
	"github.com/mguilarducci/liszt/internal/repos"
	"github.com/mguilarducci/liszt/internal/resource"
)

const (
	reposFile    = "repos.toml"
	manifestFile = "liszt.toml"
	lockFile     = "liszt.lock"
	cacheDir     = "tmp"
)

// ---------- entry ----------

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "repo":
		repoCmd(args)
	case "plugin":
		pluginCmd(args)
	case "outdated":
		outdatedCmd(args)
	default:
		if _, ok := resource.Get(cmd); ok {
			resourceCmd(cmd, args)
			return
		}
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  liszt repo add <github-url>             clone marketplace repo
  liszt plugin list                       list plugins across all repos
  liszt plugin install <slug> --flavor <claude|copilot>
  liszt <kind> list [--plugin <name>]     list resources of a kind
  liszt <kind> install <slug> --flavor <claude|copilot>
                                          install: writes liszt.toml (manifest) + liszt.lock
                                          kinds: skill, agent, command, hook, mcp, lsp
  liszt outdated                          compare liszt.lock SHAs vs remote HEAD`)
}

// ---------- repo add ----------

func repoCmd(args []string) {
	if len(args) < 2 || args[0] != "add" {
		fmt.Fprintln(os.Stderr, "usage: liszt repo add <github-url>")
		os.Exit(2)
	}
	owner, repo, err := gitx.ParseGitHubURL(args[1])
	must(err)

	dest := gitx.RepoPath(cacheDir, owner, repo)
	must(gitx.EnsureClone(args[1], dest))

	sha, err := gitx.HeadSHA(dest)
	must(err)
	if _, _, err := marketplace.Read(dest); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}

	cfg, err := repos.Load(reposFile)
	must(err)
	cfg.Upsert(repos.Entry{Name: owner + "/" + repo, URL: args[1], SHA: sha})
	must(repos.Save(reposFile, cfg))

	fmt.Printf("added %s/%s @ %s\n", owner, repo, sha[:12])
}

// ---------- plugin list ----------

func pluginCmd(args []string) {
	if len(args) >= 2 && args[0] == "install" {
		slug, flavor := parseInstallArgs(args[1:])
		installPlugin(slug, flavor)
		return
	}
	if len(args) < 1 || args[0] != "list" {
		fmt.Fprintln(os.Stderr, "usage: liszt plugin {list | install <slug> --flavor <claude|copilot>}")
		os.Exit(2)
	}
	cfg, err := repos.Load(reposFile)
	must(err)
	if len(cfg.Repos) == 0 {
		fmt.Println("no repos. add one with: liszt repo add <url>")
		return
	}
	for i, r := range cfg.Repos {
		owner, repo, err := gitx.ParseGitHubURL(r.URL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skip %s: %v\n", r.Name, err)
			continue
		}
		mp, _, err := marketplace.Read(gitx.RepoPath(cacheDir, owner, repo))
		if err != nil {
			fmt.Fprintf(os.Stderr, "skip %s: %v\n", r.Name, err)
			continue
		}
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("== %s (%d plugins) ==\n", r.Name, len(mp.Plugins))
		for _, p := range mp.Plugins {
			fmt.Printf("- %s", p.Name)
			if p.Version != "" {
				fmt.Printf(" (v%s)", p.Version)
			}
			fmt.Println()
			if p.Description != "" {
				fmt.Printf("  %s\n", p.Description)
			}
		}
	}
}

// ---------- generic resource list ----------

func resourceCmd(kind string, args []string) {
	if len(args) >= 2 && args[0] == "install" {
		slug, flavor := parseInstallArgs(args[1:])
		installResource(kind, slug, flavor)
		return
	}
	if len(args) < 1 || args[0] != "list" {
		fmt.Fprintf(os.Stderr, "usage: liszt %s {list [--plugin <name>] | install <slug> --flavor <claude|copilot>}\n", kind)
		os.Exit(2)
	}
	var pluginName string
	for i := 1; i < len(args); i++ {
		if args[i] == "--plugin" && i+1 < len(args) {
			pluginName = args[i+1]
			i++
		}
	}

	def, _ := resource.Get(kind)
	cfg, err := repos.Load(reposFile)
	must(err)

	matched := false
	first := true
	for _, r := range cfg.Repos {
		owner, repo, err := gitx.ParseGitHubURL(r.URL)
		if err != nil {
			continue
		}
		root := gitx.RepoPath(cacheDir, owner, repo)
		mp, _, err := marketplace.Read(root)
		if err != nil {
			continue
		}
		for _, p := range mp.Plugins {
			if pluginName != "" && p.Name != pluginName {
				continue
			}
			pluginRoot := filepath.Join(root, mp.ResolvePluginPath(p))
			items, err := def.List(pluginRoot)
			if err != nil || len(items) == 0 {
				if pluginName != "" && p.Name == pluginName {
					matched = true
					printHeader(&first, r.Name, p.Name, kind, len(items))
				}
				continue
			}
			matched = true
			printHeader(&first, r.Name, p.Name, kind, len(items))
			for _, it := range items {
				if it.Extra != "" {
					fmt.Printf("- %s (%s)\n", it.Slug, it.Extra)
				} else {
					fmt.Printf("- %s\n", it.Slug)
				}
			}
		}
	}
	if pluginName != "" && !matched {
		fmt.Fprintf(os.Stderr, "plugin %q not found (add repo with: liszt repo add <url>)\n", pluginName)
		os.Exit(1)
	}
}

// ---------- outdated ----------

func outdatedCmd(_ []string) {
	cfg, err := lock.Load(lockFile)
	must(err)
	reposCfg, err := repos.Load(reposFile)
	must(err)

	// Cache remote HEAD per repo (one git ls-remote per repo).
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
		return
	}
	fmt.Printf("outdated: %d (up to date: %d, unknown: %d)\n\n", len(drifts), upToDate, unknown)
	for _, d := range drifts {
		fmt.Printf("- %s %s [%s] (plugin: %s)\n", d.entry.Kind, d.entry.Slug, d.entry.Flavor, d.entry.Plugin)
		fmt.Printf("    repo:    %s\n", d.entry.Repo)
		fmt.Printf("    locked:  %s\n", d.entry.SHA[:12])
		fmt.Printf("    remote:  %s\n", d.latest[:12])
	}
}

// ---------- install ----------

// parseInstallArgs reads "<slug> [--flavor X]" from positional args.
func parseInstallArgs(args []string) (slug, flavor string) {
	if len(args) > 0 {
		slug = args[0]
	}
	for i := 1; i < len(args); i++ {
		if args[i] == "--flavor" && i+1 < len(args) {
			flavor = args[i+1]
			i++
		}
	}
	if flavor != "claude" && flavor != "copilot" {
		fmt.Fprintln(os.Stderr, "error: --flavor required, must be 'claude' or 'copilot'")
		os.Exit(2)
	}
	return
}

// match holds a resolved artifact location.
type match struct {
	kind       string
	flavor     string
	slug       string
	pluginName string
	repoName   string
	sha        string
	path       string // path relative to repo root
}

// resolveSlug scans repos for an artifact of the given kind matching slug.
// slug may be qualified as "<plugin>:<artifact>" to disambiguate.
func resolveSlug(kind, raw string) ([]match, error) {
	var wantPlugin, wantSlug string
	if i := strings.Index(raw, ":"); i > 0 {
		wantPlugin = raw[:i]
		wantSlug = raw[i+1:]
	} else {
		wantSlug = raw
	}

	cfg, err := repos.Load(reposFile)
	if err != nil {
		return nil, err
	}
	def, ok := resource.Get(kind)
	if !ok {
		return nil, fmt.Errorf("unknown kind %q", kind)
	}

	var out []match
	for _, r := range cfg.Repos {
		owner, repo, err := gitx.ParseGitHubURL(r.URL)
		if err != nil {
			continue
		}
		root := gitx.RepoPath(cacheDir, owner, repo)
		mp, _, err := marketplace.Read(root)
		if err != nil {
			continue
		}
		for _, p := range mp.Plugins {
			if wantPlugin != "" && p.Name != wantPlugin {
				continue
			}
			rel := mp.ResolvePluginPath(p)
			pluginRoot := filepath.Join(root, rel)
			items, err := def.List(pluginRoot)
			if err != nil {
				continue
			}
			for _, it := range items {
				if it.Slug != wantSlug {
					continue
				}
				out = append(out, match{
					kind:       kind,
					slug:       wantSlug,
					pluginName: p.Name,
					repoName:   r.Name,
					sha:        r.SHA,
					path:       filepath.ToSlash(filepath.Join(rel, it.Path)),
				})
			}
		}
	}
	return out, nil
}

func installResource(kind, slug, flavor string) {
	matches, err := resolveSlug(kind, slug)
	must(err)
	if len(matches) == 0 {
		fmt.Fprintf(os.Stderr, "%s %q not found in cached repos\n", kind, slug)
		os.Exit(1)
	}
	m := matches[0]
	m.flavor = flavor
	if len(matches) > 1 {
		fmt.Fprintf(os.Stderr, "note: %d sources for %q, picking %s:%s (%s); qualify as <plugin>:%s to override\n",
			len(matches), slug, m.pluginName, m.slug, m.repoName, slug)
	}
	recordInstall(m, slug)
}

func installPlugin(slug, flavor string) {
	cfg, err := repos.Load(reposFile)
	must(err)
	for _, r := range cfg.Repos {
		owner, repo, err := gitx.ParseGitHubURL(r.URL)
		if err != nil {
			continue
		}
		root := gitx.RepoPath(cacheDir, owner, repo)
		mp, _, err := marketplace.Read(root)
		if err != nil {
			continue
		}
		for _, p := range mp.Plugins {
			if p.Name != slug {
				continue
			}
			recordInstall(match{
				kind:       "plugin",
				flavor:     flavor,
				slug:       p.Name,
				pluginName: p.Name,
				repoName:   r.Name,
				sha:        r.SHA,
				path:       mp.ResolvePluginPath(p),
			}, slug)
			return
		}
	}
	fmt.Fprintf(os.Stderr, "plugin %q not found\n", slug)
	os.Exit(1)
}

func recordInstall(m match, requestedSlug string) {
	// 1. Manifest (declarative): preserve user-typed slug (possibly qualified).
	man, err := manifest.Load(manifestFile)
	must(err)
	man.Upsert(manifest.Entry{Kind: m.kind, Slug: requestedSlug, Flavor: m.flavor})
	must(manifest.Save(manifestFile, man))

	// 2. Lock (resolved): records exact source + SHA at install time.
	lockCfg, err := lock.Load(lockFile)
	must(err)
	lockCfg.Upsert(lock.Entry{
		Kind: m.kind, Flavor: m.flavor, Slug: m.slug, Plugin: m.pluginName,
		Repo: m.repoName, SHA: m.sha, Path: m.path,
	})
	must(lock.Save(lockFile, lockCfg))

	fmt.Printf("installed %s %s [%s] (from %s @ %s)\n", m.kind, m.slug, m.flavor, m.repoName, m.sha[:12])
}

// ---------- shared helpers ----------

func printHeader(first *bool, repoName, pluginName, kind string, n int) {
	if !*first {
		fmt.Println()
	}
	*first = false
	fmt.Printf("== %s :: %s (%d %ss) ==\n", repoName, pluginName, n, kind)
}

func must(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
