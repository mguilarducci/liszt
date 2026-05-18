# Modularize liszt Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor single-file `main.go` (~864 LOC) into `cmd/liszt` + `internal/*` packages without changing any observable behavior.

**Architecture:** Bottom-up extraction. Capture baseline first, relocate `main.go` to `cmd/liszt/`, then peel out leaf packages (`gitx`, `marketplace`, `repos`, `manifest`, `lock`, `resource`) one at a time, then `cli` handlers, leaving `cmd/liszt/main.go` as a thin dispatcher. Each task ends with `go build ./...` + `go vet ./...` green and a commit. Final task runs full parity diff against baseline.

**Tech Stack:** Go 1.26, `github.com/pelletier/go-toml/v2`. No new dependencies. No new tests in this refactor (verification = output-diff against pre-refactor binary).

**Reference:** Design spec at `docs/superpowers/specs/2026-05-18-modularize-liszt-design.md`.

---

## Conventions used across all tasks

- All commands run from `/Users/mguilarducci/Projects/mguilarducci/liszt` unless noted.
- After moving/extracting code, **do not** add features, rename identifiers semantically, change TOML struct tags, or alter error/warning strings.
- Exported identifiers in `internal/*` use `CamelCase`. Unexported helpers stay unexported.
- Commits use Conventional Commits: `refactor(<scope>): ...`.
- After every extraction: `go build ./...` and `go vet ./...` must be clean.
- Each task ends with one commit.

---

### Task 1: Capture pre-refactor baseline

**Files:**
- Create: `scripts/capture-baseline.sh` (committed)
- Create: `/tmp/liszt-baseline/` (workspace, not committed)
- Create: `/tmp/liszt-baseline/old-*` (output captures, not committed)

**Purpose:** Lock down current binary's observable behavior before any code moves. This is the oracle for the final parity check in Task 10. Commands are wrapped in a script so the executing agent runs one bash invocation at a time (project rule: no compound bash in tool calls).

- [ ] **Step 1: Create `scripts/capture-baseline.sh`**

```bash
#!/usr/bin/env bash
set -euo pipefail

OLD_BIN=${OLD_BIN:-/tmp/liszt-old}
BASE=${BASE:-/tmp/liszt-baseline}
SRC_DIR=${SRC_DIR:-$(pwd)}

rm -rf "$BASE"
mkdir -p "$BASE"
cp "$SRC_DIR/repos.toml" "$BASE/repos.toml"
cp -R "$SRC_DIR/tmp" "$BASE/tmp"

cd "$BASE"
rm -f liszt.toml liszt.lock

run() {
  local label=$1
  shift
  "$OLD_BIN" "$@" > "old-$label.out" 2> "old-$label.err" || true
  echo "exit=$?" > "old-$label.code"
}

run plugin-list plugin list
run skill-list skill list
run agent-list agent list
run command-list command list
run hook-list hook list
run mcp-list mcp list
run lsp-list lsp list

run install-skill   skill   install brainstorming   --flavor claude
run install-agent   agent   install code-reviewer   --flavor claude
run install-plugin  plugin  install hookify         --flavor claude
run install-command command install commit          --flavor copilot
run install-hook    hook    install SessionStart    --flavor copilot
cp liszt.toml old-liszt.toml
cp liszt.lock old-liszt.lock

run outdated      outdated
run usage-empty
run usage-bogus   bogus
run usage-skill   skill

echo "baseline captured at $BASE"
```

- [ ] **Step 2: Make the script executable**

Run: `chmod +x scripts/capture-baseline.sh`
Expected: no output.

- [ ] **Step 3: Build the current binary**

Run: `go build -o /tmp/liszt-old .`
Expected: `/tmp/liszt-old` exists, no compile errors.

- [ ] **Step 4: Run the baseline script**

Run: `bash scripts/capture-baseline.sh`
Expected: prints `baseline captured at /tmp/liszt-baseline`. The directory contains `old-*.out`, `old-*.err`, `old-*.code` for every captured command, plus `old-liszt.toml` and `old-liszt.lock`.

- [ ] **Step 5: Spot-check the captures**

Run: `cat /tmp/liszt-baseline/old-plugin-list.code`
Expected: `exit=0`.

Run: `cat /tmp/liszt-baseline/old-usage-empty.code`
Expected: `exit=2`.

Run: `cat /tmp/liszt-baseline/old-liszt.lock`
Expected: TOML content with `[[locked]]` sections.

- [ ] **Step 6: Commit only the script**

Run: `git add scripts/capture-baseline.sh`
Run: `git commit -m "chore: add baseline capture script for refactor parity check"`
Expected: one commit, working tree clean otherwise.

---

### Task 2: Relocate `main.go` to `cmd/liszt/main.go`

**Files:**
- Create: `cmd/liszt/main.go` (verbatim copy of root `main.go`)
- Delete: `main.go`

**Purpose:** Adopt the `cmd/<binary>` layout without changing any code. Single move, single verification.

- [ ] **Step 1: Move the file**

```bash
mkdir -p cmd/liszt
git mv main.go cmd/liszt/main.go
```

Expected: `cmd/liszt/main.go` exists, `main.go` does not.

- [ ] **Step 2: Confirm the package still builds**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 3: Confirm `go vet`**

```bash
go vet ./...
```

Expected: no output.

- [ ] **Step 4: Confirm new binary still works**

```bash
go build -o /tmp/liszt-smoke ./cmd/liszt
/tmp/liszt-smoke
```

Expected: prints usage to stderr, exits 2.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor: relocate main.go to cmd/liszt"
```

---

### Task 3: Extract `internal/gitx`

**Files:**
- Create: `internal/gitx/git.go`
- Create: `internal/gitx/url.go`
- Modify: `cmd/liszt/main.go` (replace inline funcs with `gitx.*` calls; drop `cacheDir` use inside `repoPath` body)

**Purpose:** Isolate all git-plumbing + URL parsing + path helpers behind an importable API.

- [ ] **Step 1: Create `internal/gitx/git.go`**

```go
package gitx

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// EnsureClone clones url into dest if .git is absent. No-op if already cloned.
func EnsureClone(url, dest string) error {
	if _, err := os.Stat(filepath.Join(dest, ".git")); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	cmd := exec.Command("git", "clone", "--depth=1", url, dest)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}
	return nil
}

// HeadSHA returns the local repo's HEAD commit SHA.
func HeadSHA(dir string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// LsRemoteHead returns the remote's HEAD SHA without cloning.
func LsRemoteHead(url string) (string, error) {
	out, err := exec.Command("git", "ls-remote", url, "HEAD").Output()
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(string(out))
	if i := strings.IndexAny(line, " \t"); i > 0 {
		return line[:i], nil
	}
	return "", fmt.Errorf("unexpected ls-remote output: %q", line)
}
```

- [ ] **Step 2: Create `internal/gitx/url.go`**

```go
package gitx

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

// ParseGitHubURL extracts owner and repo from a github.com URL.
func ParseGitHubURL(raw string) (string, string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", "", fmt.Errorf("invalid url: %w", err)
	}
	if u.Host != "github.com" {
		return "", "", fmt.Errorf("only github.com URLs supported, got %q", u.Host)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("URL must include owner/repo")
	}
	return parts[0], strings.TrimSuffix(parts[1], ".git"), nil
}

// RepoPath returns cacheDir/owner/repo.
func RepoPath(cacheDir, owner, repo string) string {
	return filepath.Join(cacheDir, owner, repo)
}
```

- [ ] **Step 3: Update `cmd/liszt/main.go` imports**

In the import block, add:

```go
"github.com/mguilarducci/liszt/internal/gitx"
```

Remove `"net/url"` (now only used inside `gitx`). Keep `"os/exec"` only if other call sites still need it — after this task, the remaining `exec.Command` usage is gone, so remove it too.

- [ ] **Step 4: Replace inline definitions with `gitx` calls in `cmd/liszt/main.go`**

Delete the following from `cmd/liszt/main.go`:
- `func gitLsRemoteHead(url string) (string, error)` (was around line 481)
- `func repoPath(owner, repo string) string` (was around line 729)
- `func ensureClone(url, dest string) error` (was around line 733)
- `func readHeadSHA(dir string) (string, error)` (was around line 749)
- `func parseGitHubURL(raw string) (string, string, error)` (was around line 808)

Then update call sites in the same file:
- `parseGitHubURL(...)` → `gitx.ParseGitHubURL(...)` (all 5 occurrences: `repoCmd`, `pluginCmd`, `resourceCmd`, `outdatedCmd` URL map loop, `resolveSlug`)
- `repoPath(owner, repo)` → `gitx.RepoPath(cacheDir, owner, repo)` (all 4 occurrences)
- `ensureClone(...)` → `gitx.EnsureClone(...)` (1 occurrence in `repoCmd`)
- `readHeadSHA(dest)` → `gitx.HeadSHA(dest)` (1 occurrence in `repoCmd`)
- `gitLsRemoteHead(url)` → `gitx.LsRemoteHead(url)` (1 occurrence in `outdatedCmd`)

- [ ] **Step 5: Build and vet**

```bash
go build ./...
go vet ./...
```

Expected: both clean.

- [ ] **Step 6: Smoke-test `repo add` path is still callable**

```bash
go build -o /tmp/liszt-smoke ./cmd/liszt
/tmp/liszt-smoke
```

Expected: usage to stderr, exit 2.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "refactor(gitx): extract git ops and URL parsing"
```

---

### Task 4: Extract `internal/marketplace`

**Files:**
- Create: `internal/marketplace/marketplace.go`
- Modify: `cmd/liszt/main.go`

**Purpose:** Isolate marketplace.json parsing + plugin path resolution. The `Marketplace`, `Plugin` types and `Read` / `ResolvePluginPath` become this package's public API.

- [ ] **Step 1: Create `internal/marketplace/marketplace.go`**

```go
package marketplace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Marketplace mirrors marketplace.json.
type Marketplace struct {
	Name     string   `json:"name"`
	Plugins  []Plugin `json:"plugins"`
	Metadata struct {
		PluginRoot string `json:"pluginRoot"`
	} `json:"metadata"`
}

// Plugin is one entry in Marketplace.Plugins.
type Plugin struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version,omitempty"`
	Source      any    `json:"source,omitempty"`
}

// Read reads marketplace.json from one of the canonical locations and reports
// which flavor ("claude" | "copilot") the location implies.
func Read(repoRoot string) (*Marketplace, string, error) {
	data, src, ok, err := readFirstWithSource(repoRoot,
		".claude-plugin/marketplace.json", // Claude Code canonical
		".github/plugin/marketplace.json", // Copilot CLI canonical
	)
	if err != nil {
		return nil, "", fmt.Errorf("read marketplace.json: %w", err)
	}
	if !ok {
		return nil, "", fmt.Errorf("marketplace.json not found (tried .claude-plugin/, .github/plugin/)")
	}
	var mp Marketplace
	if err := json.Unmarshal(data, &mp); err != nil {
		return nil, "", fmt.Errorf("parse marketplace.json: %w", err)
	}
	flavor := "claude"
	if strings.HasPrefix(src, ".github/plugin/") {
		flavor = "copilot"
	}
	return &mp, flavor, nil
}

// ResolvePluginPath combines the marketplace's PluginRoot with a plugin's source path.
func (m *Marketplace) ResolvePluginPath(p Plugin) string {
	src := pluginSourcePath(p.Source)
	base := strings.TrimPrefix(m.Metadata.PluginRoot, "./")
	switch {
	case base == "" || base == ".":
		return src
	case src == "":
		return base
	default:
		return filepath.Join(base, src)
	}
}

func pluginSourcePath(src any) string {
	switch v := src.(type) {
	case string:
		p := strings.TrimPrefix(v, "./")
		if p == "" || p == "." {
			return ""
		}
		return p
	case map[string]any:
		if p, ok := v["path"].(string); ok {
			return strings.TrimPrefix(p, "./")
		}
	}
	return ""
}

// readFirstWithSource returns the first existing file under root and the relative path that matched.
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
```

- [ ] **Step 2: Update `cmd/liszt/main.go`**

Add import:

```go
"github.com/mguilarducci/liszt/internal/marketplace"
```

Delete the following from `cmd/liszt/main.go`:
- `type plugin struct { ... }` (around line 25)
- `type marketplace struct { ... }` (around line 32)
- `func readMarketplace(repoRoot string) (*marketplace, string, error)` (around line 757)
- `func resolvePluginPath(mp *marketplace, p plugin) string` (around line 779)
- `func pluginSourcePath(src any) string` (around line 792)

Update all call sites:
- `readMarketplace(root)` → `marketplace.Read(root)` (4 occurrences: `repoCmd`, `pluginCmd`, `resourceCmd`, `resolveSlug`)
- `resolvePluginPath(mp, p)` → `mp.ResolvePluginPath(p)` (2 occurrences: `resourceCmd`, `resolveSlug`)
- The local type names `*marketplace` and `plugin` in `resolvePluginPath`'s old signature are now `*marketplace.Marketplace` and `marketplace.Plugin`. Loop variables `for _, p := range mp.Plugins` keep working since `Plugins` is exported.

**Important:** The file still keeps `readFirstWithSource` (used by `listHooks`, `listMCP`, `listLSP`). Do NOT delete it from `main.go` in this task — it moves with the `resource` package in Task 8.

- [ ] **Step 3: Build and vet**

```bash
go build ./...
go vet ./...
```

Expected: clean.

- [ ] **Step 4: Smoke test**

```bash
go build -o /tmp/liszt-smoke ./cmd/liszt
/tmp/liszt-smoke plugin list 2>&1 | head -5
```

Expected: header lines like `== anthropics/claude-code (N plugins) ==` followed by plugin lines.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor(marketplace): extract marketplace.json parsing"
```

---

### Task 5: Extract `internal/repos`

**Files:**
- Create: `internal/repos/repos.go`
- Modify: `cmd/liszt/main.go`

**Purpose:** Isolate repos.toml load/save/upsert. First of three uniform TOML packages.

- [ ] **Step 1: Create `internal/repos/repos.go`**

```go
package repos

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

// Entry is one row in repos.toml.
type Entry struct {
	Name string `toml:"name"`
	URL  string `toml:"url"`
	SHA  string `toml:"sha"`
}

// Config models repos.toml.
type Config struct {
	Repos []Entry `toml:"repos"`
}

// Load reads path. Missing file returns an empty Config.
func Load(path string) (*Config, error) {
	cfg := &Config{}
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

// Save writes cfg to path.
func Save(path string, cfg *Config) error {
	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Upsert replaces an entry with the same Name or appends a new one.
func (c *Config) Upsert(e Entry) {
	for i, r := range c.Repos {
		if r.Name == e.Name {
			c.Repos[i] = e
			return
		}
	}
	c.Repos = append(c.Repos, e)
}
```

- [ ] **Step 2: Update `cmd/liszt/main.go`**

Add import:

```go
"github.com/mguilarducci/liszt/internal/repos"
```

Delete from `cmd/liszt/main.go`:
- `type repoEntry struct { ... }` (around line 40)
- `type reposConfig struct { ... }` (around line 46)
- `func loadRepos(path string) (*reposConfig, error)` (around line 825)
- `func saveRepos(path string, cfg *reposConfig) error` (around line 840)
- `func (c *reposConfig) upsert(e repoEntry)` (around line 848)

Update call sites:
- `loadRepos(reposFile)` → `repos.Load(reposFile)` (5 occurrences)
- `saveRepos(reposFile, cfg)` → `repos.Save(reposFile, cfg)` (1 occurrence)
- `cfg.upsert(repoEntry{...})` → `cfg.Upsert(repos.Entry{...})` (1 occurrence in `repoCmd`)
- Any local variable typed `*reposConfig` becomes `*repos.Config`
- Field access `r.Name`, `r.URL`, `r.SHA` is unchanged (fields stay exported in the new struct).

- [ ] **Step 3: Build and vet**

```bash
go build ./...
go vet ./...
```

Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "refactor(repos): extract repos.toml handling"
```

---

### Task 6: Extract `internal/manifest`

**Files:**
- Create: `internal/manifest/manifest.go`
- Modify: `cmd/liszt/main.go`

**Purpose:** Same shape as Task 5, for liszt.toml.

- [ ] **Step 1: Create `internal/manifest/manifest.go`**

```go
package manifest

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

// Entry is one declarative want.
type Entry struct {
	Kind   string `toml:"kind"`
	Slug   string `toml:"slug"`   // may be qualified "<plugin>:<slug>"
	Flavor string `toml:"flavor"` // claude | copilot
}

// Config models liszt.toml.
type Config struct {
	Items []Entry `toml:"items"`
}

// Load reads path. Missing file returns an empty Config.
func Load(path string) (*Config, error) {
	cfg := &Config{}
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

// Save writes cfg to path.
func Save(path string, cfg *Config) error {
	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Upsert replaces an entry with the same (Kind, Slug, Flavor) or appends.
func (c *Config) Upsert(e Entry) {
	for i, x := range c.Items {
		if x.Kind == e.Kind && x.Slug == e.Slug && x.Flavor == e.Flavor {
			c.Items[i] = e
			return
		}
	}
	c.Items = append(c.Items, e)
}
```

- [ ] **Step 2: Update `cmd/liszt/main.go`**

Add import:

```go
"github.com/mguilarducci/liszt/internal/manifest"
```

Delete from `cmd/liszt/main.go`:
- `type manifestEntry struct { ... }` (around line 51)
- `type manifestConfig struct { ... }` (around line 57)
- `func loadManifest(...)`, `func saveManifest(...)`, `func (c *manifestConfig) upsert(...)`

Update call sites in `recordInstall`:
- `loadManifest(manifestFile)` → `manifest.Load(manifestFile)`
- `saveManifest(manifestFile, man)` → `manifest.Save(manifestFile, man)`
- `man.upsert(manifestEntry{...})` → `man.Upsert(manifest.Entry{...})`
- The local `man` variable becomes `*manifest.Config`.

- [ ] **Step 3: Build and vet**

```bash
go build ./...
go vet ./...
```

Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "refactor(manifest): extract liszt.toml handling"
```

---

### Task 7: Extract `internal/lock`

**Files:**
- Create: `internal/lock/lock.go`
- Modify: `cmd/liszt/main.go`

**Purpose:** Same shape as Tasks 5–6, for liszt.lock.

- [ ] **Step 1: Create `internal/lock/lock.go`**

```go
package lock

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

// Entry is one resolved install record.
type Entry struct {
	Kind   string `toml:"kind"`
	Flavor string `toml:"flavor"`
	Slug   string `toml:"slug"`
	Plugin string `toml:"plugin"`
	Repo   string `toml:"repo"`
	SHA    string `toml:"sha"`
	Path   string `toml:"path"`
}

// Config models liszt.lock.
type Config struct {
	Locked []Entry `toml:"locked"`
}

// Load reads path. Missing file returns an empty Config.
func Load(path string) (*Config, error) {
	cfg := &Config{}
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

// Save writes cfg to path.
func Save(path string, cfg *Config) error {
	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Upsert replaces an entry with the same (Kind, Slug, Plugin, Flavor) or appends.
func (c *Config) Upsert(e Entry) {
	for i, x := range c.Locked {
		if x.Kind == e.Kind && x.Slug == e.Slug && x.Plugin == e.Plugin && x.Flavor == e.Flavor {
			c.Locked[i] = e
			return
		}
	}
	c.Locked = append(c.Locked, e)
}
```

- [ ] **Step 2: Update `cmd/liszt/main.go`**

Add import:

```go
"github.com/mguilarducci/liszt/internal/lock"
```

Delete:
- `type lockEntry struct { ... }`
- `type lockConfig struct { ... }`
- `func loadLock`, `func saveLock`, `func (c *lockConfig) upsert`

Update call sites:
- `loadLock(lockFile)` → `lock.Load(lockFile)` (2 occurrences: `recordInstall`, `outdatedCmd`)
- `saveLock(lockFile, lockCfg)` → `lock.Save(lockFile, lockCfg)`
- `lockCfg.upsert(lockEntry{...})` → `lockCfg.Upsert(lock.Entry{...})`
- In `outdatedCmd`, the local `cfg` is now `*lock.Config` and iterates `cfg.Locked`. The `entry` field in the local `drift` struct becomes `lock.Entry`:

```go
type drift struct {
    entry  lock.Entry
    latest string
}
```

- [ ] **Step 3: Build and vet**

```bash
go build ./...
go vet ./...
```

Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "refactor(lock): extract liszt.lock handling"
```

---

### Task 8: Extract `internal/resource`

**Files:**
- Create: `internal/resource/resource.go`
- Create: `internal/resource/walk.go`
- Create: `internal/resource/skill.go`
- Create: `internal/resource/agent.go`
- Create: `internal/resource/command.go`
- Create: `internal/resource/hook.go`
- Create: `internal/resource/mcp.go`
- Create: `internal/resource/lsp.go`
- Modify: `cmd/liszt/main.go`

**Purpose:** Centralize the kind registry, the discovery walkers, and each kind's lister. Largest single task — done as one commit to keep the registry coherent.

- [ ] **Step 1: Create `internal/resource/resource.go`**

```go
package resource

// Item is a discovered resource within a plugin tree.
type Item struct {
	Slug  string
	Path  string
	Extra string
}

// Lister returns the items of a given kind under a plugin root.
type Lister func(pluginRoot string) ([]Item, error)

// Kind describes how to discover items of a resource kind.
type Kind struct {
	Name string
	List Lister
}

// Stable order matches the historical map literal in main.go.
var orderedKinds = []Kind{}
var byName = map[string]Kind{}

func register(k Kind) {
	if _, exists := byName[k.Name]; exists {
		return
	}
	orderedKinds = append(orderedKinds, k)
	byName[k.Name] = k
}

// All returns every registered kind in a stable order.
func All() []Kind { return orderedKinds }

// Get returns the kind with the given name.
func Get(name string) (Kind, bool) {
	k, ok := byName[name]
	return k, ok
}
```

- [ ] **Step 2: Create `internal/resource/walk.go`**

```go
package resource

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func isMarkdownLeaf(d fs.DirEntry) bool {
	return !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".md")
}

func trimExt(rel string) string {
	rel = filepath.ToSlash(rel)
	return strings.TrimSuffix(rel, filepath.Ext(rel))
}

func identityPath(p string) string { return p }

// walkItems walks <pluginRoot>/<subdir>. For each leaf matching `match`:
//   - slugOf(rel)         → Item.Slug   (rel = path under subdir)
//   - pathOf(pathInPlugin) → Item.Path  (pathInPlugin = subdir-prefixed path within plugin root)
func walkItems(pluginRoot, subdir string, match func(fs.DirEntry) bool, slugOf func(rel string) string, pathOf func(pathInPlugin string) string) ([]Item, error) {
	base := filepath.Join(pluginRoot, subdir)
	if _, err := os.Stat(base); os.IsNotExist(err) {
		return nil, nil
	}
	var out []Item
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
			out = append(out, Item{
				Slug: slugOf(rel),
				Path: filepath.ToSlash(pathOf(pathInPlugin)),
			})
		}
		return nil
	})
	return out, err
}

// readFirstWithSource returns the first existing file under root and the relative path that matched.
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
```

- [ ] **Step 3: Create `internal/resource/skill.go`**

```go
package resource

import (
	"io/fs"
	"path/filepath"
	"strings"
)

func init() {
	register(Kind{Name: "skill", List: listSkills})
}

// skills: <plugin>/skills/<name>/SKILL.md (recursive). Artifact = the skill dir.
func listSkills(root string) ([]Item, error) {
	return walkItems(root, "skills",
		func(d fs.DirEntry) bool { return !d.IsDir() && strings.EqualFold(d.Name(), "SKILL.md") },
		func(rel string) string { return filepath.ToSlash(filepath.Dir(rel)) },
		filepath.Dir,
	)
}
```

- [ ] **Step 4: Create `internal/resource/agent.go`**

```go
package resource

import (
	"path/filepath"
	"strings"
)

func init() {
	register(Kind{Name: "agent", List: listAgents})
}

// agents: <plugin>/agents/<name>.md (Claude) or <name>.agent.md (Copilot).
func listAgents(root string) ([]Item, error) {
	return walkItems(root, "agents", isMarkdownLeaf, agentSlug, identityPath)
}

func agentSlug(rel string) string {
	rel = filepath.ToSlash(rel)
	rel = strings.TrimSuffix(rel, ".agent.md")
	rel = strings.TrimSuffix(rel, ".md")
	return rel
}
```

- [ ] **Step 5: Create `internal/resource/command.go`**

```go
package resource

func init() {
	register(Kind{Name: "command", List: listCommands})
}

// commands: <plugin>/commands/<name>.md (flat).
func listCommands(root string) ([]Item, error) {
	return walkItems(root, "commands", isMarkdownLeaf, trimExt, identityPath)
}
```

- [ ] **Step 6: Create `internal/resource/hook.go`**

```go
package resource

import (
	"encoding/json"
	"fmt"
)

func init() {
	register(Kind{Name: "hook", List: listHooks})
}

// hooks: hooks/hooks.json (Claude) or hooks.json (Copilot).
func listHooks(root string) ([]Item, error) {
	data, src, ok, err := readFirstWithSource(root, "hooks/hooks.json", "hooks.json")
	if err != nil || !ok {
		return nil, err
	}
	var doc struct {
		Hooks map[string][]any `json:"hooks"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse hooks.json: %w", err)
	}
	var out []Item
	for event, entries := range doc.Hooks {
		out = append(out, Item{
			Slug:  event,
			Path:  src + "#" + event,
			Extra: fmt.Sprintf("%d", len(entries)),
		})
	}
	return out, nil
}
```

- [ ] **Step 7: Create `internal/resource/mcp.go`**

```go
package resource

import (
	"encoding/json"
	"fmt"
)

func init() {
	register(Kind{Name: "mcp", List: listMCP})
}

// mcp: .claude-plugin/mcp.json | .mcp.json | .github/mcp.json.
func listMCP(root string) ([]Item, error) {
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
	var out []Item
	for name := range doc.MCPServers {
		out = append(out, Item{Slug: name, Path: src + "#" + name})
	}
	return out, nil
}
```

- [ ] **Step 8: Create `internal/resource/lsp.go`**

```go
package resource

import (
	"encoding/json"
	"fmt"
)

func init() {
	register(Kind{Name: "lsp", List: listLSP})
}

// lsp: lsp.json | .github/lsp.json.
func listLSP(root string) ([]Item, error) {
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
	var out []Item
	for name := range doc.Servers {
		out = append(out, Item{Slug: name, Path: src + "#" + name})
	}
	return out, nil
}
```

- [ ] **Step 9: Update `cmd/liszt/main.go`**

Add import:

```go
"github.com/mguilarducci/liszt/internal/resource"
```

Delete from `cmd/liszt/main.go`:
- `type item struct { ... }` (around line 77)
- `type kindDef struct { ... }` (around line 84)
- `var kinds = map[string]kindDef{ ... }` (around line 88)
- All `listSkills`, `listAgents`, `agentSlug`, `listCommands`, `identityPath`, `listHooks`, `listMCP`, `listLSP` functions
- `func isMarkdownLeaf`, `func trimExt`, `func walkItems`, `func readFirstWithSource`

Update call sites in `cmd/liszt/main.go`:
- `if _, ok := kinds[cmd]; ok` → `if _, ok := resource.Get(cmd); ok` (in `main`)
- `def := kinds[kind]` → `def, _ := resource.Get(kind)` (in `resourceCmd`)
- `def.list(pluginRoot)` → `def.List(pluginRoot)` (2 occurrences: `resourceCmd`, `resolveSlug`)
- `_, ok := kinds[kind]` → `_, ok := resource.Get(kind)` (in `resolveSlug`)
- Local type `[]item` → `[]resource.Item`. Iteration `for _, it := range items` stays the same (fields `Slug`, `Path`, `Extra` are exported in both old lowercase struct via `it.Slug` and new uppercase struct — they already use uppercase access in main).

Note: the old `item` struct had exported fields (`Slug`, `Path`, `Extra`), so call sites already use the right names.

- [ ] **Step 10: Build and vet**

```bash
go build ./...
go vet ./...
```

Expected: clean.

- [ ] **Step 11: Build the smoke binary**

Run: `go build -o /tmp/liszt-smoke ./cmd/liszt`
Expected: no errors.

- [ ] **Step 12: Smoke-test each kind (one command per kind)**

Run from `/tmp/liszt-baseline`:

- `/tmp/liszt-smoke skill list > /dev/null` — expected exit 0
- `/tmp/liszt-smoke agent list > /dev/null` — expected exit 0
- `/tmp/liszt-smoke command list > /dev/null` — expected exit 0
- `/tmp/liszt-smoke hook list > /dev/null` — expected exit 0
- `/tmp/liszt-smoke mcp list > /dev/null` — expected exit 0
- `/tmp/liszt-smoke lsp list > /dev/null` — expected exit 0

If any command exits non-zero, stop and diagnose before continuing.

- [ ] **Step 13: Commit**

Run: `git add -A`
Run: `git commit -m "refactor(resource): extract kind registry and listers"`

---

### Task 9: Extract `internal/cli` and slim down `cmd/liszt/main.go`

**Files:**
- Create: `internal/cli/paths.go`
- Create: `internal/cli/repo.go`
- Create: `internal/cli/plugin.go`
- Create: `internal/cli/resource.go`
- Create: `internal/cli/outdated.go`
- Create: `internal/cli/install.go`
- Create: `internal/cli/io.go`
- Modify: `cmd/liszt/main.go` (becomes thin dispatcher)

**Purpose:** Move subcommand bodies into `internal/cli`, returning errors. `cmd/liszt/main.go` becomes the dispatch + exit-code surface only.

- [ ] **Step 1: Create `internal/cli/paths.go`**

```go
package cli

// Paths bundles on-disk file locations used by every handler.
type Paths struct {
	Repos    string
	Manifest string
	Lock     string
	Cache    string
}
```

- [ ] **Step 2: Create `internal/cli/io.go`**

```go
package cli

import "fmt"

// printHeader prints the section header for resource listings.
func printHeader(first *bool, repoName, pluginName, kind string, n int) {
	if !*first {
		fmt.Println()
	}
	*first = false
	fmt.Printf("== %s :: %s (%d %ss) ==\n", repoName, pluginName, n, kind)
}
```

- [ ] **Step 3: Create `internal/cli/install.go`**

```go
package cli

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

// ParseInstallArgs reads "<slug> [--flavor X]" from positional args.
// Exits with code 2 on malformed input (preserves current behavior).
func ParseInstallArgs(args []string) (slug, flavor string) {
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

// resolveSlug scans repos for an artifact of the given kind matching slug.
// slug may be qualified as "<plugin>:<artifact>" to disambiguate.
func resolveSlug(p Paths, kind, raw string) ([]match, error) {
	var wantPlugin, wantSlug string
	if i := strings.Index(raw, ":"); i > 0 {
		wantPlugin = raw[:i]
		wantSlug = raw[i+1:]
	} else {
		wantSlug = raw
	}

	cfg, err := repos.Load(p.Repos)
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
		root := gitx.RepoPath(p.Cache, owner, repo)
		mp, _, err := marketplace.Read(root)
		if err != nil {
			continue
		}
		for _, plug := range mp.Plugins {
			if wantPlugin != "" && plug.Name != wantPlugin {
				continue
			}
			rel := mp.ResolvePluginPath(plug)
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
					pluginName: plug.Name,
					repoName:   r.Name,
					sha:        r.SHA,
					path:       filepath.ToSlash(filepath.Join(rel, it.Path)),
				})
			}
		}
	}
	return out, nil
}

func recordInstall(p Paths, m match, requestedSlug string) error {
	man, err := manifest.Load(p.Manifest)
	if err != nil {
		return err
	}
	man.Upsert(manifest.Entry{Kind: m.kind, Slug: requestedSlug, Flavor: m.flavor})
	if err := manifest.Save(p.Manifest, man); err != nil {
		return err
	}

	lk, err := lock.Load(p.Lock)
	if err != nil {
		return err
	}
	lk.Upsert(lock.Entry{
		Kind: m.kind, Flavor: m.flavor, Slug: m.slug, Plugin: m.pluginName,
		Repo: m.repoName, SHA: m.sha, Path: m.path,
	})
	if err := lock.Save(p.Lock, lk); err != nil {
		return err
	}

	fmt.Printf("installed %s %s [%s] (from %s @ %s)\n", m.kind, m.slug, m.flavor, m.repoName, m.sha[:12])
	return nil
}
```

- [ ] **Step 4: Create `internal/cli/repo.go`**

```go
package cli

import (
	"fmt"
	"os"

	"github.com/mguilarducci/liszt/internal/gitx"
	"github.com/mguilarducci/liszt/internal/marketplace"
	"github.com/mguilarducci/liszt/internal/repos"
)

// Repo handles `liszt repo add <github-url>`.
func Repo(p Paths, args []string) error {
	if len(args) < 2 || args[0] != "add" {
		fmt.Fprintln(os.Stderr, "usage: liszt repo add <github-url>")
		os.Exit(2)
	}
	owner, repo, err := gitx.ParseGitHubURL(args[1])
	if err != nil {
		return err
	}

	dest := gitx.RepoPath(p.Cache, owner, repo)
	if err := gitx.EnsureClone(args[1], dest); err != nil {
		return err
	}

	sha, err := gitx.HeadSHA(dest)
	if err != nil {
		return err
	}
	if _, _, err := marketplace.Read(dest); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}

	cfg, err := repos.Load(p.Repos)
	if err != nil {
		return err
	}
	cfg.Upsert(repos.Entry{Name: owner + "/" + repo, URL: args[1], SHA: sha})
	if err := repos.Save(p.Repos, cfg); err != nil {
		return err
	}

	fmt.Printf("added %s/%s @ %s\n", owner, repo, sha[:12])
	return nil
}
```

- [ ] **Step 5: Create `internal/cli/plugin.go`**

```go
package cli

import (
	"fmt"
	"os"

	"github.com/mguilarducci/liszt/internal/gitx"
	"github.com/mguilarducci/liszt/internal/marketplace"
	"github.com/mguilarducci/liszt/internal/repos"
)

// PluginList handles `liszt plugin list`.
func PluginList(p Paths) error {
	cfg, err := repos.Load(p.Repos)
	if err != nil {
		return err
	}
	if len(cfg.Repos) == 0 {
		fmt.Println("no repos. add one with: liszt repo add <url>")
		return nil
	}
	for i, r := range cfg.Repos {
		owner, repo, err := gitx.ParseGitHubURL(r.URL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skip %s: %v\n", r.Name, err)
			continue
		}
		mp, _, err := marketplace.Read(gitx.RepoPath(p.Cache, owner, repo))
		if err != nil {
			fmt.Fprintf(os.Stderr, "skip %s: %v\n", r.Name, err)
			continue
		}
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("== %s (%d plugins) ==\n", r.Name, len(mp.Plugins))
		for _, plug := range mp.Plugins {
			fmt.Printf("- %s", plug.Name)
			if plug.Version != "" {
				fmt.Printf(" (v%s)", plug.Version)
			}
			fmt.Println()
			if plug.Description != "" {
				fmt.Printf("  %s\n", plug.Description)
			}
		}
	}
	return nil
}

// PluginInstall handles `liszt plugin install <slug> --flavor <flavor>`.
func PluginInstall(p Paths, slug, flavor string) error {
	cfg, err := repos.Load(p.Repos)
	if err != nil {
		return err
	}
	for _, r := range cfg.Repos {
		owner, repo, err := gitx.ParseGitHubURL(r.URL)
		if err != nil {
			continue
		}
		root := gitx.RepoPath(p.Cache, owner, repo)
		mp, _, err := marketplace.Read(root)
		if err != nil {
			continue
		}
		for _, plug := range mp.Plugins {
			if plug.Name != slug {
				continue
			}
			return recordInstall(p, match{
				kind:       "plugin",
				flavor:     flavor,
				slug:       plug.Name,
				pluginName: plug.Name,
				repoName:   r.Name,
				sha:        r.SHA,
				path:       mp.ResolvePluginPath(plug),
			}, slug)
		}
	}
	fmt.Fprintf(os.Stderr, "plugin %q not found\n", slug)
	os.Exit(1)
	return nil
}
```

- [ ] **Step 6: Create `internal/cli/resource.go`**

```go
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mguilarducci/liszt/internal/gitx"
	"github.com/mguilarducci/liszt/internal/marketplace"
	"github.com/mguilarducci/liszt/internal/repos"
	"github.com/mguilarducci/liszt/internal/resource"
)

// ResourceList handles `liszt <kind> list [--plugin <name>]`.
func ResourceList(p Paths, kind, pluginName string) error {
	def, ok := resource.Get(kind)
	if !ok {
		return fmt.Errorf("unknown kind %q", kind)
	}

	cfg, err := repos.Load(p.Repos)
	if err != nil {
		return err
	}

	matched := false
	first := true
	for _, r := range cfg.Repos {
		owner, repo, err := gitx.ParseGitHubURL(r.URL)
		if err != nil {
			continue
		}
		root := gitx.RepoPath(p.Cache, owner, repo)
		mp, _, err := marketplace.Read(root)
		if err != nil {
			continue
		}
		for _, plug := range mp.Plugins {
			if pluginName != "" && plug.Name != pluginName {
				continue
			}
			pluginRoot := filepath.Join(root, mp.ResolvePluginPath(plug))
			items, err := def.List(pluginRoot)
			if err != nil || len(items) == 0 {
				if pluginName != "" && plug.Name == pluginName {
					matched = true
					printHeader(&first, r.Name, plug.Name, kind, len(items))
				}
				continue
			}
			matched = true
			printHeader(&first, r.Name, plug.Name, kind, len(items))
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
	return nil
}

// ResourceInstall handles `liszt <kind> install <slug> --flavor <flavor>`.
func ResourceInstall(p Paths, kind, slug, flavor string) error {
	matches, err := resolveSlug(p, kind, slug)
	if err != nil {
		return err
	}
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
	return recordInstall(p, m, slug)
}
```

- [ ] **Step 7: Create `internal/cli/outdated.go`**

```go
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
```

- [ ] **Step 8: Rewrite `cmd/liszt/main.go` as the thin dispatcher**

Replace the entire file with:

```go
package main

import (
	"fmt"
	"os"

	"github.com/mguilarducci/liszt/internal/cli"
	"github.com/mguilarducci/liszt/internal/resource"
)

const (
	reposFile    = "repos.toml"
	manifestFile = "liszt.toml"
	lockFile     = "liszt.lock"
	cacheDir     = "tmp"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	paths := cli.Paths{Repos: reposFile, Manifest: manifestFile, Lock: lockFile, Cache: cacheDir}

	switch cmd {
	case "repo":
		run(cli.Repo(paths, args))
	case "plugin":
		runPlugin(paths, args)
	case "outdated":
		run(cli.Outdated(paths))
	default:
		if _, ok := resource.Get(cmd); ok {
			runResource(paths, cmd, args)
			return
		}
		usage()
		os.Exit(2)
	}
}

func runPlugin(p cli.Paths, args []string) {
	if len(args) >= 2 && args[0] == "install" {
		slug, flavor := cli.ParseInstallArgs(args[1:])
		run(cli.PluginInstall(p, slug, flavor))
		return
	}
	if len(args) < 1 || args[0] != "list" {
		fmt.Fprintln(os.Stderr, "usage: liszt plugin {list | install <slug> --flavor <claude|copilot>}")
		os.Exit(2)
	}
	run(cli.PluginList(p))
}

func runResource(p cli.Paths, kind string, args []string) {
	if len(args) >= 2 && args[0] == "install" {
		slug, flavor := cli.ParseInstallArgs(args[1:])
		run(cli.ResourceInstall(p, kind, slug, flavor))
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
	run(cli.ResourceList(p, kind, pluginName))
}

func run(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
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
```

- [ ] **Step 9: Build and vet**

```bash
go build ./...
go vet ./...
```

Expected: clean.

- [ ] **Step 10: Smoke test**

```bash
go build -o /tmp/liszt-smoke ./cmd/liszt
/tmp/liszt-smoke
echo "usage exit=$?"
```

Expected: `usage exit=2` and the usage block on stderr.

- [ ] **Step 11: Commit**

```bash
git add -A
git commit -m "refactor(cli): extract subcommand handlers; main is thin dispatcher"
```

---

### Task 10: Parity verification against baseline

**Files:**
- Create: `scripts/capture-new.sh` (committed)
- Create: `scripts/verify-parity.sh` (committed)
- Read: `/tmp/liszt-baseline/old-*` (from Task 1)
- Create: `/tmp/liszt-newrun/new-*` (fresh captures, not committed)

**Purpose:** Prove the refactor is observably identical to the pre-refactor binary. Verification logic lives in committed scripts (single bash invocation each) so the rule "no compound bash in tool calls" is respected.

- [ ] **Step 1: Create `scripts/capture-new.sh`**

```bash
#!/usr/bin/env bash
set -euo pipefail

NEW_BIN=${NEW_BIN:-/tmp/liszt-new}
BASE=${BASE:-/tmp/liszt-baseline}
RUN=${RUN:-/tmp/liszt-newrun}

rm -rf "$RUN"
mkdir -p "$RUN"
cp "$BASE/repos.toml" "$RUN/repos.toml"
cp -R "$BASE/tmp" "$RUN/tmp"

cd "$RUN"
rm -f liszt.toml liszt.lock

run() {
  local label=$1
  shift
  "$NEW_BIN" "$@" > "new-$label.out" 2> "new-$label.err" || true
  echo "exit=$?" > "new-$label.code"
}

run plugin-list plugin list
run skill-list skill list
run agent-list agent list
run command-list command list
run hook-list hook list
run mcp-list mcp list
run lsp-list lsp list

run install-skill   skill   install brainstorming   --flavor claude
run install-agent   agent   install code-reviewer   --flavor claude
run install-plugin  plugin  install hookify         --flavor claude
run install-command command install commit          --flavor copilot
run install-hook    hook    install SessionStart    --flavor copilot
cp liszt.toml new-liszt.toml
cp liszt.lock new-liszt.lock

run outdated      outdated
run usage-empty
run usage-bogus   bogus
run usage-skill   skill

echo "new captures at $RUN"
```

- [ ] **Step 2: Create `scripts/verify-parity.sh`**

```bash
#!/usr/bin/env bash
set -uo pipefail

B=${BASE:-/tmp/liszt-baseline}
N=${RUN:-/tmp/liszt-newrun}
FAIL=0

note() { echo "FAIL: $*"; FAIL=1; }

# 1. Deterministic stdout diffs (must be empty).
DET_STDOUT="plugin-list skill-list agent-list command-list outdated \
            install-skill install-agent install-plugin install-command install-hook \
            usage-empty usage-bogus usage-skill"
for f in $DET_STDOUT; do
  diff -u "$B/old-$f.out" "$N/new-$f.out" > /dev/null
  if [ $? -ne 0 ]; then
    note "stdout drift: $f"
    diff -u "$B/old-$f.out" "$N/new-$f.out"
  fi
done

# 2. TOML round-trip (must be byte-equal).
for f in liszt.toml liszt.lock; do
  diff -u "$B/old-$f" "$N/new-$f" > /dev/null
  if [ $? -ne 0 ]; then
    note "$f drift"
    diff -u "$B/old-$f" "$N/new-$f"
  fi
done

# 3. Exit codes (must match).
DET_CODE="plugin-list skill-list agent-list command-list outdated usage-empty usage-bogus usage-skill"
for f in $DET_CODE; do
  diff -u "$B/old-$f.code" "$N/new-$f.code" > /dev/null
  if [ $? -ne 0 ]; then
    note "exit-code drift: $f"
    diff -u "$B/old-$f.code" "$N/new-$f.code"
  fi
done

# 4. Non-deterministic (map iteration) outputs: compare as sets.
for k in hook mcp lsp; do
  sort "$B/old-$k-list.out" > "/tmp/cmp-old-$k"
  sort "$N/new-$k-list.out" > "/tmp/cmp-new-$k"
  diff -u "/tmp/cmp-old-$k" "/tmp/cmp-new-$k" > /dev/null
  if [ $? -ne 0 ]; then
    note "set drift: $k-list"
    diff -u "/tmp/cmp-old-$k" "/tmp/cmp-new-$k"
  fi
done

if [ "$FAIL" -ne 0 ]; then
  echo "PARITY CHECK FAILED"
  exit 1
fi
echo "PARITY CHECK PASSED"
```

- [ ] **Step 3: Make both scripts executable**

Run: `chmod +x scripts/capture-new.sh scripts/verify-parity.sh`
Expected: no output.

- [ ] **Step 4: Build the refactored binary**

Run: `go build -o /tmp/liszt-new ./cmd/liszt`
Expected: no errors.

- [ ] **Step 5: Capture outputs from the new binary**

Run: `bash scripts/capture-new.sh`
Expected: prints `new captures at /tmp/liszt-newrun`.

- [ ] **Step 6: Verify parity**

Run: `bash scripts/verify-parity.sh`
Expected: prints `PARITY CHECK PASSED` and exits 0.

If it prints `PARITY CHECK FAILED`, the offending diffs follow. Each `FAIL:` line names which capture drifted. Do **not** mark this task complete — open the diff, identify the regression, fix in a follow-up task, re-run from Step 4.

- [ ] **Step 7: Stderr spot-check (informational)**

Stderr may legitimately differ in install paths where the multi-match `note:` line depends on map iteration order. Inspect the install-* stderr by hand:

Run: `diff /tmp/liszt-baseline/old-install-skill.err /tmp/liszt-newrun/new-install-skill.err`
Expected: either empty or only the multi-match `note:` ordering differs.

Repeat for `install-agent`, `install-plugin`, `install-command`, `install-hook`. Any other stderr drift is a real regression — investigate before declaring done.

- [ ] **Step 8: Commit the verification scripts**

Run: `git add scripts/capture-new.sh scripts/verify-parity.sh`
Run: `git commit -m "chore: add parity verification scripts for refactor"`

---

## Self-Review

**Spec coverage:**
- Package tree (spec §Package tree): Task 2 (cmd/liszt move), Task 3 (gitx), Task 4 (marketplace), Tasks 5–7 (repos/manifest/lock), Task 8 (resource), Task 9 (cli + thin main). ✓
- Dependency direction (spec §Dependency direction): enforced by import statements in each created file. ✓
- Component contracts (spec §Component contracts): API signatures replicated verbatim. ✓
- Data flow (spec §Data flow): each handler in Task 9 mirrors the spec's flow exactly. ✓
- Error handling (spec §Error handling): Task 9 introduces `run(err)` in main; handlers return errors; exit codes for usage stay in handlers (preserves current `os.Exit(2)` behavior in `ParseInstallArgs`, `Repo`, `PluginList`, `ResourceList`). ✓
- Warning strings (spec §Error handling, exact prefixes): preserved verbatim in `Repo` (`warning:`), `Outdated` (`warn: ls-remote`), `PluginList`/`ResourceList` (`skip`), `ResourceInstall` (`note:`). ✓
- Behavioral parity verification (spec §Behavioral parity): Task 1 (baseline) + Task 10 (diff). ✓
- Out-of-scope items (spec §Out of scope): no new tests added; deterministic `All()` order is implemented but unused at CLI boundary, matching the spec's note. ✓

**Placeholder scan:** No TBD/TODO/"fill in details" lines. All code blocks are complete.

**Compound-bash rule:** Verification logic in Tasks 1 and 10 is wrapped in committed scripts (`scripts/capture-baseline.sh`, `scripts/capture-new.sh`, `scripts/verify-parity.sh`) so each executing tool call is a single `bash` invocation. Other commands in the plan are deliberately one-per-step.

**Type consistency:**
- `Paths` shape (Repos/Manifest/Lock/Cache) consistent across `cli/paths.go`, every handler, and `cmd/liszt/main.go`.
- `match` struct fields (`kind`, `flavor`, `slug`, `pluginName`, `repoName`, `sha`, `path`) consistent between `install.go`, `plugin.go`, `resource.go`.
- `resource.Get` returns `(Kind, bool)`; `Kind.List` used identically in `install.go` (`def.List`) and `resource.go` (`def.List`).
- TOML struct tags preserved exactly from original code (verified per-package against original main.go).
