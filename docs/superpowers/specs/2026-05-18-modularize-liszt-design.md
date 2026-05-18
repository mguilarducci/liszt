# Modularize liszt — Design

**Date:** 2026-05-18
**Status:** Approved (pre-implementation)
**Scope:** Pure refactor. Zero behavior change.

## Goal

Split the single-file `main.go` (~864 LOC) into a Go-idiomatic `cmd/` + `internal/` layout with small, well-bounded packages. No flag changes, no output changes, no on-disk format changes.

## Non-goals

- Adding tests (covered separately; this refactor is structural only).
- Adding features, flags, or new commands.
- Performance optimization.
- Cleaning up unrelated code smells beyond what the move forces.

## Package tree

```
liszt/
  cmd/liszt/
    main.go              # dispatch + usage (~40 lines)
  internal/
    gitx/
      git.go             # EnsureClone, HeadSHA, LsRemoteHead
      url.go             # ParseGitHubURL, RepoPath
    marketplace/
      marketplace.go     # Marketplace, Plugin, Read, ResolvePluginPath
    repos/
      repos.go           # Config, Entry, Load, Save, Upsert (repos.toml)
    manifest/
      manifest.go        # Config, Entry, Load, Save, Upsert (liszt.toml)
    lock/
      lock.go            # Config, Entry, Load, Save, Upsert (liszt.lock)
    resource/
      resource.go        # Item, Kind, Registry (All, Get)
      walk.go            # walkItems, readFirstWithSource
      skill.go           # listSkills + register
      agent.go           # listAgents + register
      command.go         # listCommands + register
      hook.go            # listHooks + register
      mcp.go             # listMCP + register
      lsp.go             # listLSP + register
    cli/
      repo.go            # Repo (add)
      plugin.go          # PluginList, PluginInstall
      resource.go        # ResourceList, ResourceInstall
      outdated.go        # Outdated
      install.go         # ParseInstallArgs, resolveSlug, recordInstall
      io.go              # printHeader and small print helpers
  go.mod
  go.sum
  liszt.toml / liszt.lock / repos.toml
```

## Dependency direction (no cycles)

```
cmd/liszt        ──▶ internal/cli, internal/resource
internal/cli     ──▶ repos, manifest, lock, resource, marketplace, gitx
internal/resource ──▶ stdlib only
internal/marketplace ──▶ stdlib only
internal/repos
internal/manifest ──▶ pelletier/go-toml/v2
internal/lock
internal/gitx    ──▶ os/exec, net/url
```

## Component contracts

### `internal/gitx`

```go
func EnsureClone(url, dest string) error
func HeadSHA(dir string) (string, error)
func LsRemoteHead(url string) (string, error)
func ParseGitHubURL(raw string) (owner, repo string, err error)
func RepoPath(cacheDir, owner, repo string) string
```

`cacheDir` becomes an argument (was the package-level `cacheDir = "tmp"` constant). `cmd/liszt/main.go` owns the constant and passes it down.

### `internal/marketplace`

```go
type Marketplace struct {
    Name     string
    Plugins  []Plugin
    Metadata struct{ PluginRoot string }
}
type Plugin struct {
    Name, Description, Version string
    Source                     any
}
func Read(repoRoot string) (mp *Marketplace, flavor string, err error) // flavor = "claude" | "copilot"
func (m *Marketplace) ResolvePluginPath(p Plugin) string
```

`pluginSourcePath` becomes an unexported helper in this package.

### `internal/repos`, `internal/manifest`, `internal/lock`

Uniform shape per package:

```go
type Config struct { /* slice field */ }
type Entry struct { /* fields with toml tags */ }
func Load(path string) (*Config, error)
func Save(path string, cfg *Config) error
func (c *Config) Upsert(e Entry)
```

Field tags identical to current code so TOML round-trip is byte-equivalent.

### `internal/resource`

```go
type Item struct{ Slug, Path, Extra string }
type Lister func(pluginRoot string) ([]Item, error)
type Kind   struct { Name string; List Lister }

func All() []Kind            // stable order: skill, agent, command, hook, mcp, lsp
func Get(name string) (Kind, bool)
```

Each `<kind>.go` registers via `init()`. `walkItems` and `readFirstWithSource` stay unexported in `walk.go`.

### `internal/cli`

Handlers take parsed inputs (not `os.Args`) and return `error`. Paths to TOML files are passed in by `main.go`:

```go
type Paths struct { Repos, Manifest, Lock, Cache string }

func Repo(p Paths, args []string) error
func PluginList(p Paths) error
func PluginInstall(p Paths, slug, flavor string) error
func ResourceList(p Paths, kind, pluginName string) error
func ResourceInstall(p Paths, kind, slug, flavor string) error
func Outdated(p Paths) error
```

`parseInstallArgs`, `resolveSlug`, `recordInstall`, the `match` struct, and `printHeader` move here. `usage()` stays in `cmd/liszt/main.go`.

### `cmd/liszt/main.go`

```go
const (
    reposFile    = "repos.toml"
    manifestFile = "liszt.toml"
    lockFile     = "liszt.lock"
    cacheDir     = "tmp"
)

func main() {
    if len(os.Args) < 2 { usage(); os.Exit(2) }
    cmd, args := os.Args[1], os.Args[2:]
    paths := cli.Paths{Repos: reposFile, Manifest: manifestFile, Lock: lockFile, Cache: cacheDir}
    switch cmd {
    case "repo":     run(cli.Repo(paths, args))
    case "plugin":   /* parse list|install, call cli.PluginList / cli.PluginInstall */
    case "outdated": run(cli.Outdated(paths))
    default:
        if _, ok := resource.Get(cmd); ok { /* parse list|install, dispatch */; return }
        usage(); os.Exit(2)
    }
}

func run(err error) {
    if err != nil { fmt.Fprintf(os.Stderr, "error: %v\n", err); os.Exit(1) }
}
```

## Data flow

### `liszt repo add <url>`
```
main → cli.Repo
  gitx.ParseGitHubURL(url) → owner, repo
  gitx.RepoPath(cache, owner, repo) → dest
  gitx.EnsureClone(url, dest)
  gitx.HeadSHA(dest) → sha
  marketplace.Read(dest)                    // warn-only on failure
  repos.Load(p.Repos) → cfg
  cfg.Upsert(Entry{Name, URL, SHA})
  repos.Save(p.Repos, cfg)
```

### `liszt plugin list`
```
cli.PluginList
  repos.Load → iterate
    gitx.ParseGitHubURL, gitx.RepoPath
    marketplace.Read → print
```

### `liszt <kind> list [--plugin X]`
```
cli.ResourceList(kind, pluginName)
  resource.Get(kind) → Kind
  repos.Load → iterate
    marketplace.Read → iterate plugins
      pluginRoot = filepath.Join(root, mp.ResolvePluginPath(p))
      Kind.List(pluginRoot) → print
```

### `liszt <kind|plugin> install <slug> --flavor F`
```
cli.ResourceInstall | cli.PluginInstall
  resolveSlug(kind, slug)            // repos.Load + marketplace.Read + Kind.List
  pick first match (warn if N > 1)
  recordInstall:
    manifest.Load → Upsert → Save
    lock.Load → Upsert → Save
```

### `liszt outdated`
```
cli.Outdated
  lock.Load → entries
  repos.Load → name → URL map
  cache remote SHA per repo via gitx.LsRemoteHead
  diff → print drifts
```

No global state. Each handler reads/writes TOML on demand (same as today).

## Error handling

- `internal/*` packages return `error`. No `os.Exit`, no global `must`.
- `internal/cli` handlers return `error`.
- `cmd/liszt/main.go` has a single `run(err)` that prints `error: %v\n` to stderr and `os.Exit(1)`.
- Usage errors exit with code 2 from `main` directly (before invoking a handler).
- Warnings (non-fatal) keep the **exact** prefixes used today at the same call sites:
  - `cli.Repo`: `warning: %v` when marketplace.json is missing/invalid after clone
  - `cli.Outdated`: `warn: ls-remote %s: %v` per repo when `git ls-remote` fails
  - `cli.ResourceList`: `skip %s: %v` per repo with invalid URL or missing marketplace
  - `cli.ResourceInstall`: `note: %d sources for %q, picking %s:%s (%s); qualify as <plugin>:%s to override` on multi-match
- Error wrapping (`fmt.Errorf("parse %s: %w", path, err)`) preserved as in current code.

## Behavioral parity (verification)

This is a pure refactor. Verification = output diff between old binary and new binary.

### Baseline (before refactor)

```
git checkout main
go build -o /tmp/liszt-old .
mkdir /tmp/liszt-baseline && cd /tmp/liszt-baseline
/tmp/liszt-old repo add <url>
/tmp/liszt-old plugin list        > /tmp/old-plugin-list  2> /tmp/old-plugin-list.err
/tmp/liszt-old skill list         > /tmp/old-skill-list   2> /tmp/old-skill-list.err
/tmp/liszt-old agent list         > /tmp/old-agent-list   2> /tmp/old-agent-list.err
/tmp/liszt-old command list       > /tmp/old-command-list 2> /tmp/old-command-list.err
/tmp/liszt-old hook list          > /tmp/old-hook-list    2> /tmp/old-hook-list.err
/tmp/liszt-old mcp list           > /tmp/old-mcp-list     2> /tmp/old-mcp-list.err
/tmp/liszt-old lsp list           > /tmp/old-lsp-list     2> /tmp/old-lsp-list.err
/tmp/liszt-old skill install <slug> --flavor claude
cp liszt.toml /tmp/old-liszt.toml
cp liszt.lock /tmp/old-liszt.lock
/tmp/liszt-old outdated           > /tmp/old-outdated     2> /tmp/old-outdated.err
```

### After refactor

```
go build -o /tmp/liszt-new ./cmd/liszt
mkdir /tmp/liszt-new-run && cd /tmp/liszt-new-run
# replay identical sequence
# diff each pair
```

### Success criteria

- `go build ./...` and `go vet ./...` clean.
- All stdout diffs empty.
- `liszt.toml` and `liszt.lock` diffs empty (byte-equal).
- Exit codes match across all invocations.
- Stderr diffs empty modulo:
  - Map-iteration ordering for `hook list`, `mcp list`, `lsp list` (already non-deterministic upstream).
  - Multiple-match warnings for `install` (also map-order-dependent today).

### Known risks

- TOML field-tag changes would break byte parity. Mitigation: keep tags identical when types move into new packages.
- Renaming `match` to exported `Match` (or keeping it unexported in `cli`) does not affect output.

## Out of scope

- `internal/resource` registry order: today implicit via map literal in `var kinds = map[string]kindDef{...}`. New design uses explicit `All()` returning a stable slice. This is a deliberate, observable improvement (deterministic order in any future caller) but does NOT change current observable behavior: today only `Get(cmd)` is used at the CLI boundary, never iteration over `kinds`.
- Test infrastructure. Adding tests is a follow-up plan once boundaries land.
