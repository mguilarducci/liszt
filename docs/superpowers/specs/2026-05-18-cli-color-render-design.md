# CLI Color & Render Layer — Design

**Date:** 2026-05-18
**Branch:** `feat/cli-color-render`
**Status:** Draft for user review

## 1. Goal

Bring color and styled output to the `liszt` CLI by adopting `charmbracelet/fang`
for help/version/error rendering and porting the `render` package from
`liszt-bkp` for styled message output and progress bars. Migrate the CLI
structure from manual `os.Args` parsing to `spf13/cobra` (a prerequisite for
`fang`).

## 2. Non-goals

- Animated spinners (only progress bars, per user decision).
- Light-mode adaptive palette (Gleam palette is dark-tuned and kept as-is).
- Interactive TUI (no `bubbletea`).
- Alternate or user-configurable color themes.
- Refactoring of unrelated packages (`marketplace`, `manifest`, `lock`, etc.)
  beyond minimal seams needed for the render wiring.

## 3. Architecture

Two new layers plus a structural migration of the existing CLI surface:

1. **`internal/cli/`** — rewritten on top of `spf13/cobra`. A single
   `rootCmd` owns all subcommands (`repo add`, `plugin list`, `plugin install`,
   `<kind> list`, `<kind> install`, `outdated`). Entry point becomes
   `cli.Execute(ctx)`, which delegates to `fang.Execute(ctx, rootCmd)` so fang
   can style help, version, and error output.
2. **`internal/render/`** — new package, ported from `liszt-bkp/internal/render`
   minus the spinner files. Provides a `Renderer` with TTY/color-profile
   detection, top-level convenience functions (`render.Info`, `render.Done`,
   `render.Warn`, `render.Error`, `render.Header`, `render.Hint`,
   `render.Bar`), and a `Bar` type that supports both determinate and
   indeterminate modes.
3. **`cmd/liszt/main.go`** — shrinks to a 5-line entry point that calls
   `cli.Execute(context.Background())` and exits with status 1 on error.

CLI handlers (RunE functions) call into `render.*` for user-facing output and
into the existing `cli.PluginInstall`, `cli.ResourceList`, etc., as
business-logic helpers. Errors returned by handlers bubble up to `fang`, which
renders them in red with file/line context.

```
cmd/liszt/main.go
        └── cli.Execute(ctx)
                └── fang.Execute(ctx, rootCmd)
                        └── cobra dispatches to subcommand RunE
                                ├── render.Bar / render.Info / render.Done / ...
                                └── cli.PluginInstall / cli.ResourceList / gitx / ...
```

## 4. Palette (Gleam)

Identical to `liszt-bkp/internal/render/theme.go`:

| Name          | Hex       | Use                                          |
|---------------|-----------|----------------------------------------------|
| `cPinkDeep`   | `#fe7ab2` | H2 headers                                   |
| `cPinkBright` | `#ffaff3` | Bar cells, accents                           |
| `cInfo`       | `#9ce7ff` | Info bar/label                               |
| `cDone`       | `#aadd8b` | Success bar/label                            |
| `cWarn`       | `#ffc501` | Warning bar/label                            |
| `cError`      | `#f44747` | Error bar/label                              |
| `cDim`        | `#c4c4c4` | Metadata, percentages, hints                 |

Pre-built styles:

```
styH1   = Bold + Underline                  // no color, terminal default
styH2   = Foreground(cPinkDeep) + Bold
styH3   = Foreground(cDim) + Italic
styDim  = Foreground(cDim)
styPink = Foreground(cPinkBright) + Bold
styInfoBar/Lbl   = cInfo (+ Bold on Lbl)
styDoneBar/Lbl   = cDone (+ Bold)
styWarnBar/Lbl   = cWarn (+ Bold)
styErrorBar/Lbl  = cError (+ Bold)
```

Labels are 5-cell padded (`"info "`, `"done "`, `"warn "`, `"error"`) so
multi-line messages align under the prefix.

Message format:

```
▌ <label>  <msg>  key=value key=value …
```

Key/value pairs render in `styDim`. Bar format:

```
▌ info  <24-cells>  XXX%  <label>
```

Indeterminate mode replaces `XXX%` with `····` (dim dots) and never advances the
fill; only the random-note flicker animates.

## 5. Package `internal/render/`

Files:

| File           | Purpose                                                                                  |
|----------------|------------------------------------------------------------------------------------------|
| `theme.go`     | Palette + pre-built `lipgloss.Style` values + label constants.                            |
| `detect.go`    | `detectProfile(w, env)` via `colorprofile.Detect` + `mattn/go-isatty`; honors `NO_COLOR`. |
| `renderer.go`  | `Renderer{w, profile, isTTY, mu, rng, active}`; `New(w, ...Option)`; `writeString`, `eraseLine`. |
| `default.go`   | Lazy global `Default` + top-level functions (`Info`, `Done`, `Warn`, `Error`, `Header`, `Hint`, `Bar`). |
| `message.go`   | `Info`, `Done`, `Warn`, `Error` (on `Renderer`); kv pair formatting.                      |
| `header.go`    | `Header` (H1), `Subheader` (H2), `Hint` (H3 dim italic).                                  |
| `bar.go`       | `Bar` with both determinate (`Set(pct)`) and indeterminate (`SetIndeterminate(bool)`) modes. |

Options:

| Option          | Effect                                                          |
|-----------------|------------------------------------------------------------------|
| `WithProfile`   | Override auto-detected `colorprofile.Profile`.                  |
| `WithNoColor`   | Force `colorprofile.NoTTY` (strip ANSI).                        |
| `WithTTY(bool)` | Override auto-detected TTY status (test-only).                  |
| `WithRand`      | Inject deterministic RNG for bar flicker (test-only).           |

Concurrency: `Renderer.mu` serializes all writes. `Bar` uses `atomic.Value`
(label), `atomic.Uint64` (pct via `math.Float64bits`), and two `atomic.Bool`
flags (`stopped`, `loopActive`). `repaint()` reads a snapshot under the
renderer mutex and re-renders the active animation after any other write
clears its line.

### 5.1 Bar indeterminate mode

`Bar.SetIndeterminate(true)` flips a flag (atomic). In indeterminate mode
`repaint()`:

- Renders all 24 cells with the flicker animation (random note from
  `{♬, ♪, ♩, ♫}` in `styPink`, re-rolled every 100 ms tick) so the bar looks
  alive.
- Renders the fill portion (`int(pct * width)`) as 0 — i.e. all cells use the
  "filled" flicker treatment because the bar's purpose is liveness, not
  progress.
- Replaces the `XXX%` segment with `····` (4 dim dots) so the absence of a
  percentage is visually deliberate, not a bug.

`SetIndeterminate(false)` returns to standard determinate behavior; `Set(pct)`
calls are silently respected in either mode but only displayed when
determinate.

## 6. Package `internal/cli/` (cobra migration)

Files:

| File             | Purpose                                                                            |
|------------------|------------------------------------------------------------------------------------|
| `root.go`        | `rootCmd` (`Use: "liszt"`, `Version`, `SilenceUsage`, `SilenceErrors`); `--no-color` persistent flag; `Execute(ctx)` calling `fang.Execute`. |
| `repo.go`        | `repoCmd` parent + `repoAddCmd` (`liszt repo add <url>`).                          |
| `plugin.go`      | `pluginCmd` + `pluginListCmd` + `pluginInstallCmd` (with `--flavor`).              |
| `resource.go`    | Loops over `resource.Kinds()` and registers `<kind>Cmd` + `<kind>ListCmd` (with `--plugin`) + `<kind>InstallCmd` (with `--flavor`) per kind. |
| `outdated.go`    | `outdatedCmd`.                                                                     |
| `install.go`     | Existing helper, preserved.                                                        |
| `io.go`          | Existing helper, preserved.                                                        |
| `paths.go`       | Existing helper, preserved; `Paths` resolved once in `PersistentPreRunE`.          |

The `--no-color` flag's `PersistentPreRun` calls `os.Setenv("NO_COLOR", "1")`
before the lazy `render.Default` initializes. RunE handlers return `error`;
fang renders them styled in stderr.

## 7. Version package

A new `internal/version/` package exposes:

```go
func Version() string  // semver, set via -ldflags
func Full() string     // "liszt vX.Y.Z (commit, date)"
```

`.goreleaser.yaml` already exists; ldflags will be wired to inject
`Version`, `Commit`, and `Date` symbols. `cobra.Command.Version` is set to
`version.Full()` so fang's `--version` renders the full string.

## 8. Wire bar into verbs

### 8.1 `repo add <url>`

```go
bar := render.Bar("cloning " + url)
bar.SetIndeterminate(true)
err := gitx.EnsureClone(url, dest)
if err != nil {
    bar.Fail("clone failed", "url", url, "err", err)
    return err
}
bar.Done("repo added", "url", url)
```

### 8.2 `plugin list` / `<kind> list`

No bar (filesystem walk is fast). Use:

```go
render.Header("Plugins")
for _, p := range plugins { fmt.Println(p) }   // tabular output
render.Hint(fmt.Sprintf("%d plugins across %d repos", len(plugins), len(repos)))
```

### 8.3 `plugin install <slug>` / `<kind> install <slug>`

Mixed determinate/indeterminate progression. Quick steps drive `Set(pct)`;
the clone step (the only opaque long operation) flips the bar to
indeterminate so the user does not see the bar stall at a fixed percentage
during the slowest phase.

```go
bar := render.Bar("installing " + slug)
bar.Set(0.0)
bar.Update("resolving " + slug)
// resolve marketplace entry
bar.Set(0.25)

bar.Update("cloning " + slug)
bar.SetIndeterminate(true)
// clone or cache lookup (opaque, slow)
bar.SetIndeterminate(false)
bar.Set(0.50)

bar.Update("materializing " + slug)
// materialize into Claude/Copilot tree
bar.Set(0.75)

bar.Update("writing manifest")
// update settings + lock
bar.Set(1.0)

bar.Done("installed", "slug", slug, "flavor", flavor)
```

If the cache hits and the clone step is skipped, the indeterminate flip is
also skipped and the bar advances directly from 0.25 → 0.50.

### 8.4 `outdated`

Per-repo determinate progress:

```go
bar := render.Bar("checking remotes")
for i, r := range repos {
    bar.Update(r.URL)
    sha, err := gitx.LsRemoteHead(r.URL)
    if err != nil {
        bar.Fail("ls-remote failed", "url", r.URL, "err", err)
        return err
    }
    bar.Set(float64(i+1) / float64(len(repos)))
}
bar.Done("checked", "repos", len(repos), "stale", staleCount)
for _, s := range stale {
    render.Info("stale", "url", s.URL, "local", s.Local, "remote", s.Remote)
}
```

### 8.5 `gitx` writer plumbing

`gitx.EnsureClone` and `gitx.CloneAtSHA` currently send `git` subprocess
stdout/stderr to `os.Stderr`. To avoid corrupting an active progress bar, both
will accept an `io.Writer` (or a package-level `gitx.SetOutput(io.Writer)`),
and the CLI handlers will pass `io.Discard` when a bar is active. We will
choose between the parameter form and the package-level setter during
implementation planning; the parameter form is preferred because it is
thread-safe and explicit.

## 9. Non-TTY behavior

Triggered when any of: `!isTTY`, `NO_COLOR=1`, `--no-color` flag.

- `colorprofile.NoTTY` strips ANSI in `Renderer.writeString`.
- `Bar` prints a single `info <label>` line on creation and a single
  `done <msg>` (or `error <msg>`) line on `Done`/`Fail`. No cursor-control
  sequences, no animation loop.
- Headers, info, warn, and error lines lose color but keep their label and
  body text.

`mattn/go-isatty` handles TTY detection cross-platform.

## 10. Error handling

- Handler `RunE` returns `error` → `fang` styles the error in stderr (red,
  with command path and suggested usage).
- Pre-bar errors: `render.Error(msg, kv...)` then `return err`.
- Mid-bar errors: `bar.Fail(msg, "err", err)` clears the bar, prints the
  error line, then `return err`.
- The `Renderer.mu` mutex guarantees that a `render.Info` call during an
  active bar will (a) erase the bar line, (b) write the message, and
  (c) trigger `Renderer.active.repaint()` so the bar redraws beneath the
  newly printed line.

## 11. Testing

Memory: 100% line + flow coverage, with inline-justified exceptions. The
current `liszt` repo has zero tests; this work adds the full test scaffolding
needed for the new and migrated code.

### 11.1 `internal/render/`

| File                  | Coverage                                                                                       |
|-----------------------|------------------------------------------------------------------------------------------------|
| `theme_test.go`       | Palette and label constants stable; styles bind expected foregrounds.                          |
| `detect_test.go`      | Table-driven: `NO_COLOR`, `TERM=dumb`, `FORCE_COLOR`, TTY on/off → expected `colorprofile.Profile`. |
| `renderer_test.go`    | `New` applies options in correct order; `writeString` strips ANSI under `NoTTY`; `eraseLine` bypasses profile. |
| `message_test.go`     | `Info/Done/Warn/Error` emit expected bytes (autogold golden snapshots).                        |
| `header_test.go`      | `Header/Subheader/Hint` styling.                                                               |
| `bar_test.go`         | Determinate `Set` clamps `[0,1]`; `Done` prints success; indeterminate flicker without %; non-TTY single line; mid-bar `Info` interrupt → bar repaints below new line. |

Determinism for bar tests:
- `WithRand(rand.New(rand.NewPCG(seed, seed)))` makes flicker output stable.
- `WithTTY(true)` forces the animation path on non-TTY test writers.
- Tests invoke `repaint()` directly rather than running the ticker loop.

### 11.2 `internal/cli/`

| File                   | Coverage                                                                                  |
|------------------------|-------------------------------------------------------------------------------------------|
| `root_test.go`         | `Execute` smoke; `--version`, `--help`, `--no-color` flag wires `NO_COLOR`.               |
| `repo_test.go`         | `repo add` happy path + error; reads/writes `repos.toml` in `t.TempDir()`.                |
| `plugin_test.go`       | `plugin list`, `plugin install` with fake marketplace + fake `gitx`.                      |
| `resource_test.go`     | Loop registration produces all six kinds; `list`/`install` per kind.                      |
| `outdated_test.go`     | Mocked `LsRemoteHead`; verifies stale-count summary + per-stale `render.Info`.            |

`gitx` gets an interface seam (or `gitx.SetOutput`) so handler tests can mock
git calls without spawning a real `git` process.

### 11.3 `cmd/liszt/script_test.go`

Ported from `liszt-bkp`. End-to-end via `rogpeppe/go-internal/testscript`.
Golden output captured with `NO_COLOR=1` set in the script environment so ANSI
sequences do not pollute the goldens.

### 11.4 Coverage exceptions (inline-justified)

- `cmd/liszt/main.go` — binary entry point; covered by testscript only.
- `Setenv` failure branch — only fails on unsupported platforms; harmless.

## 12. Dependencies (validated against module proxy on 2026-05-18)

All pinned to the latest stable release; no `alpha`/`beta`/`unstable`/`internal`
subpackages.

| Module                                       | Version    | Notes                                         |
|----------------------------------------------|------------|-----------------------------------------------|
| `github.com/charmbracelet/fang`              | `v1.0.0`   | Latest; first v1 release.                     |
| `github.com/spf13/cobra`                     | `v1.10.2`  | Latest.                                       |
| `charm.land/lipgloss/v2`                     | `v2.0.3`   | Latest stable; skip betas.                    |
| `github.com/charmbracelet/colorprofile`      | `v0.4.3`   | Latest; v0.x but stable public API.           |
| `github.com/mattn/go-isatty`                 | `v0.0.22`  | Latest.                                       |
| `github.com/hexops/autogold/v2`              | `v2.3.1`   | Test-only; golden snapshots.                  |
| `github.com/rogpeppe/go-internal`            | `v1.14.1`  | Test-only; `testscript`.                      |
| `github.com/google/go-cmp`                   | `v0.7.0`   | Test-only.                                    |

Existing `github.com/pelletier/go-toml/v2 v2.3.1` stays.

`go.mod` will declare `go 1.26.3` (matching the project's `.tool-versions`).
Indirects come in via `go mod tidy`.

## 13. Delivery order (commits on `feat/cli-color-render`)

1. **deps + version package** — `go.mod` additions, new `internal/version/`
   (`Version`, `Commit`, `Date`, `Full`), goreleaser ldflags wired.
2. **render core** — `theme.go`, `detect.go`, `renderer.go`, `default.go` and
   their tests. No consumers yet.
3. **render messages** — `message.go`, `header.go` and their tests.
4. **render bar** — `bar.go` (determinate + indeterminate) and tests.
5. **cobra root + fang** — `root.go`, `version.go`; `cmd/liszt/main.go`
   shrinks to delegation. Verbs temporarily broken (intentional, single PR).
6. **cobra verbs** — `repo.go`, `plugin.go`, `resource.go`, `outdated.go`;
   reuse existing `cli.PluginInstall`, `cli.ResourceList`, etc., as helpers.
7. **wire bar into verbs** — `repo`, `install`, `outdated` adopt
   `render.Bar`; `gitx` gains writer plumbing.
8. **testscript end-to-end** — `cmd/liszt/script_test.go` plus
   `testdata/script/*.txt`.

## 14. Risks

- **`charm.land/lipgloss/v2` module path**: `charm.land` is a redirect to
  `charmbracelet/lipgloss/v2`. Confirmed working in `liszt-bkp/go.mod`.
- **`colorprofile` API surface**: v0.4.x is pre-1.0; treat the API as fixed at
  the pinned version. Upgrades require explicit review.
- **Bar mid-print correctness**: every `fmt.Print*` in handlers must route
  through `render` or use `io.Discard` while a bar is active. Risk of leftover
  raw prints corrupting the bar line; the testscript suite is the safety net.
- **`gitx` writer plumbing**: must be threaded through both `EnsureClone` and
  `CloneAtSHA`; missing one path will leak raw git output during install.

## 15. Out of scope (explicit)

- Spinner animations.
- Adaptive light/dark palette.
- Configurable user themes.
- Interactive TUI (no `bubbletea`).
- Refactor of `marketplace`, `manifest`, `lock`, `claudestate`, or other
  non-CLI packages beyond the `gitx` writer seam.
