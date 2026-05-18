# Plugin Install into Claude — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `liszt plugin install <slug> --flavor claude` actually materialize the plugin under `~/.claude/plugins/` so Claude Code loads it on next session.

**Architecture:** Add `internal/claudehome` (resolve `~/.claude`) and `internal/claudestate` (JSON registries + cache copier). Extend `internal/marketplace` with typed `ParseSource` for both string and `git-subdir` forms. Extend `internal/gitx` with `CloneAtSHA`. Rewrite the Claude branch of `cli.PluginInstall` to orchestrate: discover → fetch source → copy → upsert `installed_plugins.json` → upsert `known_marketplaces.json` (+ symlink) → enable in `settings.json` → write liszt manifest + lock.

**Tech Stack:** Go 1.26, stdlib `encoding/json`, `os`, `os/exec`, `path/filepath`. No new external deps.

**Spec:** `docs/superpowers/specs/2026-05-18-plugin-install-claude-design.md`

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/claudehome/home.go` | Create | Resolve `~/.claude` with optional `$CLAUDE_HOME` override. |
| `internal/claudestate/installed.go` | Create | Load/save/upsert `installed_plugins.json` (schema v2). |
| `internal/claudestate/known.go` | Create | Load/save/upsert `known_marketplaces.json`. |
| `internal/claudestate/settings.go` | Create | Mutate `enabledPlugins` key in `settings.json` without touching other keys. |
| `internal/claudestate/cache.go` | Create | Copy plugin source tree into `~/.claude/plugins/cache/<mp>/<plugin>/<version>/`. |
| `internal/marketplace/marketplace.go` | Modify | Add `PluginSource`, `ExternalSource`, `ParseSource`. Keep `ResolvePluginPath`. |
| `internal/gitx/git.go` | Modify | Add `CloneAtSHA`. |
| `internal/cli/install.go` | Modify | New `claudeInstall(...)` orchestrator. `PluginInstall` dispatches on flavor. |

---

### Task 1: `internal/claudehome`

**Files:**
- Create: `internal/claudehome/home.go`

- [ ] **Step 1: Write `internal/claudehome/home.go`**

```go
package claudehome

import (
	"os"
	"path/filepath"
)

// Dir returns the Claude Code home directory.
// $CLAUDE_HOME if set to an absolute path, otherwise ~/.claude.
func Dir() string {
	if v := os.Getenv("CLAUDE_HOME"); v != "" && filepath.IsAbs(v) {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".claude")
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./internal/claudehome/...`
Expected: exit 0, no output.

- [ ] **Step 3: Commit**

```bash
git add internal/claudehome/home.go
git commit -m "feat(claudehome): resolve ~/.claude with env override"
```

---

### Task 2: `marketplace.ParseSource`

**Files:**
- Modify: `internal/marketplace/marketplace.go`

- [ ] **Step 1: Append new types and parser at end of file**

Append to `internal/marketplace/marketplace.go`:

```go
// PluginSource is the parsed form of Plugin.Source.
// Exactly one of Subdir / External is set.
type PluginSource struct {
	// Subdir is a path relative to the marketplace repo root,
	// e.g. "plugins/foo". Empty when External is set.
	Subdir string

	// External points to a separate git repo + subpath.
	External *ExternalSource
}

// ExternalSource describes a "git-subdir" plugin source.
type ExternalSource struct {
	URL  string
	Path string
	Ref  string
	SHA  string
}

// ParseSource decodes a Plugin.Source value.
//   string "./plugins/foo"                              -> Subdir
//   { "path": "plugins/foo" }                           -> Subdir
//   { "source": "git-subdir", "url", "path", "ref", "sha" } -> External
func ParseSource(raw any) (PluginSource, error) {
	switch v := raw.(type) {
	case string:
		s := strings.TrimPrefix(v, "./")
		return PluginSource{Subdir: s}, nil
	case map[string]any:
		if src, _ := v["source"].(string); src == "git-subdir" {
			ext := &ExternalSource{}
			ext.URL, _ = v["url"].(string)
			ext.Path, _ = v["path"].(string)
			ext.Ref, _ = v["ref"].(string)
			ext.SHA, _ = v["sha"].(string)
			if ext.URL == "" || ext.SHA == "" {
				return PluginSource{}, fmt.Errorf("git-subdir source missing url or sha")
			}
			return PluginSource{External: ext}, nil
		}
		if p, ok := v["path"].(string); ok {
			return PluginSource{Subdir: strings.TrimPrefix(p, "./")}, nil
		}
	case nil:
		return PluginSource{}, nil
	}
	return PluginSource{}, fmt.Errorf("unsupported plugin source: %T", raw)
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./internal/marketplace/...`
Expected: exit 0, no output.

- [ ] **Step 3: Commit**

```bash
git add internal/marketplace/marketplace.go
git commit -m "feat(marketplace): typed ParseSource for string and git-subdir"
```

---

### Task 3: `gitx.CloneAtSHA`

**Files:**
- Modify: `internal/gitx/git.go`

- [ ] **Step 1: Append `CloneAtSHA`**

Append to `internal/gitx/git.go`:

```go
// CloneAtSHA shallow-clones url into dest and checks out sha.
// Idempotent: if dest already has HEAD == sha, returns nil.
// On failure, leaves no partial directory behind.
func CloneAtSHA(url, sha, dest string) error {
	if head, err := HeadSHA(dest); err == nil && head == sha {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(filepath.Dir(dest), ".liszt-clone-*")
	if err != nil {
		return err
	}
	if err := cloneInto(url, sha, tmp); err != nil {
		os.RemoveAll(tmp)
		return err
	}
	if err := os.RemoveAll(dest); err != nil {
		os.RemoveAll(tmp)
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		os.RemoveAll(tmp)
		return err
	}
	return nil
}

func cloneInto(url, sha, dir string) error {
	run := func(args ...string) error {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	if err := run("init", "-q"); err != nil {
		return fmt.Errorf("git init: %w", err)
	}
	if err := run("remote", "add", "origin", url); err != nil {
		return fmt.Errorf("git remote add: %w", err)
	}
	if err := run("fetch", "--depth=1", "origin", sha); err != nil {
		// Fall back to full fetch when server refuses by-sha shallow fetch.
		if err2 := run("fetch", "origin"); err2 != nil {
			return fmt.Errorf("git fetch: %w (after shallow: %v)", err2, err)
		}
	}
	if err := run("checkout", "-q", sha); err != nil {
		return fmt.Errorf("git checkout %s: %w", sha, err)
	}
	return nil
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./internal/gitx/...`
Expected: exit 0, no output.

- [ ] **Step 3: Commit**

```bash
git add internal/gitx/git.go
git commit -m "feat(gitx): add CloneAtSHA with atomic dest rename"
```

---

### Task 4: `claudestate.InstalledPlugins`

**Files:**
- Create: `internal/claudestate/installed.go`

- [ ] **Step 1: Write `internal/claudestate/installed.go`**

```go
package claudestate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// InstalledPlugin is one entry in installed_plugins.json.
type InstalledPlugin struct {
	Scope        string `json:"scope"`
	InstallPath  string `json:"installPath"`
	Version      string `json:"version"`
	InstalledAt  string `json:"installedAt"`
	LastUpdated  string `json:"lastUpdated"`
	GitCommitSha string `json:"gitCommitSha"`
}

// InstalledPlugins mirrors installed_plugins.json (schema v2).
type InstalledPlugins struct {
	Version int                          `json:"version"`
	Plugins map[string][]InstalledPlugin `json:"plugins"`
}

// LoadInstalled reads path. Missing file returns an empty v2 registry.
func LoadInstalled(path string) (*InstalledPlugins, error) {
	ip := &InstalledPlugins{Version: 2, Plugins: map[string][]InstalledPlugin{}}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ip, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, ip); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if ip.Plugins == nil {
		ip.Plugins = map[string][]InstalledPlugin{}
	}
	if ip.Version == 0 {
		ip.Version = 2
	}
	return ip, nil
}

// SaveInstalled writes ip with 2-space indent. Creates parent dirs.
func SaveInstalled(path string, ip *InstalledPlugins) error {
	data, err := json.MarshalIndent(ip, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// FindUserEntry returns the scope=user entry for key, or nil.
func (ip *InstalledPlugins) FindUserEntry(key string) *InstalledPlugin {
	for i, e := range ip.Plugins[key] {
		if e.Scope == "user" {
			return &ip.Plugins[key][i]
		}
	}
	return nil
}

// Upsert replaces the scope=user entry for key (preserving InstalledAt
// if present) or appends a new one.
func (ip *InstalledPlugins) Upsert(key string, e InstalledPlugin) {
	e.Scope = "user"
	for i, x := range ip.Plugins[key] {
		if x.Scope == "user" {
			if e.InstalledAt == "" {
				e.InstalledAt = x.InstalledAt
			}
			ip.Plugins[key][i] = e
			return
		}
	}
	ip.Plugins[key] = append(ip.Plugins[key], e)
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./internal/claudestate/...`
Expected: exit 0, no output.

- [ ] **Step 3: Commit**

```bash
git add internal/claudestate/installed.go
git commit -m "feat(claudestate): InstalledPlugins load/save/upsert"
```

---

### Task 5: `claudestate.KnownMarketplaces`

**Files:**
- Create: `internal/claudestate/known.go`

- [ ] **Step 1: Write `internal/claudestate/known.go`**

```go
package claudestate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MarketplaceSource is the "source" object in known_marketplaces.json.
type MarketplaceSource struct {
	Source string `json:"source"` // "github"
	Repo   string `json:"repo"`   // "owner/name"
}

// KnownMarketplace is one entry in known_marketplaces.json.
type KnownMarketplace struct {
	Source          MarketplaceSource `json:"source"`
	InstallLocation string            `json:"installLocation"`
	LastUpdated     string            `json:"lastUpdated"`
}

// KnownMarketplaces maps marketplace name to its registration.
type KnownMarketplaces map[string]KnownMarketplace

// LoadKnown reads path. Missing file returns an empty map.
func LoadKnown(path string) (KnownMarketplaces, error) {
	km := KnownMarketplaces{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return km, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, &km); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return km, nil
}

// SaveKnown writes km with 2-space indent. Creates parent dirs.
func SaveKnown(path string, km KnownMarketplaces) error {
	data, err := json.MarshalIndent(km, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// UpsertMarketplace inserts name if absent. If present with a different
// Source.Repo, returns a conflict error. LastUpdated is always refreshed.
func (km KnownMarketplaces) UpsertMarketplace(name string, src MarketplaceSource, installLocation, now string) error {
	if cur, ok := km[name]; ok {
		if cur.Source.Repo != src.Repo {
			return fmt.Errorf("marketplace %q already registered with different source %q; resolve manually", name, cur.Source.Repo)
		}
		cur.LastUpdated = now
		cur.InstallLocation = installLocation
		km[name] = cur
		return nil
	}
	km[name] = KnownMarketplace{Source: src, InstallLocation: installLocation, LastUpdated: now}
	return nil
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./internal/claudestate/...`
Expected: exit 0, no output.

- [ ] **Step 3: Commit**

```bash
git add internal/claudestate/known.go
git commit -m "feat(claudestate): KnownMarketplaces load/save/upsert"
```

---

### Task 6: `claudestate.EnableSettingPlugin`

**Files:**
- Create: `internal/claudestate/settings.go`

- [ ] **Step 1: Write `internal/claudestate/settings.go`**

```go
package claudestate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// EnableSettingPlugin reads ~/.claude/settings.json as an untyped map,
// sets enabledPlugins[key] = true, writes back with 2-space indent.
// Creates the file with a minimal payload if missing.
func EnableSettingPlugin(path, key string) error {
	root := map[string]any{}
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := json.Unmarshal(data, &root); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
	case os.IsNotExist(err):
		// fall through with empty root
	default:
		return err
	}

	enabled, _ := root["enabledPlugins"].(map[string]any)
	if enabled == nil {
		enabled = map[string]any{}
	}
	enabled[key] = true
	root["enabledPlugins"] = enabled

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, out, 0644)
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./internal/claudestate/...`
Expected: exit 0, no output.

- [ ] **Step 3: Commit**

```bash
git add internal/claudestate/settings.go
git commit -m "feat(claudestate): EnableSettingPlugin toggles enabledPlugins"
```

---

### Task 7: `claudestate.MaterializePlugin`

**Files:**
- Create: `internal/claudestate/cache.go`

- [ ] **Step 1: Write `internal/claudestate/cache.go`**

```go
package claudestate

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// MaterializePlugin copies srcDir contents into
//   claudeHome/plugins/cache/<mp>/<plugin>/<version>/
// Removes any existing contents first to avoid stale files.
// Skips ".in_use" directories (Claude-internal session markers).
// Returns the absolute install path.
func MaterializePlugin(claudeHome, mp, plugin, version, srcDir string) (string, error) {
	dest := filepath.Join(claudeHome, "plugins", "cache", mp, plugin, version)
	if err := os.RemoveAll(dest); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dest, 0755); err != nil {
		return "", err
	}
	err := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == ".in_use" && d.IsDir() {
			return fs.SkipDir
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return copyFile(path, target)
	})
	if err != nil {
		return "", err
	}
	return dest, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./internal/claudestate/...`
Expected: exit 0, no output.

- [ ] **Step 3: Commit**

```bash
git add internal/claudestate/cache.go
git commit -m "feat(claudestate): MaterializePlugin copies tree into Claude cache"
```

---

### Task 8: Orchestrate Claude install in CLI

**Files:**
- Modify: `internal/cli/plugin.go`

- [ ] **Step 1: Replace the import block in `internal/cli/plugin.go`**

```go
import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mguilarducci/liszt/internal/claudehome"
	"github.com/mguilarducci/liszt/internal/claudestate"
	"github.com/mguilarducci/liszt/internal/gitx"
	"github.com/mguilarducci/liszt/internal/marketplace"
	"github.com/mguilarducci/liszt/internal/repos"
)
```

- [ ] **Step 2: Replace the body of `PluginInstall`**

Replace the existing `PluginInstall` in `internal/cli/plugin.go` with:

```go
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
			m := match{
				kind:       "plugin",
				flavor:     flavor,
				slug:       plug.Name,
				pluginName: plug.Name,
				repoName:   r.Name,
				sha:        r.SHA,
				path:       mp.ResolvePluginPath(plug),
			}
			if flavor == "claude" {
				if err := claudeInstall(p, r, owner, repo, root, mp, plug); err != nil {
					return err
				}
			}
			return recordInstall(p, m, slug)
		}
	}
	fmt.Fprintf(os.Stderr, "plugin %q not found\n", slug)
	os.Exit(1)
	return nil
}
```

Add the orchestrator at the bottom of `internal/cli/plugin.go`:

```go
// claudeInstall materializes plug into ~/.claude/plugins/ and enables it.
func claudeInstall(p Paths, r repos.Entry, owner, repoName, mpClone string, mp *marketplace.Marketplace, plug marketplace.Plugin) error {
	src, err := marketplace.ParseSource(plug.Source)
	if err != nil {
		return err
	}

	var srcDir, srcSha string
	switch {
	case src.External != nil:
		extOwner, extRepo, err := gitx.ParseGitHubURL(src.External.URL)
		if err != nil {
			return fmt.Errorf("parse git-subdir url %q: %w", src.External.URL, err)
		}
		extClone := gitx.RepoPath(p.Cache, extOwner, extRepo)
		if err := gitx.CloneAtSHA(src.External.URL, src.External.SHA, extClone); err != nil {
			return err
		}
		srcDir = filepath.Join(extClone, src.External.Path)
		srcSha = src.External.SHA
	default:
		srcDir = filepath.Join(mpClone, src.Subdir)
		srcSha = r.SHA
	}

	version := plug.Version
	if version == "" {
		version = "unknown"
	}

	home := claudehome.Dir()
	installedPath := filepath.Join(home, "plugins", "installed_plugins.json")
	knownPath := filepath.Join(home, "plugins", "known_marketplaces.json")
	settingsPath := filepath.Join(home, "settings.json")
	mpName := mp.Name
	key := plug.Name + "@" + mpName
	now := time.Now().UTC().Format(time.RFC3339)

	installed, err := claudestate.LoadInstalled(installedPath)
	if err != nil {
		return err
	}
	cur := installed.FindUserEntry(key)
	needCopy := cur == nil || cur.GitCommitSha != srcSha

	installPath := ""
	if needCopy {
		installPath, err = claudestate.MaterializePlugin(home, mpName, plug.Name, version, srcDir)
		if err != nil {
			return err
		}
		entry := claudestate.InstalledPlugin{
			InstallPath:  installPath,
			Version:      version,
			LastUpdated:  now,
			GitCommitSha: srcSha,
		}
		if cur == nil {
			entry.InstalledAt = now
		}
		installed.Upsert(key, entry)
		if err := claudestate.SaveInstalled(installedPath, installed); err != nil {
			return err
		}
	}

	known, err := claudestate.LoadKnown(knownPath)
	if err != nil {
		return err
	}
	mpInstallLoc := filepath.Join(home, "plugins", "marketplaces", mpName)
	src2 := claudestate.MarketplaceSource{Source: "github", Repo: owner + "/" + repoName}
	if err := known.UpsertMarketplace(mpName, src2, mpInstallLoc, now); err != nil {
		return err
	}
	if err := claudestate.SaveKnown(knownPath, known); err != nil {
		return err
	}
	if err := ensureMarketplaceSymlink(mpInstallLoc, mpClone); err != nil {
		return err
	}

	if err := claudestate.EnableSettingPlugin(settingsPath, key); err != nil {
		return err
	}
	return nil
}

// ensureMarketplaceSymlink makes link point at target. If link exists and
// already points at target, it's a no-op. If it points elsewhere, returns
// an error so the user can resolve manually.
func ensureMarketplaceSymlink(link, target string) error {
	if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
		return err
	}
	if cur, err := os.Readlink(link); err == nil {
		if cur == target {
			return nil
		}
		return fmt.Errorf("symlink %s exists and points to %s, not %s; resolve manually", link, cur, target)
	} else if _, statErr := os.Lstat(link); statErr == nil {
		return fmt.Errorf("%s exists and is not a symlink; resolve manually", link)
	}
	return os.Symlink(target, link)
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: exit 0, no output.

- [ ] **Step 4: Smoke test (string source)**

Pick a known plugin from `claude-plugins-official` whose `source` is a string (e.g. `superpowers`).

Run: `go run ./cmd/liszt plugin install superpowers --flavor claude`
Expected stdout: `installed plugin superpowers [claude] (from anthropics/claude-plugins-official @ <sha-12>)`

Verify:
- `~/.claude/plugins/cache/claude-plugins-official/superpowers/<version>/.claude-plugin/` exists.
- `~/.claude/plugins/installed_plugins.json` has `"superpowers@claude-plugins-official"` entry with `scope: "user"`.
- `~/.claude/plugins/known_marketplaces.json` has `"claude-plugins-official"` entry.
- `~/.claude/plugins/marketplaces/claude-plugins-official` is a symlink to `~/.cache/liszt/repos/anthropics/claude-plugins-official`.
- `~/.claude/settings.json` `enabledPlugins["superpowers@claude-plugins-official"] == true`.

- [ ] **Step 5: Smoke test (idempotency)**

Run the same command again.
Expected: same stdout. `~/.claude/plugins/cache/...` directory mtime unchanged for files (no copy performed).

- [ ] **Step 6: Smoke test (git-subdir source)**

Pick a plugin from `claude-plugins-official` whose `source` is `git-subdir` (e.g. `42crunch-api-security-testing` or `adobe-for-creativity`).

Run: `go run ./cmd/liszt plugin install adobe-for-creativity --flavor claude`
Expected stdout: install confirmation.

Verify:
- `~/.cache/liszt/repos/adobe/skills/` exists (external clone).
- `~/.claude/plugins/cache/claude-plugins-official/adobe-for-creativity/<version>/` populated.
- `installed_plugins.json` entry has `gitCommitSha` matching the `sha` from `marketplace.json` (not the parent marketplace SHA).

- [ ] **Step 7: Commit**

```bash
git add internal/cli/plugin.go
git commit -m "feat(cli): materialize plugin into ~/.claude on install --flavor claude"
```

---

### Task 9: Final verification

- [ ] **Step 1: Full build**

Run: `go build ./...`
Expected: exit 0.

- [ ] **Step 2: `go vet`**

Run: `go vet ./...`
Expected: exit 0.

- [ ] **Step 3: Confirm no stray references**

Run: `grep -rn 'TODO\|FIXME' internal/claudehome internal/claudestate`
Expected: no matches.

- [ ] **Step 4: Smoke roll-back (optional)**

If smoke tests produced state you don't want kept:

```bash
rm -rf ~/.claude/plugins/cache/claude-plugins-official/adobe-for-creativity
```

And manually remove the corresponding entries from `installed_plugins.json` / `enabledPlugins`. (Out of scope: liszt does not have an uninstall command yet.)
