# Trim liszt to `run` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Strip liszt down to the `run` and `version` commands, delete every other command and its now-orphaned support code, and collapse the `render` package to a color palette.

**Architecture:** Pure deletion + small edits. Removal is ordered so the build and tests stay green at every commit: first drop the command layer and fix the test scripts that drive removed commands, then delete the internal packages that just lost their only importers, then edit `root.go` to drop the intro animation and `--verbose` flag, then reduce `render` to `theme.go`, then `go mod tidy`.

**Tech Stack:** Go, cobra + charmbracelet/fang (CLI), rogpeppe/go-internal/testscript (integration tests), lipgloss (palette colors). All Go commands run through `rtk` (e.g. `rtk go build ./...`) — never call `go` by an absolute path.

**Spec:** `docs/superpowers/specs/2026-05-22-trim-liszt-to-run-design.md`

---

## File map

- `internal/cli/` — delete every command except root/run/version; edit `root.go`, `root_test.go`.
- `internal/{intro,resource,marketplace,repos,gitx,claudehome,claudestate,lock,manifest,xdg}/` — delete whole directories.
- `internal/render/` — delete everything except `theme.go` (trimmed) + `theme_test.go` (trimmed).
- `cmd/liszt/testdata/script/` — delete `repo_add_duplicate.txtar`, `verbose.txtar`; edit `help.txtar`.
- `go.mod` / `go.sum` — pruned by `go mod tidy`.

Survivors: `internal/runner`, `internal/version`, `internal/render` (palette only), `internal/cli` (root/run/version), `cmd/liszt`.

---

## Task 1: Remove the non-run/version command layer + fix test scripts

The removed command files and the testscript files that drive them are mutually
entangled (`defaultPaths`, `Paths`, `printHeader`, `validateFlavor` are shared
across them), so they come out together. The orphaned `internal` packages still
compile on their own after this; they are deleted in Task 2.

**Files:**
- Delete: `internal/cli/outdated_cmd.go`
- Delete: `internal/cli/outdated.go`
- Delete: `internal/cli/outdated_cmd_test.go`
- Delete: `internal/cli/repo_cmd.go`
- Delete: `internal/cli/repo.go`
- Delete: `internal/cli/repo_test.go`
- Delete: `internal/cli/repo_cmd_test.go`
- Delete: `internal/cli/plugin_cmd.go`
- Delete: `internal/cli/plugin.go`
- Delete: `internal/cli/plugin_cmd_test.go`
- Delete: `internal/cli/resource_cmd.go`
- Delete: `internal/cli/resource.go`
- Delete: `internal/cli/resource_cmd_test.go`
- Delete: `internal/cli/install.go`
- Delete: `internal/cli/io.go`
- Delete: `internal/cli/paths.go`
- Delete: `cmd/liszt/testdata/script/repo_add_duplicate.txtar`
- Delete: `cmd/liszt/testdata/script/verbose.txtar`
- Modify: `cmd/liszt/testdata/script/help.txtar`

- [ ] **Step 1: Delete the command + handler files**

Delete these 16 files under `internal/cli/`:
`outdated_cmd.go`, `outdated.go`, `outdated_cmd_test.go`,
`repo_cmd.go`, `repo.go`, `repo_test.go`, `repo_cmd_test.go`,
`plugin_cmd.go`, `plugin.go`, `plugin_cmd_test.go`,
`resource_cmd.go`, `resource.go`, `resource_cmd_test.go`,
`install.go`, `io.go`, `paths.go`.

Leave `root.go`, `root_test.go`, `run_cmd.go`, `version_cmd.go`,
`version_cmd_test.go` untouched.

- [ ] **Step 2: Delete the two dead test scripts**

Delete `cmd/liszt/testdata/script/repo_add_duplicate.txtar` and
`cmd/liszt/testdata/script/verbose.txtar`.

- [ ] **Step 3: Rewrite `help.txtar` to assert only surviving verbs**

Replace the entire contents of `cmd/liszt/testdata/script/help.txtar` with:

```
# --help prints program name and lists surviving verbs
env NO_COLOR=1
exec liszt --help
stdout 'liszt'
stdout 'run'
stdout 'version'
! stdout 'plugin'
! stdout 'outdated'
```

- [ ] **Step 4: Verify the build compiles**

Run: `rtk go build ./...`
Expected: no output, exit 0. (`internal/cli` no longer imports the soon-to-be
deleted packages; the packages themselves still compile standalone.)

- [ ] **Step 5: Verify the full test suite passes**

Run: `rtk go test ./...`
Expected: all packages `ok`. The `cmd/liszt` testscript run now exercises only
`help`, `no_color`, `run`, `version`.

- [ ] **Step 6: Commit**

```bash
git add -u
git commit -m "refactor(cli): remove all commands except run and version"
```

---

## Task 2: Delete the orphaned internal packages

After Task 1 these nine packages have no importer anywhere in the module
(`intro` is still imported by `root.go`, so it is NOT in this task — it goes in
Task 3).

**Files:**
- Delete directory: `internal/resource/`
- Delete directory: `internal/marketplace/`
- Delete directory: `internal/repos/`
- Delete directory: `internal/gitx/`
- Delete directory: `internal/claudehome/`
- Delete directory: `internal/claudestate/`
- Delete directory: `internal/lock/`
- Delete directory: `internal/manifest/`
- Delete directory: `internal/xdg/`

- [ ] **Step 1: Confirm nothing outside these dirs imports them**

Run: `rtk grep -rn "internal/resource\|internal/marketplace\|internal/repos\|internal/gitx\|internal/claudehome\|internal/claudestate\|internal/lock\|internal/manifest\|internal/xdg" internal cmd`
Expected: only matches inside the directories being deleted (their own
intra-package imports). No matches in `internal/cli`, `internal/runner`,
`internal/render`, `internal/version`, or `cmd`. If any survivor matches, stop
and re-evaluate.

- [ ] **Step 2: Delete the nine directories**

Remove `internal/resource`, `internal/marketplace`, `internal/repos`,
`internal/gitx`, `internal/claudehome`, `internal/claudestate`, `internal/lock`,
`internal/manifest`, `internal/xdg` and all their contents.

- [ ] **Step 3: Verify build**

Run: `rtk go build ./...`
Expected: no output, exit 0.

- [ ] **Step 4: Verify tests**

Run: `rtk go test ./...`
Expected: all packages `ok`.

- [ ] **Step 5: Commit**

```bash
git add -u
git commit -m "refactor: delete internal packages orphaned by command removal"
```

---

## Task 3: Drop the intro animation and `--verbose`, delete `internal/intro`

`root.go` is the only thing importing `intro`, and the only thing calling
`render.SetVerbose`. Remove both, then delete `internal/intro`.

**Files:**
- Modify: `internal/cli/root.go`
- Modify: `internal/cli/root_test.go`
- Delete directory: `internal/intro/`

- [ ] **Step 1: Edit `root.go` — drop intro, drop verbose**

Replace the `import` block so `intro` is gone (keep the rest):

```go
import (
	"context"
	"fmt"
	"image/color"
	"os"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"

	"github.com/mguilarducci/liszt/internal/render"
	"github.com/mguilarducci/liszt/internal/version"
)
```

Replace the `rootCmd` `RunE` so the bare invocation just prints help:

```go
	RunE: func(cmd *cobra.Command, _ []string) error {
		return cmd.Help()
	},
```

Replace the `var (...)` block and `init()` with a `--no-color`-only version:

```go
var noColor bool

func init() {
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable color output")
	rootCmd.PersistentPreRun = func(_ *cobra.Command, _ []string) {
		if noColor {
			// Setenv only fails on platforms without env support; failure
			// here means colors stay on, which is harmless.
			_ = os.Setenv("NO_COLOR", "1")
		}
	}
}
```

Update the stale doc comment on `gleamColorScheme` (it currently references
`render.Step` / `render.Bar`). Replace that sentence with:

```go
// gleamColorScheme maps the Gleam palette onto fang's ColorScheme so help,
// version, and error output share the Gleam look-and-feel. The function
// signature matches fang.ColorSchemeFunc and is invoked with a
// lipgloss.LightDarkFunc that resolves to the terminal's preferred variant;
// we ignore the light/dark argument because the Gleam palette is dark-tuned.
```

- [ ] **Step 2: Edit `root_test.go` — remove the verbose tests**

Delete `TestVerboseFlagRegistered` and `TestVerbosePreRunWiresRender` in full.
Keep `TestRootHasUseAndVersion`, `TestRootSilencesUsageAndErrors`,
`TestNoColorFlagSetsEnv`, `TestExecuteHelpDoesNotError`. The import block
(`bytes`, `context`, `os`, `strings`, `testing`) stays unchanged — all remaining
imports are still used.

- [ ] **Step 3: Verify intro is now unimported**

Run: `rtk grep -rn "internal/intro" internal cmd`
Expected: no matches.

- [ ] **Step 4: Delete `internal/intro/`**

Remove the directory and all its contents
(`lisztomania.go`, `logo.go`, and the test files).

- [ ] **Step 5: Verify build**

Run: `rtk go build ./...`
Expected: no output, exit 0.

- [ ] **Step 6: Verify tests**

Run: `rtk go test ./...`
Expected: all packages `ok`. Bare `liszt` now prints help with no intro;
`liszt version` and `liszt run ...` unaffected.

- [ ] **Step 7: Commit**

```bash
git add -u
git commit -m "refactor(cli): drop intro animation and --verbose flag"
```

---

## Task 4: Reduce `render` to a color palette

`root.go` now uses only `render.Palette.*`. Delete the renderer core, every
message verb, and the bar/spinner; keep `theme.go` (trimmed to colors +
`Palette`) and `theme_test.go` (trimmed to match).

**Files:**
- Delete: `internal/render/bar.go`, `bar_test.go`
- Delete: `internal/render/progress.go`, `progress_test.go`
- Delete: `internal/render/step.go`, `step_test.go`
- Delete: `internal/render/header.go`, `header_test.go`
- Delete: `internal/render/detail.go`, `detail_test.go`
- Delete: `internal/render/message.go`, `message_test.go`
- Delete: `internal/render/renderer.go`, `renderer_test.go`
- Delete: `internal/render/default.go`, `default_test.go`
- Delete: `internal/render/detect.go`, `detect_test.go`
- Modify: `internal/render/theme.go`
- Modify: `internal/render/theme_test.go`

- [ ] **Step 1: Delete the 18 render files**

Delete these from `internal/render/`:
`bar.go`, `bar_test.go`, `progress.go`, `progress_test.go`,
`step.go`, `step_test.go`, `header.go`, `header_test.go`,
`detail.go`, `detail_test.go`, `message.go`, `message_test.go`,
`renderer.go`, `renderer_test.go`, `default.go`, `default_test.go`,
`detect.go`, `detect_test.go`.

Only `theme.go` and `theme_test.go` remain.

- [ ] **Step 2: Trim `theme.go` to colors + Palette**

Replace the entire contents of `internal/render/theme.go` with:

```go
// Package render exposes the Gleam color palette used to style the liszt
// CLI's help, version, and error output via charmbracelet/fang.
package render

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

var (
	cPinkDeep   color.Color = lipgloss.Color("#fe7ab2")
	cPinkBright color.Color = lipgloss.Color("#ffaff3")
	cInfo       color.Color = lipgloss.Color("#9ce7ff")
	cDone       color.Color = lipgloss.Color("#aadd8b")
	cWarn       color.Color = lipgloss.Color("#ffc501")
	cError      color.Color = lipgloss.Color("#f44747")
	cDim        color.Color = lipgloss.Color("#c4c4c4")
)

// Palette exposes the Gleam palette colors. The CLI's fang integration uses
// these to style help/version/error output.
var Palette = struct {
	PinkDeep   color.Color
	PinkBright color.Color
	Info       color.Color
	Done       color.Color
	Warn       color.Color
	Error      color.Color
	Dim        color.Color
}{
	PinkDeep:   cPinkDeep,
	PinkBright: cPinkBright,
	Info:       cInfo,
	Done:       cDone,
	Warn:       cWarn,
	Error:      cError,
	Dim:        cDim,
}
```

- [ ] **Step 3: Trim `theme_test.go` to match**

Replace the entire contents of `internal/render/theme_test.go` with:

```go
package render

import (
	"image/color"
	"testing"
)

func TestThemeColorsNotNil(t *testing.T) {
	t.Parallel()

	cases := map[string]color.Color{
		"cPinkDeep":   cPinkDeep,
		"cPinkBright": cPinkBright,
		"cInfo":       cInfo,
		"cDone":       cDone,
		"cWarn":       cWarn,
		"cError":      cError,
		"cDim":        cDim,
	}
	for name, c := range cases {
		if c == nil {
			t.Errorf("%s: expected non-nil color", name)
		}
	}
}

func TestPaletteMapsColors(t *testing.T) {
	t.Parallel()

	cases := map[string][2]color.Color{
		"PinkDeep":   {Palette.PinkDeep, cPinkDeep},
		"PinkBright": {Palette.PinkBright, cPinkBright},
		"Info":       {Palette.Info, cInfo},
		"Done":       {Palette.Done, cDone},
		"Warn":       {Palette.Warn, cWarn},
		"Error":      {Palette.Error, cError},
		"Dim":        {Palette.Dim, cDim},
	}
	for name, pair := range cases {
		if pair[0] != pair[1] {
			t.Errorf("Palette.%s = %v; want %v", name, pair[0], pair[1])
		}
	}
}
```

- [ ] **Step 4: Verify build**

Run: `rtk go build ./...`
Expected: no output, exit 0.

- [ ] **Step 5: Verify tests**

Run: `rtk go test ./...`
Expected: all packages `ok`.

- [ ] **Step 6: Verify render coverage is 100%**

Run: `rtk go test -cover ./internal/render/`
Expected: `coverage: 100.0% of statements`.

- [ ] **Step 7: Commit**

```bash
git add -u
git commit -m "refactor(render): reduce to color palette"
```

---

## Task 5: Prune dependencies and final verification

**Files:**
- Modify: `go.mod`, `go.sum` (via tooling only)

- [ ] **Step 1: Tidy the module**

Run: `rtk go mod tidy`
Expected: removes now-unused dependencies (e.g. `colorprofile` and other deps
only used by the deleted renderer/marketplace/git code). Do not hand-edit
`go.mod` or `go.sum`.

- [ ] **Step 2: Verify build**

Run: `rtk go build ./...`
Expected: no output, exit 0.

- [ ] **Step 3: Verify the full suite**

Run: `rtk go test ./...`
Expected: all packages `ok`.

- [ ] **Step 4: Verify whole-module coverage holds**

Run: `rtk go test -cover ./...`
Expected: surviving packages (`runner`, `version`, `render`, `cli`,
`cmd/liszt`) report their coverage; nothing regresses below the 100% target.

- [ ] **Step 5: Smoke-check the surviving CLI surface**

Run: `rtk go run ./cmd/liszt --help`
Expected: help output names `run` and `version`, and does NOT name `plugin`,
`repo`, `outdated`, or any resource kind.

Run: `rtk go run ./cmd/liszt version`
Expected: prints `liszt 0.0.0-dev` (or the current build identifier).

- [ ] **Step 6: Commit**

```bash
git add -u
git commit -m "chore: go mod tidy after command + render trim"
```

---

## Self-review notes

- **Spec coverage:** Part A (command + package + testscript removal) → Tasks 1–3.
  Part B (render reduction, `--verbose`/intro drop, `root.go`/`root_test.go`
  edits) → Tasks 3–4. Dependency tidy → Task 5. All spec sections mapped.
- **Ordering safety:** each task ends on a green `rtk go build ./...` +
  `rtk go test ./...`. `intro` deletion waits until `root.go` stops importing it
  (Task 3); the nine other packages are deleted in Task 2 after their importers
  are gone in Task 1.
- **No placeholders:** every edited file shows full replacement content or an
  exact deletion list.
