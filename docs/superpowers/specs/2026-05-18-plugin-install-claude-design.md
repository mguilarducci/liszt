# Real Plugin Install into Claude Code — Design

**Date:** 2026-05-18
**Status:** Approved (pending user spec review)

## Problem

`liszt plugin install <slug> --flavor claude` currently only records the install in `./liszt.toml` (manifest) and `./liszt.lock`. It does not place any files in `~/.claude/plugins/` and does not update Claude Code's own plugin registry. The plugin never actually loads in Claude.

Claude Code maintains its plugin state under `~/.claude/plugins/`:

| File / dir | Purpose |
|---|---|
| `~/.claude/plugins/known_marketplaces.json` | Registered marketplaces (`{<name>: {source, installLocation, lastUpdated}}`). |
| `~/.claude/plugins/marketplaces/<name>/` | Cloned marketplace repo (Claude runs `git pull` here for `/plugin update`). |
| `~/.claude/plugins/installed_plugins.json` | Installed plugin registry, schema v2. |
| `~/.claude/plugins/cache/<mp>/<plugin>/<version>/` | Plugin file tree Claude loads at session start. |
| `~/.claude/settings.json` `enabledPlugins` | `{"<plugin>@<mp>": bool}` enable map. |
| `~/.claude/plugins/cache/<mp>/<plugin>/<version>/.in_use/<pid>` | Claude-internal session markers. Liszt must ignore. |

Liszt must produce this state to make a plugin actually run.

## Goals

1. `liszt plugin install <slug> --flavor claude` materializes the plugin into `~/.claude/plugins/` and enables it.
2. Idempotent: re-running with the same SHA is a no-op for filesystem writes.
3. Liszt's own state (`repos.toml`, XDG cache, `liszt.toml`, `liszt.lock`) stays the canonical source for `liszt repo add` / `liszt outdated`; Claude state is a derived artifact.
4. Output the existing one-line confirmation (`installed plugin ...`).

## Non-Goals

- `liszt plugin uninstall` / `update`.
- `--flavor copilot` (continues to record-only as today).
- `liszt skill / agent / command / hook / mcp / lsp install` (Claude treats these as parts of plugins, not independently installable; out of scope).
- Tests (per user decision; verification is `go build` + smoke install).
- Migrating plugins already installed by Claude Code itself into liszt's manifest.

## Architecture

```
liszt plugin install <slug> --flavor claude
  │
  ├─ Liszt discovery (existing): repos.toml + XDG marketplace cache
  │     → marketplace name (from marketplace.json), plugin entry, source spec, SHA
  │
  ├─ Source materialization (new):
  │     string source        → use subdir inside marketplace clone
  │     git-subdir source    → shallow clone external repo at pinned SHA into liszt XDG cache
  │
  ├─ Claude state writes (new):
  │     1. copy source tree   → ~/.claude/plugins/cache/<mp>/<plugin>/<version>/
  │     2. upsert             → ~/.claude/plugins/installed_plugins.json
  │     3. upsert + symlink   → ~/.claude/plugins/known_marketplaces.json
  │                             ~/.claude/plugins/marketplaces/<mp> → liszt XDG clone
  │     4. enable             → ~/.claude/settings.json enabledPlugins[<plugin>@<mp>] = true
  │
  └─ Liszt manifest + lock (existing recordInstall, adapted)
```

## New Packages

### `internal/claudehome`

```go
package claudehome

// Dir returns $CLAUDE_HOME if set and absolute, otherwise ~/.claude.
func Dir() string
```

Resolution mirrors `internal/xdg`: env var if set and absolute, else `os.UserHomeDir() + "/.claude"`. Does not create the directory.

### `internal/claudestate`

Four small files, each with a single concern.

**`installed.go`** — `installed_plugins.json` (schema v2):

```go
type InstalledPlugin struct {
    Scope        string `json:"scope"`
    InstallPath  string `json:"installPath"`
    Version      string `json:"version"`
    InstalledAt  string `json:"installedAt"`  // RFC3339
    LastUpdated  string `json:"lastUpdated"`
    GitCommitSha string `json:"gitCommitSha"`
}

type InstalledPlugins struct {
    Version int                            `json:"version"` // always 2
    Plugins map[string][]InstalledPlugin   `json:"plugins"` // key: "<plugin>@<marketplace>"
}

func LoadInstalled(path string) (*InstalledPlugins, error)
func SaveInstalled(path string, ip *InstalledPlugins) error
// Upsert appends or replaces the (scope=user) entry for key.
// On replace: preserves InstalledAt; sets LastUpdated, GitCommitSha, InstallPath, Version.
func (ip *InstalledPlugins) Upsert(key string, e InstalledPlugin)
```

Initialised with `Version=2`, `Plugins={}` when file is missing.

**`known.go`** — `known_marketplaces.json`:

```go
type MarketplaceSource struct {
    Source string `json:"source"` // "github"
    Repo   string `json:"repo"`   // "owner/name"
}

type KnownMarketplace struct {
    Source          MarketplaceSource `json:"source"`
    InstallLocation string            `json:"installLocation"`
    LastUpdated     string            `json:"lastUpdated"`
}

type KnownMarketplaces map[string]KnownMarketplace

func LoadKnown(path string) (KnownMarketplaces, error)
func SaveKnown(path string, km KnownMarketplaces) error
```

`UpsertMarketplace(km, name, src, installLoc)` adds if missing; if present with different `Source`, returns conflict error.

**`settings.go`** — minimal mutation of `~/.claude/settings.json`:

```go
// EnableSettingPlugin reads settings.json (as map[string]any), sets
// enabledPlugins[key] = true, writes back with 2-space indent.
// Creates the file if missing with {"enabledPlugins": {key: true}}.
// Creates the "enabledPlugins" sub-object if missing.
func EnableSettingPlugin(path, key string) error
```

Top-level keys are re-sorted alphabetically by `encoding/json` (matches Claude's own behaviour).

**`cache.go`** — copy source tree into Claude cache:

```go
// MaterializePlugin copies srcDir contents into
// claudeHome/plugins/cache/<mp>/<plugin>/<version>/.
// Uses os.CopyFS (Go 1.23+). Creates parents. Overwrites existing files.
// Skips ".in_use" directories at any depth (Claude-internal).
func MaterializePlugin(claudeHome, mp, plugin, version, srcDir string) (installPath string, err error)
```

## Extensions to Existing Packages

### `internal/marketplace`

Add typed source variants. Today `Plugin.Source any` is decoded ad-hoc by `pluginSourcePath`. Replace with:

```go
type PluginSource struct {
    // String form: "./plugins/foo" (relative to marketplace root).
    Subdir string

    // Object form: {"source":"git-subdir","url","path","ref","sha"}.
    External *ExternalSource
}

type ExternalSource struct {
    URL  string
    Path string
    Ref  string
    SHA  string
}

// ParseSource decodes plug.Source into PluginSource. Returns Subdir
// when the JSON value is a string or {"path": "..."}. Returns External
// when {"source":"git-subdir", ...}.
func ParseSource(raw any) (PluginSource, error)
```

`ResolvePluginPath` continues to exist for the string/subdir case (used by listers).

### `internal/gitx`

Add:

```go
// CloneAtSHA shallow-clones url into dest and checks out sha.
// If dest already exists and HEAD == sha, returns nil (idempotent).
func CloneAtSHA(url, sha, dest string) error
```

Implementation: `git init` + `git remote add` + `git fetch --depth=1 <sha>` + `git checkout FETCH_HEAD`. Falls back to full clone if shallow fetch by SHA is unsupported by the remote.

## Plugin Install Flow

The new orchestration lives in `internal/cli/install.go`, replacing the body of `PluginInstall` for `--flavor claude`. Copilot path remains record-only.

1. **Discover** (existing logic): scan repos, find `plug` matching `slug`, capture `mp` (marketplace.json), `repo.URL`, `repo.SHA`.
2. **Marketplace name**: `mpName := mp.Name` (from `marketplace.json`).
3. **Parse source**: `src, err := marketplace.ParseSource(plug.Source)`.
4. **Materialise source dir**:
   - `src.Subdir != ""`: `srcDir = filepath.Join(gitx.RepoPath(p.Cache, owner, repo), src.Subdir)`; `srcSha = repo.SHA`.
   - `src.External != nil`: parse `extOwner, extRepo` from `src.External.URL` via `gitx.ParseGitHubURL`; call `gitx.CloneAtSHA(src.External.URL, src.External.SHA, gitx.RepoPath(p.Cache, extOwner, extRepo))`; `srcDir = filepath.Join(<clone>, src.External.Path)`; `srcSha = src.External.SHA`.
5. **Version**: `version := plug.Version; if version == "" { version = "unknown" }`.
6. **Idempotency check**: load `installed_plugins.json`; if `Plugins["<plugin>@<mp>"]` contains an entry with `scope == "user"` and `gitCommitSha == srcSha`, skip steps 7 and 8 (still run 9–11).
7. **Copy**: `installPath, err := claudestate.MaterializePlugin(claudeHome, mpName, plug.Name, version, srcDir)`.
8. **Upsert installed**: build `InstalledPlugin{scope="user", installPath, version, installedAt=<preserved or now>, lastUpdated=now, gitCommitSha=srcSha}`; save.
9. **Upsert known marketplace**:
   - Compute `owner/repo` from `repo.URL` (existing `gitx.ParseGitHubURL`).
   - `installLocation = ~/.claude/plugins/marketplaces/<mpName>`.
   - If entry missing → add and `os.Symlink(~/.cache/liszt/repos/<owner>/<repo>, installLocation)`.
   - If entry present with same source → no-op.
   - If entry present with different source → return conflict error.
10. **Enable**: `claudestate.EnableSettingPlugin(~/.claude/settings.json, "<plugin>@<mp>")`.
11. **Liszt manifest + lock**: existing `recordInstall` writes `liszt.toml` + `liszt.lock` (unchanged).

## Errors & Edge Cases

- **No repos in `repos.toml`** → existing message "no repos. add one with: liszt repo add <url>".
- **Slug not found** → existing "plugin %q not found", exit 1.
- **`git-subdir` clone failure** → propagate error; partial clone dir is removed by `gitx.CloneAtSHA` (clone into temp dir, atomic rename on success).
- **Same SHA already installed** → skip materialise + installed-upsert; still run manifest + lock + enable.
- **Different SHA already installed** → overwrite `installPath` directory; preserve `installedAt`; update `lastUpdated` and `gitCommitSha`.
- **`known_marketplaces.json` missing** → create with one entry.
- **`settings.json` missing** → create `{"enabledPlugins": {key: true}}` with 2-space indent.
- **`enabledPlugins` missing** → add sub-map.
- **Marketplace name conflict** (`mpName` already in `known_marketplaces.json` with a different `Source.Repo`) → return error `marketplace name %q already registered with a different source; resolve manually`, no partial writes.
- **Symlink target exists pointing elsewhere** → treated as same conflict above.
- **Write permission on `~/.claude/`** → stdlib error propagates with path context.
- **`.in_use/` directory present in source** (shouldn't happen for marketplace subdirs but guarded anyway) → skipped during copy.

## File-Level Changes

| File | Action |
|------|--------|
| `internal/claudehome/home.go` | NEW. |
| `internal/claudestate/installed.go` | NEW. |
| `internal/claudestate/known.go` | NEW. |
| `internal/claudestate/settings.go` | NEW. |
| `internal/claudestate/cache.go` | NEW. |
| `internal/marketplace/marketplace.go` | Add `PluginSource`, `ExternalSource`, `ParseSource`. Keep `ResolvePluginPath`. |
| `internal/gitx/git.go` | Add `CloneAtSHA`. |
| `internal/cli/install.go` | Add `claudeInstall(...)` orchestrator and switch `PluginInstall` to call it on `--flavor claude`. |
| `cmd/liszt/main.go` | No change. |

## Risks

- **`settings.json` reorder.** `encoding/json` sorts map keys alphabetically on marshal. Acceptable: Claude itself produces alphabetised output today; users running `/plugin install` already accept this. Documented in the design.
- **Shallow fetch by SHA may be refused** by some Git servers. `CloneAtSHA` falls back to full clone + checkout, so correctness holds; the cost is extra bandwidth on those repos.
- **Symlink portability.** macOS + Linux both support `os.Symlink`. Windows is not a supported target for `liszt` today.
- **Manifest/lock divergence from Claude state.** Liszt writes both. If a user mutates Claude's `installed_plugins.json` directly (e.g. via `/plugin uninstall`) between liszt invocations, liszt's lock and Claude's registry can disagree. Out of scope to reconcile here; `liszt outdated` or a future `liszt status` can surface this.
