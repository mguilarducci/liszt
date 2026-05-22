# Trim liszt to `run` + reduce render to palette — Design

Date: 2026-05-22

## Goal

Strip liszt down to the `run` command. Remove every other command and its
now-orphaned support code. As a consequence, the `render` package collapses to
a color palette, because nothing animated or message-styled has a caller
anymore.

No backward-compatibility is required (no external users).

## Surviving surface

- `liszt run <name>` — run a named command group from `.liszt/liszt.toml`.
- `liszt version` — print the build identifier.
- bare `liszt` — print help only (no intro animation).
- flags: `--no-color` (root), `--version` / `--help` (fang built-ins).

## Removed commands

`outdated`, `repo add`, `plugin list`, `plugin install`, and the six resource
kinds (`command`, `skill`, `hook`, `agent`, `mcp`, `lsp`) with their
`list` / `install` subcommands.

Also removed: the intro animation on bare `liszt`, and the `--verbose` flag
(it only gated `render.Detail`, which no longer exists).

## Part A — command trim

### Delete (cli files, `internal/cli/`)

`outdated_cmd.go` · `outdated.go` · `outdated_cmd_test.go` ·
`repo_cmd.go` · `repo.go` · `repo_test.go` · `repo_cmd_test.go` ·
`plugin_cmd.go` · `plugin.go` · `plugin_cmd_test.go` ·
`resource_cmd.go` · `resource.go` · `resource_cmd_test.go` ·
`install.go` · `io.go` (orphaned `printHeader`) · `paths.go` (orphaned `Paths`).

### Delete (whole internal packages)

`intro/` · `resource/` · `marketplace/` · `repos/` · `gitx/` ·
`claudehome/` · `claudestate/` · `lock/` · `manifest/` · `xdg/`.

Verified: none of these is imported by a surviving package. The only Go
consumers are the dying `internal/cli` handler files and `root.go`'s `intro`
import (removed below). `docs/` references are historical and stay.

### Survive (internal)

`runner/` (the `run` engine; imports no other internal package) ·
`version/` · `render/` (reduced — see Part B).

### testscript (`cmd/liszt/testdata/script/`)

- Delete `repo_add_duplicate.txtar` (repo removed).
- Delete `verbose.txtar` (drove `plugin list`; `--verbose` is being removed).
- Edit `help.txtar`: drop the `repo` / `plugin` / `outdated` assertions; assert
  the help output still names `liszt`, `run`, and `version`.
- Keep `run.txtar`, `version.txtar`, `no_color.txtar` (uses `version`).

## Part B — reduce render + drop flags

Post-trim, `root.go` is render's only consumer and uses only `render.Palette.*`
(in `gleamColorScheme`) plus `render.SetVerbose` (which goes with `--verbose`).
So everything except the palette is dead. The animated bar/spinner machinery
(`active` / `eraseLine` / `repaint`) is also coupled into the message verbs
(`step.go`, `header.go`, `detail.go`), so a partial cut is not clean — the whole
verb layer goes.

### Delete (render files + tests)

`bar.go` · `bar_test.go` · `progress.go` · `progress_test.go` ·
`step.go` · `step_test.go` · `header.go` · `header_test.go` ·
`detail.go` · `detail_test.go` · `message.go` · `message_test.go` ·
`renderer.go` · `renderer_test.go` · `default.go` · `default_test.go` ·
`detect.go` · `detect_test.go`.

This removes: the `Renderer` type, `New`, all options (`WithProfile`,
`WithNoColor`, `WithTTY`, `WithRand`), `rng`, `active`/`anim`, `eraseLine`,
`writeString`, `detectProfile`, every message verb (`Step`, `StepDone`,
`StepFail`, `Done`, `Fail`, `Warn`, `Header`, `Subheader`, `Hint`, `Detail`),
`SetVerbose`, `NewBar`, `NewProgress`, `Bar`, `Progress`, and the musical-note
spinner.

### Keep + trim

- `theme.go`: keep the color vars (`cPinkDeep`, `cPinkBright`, `cInfo`,
  `cDone`, `cWarn`, `cError`, `cDim`) and the `Palette` struct. Delete the
  `sty*` styles and `lbl*` consts (only the deleted verbs/bar used them).
  Keep the `lipgloss` import (still used by `lipgloss.Color(...)`) and
  `image/color`.
- `theme_test.go`: trim to assert the color vars are non-nil and `Palette`
  maps them. Drop `styPink` / style assertions.

Result: `render` is a palette-only package — `theme.go` + `theme_test.go`.

### Edit `root.go`

- Drop the `intro` import; `RunE` becomes just `return cmd.Help()` (no
  `intro.Play`).
- Remove the `verbose` var and the `--verbose` flag registration.
- `PersistentPreRun`: keep only the `noColor → NO_COLOR` branch; drop
  `render.SetVerbose(verbose)`.
- Update the stale comments that mention `render.Step` / `render.Bar` and the
  intro.
- Keep imports: `render` (Palette), `version`, `fang`, `lipgloss`,
  `image/color`.

### Edit `root_test.go`

- Remove `TestVerboseFlagRegistered` and `TestVerbosePreRunWiresRender`.
- Keep `TestRootHasUseAndVersion`, `TestRootSilencesUsageAndErrors`,
  `TestNoColorFlagSetsEnv`, `TestExecuteHelpDoesNotError`.

## Dependency tidy

Deleting the render core and the internal packages likely drops the only uses
of some third-party deps (e.g. `colorprofile`, charm bits used only by the
animated renderer, and the marketplace/git deps). Run `go mod tidy` and let it
prune `go.mod` / `go.sum`. Do not hand-edit.

## Verification

- `rtk go build ./...` — compiles with no unused-import / undefined-symbol
  errors.
- `rtk go test ./...` — all packages pass.
- Coverage stays at the 100% target for the surviving packages (`runner`,
  `version`, `render`, `cli` root/run/version, `cmd/liszt` via testscript).
- `liszt --help` lists `run` and `version`; `liszt run ...` and
  `liszt version` behave as before; bare `liszt` prints help with no intro.

## Out of scope

- Changing `run` behavior or its output (still raw `fmt.Fprintf`).
- Re-styling anything — there is no progress bar/spinner left to style.
- Touching historical docs under `docs/`.
