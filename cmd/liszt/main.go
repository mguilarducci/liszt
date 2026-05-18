package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mguilarducci/liszt/internal/gitx"
	"github.com/mguilarducci/liszt/internal/marketplace"
	"github.com/pelletier/go-toml/v2"
)

const (
	reposFile    = "repos.toml"
	manifestFile = "liszt.toml"
	lockFile     = "liszt.lock"
	cacheDir     = "tmp"
)

// ---------- types ----------

type repoEntry struct {
	Name string `toml:"name"`
	URL  string `toml:"url"`
	SHA  string `toml:"sha"`
}

type reposConfig struct {
	Repos []repoEntry `toml:"repos"`
}

// manifestEntry = declarative want (committed to liszt.toml).
type manifestEntry struct {
	Kind   string `toml:"kind"`
	Slug   string `toml:"slug"`   // may be qualified "<plugin>:<slug>"
	Flavor string `toml:"flavor"` // claude | copilot
}

type manifestConfig struct {
	Items []manifestEntry `toml:"items"`
}

// lockEntry = resolved state at install time (committed to liszt.lock).
type lockEntry struct {
	Kind   string `toml:"kind"`
	Flavor string `toml:"flavor"`
	Slug   string `toml:"slug"`
	Plugin string `toml:"plugin"`
	Repo   string `toml:"repo"`
	SHA    string `toml:"sha"`
	Path   string `toml:"path"`
}

type lockConfig struct {
	Locked []lockEntry `toml:"locked"`
}

// item is a discovered resource: slug (display id) + path (relative to plugin root, "" if config-only).
type item struct {
	Slug  string
	Path  string
	Extra string // optional display detail (e.g. hook event count)
}

// kindDef describes how to discover items of a given resource kind inside a plugin.
type kindDef struct {
	list func(pluginRoot string) ([]item, error)
}

var kinds = map[string]kindDef{
	"skill":   {list: listSkills},
	"agent":   {list: listAgents},
	"command": {list: listCommands},
	"hook":    {list: listHooks},
	"mcp":     {list: listMCP},
	"lsp":     {list: listLSP},
}

// skills: <plugin>/skills/<name>/SKILL.md (recursive). Artifact = the skill dir.
func listSkills(root string) ([]item, error) {
	return walkItems(root, "skills",
		func(d fs.DirEntry) bool { return !d.IsDir() && strings.EqualFold(d.Name(), "SKILL.md") },
		func(rel string) string { return filepath.ToSlash(filepath.Dir(rel)) },
		filepath.Dir, // path = "skills/<name>"
	)
}

// agents: <plugin>/agents/<name>.md (Claude) or <name>.agent.md (Copilot). Artifact = the file.
func listAgents(root string) ([]item, error) {
	return walkItems(root, "agents", isMarkdownLeaf, agentSlug, identityPath)
}

func agentSlug(rel string) string {
	rel = filepath.ToSlash(rel)
	rel = strings.TrimSuffix(rel, ".agent.md")
	rel = strings.TrimSuffix(rel, ".md")
	return rel
}

// commands: <plugin>/commands/<name>.md (flat). Artifact = the file.
func listCommands(root string) ([]item, error) {
	return walkItems(root, "commands", isMarkdownLeaf, trimExt, identityPath)
}

func identityPath(p string) string { return p }

// hooks: hooks/hooks.json (Claude) or hooks.json (Copilot)
func listHooks(root string) ([]item, error) {
	candidates := []string{"hooks/hooks.json", "hooks.json"}
	data, src, ok, err := readFirstWithSource(root, candidates...)
	if err != nil || !ok {
		return nil, err
	}
	var doc struct {
		Hooks map[string][]any `json:"hooks"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse hooks.json: %w", err)
	}
	var out []item
	for event, entries := range doc.Hooks {
		out = append(out, item{
			Slug:  event,
			Path:  src + "#" + event,
			Extra: fmt.Sprintf("%d", len(entries)),
		})
	}
	return out, nil
}

// mcp: .claude-plugin/mcp.json (Claude) | .mcp.json (Copilot root) | .github/mcp.json (Copilot alt)
func listMCP(root string) ([]item, error) {
	data, src, ok, err := readFirstWithSource(root, ".claude-plugin/mcp.json", ".mcp.json", ".github/mcp.json")
	if err != nil || !ok {
		return nil, err
	}
	var doc struct {
		MCPServers map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse mcp.json: %w", err)
	}
	var out []item
	for name := range doc.MCPServers {
		out = append(out, item{Slug: name, Path: src + "#" + name})
	}
	return out, nil
}

// lsp: lsp.json (Copilot root) | .github/lsp.json
func listLSP(root string) ([]item, error) {
	data, src, ok, err := readFirstWithSource(root, "lsp.json", ".github/lsp.json")
	if err != nil || !ok {
		return nil, err
	}
	var doc struct {
		Servers map[string]any `json:"servers"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse lsp.json: %w", err)
	}
	var out []item
	for name := range doc.Servers {
		out = append(out, item{Slug: name, Path: src + "#" + name})
	}
	return out, nil
}


func isMarkdownLeaf(d fs.DirEntry) bool {
	return !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".md")
}

func trimExt(rel string) string {
	rel = filepath.ToSlash(rel)
	return strings.TrimSuffix(rel, filepath.Ext(rel))
}

// walkItems walks <pluginRoot>/<subdir> and returns items.
// match selects leaf files; slugOf maps the file's rel path to a slug.
// item.Path is relative to plugin root (subdir + rel).
// walkItems walks <pluginRoot>/<subdir>. For each leaf matching `match`:
// - slugOf(rel)         → item.Slug   (rel = path under subdir)
// - pathOf(pathInPlugin) → item.Path  (pathInPlugin = subdir-prefixed path within plugin root)
func walkItems(pluginRoot, subdir string, match func(fs.DirEntry) bool, slugOf func(rel string) string, pathOf func(pathInPlugin string) string) ([]item, error) {
	base := filepath.Join(pluginRoot, subdir)
	if _, err := os.Stat(base); os.IsNotExist(err) {
		return nil, nil
	}
	var out []item
	err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if match(d) {
			rel, _ := filepath.Rel(base, path)
			pathInPlugin, _ := filepath.Rel(pluginRoot, path)
			out = append(out, item{
				Slug: slugOf(rel),
				Path: filepath.ToSlash(pathOf(pathInPlugin)),
			})
		}
		return nil
	})
	return out, err
}

// readFirstWithSource extends readFirst with the matched path (relative to root).
func readFirstWithSource(root string, paths ...string) ([]byte, string, bool, error) {
	for _, p := range paths {
		data, err := os.ReadFile(filepath.Join(root, p))
		if err == nil {
			return data, p, true, nil
		}
		if !os.IsNotExist(err) {
			return nil, "", false, err
		}
	}
	return nil, "", false, nil
}

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
		if _, ok := kinds[cmd]; ok {
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

	cfg, err := loadRepos(reposFile)
	must(err)
	cfg.upsert(repoEntry{Name: owner + "/" + repo, URL: args[1], SHA: sha})
	must(saveRepos(reposFile, cfg))

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
	cfg, err := loadRepos(reposFile)
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

	def := kinds[kind]
	cfg, err := loadRepos(reposFile)
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
			items, err := def.list(pluginRoot)
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
	cfg, err := loadLock(lockFile)
	must(err)
	repos, err := loadRepos(reposFile)
	must(err)

	// Cache remote HEAD per repo (one git ls-remote per repo).
	remoteHead := map[string]string{}
	urlFor := map[string]string{}
	for _, r := range repos.Repos {
		urlFor[r.Name] = r.URL
	}

	type drift struct {
		entry  lockEntry
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

	cfg, err := loadRepos(reposFile)
	if err != nil {
		return nil, err
	}
	def, ok := kinds[kind]
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
			items, err := def.list(pluginRoot)
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
	cfg, err := loadRepos(reposFile)
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
	man, err := loadManifest(manifestFile)
	must(err)
	man.upsert(manifestEntry{Kind: m.kind, Slug: requestedSlug, Flavor: m.flavor})
	must(saveManifest(manifestFile, man))

	// 2. Lock (resolved): records exact source + SHA at install time.
	lock, err := loadLock(lockFile)
	must(err)
	lock.upsert(lockEntry{
		Kind: m.kind, Flavor: m.flavor, Slug: m.slug, Plugin: m.pluginName,
		Repo: m.repoName, SHA: m.sha, Path: m.path,
	})
	must(saveLock(lockFile, lock))

	fmt.Printf("installed %s %s [%s] (from %s @ %s)\n", m.kind, m.slug, m.flavor, m.repoName, m.sha[:12])
}

func loadManifest(path string) (*manifestConfig, error) {
	cfg := &manifestConfig{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
}

func saveManifest(path string, cfg *manifestConfig) error {
	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (c *manifestConfig) upsert(e manifestEntry) {
	for i, x := range c.Items {
		if x.Kind == e.Kind && x.Slug == e.Slug && x.Flavor == e.Flavor {
			c.Items[i] = e
			return
		}
	}
	c.Items = append(c.Items, e)
}

func loadLock(path string) (*lockConfig, error) {
	cfg := &lockConfig{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
}

func saveLock(path string, cfg *lockConfig) error {
	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (c *lockConfig) upsert(e lockEntry) {
	for i, x := range c.Locked {
		if x.Kind == e.Kind && x.Slug == e.Slug && x.Plugin == e.Plugin && x.Flavor == e.Flavor {
			c.Locked[i] = e
			return
		}
	}
	c.Locked = append(c.Locked, e)
}

// ---------- shared helpers ----------

func printHeader(first *bool, repoName, pluginName, kind string, n int) {
	if !*first {
		fmt.Println()
	}
	*first = false
	fmt.Printf("== %s :: %s (%d %ss) ==\n", repoName, pluginName, n, kind)
}

// ---------- repos.toml ----------

func loadRepos(path string) (*reposConfig, error) {
	cfg := &reposConfig{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
}

func saveRepos(path string, cfg *reposConfig) error {
	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (c *reposConfig) upsert(e repoEntry) {
	for i, r := range c.Repos {
		if r.Name == e.Name {
			c.Repos[i] = e
			return
		}
	}
	c.Repos = append(c.Repos, e)
}

func must(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
