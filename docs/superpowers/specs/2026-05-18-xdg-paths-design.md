# XDG Paths for liszt — Design

**Date:** 2026-05-18
**Status:** Approved (pending user spec review)

## Problem

`cmd/liszt/main.go` hardcodes four paths relative to the current working directory:

```go
const (
    reposFile    = "repos.toml"
    manifestFile = "liszt.toml"
    lockFile     = "liszt.lock"
    cacheDir     = "tmp"
)
```

This conflates two distinct concerns:

- **User-level state** (`repos.toml` registry, cloned marketplace repos) — should be global to the user, not per cwd.
- **Project-level state** (`liszt.toml` manifest, `liszt.lock` lockfile) — correctly per-project, but should not live next to global state.

Running `liszt` from a different directory creates a separate `repos.toml`, re-cloning the same marketplaces. Cache directory `tmp/` pollutes every project root.

## Goals

1. `repos.toml` and clone cache live under XDG-compliant user dirs.
2. `liszt.toml` and `liszt.lock` remain per-project at the cwd.
3. Zero changes to handler signatures in `internal/cli/*` — they already receive a `Paths` struct.
4. Clean break: no migration logic for the previous layout.

## Non-Goals

- Walk-up project discovery (rejected: project files must sit at the cwd).
- Config file under `$XDG_CONFIG_HOME`: no user-facing configuration exists today.
- Per-project override of the global repos registry.
- Automated tests for the `xdg` package (out of scope per user decision).

## Design

### On-disk layout

```
$XDG_DATA_HOME/liszt/        (fallback ~/.local/share/liszt)
  repos.toml                 marketplace repo registry

$XDG_CACHE_HOME/liszt/       (fallback ~/.cache/liszt)
  repos/<name>/              git clones (re-fetchable from repos.toml)

./liszt.toml                 manifest (current working directory)
./liszt.lock                 lockfile (current working directory)
```

**Rationale for the data/cache split:** every clone under `$XDG_CACHE_HOME/liszt/repos/` is reconstructible from the URL + SHA recorded in `repos.toml`. `rm -rf` the cache dir is non-destructive. `repos.toml` is authoritative state and lives under the data dir.

### New package: `internal/xdg`

```go
package xdg

// DataDir returns "$XDG_DATA_HOME/liszt" if XDG_DATA_HOME is set to an
// absolute path, otherwise "~/.local/share/liszt".
func DataDir() string

// CacheDir returns "$XDG_CACHE_HOME/liszt" if XDG_CACHE_HOME is set to an
// absolute path, otherwise "~/.cache/liszt".
func CacheDir() string
```

Resolution rules (both functions):

1. Read the corresponding env var.
2. If non-empty AND absolute path → use `filepath.Join(envVal, "liszt")`.
3. Otherwise → fall back to `filepath.Join(homeDir, defaultRelPath, "liszt")`, where `homeDir` comes from `os.UserHomeDir()`.
4. Relative env vars are ignored (XDG spec mandates absolute paths).

The package does NOT create directories. Mkdir happens lazily at the call sites that perform I/O. Today only `gitx.Clone` (`internal/gitx/git.go:16`) does this. Under the new layout, `repos.Save` must also create its parent directory because `$XDG_DATA_HOME/liszt/` is not guaranteed to exist. `lock.Save` and `manifest.Save` always write to the cwd (parent `.`) but should adopt the same pattern for consistency and to keep future relocations friction-free.

### Wiring

`cmd/liszt/main.go`:

```go
import (
    "path/filepath"
    "github.com/mguilarducci/liszt/internal/xdg"
)

func main() {
    // ...
    paths := cli.Paths{
        Repos:    filepath.Join(xdg.DataDir(), "repos.toml"),
        Manifest: "liszt.toml",
        Lock:     "liszt.lock",
        Cache:    filepath.Join(xdg.CacheDir(), "repos"),
    }
    // ...
}
```

The four hardcoded constants (`reposFile`, `manifestFile`, `lockFile`, `cacheDir`) are deleted. The `cli.Paths` struct is unchanged. Handlers in `internal/cli/*.go` receive resolved absolute paths exactly as before and require no modification.

### Migration

None. The project shipped today (commit `7230a7f`) with no external users. Anyone with a stale `./repos.toml` or `./tmp/` from the previous layout deletes them manually. No detection, no warning, no fallback read.

## File-level changes

| File | Change |
|------|--------|
| `internal/xdg/xdg.go` | NEW: `DataDir`, `CacheDir` functions. |
| `cmd/liszt/main.go` | Remove path constants; build `cli.Paths` via `xdg.DataDir()` / `xdg.CacheDir()`. |
| `internal/repos/repos.go` | `Save` calls `os.MkdirAll(filepath.Dir(path), 0755)` before `WriteFile`. |
| `internal/lock/lock.go` | Same MkdirAll change in `Save` for consistency. |
| `internal/manifest/manifest.go` | Same MkdirAll change in `Save` for consistency. |

No changes to `internal/cli/*` or `internal/gitx/*`.

## Risks

- **Mkdir failure surfacing late.** If the data or cache dir cannot be created (read-only home, permission denied), the first write fails with an I/O error rather than a clean startup error. Acceptable: errors propagate through existing handler error paths and the message includes the path being written.
- **Env var injection.** A user setting `XDG_DATA_HOME` to a relative path silently falls back to the home-dir default. This matches the XDG base directory spec and is documented in the resolution rules.
