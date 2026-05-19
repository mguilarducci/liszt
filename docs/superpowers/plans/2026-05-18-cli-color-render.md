# CLI Color & Render Layer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `charmbracelet/fang` + `lipgloss`-based color and progress-bar rendering to the `liszt` CLI by porting the `render` package from `liszt-bkp` and migrating verb routing to `spf13/cobra`.

**Architecture:** New `internal/render/` package (palette, renderer, messages, headers, progress bar with determinate + indeterminate modes) consumed by a rewritten `internal/cli/` package built on `cobra`. The CLI entry point delegates to `fang.Execute` for styled help / version / error output. Existing business-logic helpers (`cli.PluginInstall`, `cli.ResourceList`, `gitx.EnsureClone`, etc.) are preserved and called from cobra RunE handlers.

**Tech Stack:** Go 1.26.3, `spf13/cobra v1.10.2`, `charmbracelet/fang v1.0.0`, `charm.land/lipgloss/v2 v2.0.3`, `charmbracelet/colorprofile v0.4.3`, `mattn/go-isatty v0.0.22`, `pelletier/go-toml/v2 v2.3.1`, `hexops/autogold/v2 v2.3.1`, `rogpeppe/go-internal v1.14.1`, `google/go-cmp v0.7.0`.

**Spec:** `docs/superpowers/specs/2026-05-18-cli-color-render-design.md`

---

## File Structure

### Created

| Path                                        | Responsibility                                                  |
|---------------------------------------------|------------------------------------------------------------------|
| `internal/version/version.go`               | Version, Commit, Date vars + `Full()` formatter.                |
| `internal/version/version_test.go`          | Snapshot of `Full()`.                                            |
| `internal/render/theme.go`                  | Palette + pre-built styles + label constants.                    |
| `internal/render/theme_test.go`             | Palette non-nil, styles produce output, labels same width.       |
| `internal/render/detect.go`                 | `detectProfile`, `writerIsTTY`.                                  |
| `internal/render/detect_test.go`            | `NO_COLOR`, `TERM=dumb`, non-TTY writer.                         |
| `internal/render/renderer.go`               | `Renderer` struct + `New` + options + `writeString` + `eraseLine`. |
| `internal/render/renderer_test.go`          | Option wiring + profile override.                                |
| `internal/render/default.go`                | Lazy global `Default` + top-level delegating functions.          |
| `internal/render/default_test.go`           | `ensureDefault` returns same instance.                           |
| `internal/render/message.go`                | `Info`, `Warn`, `Error`, `Done` + `formatLine` + `formatKV`.     |
| `internal/render/message_test.go`           | Each level emits expected bytes; kv pair formatting; odd kv.      |
| `internal/render/header.go`                 | `Header`, `Subheader`, `Hint`.                                   |
| `internal/render/header_test.go`            | Each header style applied.                                       |
| `internal/render/bar.go`                    | `Bar` determinate + indeterminate.                               |
| `internal/render/bar_test.go`               | Determinate, indeterminate, non-TTY, mid-print interrupt.        |
| `internal/cli/root.go`                      | `rootCmd`, `Execute(ctx)`, `--no-color` persistent flag.         |
| `internal/cli/root_test.go`                 | `--version`, `--help`, `--no-color` smoke.                       |
| `internal/cli/version_cmd.go`               | `liszt version` subcommand.                                      |
| `internal/cli/version_cmd_test.go`          | Subcommand emits `Full()` line.                                  |
| `internal/cli/repo_cmd.go`                  | `repo` parent + `repo add` subcommand.                           |
| `internal/cli/repo_cmd_test.go`             | `repo add` writes manifest; surfaces clone errors.               |
| `internal/cli/plugin_cmd.go`                | `plugin list` + `plugin install`.                                |
| `internal/cli/plugin_cmd_test.go`           | List + install happy paths and errors.                           |
| `internal/cli/resource_cmd.go`              | Per-kind subcommand registration loop.                           |
| `internal/cli/resource_cmd_test.go`         | All six kinds register; flag parsing.                            |
| `internal/cli/outdated_cmd.go`              | `outdated` subcommand.                                           |
| `internal/cli/outdated_cmd_test.go`         | Stale-count summary + per-stale `render.Info`.                   |
| `cmd/liszt/script_test.go`                  | `testscript` end-to-end harness.                                 |
| `cmd/liszt/testdata/script/version.txtar`   | `liszt version` golden.                                          |
| `cmd/liszt/testdata/script/help.txtar`      | `liszt --help` golden (NO_COLOR=1).                              |
| `cmd/liszt/testdata/script/repo_add.txtar`  | `repo add` golden with fake git.                                 |
| `cmd/liszt/testdata/script/no_color.txtar`  | `--no-color` strips ANSI.                                        |

### Modified

| Path                                        | Change                                                          |
|---------------------------------------------|------------------------------------------------------------------|
| `go.mod` / `go.sum`                         | Add dependencies.                                                |
| `cmd/liszt/main.go`                         | Shrink to delegate to `cli.Execute`.                             |
| `internal/cli/install.go`                   | Convert printlns to `render.*` calls.                            |
| `internal/cli/io.go`                        | Convert printlns to `render.*` calls.                            |
| `internal/cli/outdated.go`                  | Convert printlns to `render.*` calls; bar wiring (see Task 7.4). |
| `internal/cli/plugin.go`                    | Convert printlns to `render.*` calls.                            |
| `internal/cli/repo.go`                      | Convert printlns to `render.*` calls; bar wiring (see Task 7.2). |
| `internal/cli/resource.go`                  | Convert printlns to `render.*` calls.                            |
| `internal/gitx/git.go`                      | Add `Writer io.Writer` package-level setter for subprocess output. |

### Deleted

None. `internal/cli/paths.go` stays as-is.

---

## Phase 1: Dependencies + Version Package

### Task 1.1: Add module dependencies

**Files:**
- Modify: `go.mod`
- Modify: `go.sum` (regenerated)

- [ ] **Step 1: Run go get for each direct dependency**

Run each command separately (no compound commands):

```bash
go get github.com/charmbracelet/fang@v1.0.0
```

```bash
go get github.com/spf13/cobra@v1.10.2
```

```bash
go get charm.land/lipgloss/v2@v2.0.3
```

```bash
go get github.com/charmbracelet/colorprofile@v0.4.3
```

```bash
go get github.com/mattn/go-isatty@v0.0.22
```

```bash
go get github.com/hexops/autogold/v2@v2.3.1
```

```bash
go get github.com/rogpeppe/go-internal@v1.14.1
```

```bash
go get github.com/google/go-cmp@v0.7.0
```

- [ ] **Step 2: Tidy modules**

```bash
go mod tidy
```

Expected: `go.mod` lists all eight modules in `require` block; `go.sum` populated.

- [ ] **Step 3: Verify build still compiles**

```bash
go build ./...
```

Expected: exit 0. Existing code is untouched so this must succeed.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
```

```bash
git commit -m "build(deps): add fang, cobra, lipgloss, colorprofile, isatty, autogold, testscript, go-cmp"
```

### Task 1.2: Version package

**Files:**
- Create: `internal/version/version.go`
- Test: `internal/version/version_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/version/version_test.go`:

```go
package version_test

import (
	"testing"

	"github.com/hexops/autogold/v2"

	"github.com/mguilarducci/liszt/internal/version"
)

func TestFullSnapshot(t *testing.T) {
	t.Parallel()

	autogold.Expect("liszt 0.0.0-dev").Equal(t, version.Full())
}

func TestFullUsesVersionVar(t *testing.T) {
	t.Parallel()

	orig := version.Version
	t.Cleanup(func() { version.Version = orig })
	version.Version = "9.9.9"

	if got := version.Full(); got != "liszt 9.9.9" {
		t.Errorf("Full() = %q; want %q", got, "liszt 9.9.9")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/version/...
```

Expected: build failure — package does not exist.

- [ ] **Step 3: Write minimal implementation**

Create `internal/version/version.go`:

```go
// Package version holds build-time identification strings injected via
// -ldflags. The .goreleaser.yaml file wires Version, Commit, and Date for
// release builds; local builds fall back to the dev defaults below.
package version

// Build-time variables. Tests mutate them via t.Cleanup; production callers
// must treat them as read-only.
var (
	Version = "0.0.0-dev"
	Commit  = "none"
	Date    = "unknown"
)

// Full returns the user-facing version string used by the `liszt version`
// subcommand and cobra's `--version` flag.
func Full() string {
	return "liszt " + Version
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/version/...
```

Expected: PASS for both tests.

- [ ] **Step 5: Commit**

```bash
git add internal/version/version.go internal/version/version_test.go
```

```bash
git commit -m "feat(version): add build-time identification package"
```

---

## Phase 2: Render Core

### Task 2.1: Theme (palette + styles + labels)

**Files:**
- Create: `internal/render/theme.go`
- Test: `internal/render/theme_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/render/theme_test.go`:

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

func TestThemeStylesProduceOutput(t *testing.T) {
	t.Parallel()

	if styH1.Render("X") == "" {
		t.Errorf("styH1 produced empty output")
	}
	if styH2.Render("Y") == "" {
		t.Errorf("styH2 produced empty output")
	}
	if styH3.Render("Z") == "" {
		t.Errorf("styH3 produced empty output")
	}
	if styPink.Render("W") == "" {
		t.Errorf("styPink produced empty output")
	}
}

func TestLabelConstantsSameWidth(t *testing.T) {
	t.Parallel()

	labels := []string{lblInfo, lblDone, lblWarn, lblError}
	want := len(lblError)
	for _, l := range labels {
		if len(l) != want {
			t.Errorf("label %q has width %d; want %d", l, len(l), want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/render/...
```

Expected: build failure — package does not exist.

- [ ] **Step 3: Write minimal implementation**

Create `internal/render/theme.go`:

```go
// Package render is the styled CLI output layer for the liszt binary. All
// verbs print user-facing output through this package so the look-and-feel
// stays consistent across the CLI surface. See the design spec at
// docs/superpowers/specs/2026-05-18-cli-color-render-design.md.
package render

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Gleam-inspired palette. Hex values come from the design spec §4 and the
// Gleam VS Code theme. lipgloss/v2 returns image/color.Color from Color().
var (
	cPinkDeep   color.Color = lipgloss.Color("#fe7ab2")
	cPinkBright color.Color = lipgloss.Color("#ffaff3")
	cInfo       color.Color = lipgloss.Color("#9ce7ff")
	cDone       color.Color = lipgloss.Color("#aadd8b")
	cWarn       color.Color = lipgloss.Color("#ffc501")
	cError      color.Color = lipgloss.Color("#f44747")
	cDim        color.Color = lipgloss.Color("#c4c4c4")
)

// Pre-built styles. Each is constructed once at package init so the hot path
// avoids repeated lipgloss.NewStyle allocation.
var (
	styH1   = lipgloss.NewStyle().Bold(true).Underline(true)
	styH2   = lipgloss.NewStyle().Foreground(cPinkDeep).Bold(true)
	styH3   = lipgloss.NewStyle().Foreground(cDim).Italic(true)
	styDim  = lipgloss.NewStyle().Foreground(cDim)
	styPink = lipgloss.NewStyle().Foreground(cPinkBright).Bold(true)

	styInfoBar  = lipgloss.NewStyle().Foreground(cInfo)
	styDoneBar  = lipgloss.NewStyle().Foreground(cDone)
	styWarnBar  = lipgloss.NewStyle().Foreground(cWarn)
	styErrorBar = lipgloss.NewStyle().Foreground(cError)

	styInfoLbl  = lipgloss.NewStyle().Foreground(cInfo).Bold(true)
	styDoneLbl  = lipgloss.NewStyle().Foreground(cDone).Bold(true)
	styWarnLbl  = lipgloss.NewStyle().Foreground(cWarn).Bold(true)
	styErrorLbl = lipgloss.NewStyle().Foreground(cError).Bold(true)
)

// Five-cell padded labels so multi-line messages align under the prefix.
// "error" is the widest label, so all four pad to its width.
const (
	lblInfo  = "info "
	lblDone  = "done "
	lblWarn  = "warn "
	lblError = "error"
)
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/render/...
```

Expected: PASS for all three theme tests.

- [ ] **Step 5: Commit**

```bash
git add internal/render/theme.go internal/render/theme_test.go
```

```bash
git commit -m "feat(render): add Gleam palette, styles, and labels"
```

### Task 2.2: Profile and TTY detection

**Files:**
- Create: `internal/render/detect.go`
- Test: `internal/render/detect_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/render/detect_test.go`:

```go
package render

import (
	"bytes"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

func TestDetectProfile_NoColorEnv(t *testing.T) {
	t.Parallel()

	got, isTTY := detectProfile(&bytes.Buffer{}, []string{"NO_COLOR=1", "TERM=xterm-256color"})
	if got != colorprofile.NoTTY {
		t.Errorf("with NO_COLOR set: got profile=%v, want NoTTY", got)
	}
	if isTTY {
		t.Errorf("with bytes.Buffer writer: got isTTY=true, want false")
	}
}

func TestDetectProfile_DumbTerm(t *testing.T) {
	t.Parallel()

	got, _ := detectProfile(&bytes.Buffer{}, []string{"TERM=dumb"})
	if got != colorprofile.NoTTY {
		t.Errorf("with TERM=dumb: got profile=%v, want NoTTY", got)
	}
}

func TestDetectProfile_NonTTYWriter(t *testing.T) {
	t.Parallel()

	_, isTTY := detectProfile(&bytes.Buffer{}, []string{"TERM=xterm-256color", "COLORTERM=truecolor"})
	if isTTY {
		t.Errorf("with bytes.Buffer writer: got isTTY=true, want false")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/render/...
```

Expected: build failure — `detectProfile` undefined.

- [ ] **Step 3: Write minimal implementation**

Create `internal/render/detect.go`:

```go
package render

import (
	"io"
	"os"

	"github.com/charmbracelet/colorprofile"
	"github.com/mattn/go-isatty"
)

// detectProfile reads the writer and environment to determine the color
// profile and whether the writer is attached to a terminal. The environ
// slice mirrors os.Environ() so tests can drive it without mutating real env.
//
// colorprofile.Detect already honors NO_COLOR / CLICOLOR / CLICOLOR_FORCE /
// TERM=dumb / COLORTERM. We layer on a strict isTTY check because the bar
// goroutine spawns only when isTTY is true.
func detectProfile(w io.Writer, environ []string) (colorprofile.Profile, bool) {
	profile := colorprofile.Detect(w, environ)
	isTTY := writerIsTTY(w)
	return profile, isTTY
}

// writerIsTTY reports whether w writes to a terminal. Returns false for any
// non-*os.File writer (e.g. bytes.Buffer in tests).
func writerIsTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(f.Fd())
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/render/...
```

Expected: PASS for detect tests; theme tests still pass.

- [ ] **Step 5: Commit**

```bash
git add internal/render/detect.go internal/render/detect_test.go
```

```bash
git commit -m "feat(render): add profile and TTY detection"
```

### Task 2.3: Renderer (struct, New, options, write helpers)

**Files:**
- Create: `internal/render/renderer.go`
- Test: `internal/render/renderer_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/render/renderer_test.go`:

```go
package render

import (
	"bytes"
	"math/rand/v2"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

func TestNew_DefaultProfileDetected(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := New(&buf)
	if r == nil {
		t.Fatal("New returned nil")
	}
	if r.isTTY {
		t.Errorf("bytes.Buffer is not a TTY")
	}
}

func TestNew_WithProfileOverride(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := New(&buf, WithProfile(colorprofile.TrueColor))
	if r.profile != colorprofile.TrueColor {
		t.Errorf("WithProfile: got %v, want TrueColor", r.profile)
	}
}

func TestNew_WithNoColor(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := New(&buf, WithProfile(colorprofile.TrueColor), WithNoColor())
	if r.profile != colorprofile.NoTTY {
		t.Errorf("WithNoColor: got %v, want NoTTY", r.profile)
	}
}

func TestNew_WithTTY(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := New(&buf, WithTTY(true))
	if !r.isTTY {
		t.Errorf("WithTTY(true): got false, want true")
	}
}

func TestNew_WithRand(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	rng := rand.New(rand.NewPCG(1, 2))
	r := New(&buf, WithRand(rng))
	if r.rng != rng {
		t.Errorf("WithRand: did not store provided rand")
	}
}

func TestWriteString_StripsANSIOnNoTTY(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := New(&buf, WithProfile(colorprofile.NoTTY))
	r.writeString("\x1b[31mred\x1b[0m")
	if got := buf.String(); got != "red" {
		t.Errorf("NoTTY profile did not strip ANSI: got %q", got)
	}
}

func TestEraseLine_BypassesProfile(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := New(&buf, WithProfile(colorprofile.NoTTY))
	r.eraseLine()
	if got := buf.String(); got != "\r\x1b[K" {
		t.Errorf("eraseLine emitted %q; want %q", got, "\r\x1b[K")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/render/...
```

Expected: build failure — `New`, `Renderer`, options undefined.

- [ ] **Step 3: Write minimal implementation**

Create `internal/render/renderer.go`:

```go
package render

import (
	"io"
	"math/rand/v2"
	"os"
	"sync"

	"github.com/charmbracelet/colorprofile"
)

// Renderer is the styled output engine. Construct via New or use the
// package-level Default through the delegating top-level functions (see
// default.go).
type Renderer struct {
	w       io.Writer
	profile colorprofile.Profile
	isTTY   bool
	mu      sync.Mutex
	rng     *rand.Rand
	active  anim
}

// anim is the surface a Renderer needs to ask the currently-active animation
// (e.g. a progress bar) to redraw itself after a print clears its line.
type anim interface {
	repaint()
}

// writeString writes s through a colorprofile.Writer keyed on the Renderer's
// profile. The Writer strips/downgrades color escape sequences on
// NoTTY/ASCII/ANSI/ANSI256 profiles and passes TrueColor through untouched.
// Cursor-control sequences (\r, \x1b[K) that must survive a NoColor profile
// must use eraseLine instead — the profile writer's ansi.Strip path would
// otherwise consume them along with the color codes.
func (r *Renderer) writeString(s string) {
	cw := &colorprofile.Writer{Forward: r.w, Profile: r.profile}
	_, _ = cw.Write([]byte(s))
}

// eraseLine emits the cursor-control sequence that returns the cursor to
// column 0 and clears the current line. Bypasses the colorprofile.Writer so
// the sequence survives on NoColor profiles. Reserved for the bar redraw
// path.
func (r *Renderer) eraseLine() {
	_, _ = r.w.Write([]byte("\r\x1b[K"))
}

// Option mutates a Renderer at construction time.
type Option func(*Renderer)

// WithProfile overrides the auto-detected color profile.
func WithProfile(p colorprofile.Profile) Option {
	return func(r *Renderer) { r.profile = p }
}

// WithNoColor forces no-color output regardless of environment.
func WithNoColor() Option {
	return func(r *Renderer) { r.profile = colorprofile.NoTTY }
}

// WithTTY overrides the auto-detected TTY status. Test-only.
func WithTTY(isTTY bool) Option {
	return func(r *Renderer) { r.isTTY = isTTY }
}

// WithRand swaps the random source used by the progress bar's flicker.
// Test-only — production callers should leave this unset.
func WithRand(rng *rand.Rand) Option {
	return func(r *Renderer) { r.rng = rng }
}

// New constructs a Renderer writing to w. Auto-detection of color profile
// and TTY status runs before options apply so options always win.
func New(w io.Writer, opts ...Option) *Renderer {
	profile, isTTY := detectProfile(w, os.Environ())
	r := &Renderer{
		w:       w,
		profile: profile,
		isTTY:   isTTY,
		// G404 false positive: this rand drives a visual flicker, not crypto.
		rng: rand.New(rand.NewPCG(uint64(os.Getpid()), 0xDEADBEEF)), //nolint:gosec
	}
	for _, o := range opts {
		o(r)
	}
	return r
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/render/...
```

Expected: PASS for all renderer tests; theme + detect tests still pass.

- [ ] **Step 5: Commit**

```bash
git add internal/render/renderer.go internal/render/renderer_test.go
```

```bash
git commit -m "feat(render): add Renderer with options and write helpers"
```

### Task 2.4: Default lazy global

**Files:**
- Create: `internal/render/default.go`
- Test: `internal/render/default_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/render/default_test.go`:

```go
package render

import "testing"

func TestEnsureDefaultReturnsSingleton(t *testing.T) {
	a := ensureDefault()
	b := ensureDefault()
	if a != b {
		t.Errorf("ensureDefault returned different instances: %p vs %p", a, b)
	}
	if Default == nil {
		t.Errorf("Default is nil after ensureDefault")
	}
}
```

This test deliberately does not run in parallel because it inspects the
package-level Default singleton.

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/render/...
```

Expected: build failure — `ensureDefault`, `Default` undefined.

- [ ] **Step 3: Write minimal implementation**

Create `internal/render/default.go`:

```go
package render

import (
	"os"
	"sync"
)

// Default is the package-level Renderer used by the top-level delegating
// functions. It is constructed lazily on first use so tests and CLI startup
// code can mutate env (NO_COLOR, CLICOLOR_FORCE, ...) before any render call
// triggers construction.
var (
	defaultOnce sync.Once
	Default     *Renderer
)

func ensureDefault() *Renderer {
	defaultOnce.Do(func() {
		Default = New(os.Stderr)
	})
	return Default
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/render/...
```

Expected: PASS for default test plus all earlier tests.

- [ ] **Step 5: Commit**

```bash
git add internal/render/default.go internal/render/default_test.go
```

```bash
git commit -m "feat(render): add lazy package-level Default Renderer"
```

---

## Phase 3: Render Messages and Headers

### Task 3.1: Message helpers (Info / Warn / Error / Done)

**Files:**
- Create: `internal/render/message.go`
- Test: `internal/render/message_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/render/message_test.go`:

```go
package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

func newTestRenderer(buf *bytes.Buffer) *Renderer {
	return New(buf, WithProfile(colorprofile.NoTTY))
}

func TestInfo_PlainOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	newTestRenderer(&buf).Info("hello")
	got := buf.String()
	if !strings.HasPrefix(got, "▌ info  hello") {
		t.Errorf("Info output missing prefix: %q", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("Info output missing newline: %q", got)
	}
}

func TestInfo_WithKV(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	newTestRenderer(&buf).Info("hello", "key", "value", "n", 42)
	got := buf.String()
	if !strings.Contains(got, "key=value") {
		t.Errorf("missing key=value: %q", got)
	}
	if !strings.Contains(got, "n=42") {
		t.Errorf("missing n=42: %q", got)
	}
}

func TestInfo_OddKV(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	newTestRenderer(&buf).Info("hello", "lonely")
	if !strings.Contains(buf.String(), "lonely=<missing>") {
		t.Errorf("odd kv should render <missing>: %q", buf.String())
	}
}

func TestEachLevelLabel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		fn    func(*Renderer, string, ...any)
		label string
	}{
		{"info", (*Renderer).Info, "info "},
		{"warn", (*Renderer).Warn, "warn "},
		{"error", (*Renderer).Error, "error"},
		{"done", (*Renderer).Done, "done "},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var buf bytes.Buffer
			c.fn(newTestRenderer(&buf), "msg")
			if !strings.Contains(buf.String(), c.label) {
				t.Errorf("level %s missing label %q: %q", c.name, c.label, buf.String())
			}
		})
	}
}

func TestMultilineMessageIndents(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	newTestRenderer(&buf).Info("line1\nline2")
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines; got %d: %q", len(lines), buf.String())
	}
	if !strings.HasPrefix(lines[1], "        ") {
		t.Errorf("continuation line not indented: %q", lines[1])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/render/...
```

Expected: build failure — `Info`, `Warn`, `Error`, `Done` methods undefined.

- [ ] **Step 3: Write minimal implementation**

Create `internal/render/message.go`:

```go
package render

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// Info prints an informational line.
func (r *Renderer) Info(msg string, kv ...any) {
	r.writeLine(styInfoBar, styInfoLbl, lblInfo, msg, kv)
}

// Warn prints a warning line.
func (r *Renderer) Warn(msg string, kv ...any) {
	r.writeLine(styWarnBar, styWarnLbl, lblWarn, msg, kv)
}

// Error prints an error line. Callers should still return the error from
// the cobra RunE — fang prints the styled terminal error separately.
func (r *Renderer) Error(msg string, kv ...any) {
	r.writeLine(styErrorBar, styErrorLbl, lblError, msg, kv)
}

// Done prints a success line.
func (r *Renderer) Done(msg string, kv ...any) {
	r.writeLine(styDoneBar, styDoneLbl, lblDone, msg, kv)
}

func (r *Renderer) writeLine(barSty, lblSty lipgloss.Style, label, msg string, kv []any) {
	line := r.formatLine(barSty, lblSty, label, msg, kv)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active != nil {
		r.eraseLine()
	}
	r.writeString(line)
	if r.active != nil {
		// repaint takes r.mu, so drop and re-acquire (the deferred Unlock
		// at the top will fire on a held lock at return).
		active := r.active
		r.mu.Unlock()
		active.repaint()
		r.mu.Lock()
	}
}

// formatLine builds:
//
//	▌ <label>  <msg>  k1=v1 k2=v2\n
//
// Continuation lines in msg are indented under the message column.
func (r *Renderer) formatLine(barSty, lblSty lipgloss.Style, label, msg string, kv []any) string {
	bar := barSty.Render("▌")
	lbl := lblSty.Render(label)
	prefix := bar + " " + lbl + "  "
	indent := strings.Repeat(" ", 1+1+len(label)+2)

	msgLines := strings.Split(msg, "\n")
	var sb strings.Builder
	for i, line := range msgLines {
		if i == 0 {
			sb.WriteString(prefix)
		} else {
			sb.WriteString(indent)
		}
		sb.WriteString(line)
		if i < len(msgLines)-1 {
			sb.WriteString("\n")
		}
	}

	if len(kv) > 0 {
		sb.WriteString("  ")
		sb.WriteString(formatKV(kv))
	}
	sb.WriteString("\n")
	return sb.String()
}

func formatKV(kv []any) string {
	parts := make([]string, 0, (len(kv)+1)/2)
	for i := 0; i < len(kv); i += 2 {
		key := fmt.Sprint(kv[i])
		var value string
		if i+1 >= len(kv) {
			value = "<missing>"
		} else {
			value = fmt.Sprint(kv[i+1])
		}
		parts = append(parts, styDim.Render(key+"="+value))
	}
	return strings.Join(parts, " ")
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/render/...
```

Expected: PASS for all message tests; earlier tests still pass.

- [ ] **Step 5: Commit**

```bash
git add internal/render/message.go internal/render/message_test.go
```

```bash
git commit -m "feat(render): add Info/Warn/Error/Done message helpers"
```

### Task 3.2: Headers (Header / Subheader / Hint)

**Files:**
- Create: `internal/render/header.go`
- Test: `internal/render/header_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/render/header_test.go`:

```go
package render

import (
	"bytes"
	"strings"
	"testing"
)

func TestHeader_PlainText(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	newTestRenderer(&buf).Header("Plugins")
	if !strings.Contains(buf.String(), "Plugins") {
		t.Errorf("Header missing text: %q", buf.String())
	}
}

func TestSubheader_Prefix(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	newTestRenderer(&buf).Subheader("Skills")
	if !strings.Contains(buf.String(), "▸ Skills") {
		t.Errorf("Subheader missing prefix: %q", buf.String())
	}
}

func TestHint_PlainText(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	newTestRenderer(&buf).Hint("3 plugins across 2 repos")
	if !strings.Contains(buf.String(), "3 plugins across 2 repos") {
		t.Errorf("Hint missing text: %q", buf.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/render/...
```

Expected: build failure — `Header`, `Subheader`, `Hint` undefined.

- [ ] **Step 3: Write minimal implementation**

Create `internal/render/header.go`:

```go
package render

// Header prints an H1: bold + underline, no color override (terminal default
// foreground keeps it readable on every theme).
func (r *Renderer) Header(text string) {
	r.write(styH1.Render(text) + "\n")
}

// Subheader prints an H2: pink-deep bold with a ▸ prefix.
func (r *Renderer) Subheader(text string) {
	r.write(styH2.Render("▸ "+text) + "\n")
}

// Hint prints an H3: dim italic metadata line.
func (r *Renderer) Hint(text string) {
	r.write(styH3.Render(text) + "\n")
}

func (r *Renderer) write(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active != nil {
		r.eraseLine()
	}
	r.writeString(s)
	if r.active != nil {
		active := r.active
		r.mu.Unlock()
		active.repaint()
		r.mu.Lock()
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/render/...
```

Expected: PASS for header tests; earlier tests still pass.

- [ ] **Step 5: Add package-level delegating functions**

Append to `internal/render/default.go`:

```go
// Info delegates to Default.Info.
func Info(msg string, kv ...any) { ensureDefault().Info(msg, kv...) }

// Warn delegates to Default.Warn.
func Warn(msg string, kv ...any) { ensureDefault().Warn(msg, kv...) }

// Error delegates to Default.Error. (Lowercase `error` is the built-in
// type; this Error is a function — no shadowing.)
func Error(msg string, kv ...any) { ensureDefault().Error(msg, kv...) }

// Done delegates to Default.Done.
func Done(msg string, kv ...any) { ensureDefault().Done(msg, kv...) }

// Header delegates to Default.Header.
func Header(text string) { ensureDefault().Header(text) }

// Subheader delegates to Default.Subheader.
func Subheader(text string) { ensureDefault().Subheader(text) }

// Hint delegates to Default.Hint.
func Hint(text string) { ensureDefault().Hint(text) }
```

- [ ] **Step 6: Run all render tests to make sure nothing breaks**

```bash
go test ./internal/render/...
```

Expected: all tests still pass.

- [ ] **Step 7: Commit**

```bash
git add internal/render/header.go internal/render/header_test.go internal/render/default.go
```

```bash
git commit -m "feat(render): add Header/Subheader/Hint and top-level delegates"
```

---

## Phase 4: Progress Bar

### Task 4.1: Determinate Bar

**Files:**
- Create: `internal/render/bar.go`
- Test: `internal/render/bar_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/render/bar_test.go`:

```go
package render

import (
	"bytes"
	"math/rand/v2"
	"strings"
	"testing"
)

func newTTYRenderer(buf *bytes.Buffer) *Renderer {
	return New(buf, WithTTY(true), WithRand(rand.New(rand.NewPCG(1, 2))))
}

func TestBar_SetClampsToZeroOne(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	b := newTTYRenderer(&buf).Bar("label")
	b.Set(-0.5)
	b.repaint()
	if !strings.Contains(buf.String(), "  0%") {
		t.Errorf("Set(-0.5) should clamp to 0%%: %q", buf.String())
	}

	buf.Reset()
	b.Set(2.0)
	b.repaint()
	if !strings.Contains(buf.String(), "100%") {
		t.Errorf("Set(2.0) should clamp to 100%%: %q", buf.String())
	}
	b.Stop()
}

func TestBar_DoneEmitsSuccessLine(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	b := newTTYRenderer(&buf).Bar("label")
	b.Done("installed", "slug", "x")
	got := buf.String()
	if !strings.Contains(got, "done ") {
		t.Errorf("Done did not emit done line: %q", got)
	}
	if !strings.Contains(got, "slug=x") {
		t.Errorf("Done did not include kv: %q", got)
	}
}

func TestBar_FailEmitsErrorLine(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	b := newTTYRenderer(&buf).Bar("label")
	b.Fail("boom", "err", "network")
	if !strings.Contains(buf.String(), "error") {
		t.Errorf("Fail did not emit error line: %q", buf.String())
	}
}

func TestBar_UpdateChangesLabel(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	b := newTTYRenderer(&buf).Bar("first")
	b.Update("second")
	b.repaint()
	if !strings.Contains(buf.String(), "second") {
		t.Errorf("Update did not change label: %q", buf.String())
	}
	b.Stop()
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/render/...
```

Expected: build failure — `Bar`, `Renderer.Bar`, `Set`, `Done`, `Fail`,
`Update`, `repaint` undefined.

- [ ] **Step 3: Write minimal implementation**

Create `internal/render/bar.go`:

```go
package render

import (
	"fmt"
	"math"
	"strings"
	"sync/atomic"
	"time"
)

// Bar is a single-line progress bar. Filled cells re-roll a random note
// from {♬, ♪, ♩, ♫} on every tick. Construct via Renderer.Bar.
//
// Callers must invoke exactly one of Stop, Done, or Fail.
type Bar struct {
	r              *Renderer
	label          atomic.Value  // string
	pct            atomic.Uint64 // float64 via math.Float64bits
	indeterminate  atomic.Bool
	width          int
	notes          []string
	tick           time.Duration
	stop           chan struct{}
	done           chan struct{}
	stopped        atomic.Bool
	loopActive     atomic.Bool
}

// Bar constructs a new progress bar with the given initial label.
func (r *Renderer) Bar(label string) *Bar {
	b := &Bar{
		r:     r,
		width: 24,
		notes: []string{"♬", "♪", "♩", "♫"},
		tick:  100 * time.Millisecond,
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
	}
	b.label.Store(label)
	b.pct.Store(math.Float64bits(0))
	if !r.isTTY {
		// Non-TTY: emit a single info line on creation. Stop/Done/Fail
		// emit the final line. No animation loop, no cursor control.
		line := r.formatLine(styInfoBar, styInfoLbl, lblInfo, label, nil)
		r.mu.Lock()
		r.writeString(line)
		r.mu.Unlock()
		close(b.done)
		return b
	}
	r.mu.Lock()
	r.active = b
	r.mu.Unlock()
	b.loopActive.Store(true)
	go b.loop()
	return b
}

func (b *Bar) loop() {
	defer close(b.done)
	t := time.NewTicker(b.tick)
	defer t.Stop()
	for {
		select {
		case <-b.stop:
			return
		case <-t.C:
			b.repaint()
		}
	}
}

// repaint renders one frame of the bar in its current mode.
func (b *Bar) repaint() {
	pct := math.Float64frombits(b.pct.Load())
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	label, _ := b.label.Load().(string)
	cells := b.renderCells(pct)
	bar := styInfoBar.Render("▌")
	lbl := styInfoLbl.Render(lblInfo)
	var pctStr string
	if b.indeterminate.Load() {
		pctStr = styDim.Render("····")
	} else {
		pctStr = styDim.Render(fmt.Sprintf("%3d%%", int(pct*100)))
	}
	b.r.mu.Lock()
	defer b.r.mu.Unlock()
	b.r.eraseLine()
	b.r.writeString(fmt.Sprintf("%s %s  %s  %s  %s", bar, lbl, cells, pctStr, label))
}

// renderCells assumes pct is already clamped to [0,1] by the caller.
func (b *Bar) renderCells(pct float64) string {
	filled := int(pct * float64(b.width))
	if b.indeterminate.Load() {
		// Indeterminate: every cell uses the filled-flicker treatment so
		// the bar looks alive without claiming a percentage.
		filled = b.width
	}
	var sb strings.Builder
	for i := range b.width {
		if i < filled {
			sb.WriteString(styPink.Render(b.notes[b.r.rng.IntN(len(b.notes))]))
		} else {
			sb.WriteString(styDim.Render("·"))
		}
	}
	return sb.String()
}

// Set updates the bar's percentage. Values are clamped to [0,1].
// In indeterminate mode the stored value is preserved but not displayed.
func (b *Bar) Set(pct float64) { b.pct.Store(math.Float64bits(pct)) }

// Update changes the label. No-op on non-TTY (matches the non-TTY one-shot
// line emitted at construction).
func (b *Bar) Update(label string) { b.label.Store(label) }

// Stop ends the animation and clears the bar line. No-op on non-TTY.
func (b *Bar) Stop() {
	if !b.r.isTTY {
		return
	}
	if !b.stopped.CompareAndSwap(false, true) {
		return
	}
	if b.loopActive.CompareAndSwap(true, false) {
		close(b.stop)
		<-b.done
	}
	b.r.mu.Lock()
	b.r.active = nil
	b.r.eraseLine()
	b.r.mu.Unlock()
}

// Done stops the bar and prints a Done line.
func (b *Bar) Done(msg string, kv ...any) {
	b.Stop()
	b.r.Done(msg, kv...)
}

// Fail stops the bar and prints an Error line.
func (b *Bar) Fail(msg string, kv ...any) {
	b.Stop()
	b.r.Error(msg, kv...)
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/render/...
```

Expected: PASS for all determinate bar tests; earlier tests still pass.

- [ ] **Step 5: Commit**

```bash
git add internal/render/bar.go internal/render/bar_test.go
```

```bash
git commit -m "feat(render): add determinate progress bar"
```

### Task 4.2: Indeterminate Bar mode

**Files:**
- Modify: `internal/render/bar_test.go`
- Modify: `internal/render/bar.go` (already covers it, add tests)

- [ ] **Step 1: Append failing tests**

Add to `internal/render/bar_test.go`:

```go
func TestBar_IndeterminateShowsDots(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	b := newTTYRenderer(&buf).Bar("cloning")
	b.SetIndeterminate(true)
	b.Set(0.5)
	b.repaint()
	got := buf.String()
	if !strings.Contains(got, "····") {
		t.Errorf("indeterminate did not render dots: %q", got)
	}
	if strings.Contains(got, " 50%") {
		t.Errorf("indeterminate should hide percentage: %q", got)
	}
	b.Stop()
}

func TestBar_IndeterminateBackToDeterminate(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	b := newTTYRenderer(&buf).Bar("step")
	b.SetIndeterminate(true)
	b.repaint()
	buf.Reset()
	b.SetIndeterminate(false)
	b.Set(0.5)
	b.repaint()
	if !strings.Contains(buf.String(), " 50%") {
		t.Errorf("returning to determinate did not show percentage: %q", buf.String())
	}
	b.Stop()
}
```

- [ ] **Step 2: Add `SetIndeterminate` to bar.go**

Add method to `internal/render/bar.go`:

```go
// SetIndeterminate toggles indeterminate mode. In indeterminate mode all
// cells animate with the flicker pattern and the percentage column shows
// dim dots instead of a number. Used for opaque long operations (e.g.
// `git clone`) where progress cannot be measured.
func (b *Bar) SetIndeterminate(on bool) { b.indeterminate.Store(on) }
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/render/...
```

Expected: PASS for indeterminate tests; all earlier tests still pass.

- [ ] **Step 4: Commit**

```bash
git add internal/render/bar.go internal/render/bar_test.go
```

```bash
git commit -m "feat(render): add indeterminate mode to progress bar"
```

### Task 4.3: Non-TTY behavior and mid-print interrupt

**Files:**
- Modify: `internal/render/bar_test.go`

- [ ] **Step 1: Append failing tests**

Add to `internal/render/bar_test.go`:

```go
import (
	// (existing imports + add nothing new)
)

func TestBar_NonTTYSingleLineOnConstruction(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := New(&buf) // bytes.Buffer is non-TTY
	r.Bar("hello")
	got := buf.String()
	if !strings.Contains(got, "info  hello") {
		t.Errorf("non-TTY bar should emit single info line: %q", got)
	}
}

func TestBar_NonTTYDoneEmitsDoneLine(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := New(&buf)
	b := r.Bar("hello")
	b.Done("finished")
	got := buf.String()
	if !strings.Contains(got, "done  finished") {
		t.Errorf("non-TTY Done should emit done line: %q", got)
	}
}

func TestBar_StopIdempotent(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	b := newTTYRenderer(&buf).Bar("x")
	b.Stop()
	b.Stop() // must not panic, must not double-close
}

func TestBar_MidPrintInterruptRepaints(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := newTTYRenderer(&buf)
	b := r.Bar("running")
	b.Set(0.5)
	r.Info("interrupting message")
	// After Info, the bar should have repainted. Buffer contains the
	// info line plus a fresh bar render.
	got := buf.String()
	if !strings.Contains(got, "interrupting message") {
		t.Errorf("Info line missing: %q", got)
	}
	if !strings.Contains(got, "running") {
		t.Errorf("bar did not repaint after Info: %q", got)
	}
	b.Stop()
}
```

- [ ] **Step 2: Run tests**

```bash
go test ./internal/render/...
```

Expected: PASS — bar.go already implements all four behaviors. If any fail,
re-read the spec §10 mid-print interrupt path and verify `writeLine` /
`write` call `r.active.repaint()`.

- [ ] **Step 3: Commit**

```bash
git add internal/render/bar_test.go
```

```bash
git commit -m "test(render): cover non-TTY bar paths and mid-print interrupt"
```

---

## Phase 5: Cobra Root + Fang

### Task 5.1: Root command + Execute + --no-color

**Files:**
- Create: `internal/cli/root.go`
- Test: `internal/cli/root_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/cli/root_test.go`:

```go
package cli

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

func TestRootHasUseAndVersion(t *testing.T) {
	t.Parallel()

	if rootCmd.Use != "liszt" {
		t.Errorf("rootCmd.Use = %q; want %q", rootCmd.Use, "liszt")
	}
	if rootCmd.Version == "" {
		t.Errorf("rootCmd.Version is empty")
	}
}

func TestRootSilencesUsageAndErrors(t *testing.T) {
	t.Parallel()

	if !rootCmd.SilenceUsage {
		t.Errorf("rootCmd.SilenceUsage = false; want true")
	}
	if !rootCmd.SilenceErrors {
		t.Errorf("rootCmd.SilenceErrors = false; want true")
	}
}

func TestNoColorFlagSetsEnv(t *testing.T) {
	orig, hadOrig := os.LookupEnv("NO_COLOR")
	t.Cleanup(func() {
		if hadOrig {
			os.Setenv("NO_COLOR", orig)
		} else {
			os.Unsetenv("NO_COLOR")
		}
	})
	os.Unsetenv("NO_COLOR")

	noColor = true
	rootCmd.PersistentPreRun(rootCmd, nil)

	if os.Getenv("NO_COLOR") != "1" {
		t.Errorf("--no-color did not set NO_COLOR=1")
	}
}

func TestExecuteHelpDoesNotError(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"--help"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := Execute(context.Background()); err != nil {
		t.Errorf("Execute(--help) returned error: %v", err)
	}
	if !strings.Contains(stdout.String()+stderr.String(), "liszt") {
		t.Errorf("--help output missing program name")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/cli/...
```

Expected: build failure — `rootCmd`, `noColor`, `Execute` undefined.

- [ ] **Step 3: Write minimal implementation**

Create `internal/cli/root.go`:

```go
package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"

	"github.com/mguilarducci/liszt/internal/version"
)

var rootCmd = &cobra.Command{
	Use:           "liszt",
	Short:         "liszt — agent-agnostic plugin package manager",
	Version:       version.Full(),
	SilenceUsage:  true,
	SilenceErrors: true,
}

var noColor bool

func init() {
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable color output")
	rootCmd.PersistentPreRun = func(_ *cobra.Command, _ []string) {
		if noColor {
			// Setenv only fails on platforms without env support; failure
			// here only means colors stay on, which is harmless.
			_ = os.Setenv("NO_COLOR", "1")
		}
	}
}

// Execute runs the root command through charmbracelet/fang. fang styles
// --help, --version, and error output. Callers should pass a
// context.Background() unless they need cancellation semantics.
func Execute(ctx context.Context) error {
	if err := fang.Execute(ctx, rootCmd); err != nil {
		return fmt.Errorf("execute: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/cli/...
```

Expected: PASS for root tests.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/root.go internal/cli/root_test.go
```

```bash
git commit -m "feat(cli): add cobra rootCmd + fang Execute + --no-color flag"
```

### Task 5.2: Version subcommand

**Files:**
- Create: `internal/cli/version_cmd.go`
- Test: `internal/cli/version_cmd_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/cli/version_cmd_test.go`:

```go
package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"

	"github.com/mguilarducci/liszt/internal/render"
	"github.com/mguilarducci/liszt/internal/version"
)

func TestVersionCmd_EmitsFullString(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := render.New(&buf, render.WithProfile(colorprofile.NoTTY))
	cmd := NewVersionCmdWithRenderer(r)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE error: %v", err)
	}
	if !strings.Contains(buf.String(), version.Full()) {
		t.Errorf("version output missing Full(): %q", buf.String())
	}
}

func TestVersionCmd_RegisteredOnRoot(t *testing.T) {
	t.Parallel()

	found := false
	for _, c := range rootCmd.Commands() {
		if c.Use == "version" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("version subcommand not registered on rootCmd")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/cli/...
```

Expected: build failure — `NewVersionCmdWithRenderer` undefined.

- [ ] **Step 3: Write minimal implementation**

Create `internal/cli/version_cmd.go`:

```go
package cli

import (
	"github.com/spf13/cobra"

	"github.com/mguilarducci/liszt/internal/render"
	"github.com/mguilarducci/liszt/internal/version"
)

var versionCmd = NewVersionCmdWithRenderer(nil)

// NewVersionCmdWithRenderer constructs the `liszt version` subcommand using
// the given Renderer. Pass nil to delegate to render.Default; tests inject
// a Renderer wrapping a buffer.
func NewVersionCmdWithRenderer(r *render.Renderer) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		RunE: func(_ *cobra.Command, _ []string) error {
			if r == nil {
				render.Info(version.Full())
			} else {
				r.Info(version.Full())
			}
			return nil
		},
	}
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/cli/...
```

Expected: PASS for version tests.

- [ ] **Step 5: Shrink cmd/liszt/main.go**

Replace `cmd/liszt/main.go` entirely:

```go
package main

import (
	"context"
	"os"

	"github.com/mguilarducci/liszt/internal/cli"
)

// coverage: binary entry point; exercised only via the testscript harness.
func main() {
	if err := cli.Execute(context.Background()); err != nil {
		os.Exit(1)
	}
}
```

- [ ] **Step 6: Verify build still compiles**

```bash
go build ./...
```

Expected: failure — the old `main.go` referenced verbs (`cli.Repo`,
`cli.PluginInstall`, etc.) that have not been migrated yet, but the new
main.go does not. The compile error will be in `internal/cli/*.go` files
that still reference unmigrated package symbols. Continue to Phase 6 to
fix.

If build fails ONLY because of the existing `internal/cli/repo.go`,
`plugin.go`, etc. functions being unused or because they reference symbols
the new files do not provide, that is expected — those files get rewritten
in Phase 6.

To unblock the test suite in the meantime, run just the new packages:

```bash
go test ./internal/render/... ./internal/version/...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/version_cmd.go internal/cli/version_cmd_test.go cmd/liszt/main.go
```

```bash
git commit -m "feat(cli): add version subcommand and shrink main entry"
```

---

## Phase 6: Cobra Verbs

### Task 6.1: `repo` subcommand

**Files:**
- Create: `internal/cli/repo_cmd.go`
- Test: `internal/cli/repo_cmd_test.go`
- Delete (replace): `internal/cli/repo.go` is kept; we wrap its existing helpers.

- [ ] **Step 1: Inspect existing repo helpers**

Run:

```bash
cat internal/cli/repo.go
```

Note the function signatures (likely `func Repo(p Paths, args []string) error`
or similar). Adapt the cobra handler to call them.

- [ ] **Step 2: Write failing test**

Create `internal/cli/repo_cmd_test.go`:

```go
package cli

import (
	"testing"
)

func TestRepoCmd_RegisteredOnRoot(t *testing.T) {
	t.Parallel()

	found := false
	for _, c := range rootCmd.Commands() {
		if c.Use == "repo" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("repo subcommand not registered on rootCmd")
	}
}

func TestRepoAddCmd_HasURLArg(t *testing.T) {
	t.Parallel()

	var addCmd *cobra.Command
	for _, c := range rootCmd.Commands() {
		if c.Use == "repo" {
			for _, sub := range c.Commands() {
				if sub.Use == "add <url>" {
					addCmd = sub
				}
			}
		}
	}
	if addCmd == nil {
		t.Fatal("repo add subcommand not registered")
	}
	if addCmd.Args == nil {
		t.Errorf("repo add must require exactly 1 argument")
	}
}
```

Add the missing import:

```go
import "github.com/spf13/cobra"
```

- [ ] **Step 3: Run test to verify it fails**

```bash
go test ./internal/cli/...
```

Expected: build failure — `repoCmd` not registered.

- [ ] **Step 4: Write minimal implementation**

Create `internal/cli/repo_cmd.go`:

```go
package cli

import (
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mguilarducci/liszt/internal/xdg"
)

var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Manage marketplace repositories",
}

var repoAddCmd = &cobra.Command{
	Use:   "add <url>",
	Short: "Clone a marketplace repository",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		paths := defaultPaths()
		return RepoAdd(paths, args[0])
	},
}

func init() {
	repoCmd.AddCommand(repoAddCmd)
	rootCmd.AddCommand(repoCmd)
}

// defaultPaths returns the production filesystem layout. Tests inject
// custom Paths through the underlying helpers; this wrapper exists only
// for the cobra hot path.
func defaultPaths() Paths {
	return Paths{
		Repos:    filepath.Join(xdg.DataDir(), "repos.toml"),
		Manifest: "liszt.toml",
		Lock:     "liszt.lock",
		Cache:    filepath.Join(xdg.CacheDir(), "repos"),
	}
}
```

- [ ] **Step 5: Refactor `internal/cli/repo.go` to export `RepoAdd`**

Read the current `internal/cli/repo.go`. If it has a function like
`func Repo(p Paths, args []string) error` that dispatches on `args[0] == "add"`,
extract the add path into a new function:

```go
// RepoAdd clones url and upserts it into the repos.toml at p.Repos.
func RepoAdd(p Paths, url string) error {
	// (move existing add implementation here verbatim)
}
```

If `Repo` is no longer called from anywhere else, delete it.

- [ ] **Step 6: Run test to verify it passes**

```bash
go test ./internal/cli/...
```

Expected: PASS for repo tests; root + version tests still pass.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/repo_cmd.go internal/cli/repo_cmd_test.go internal/cli/repo.go
```

```bash
git commit -m "feat(cli): add cobra repo subcommand"
```

### Task 6.2: `plugin` subcommand (list + install)

**Files:**
- Create: `internal/cli/plugin_cmd.go`
- Test: `internal/cli/plugin_cmd_test.go`
- Modify: `internal/cli/plugin.go` (extract helpers if needed)

- [ ] **Step 1: Write failing test**

Create `internal/cli/plugin_cmd_test.go`:

```go
package cli

import "testing"

func TestPluginCmd_RegisteredOnRoot(t *testing.T) {
	t.Parallel()

	found := false
	for _, c := range rootCmd.Commands() {
		if c.Use == "plugin" {
			found = true
		}
	}
	if !found {
		t.Errorf("plugin subcommand not registered on rootCmd")
	}
}

func TestPluginListAndInstallRegistered(t *testing.T) {
	t.Parallel()

	wantSubs := map[string]bool{"list": false, "install <slug>": false}
	for _, c := range rootCmd.Commands() {
		if c.Use != "plugin" {
			continue
		}
		for _, sub := range c.Commands() {
			if _, ok := wantSubs[sub.Use]; ok {
				wantSubs[sub.Use] = true
			}
		}
	}
	for name, found := range wantSubs {
		if !found {
			t.Errorf("plugin %s not registered", name)
		}
	}
}

func TestPluginInstall_HasFlavorFlag(t *testing.T) {
	t.Parallel()

	var installCmd *cobra.Command
	for _, c := range rootCmd.Commands() {
		if c.Use == "plugin" {
			for _, sub := range c.Commands() {
				if sub.Use == "install <slug>" {
					installCmd = sub
				}
			}
		}
	}
	if installCmd == nil {
		t.Fatal("plugin install subcommand not found")
	}
	if installCmd.Flags().Lookup("flavor") == nil {
		t.Errorf("plugin install missing --flavor flag")
	}
}
```

Add import:

```go
import "github.com/spf13/cobra"
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/cli/...
```

Expected: build failure.

- [ ] **Step 3: Write minimal implementation**

Create `internal/cli/plugin_cmd.go`:

```go
package cli

import (
	"github.com/spf13/cobra"
)

var (
	pluginCmd = &cobra.Command{
		Use:   "plugin",
		Short: "Manage plugins",
	}

	pluginListCmd = &cobra.Command{
		Use:   "list",
		Short: "List plugins across all repos",
		RunE: func(_ *cobra.Command, _ []string) error {
			return PluginList(defaultPaths())
		},
	}

	pluginInstallFlavor string

	pluginInstallCmd = &cobra.Command{
		Use:   "install <slug>",
		Short: "Install a plugin",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return PluginInstall(defaultPaths(), args[0], pluginInstallFlavor)
		},
	}
)

func init() {
	pluginInstallCmd.Flags().StringVar(&pluginInstallFlavor, "flavor", "", "claude|copilot")
	_ = pluginInstallCmd.MarkFlagRequired("flavor")

	pluginCmd.AddCommand(pluginListCmd)
	pluginCmd.AddCommand(pluginInstallCmd)
	rootCmd.AddCommand(pluginCmd)
}
```

- [ ] **Step 4: Adjust `internal/cli/plugin.go` signatures if needed**

If `PluginInstall` currently takes `(p Paths, slug, flavor string)`, leave
it. If it takes `(p Paths, args []string)`, refactor to match the new
signature and delete the old `ParseInstallArgs` helper from
`internal/cli/install.go`.

- [ ] **Step 5: Run test to verify it passes**

```bash
go test ./internal/cli/...
```

Expected: PASS for plugin tests.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/plugin_cmd.go internal/cli/plugin_cmd_test.go internal/cli/plugin.go internal/cli/install.go
```

```bash
git commit -m "feat(cli): add cobra plugin list + install subcommands"
```

### Task 6.3: `<kind>` subcommands (resource loop)

**Files:**
- Create: `internal/cli/resource_cmd.go`
- Test: `internal/cli/resource_cmd_test.go`

- [ ] **Step 1: Inspect resource package**

```bash
cat internal/resource/resource.go
```

Confirm there is a function like `Kinds() []string` returning
`{"skill", "agent", "command", "hook", "mcp", "lsp"}`. If not, add it
before continuing.

- [ ] **Step 2: Write failing test**

Create `internal/cli/resource_cmd_test.go`:

```go
package cli

import (
	"testing"

	"github.com/mguilarducci/liszt/internal/resource"
)

func TestResourceCmds_AllKindsRegistered(t *testing.T) {
	t.Parallel()

	registered := map[string]bool{}
	for _, c := range rootCmd.Commands() {
		registered[c.Use] = true
	}
	for _, kind := range resource.Kinds() {
		if !registered[kind] {
			t.Errorf("kind %q not registered as subcommand", kind)
		}
	}
}

func TestResourceCmds_ListHasPluginFlag(t *testing.T) {
	t.Parallel()

	for _, c := range rootCmd.Commands() {
		var kind string
		for _, k := range resource.Kinds() {
			if c.Use == k {
				kind = k
			}
		}
		if kind == "" {
			continue
		}
		for _, sub := range c.Commands() {
			if sub.Use != "list" {
				continue
			}
			if sub.Flags().Lookup("plugin") == nil {
				t.Errorf("%s list missing --plugin flag", kind)
			}
		}
	}
}

func TestResourceCmds_InstallHasFlavorFlag(t *testing.T) {
	t.Parallel()

	for _, c := range rootCmd.Commands() {
		var kind string
		for _, k := range resource.Kinds() {
			if c.Use == k {
				kind = k
			}
		}
		if kind == "" {
			continue
		}
		for _, sub := range c.Commands() {
			if sub.Use != "install <slug>" {
				continue
			}
			if sub.Flags().Lookup("flavor") == nil {
				t.Errorf("%s install missing --flavor flag", kind)
			}
		}
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
go test ./internal/cli/...
```

Expected: build failure.

- [ ] **Step 4: Write minimal implementation**

Create `internal/cli/resource_cmd.go`:

```go
package cli

import (
	"github.com/spf13/cobra"

	"github.com/mguilarducci/liszt/internal/resource"
)

func init() {
	for _, kind := range resource.Kinds() {
		registerResourceCmd(kind)
	}
}

// registerResourceCmd builds the parent subcommand and its list/install
// children for a single resource kind, then attaches them to rootCmd. The
// closure captures kind so each child handler routes to the correct kind
// in the resource package.
func registerResourceCmd(kind string) {
	parent := &cobra.Command{
		Use:   kind,
		Short: "Manage " + kind + "s",
	}

	var listPlugin string
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List " + kind + "s",
		RunE: func(_ *cobra.Command, _ []string) error {
			return ResourceList(defaultPaths(), kind, listPlugin)
		},
	}
	listCmd.Flags().StringVar(&listPlugin, "plugin", "", "filter by plugin name")

	var installFlavor string
	installCmd := &cobra.Command{
		Use:   "install <slug>",
		Short: "Install a " + kind,
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return ResourceInstall(defaultPaths(), kind, args[0], installFlavor)
		},
	}
	installCmd.Flags().StringVar(&installFlavor, "flavor", "", "claude|copilot")
	_ = installCmd.MarkFlagRequired("flavor")

	parent.AddCommand(listCmd)
	parent.AddCommand(installCmd)
	rootCmd.AddCommand(parent)
}
```

- [ ] **Step 5: Ensure `resource.Kinds()` exists**

Check `internal/resource/resource.go` for a `Kinds()` function. If absent,
add:

```go
// Kinds returns the resource kind identifiers supported by the CLI.
func Kinds() []string {
	return []string{"skill", "agent", "command", "hook", "mcp", "lsp"}
}
```

- [ ] **Step 6: Run test to verify it passes**

```bash
go test ./internal/cli/...
```

Expected: PASS for resource tests.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/resource_cmd.go internal/cli/resource_cmd_test.go internal/cli/resource.go internal/resource/resource.go
```

```bash
git commit -m "feat(cli): add cobra subcommands for all resource kinds"
```

### Task 6.4: `outdated` subcommand

**Files:**
- Create: `internal/cli/outdated_cmd.go`
- Test: `internal/cli/outdated_cmd_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/cli/outdated_cmd_test.go`:

```go
package cli

import "testing"

func TestOutdatedCmd_Registered(t *testing.T) {
	t.Parallel()

	found := false
	for _, c := range rootCmd.Commands() {
		if c.Use == "outdated" {
			found = true
		}
	}
	if !found {
		t.Errorf("outdated subcommand not registered")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/cli/...
```

Expected: build failure.

- [ ] **Step 3: Write minimal implementation**

Create `internal/cli/outdated_cmd.go`:

```go
package cli

import (
	"github.com/spf13/cobra"
)

var outdatedCmd = &cobra.Command{
	Use:   "outdated",
	Short: "Compare local lock SHAs against remote HEAD",
	RunE: func(_ *cobra.Command, _ []string) error {
		return Outdated(defaultPaths())
	},
}

func init() {
	rootCmd.AddCommand(outdatedCmd)
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/cli/...
```

Expected: PASS.

- [ ] **Step 5: Run full build**

```bash
go build ./...
```

Expected: SUCCESS. All cobra verbs now wire to the existing
`internal/cli/*.go` helpers.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/outdated_cmd.go internal/cli/outdated_cmd_test.go
```

```bash
git commit -m "feat(cli): add cobra outdated subcommand"
```

---

## Phase 7: Wire Progress Bar Into Verbs

### Task 7.1: `gitx` writer plumbing

**Files:**
- Modify: `internal/gitx/git.go`
- Create: `internal/gitx/git_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/gitx/git_test.go`:

```go
package gitx

import (
	"bytes"
	"io"
	"testing"
)

func TestSetOutput_DefaultsToStderr(t *testing.T) {
	// Verifies that SetOutput returns the previous writer so callers can
	// restore it after their operation.
	prev := SetOutput(io.Discard)
	t.Cleanup(func() { SetOutput(prev) })

	got := Output()
	if got != io.Discard {
		t.Errorf("Output() did not return the configured writer")
	}
}

func TestSetOutput_ThreadSafe(t *testing.T) {
	// Smoke: just ensure concurrent calls do not race when run with -race.
	prev := SetOutput(&bytes.Buffer{})
	t.Cleanup(func() { SetOutput(prev) })

	done := make(chan struct{})
	for i := 0; i < 4; i++ {
		go func() {
			SetOutput(&bytes.Buffer{})
			_ = Output()
			done <- struct{}{}
		}()
	}
	for i := 0; i < 4; i++ {
		<-done
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/gitx/...
```

Expected: build failure — `SetOutput`, `Output` undefined.

- [ ] **Step 3: Modify `internal/gitx/git.go`**

Add at the top of the file (after imports):

```go
import (
	"sync"
	// (keep existing imports)
)

var (
	outputMu sync.RWMutex
	output   io.Writer = os.Stderr
)

// SetOutput swaps the writer that git subprocess stdout/stderr is forwarded
// to. Returns the previous writer so callers can restore it. Defaults to
// os.Stderr.
func SetOutput(w io.Writer) io.Writer {
	outputMu.Lock()
	defer outputMu.Unlock()
	prev := output
	output = w
	return prev
}

// Output returns the current subprocess output writer.
func Output() io.Writer {
	outputMu.RLock()
	defer outputMu.RUnlock()
	return output
}
```

Replace every `cmd.Stdout = os.Stderr` and `cmd.Stderr = os.Stderr` in
`git.go` with:

```go
out := Output()
cmd.Stdout = out
cmd.Stderr = out
```

(Both `EnsureClone` and `cloneInto` need this change.)

Add the missing import to `internal/gitx/git.go` if not already present:

```go
import "io"
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/gitx/...
```

Expected: PASS.

- [ ] **Step 5: Run race test**

```bash
go test -race ./internal/gitx/...
```

Expected: PASS, no data races.

- [ ] **Step 6: Commit**

```bash
git add internal/gitx/git.go internal/gitx/git_test.go
```

```bash
git commit -m "feat(gitx): add SetOutput/Output for subprocess writer plumbing"
```

### Task 7.2: `repo add` progress bar

**Files:**
- Modify: `internal/cli/repo.go` (or wherever `RepoAdd` lives after Task 6.1)

- [ ] **Step 1: Read current `RepoAdd` implementation**

```bash
cat internal/cli/repo.go
```

- [ ] **Step 2: Modify `RepoAdd` to wire a bar**

Replace the body of `RepoAdd` so it begins with:

```go
func RepoAdd(p Paths, url string) error {
	bar := render.Default.Bar("cloning " + url)
	bar.SetIndeterminate(true)
	prev := gitx.SetOutput(io.Discard)
	defer gitx.SetOutput(prev)

	dest := /* existing destination computation */
	if err := gitx.EnsureClone(url, dest); err != nil {
		bar.Fail("clone failed", "url", url, "err", err)
		return err
	}

	// existing manifest upsert logic …

	bar.Done("repo added", "url", url)
	return nil
}
```

Required imports:

```go
import (
	"io"

	"github.com/mguilarducci/liszt/internal/gitx"
	"github.com/mguilarducci/liszt/internal/render"
)
```

- [ ] **Step 3: Manual smoke test**

```bash
go build ./...
```

Expected: SUCCESS.

- [ ] **Step 4: Run all tests**

```bash
go test ./...
```

Expected: PASS. Existing tests do not exercise the bar wiring; the
testscript phase will cover it end-to-end.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/repo.go
```

```bash
git commit -m "feat(cli): wire indeterminate progress bar into repo add"
```

### Task 7.3: `plugin install` and `<kind> install` multi-step bar

**Files:**
- Modify: `internal/cli/plugin.go`
- Modify: `internal/cli/resource.go`
- Modify: `internal/cli/install.go`

- [ ] **Step 1: Identify the shared install pipeline**

```bash
cat internal/cli/install.go
```

Note the stages (resolve marketplace, clone/cache, materialize, write
lock). Confirm `PluginInstall` and `ResourceInstall` share these stages.

- [ ] **Step 2: Add a bar driver helper in `install.go`**

Append to `internal/cli/install.go`:

```go
// installBar drives the install progress bar through the four stages.
// Callers invoke each Stage* method between the corresponding work step.
type installBar struct {
	bar *render.Bar
}

func newInstallBar(label string) *installBar {
	return &installBar{bar: render.Default.Bar("installing " + label)}
}

func (b *installBar) StageResolve(slug string) {
	b.bar.Update("resolving " + slug)
	b.bar.Set(0.0)
}

func (b *installBar) StageCloneBegin(slug string) {
	b.bar.Update("cloning " + slug)
	b.bar.SetIndeterminate(true)
	b.bar.Set(0.25)
}

func (b *installBar) StageCloneEnd() {
	b.bar.SetIndeterminate(false)
	b.bar.Set(0.50)
}

func (b *installBar) StageMaterialize(slug string) {
	b.bar.Update("materializing " + slug)
	b.bar.Set(0.75)
}

func (b *installBar) StageManifest() {
	b.bar.Update("writing manifest")
	b.bar.Set(1.0)
}

func (b *installBar) Done(slug, flavor string) {
	b.bar.Done("installed", "slug", slug, "flavor", flavor)
}

func (b *installBar) Fail(msg string, kv ...any) {
	b.bar.Fail(msg, kv...)
}
```

Required import:

```go
import "github.com/mguilarducci/liszt/internal/render"
```

- [ ] **Step 3: Wire bar into `PluginInstall`**

Modify `PluginInstall` in `internal/cli/plugin.go`:

```go
func PluginInstall(p Paths, slug, flavor string) error {
	bar := newInstallBar(slug)
	prev := gitx.SetOutput(io.Discard)
	defer gitx.SetOutput(prev)

	bar.StageResolve(slug)
	// existing marketplace resolve …

	bar.StageCloneBegin(slug)
	// existing clone-or-cache-lookup …
	bar.StageCloneEnd()

	bar.StageMaterialize(slug)
	// existing materialize …

	bar.StageManifest()
	// existing manifest + lock write …

	bar.Done(slug, flavor)
	return nil
}
```

On error at any stage, call `bar.Fail("<stage> failed", "slug", slug, "err", err)`
and return.

Required imports:

```go
import (
	"io"

	"github.com/mguilarducci/liszt/internal/gitx"
)
```

- [ ] **Step 4: Apply the same wiring pattern to `ResourceInstall`**

Modify `ResourceInstall` in `internal/cli/resource.go` with the identical
stage calls. Pass `kind` into the bar label if helpful (e.g.
`installing skill/<slug>`).

- [ ] **Step 5: Run tests**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/install.go internal/cli/plugin.go internal/cli/resource.go
```

```bash
git commit -m "feat(cli): wire multi-step bar with indeterminate clone phase"
```

### Task 7.4: `outdated` progress bar

**Files:**
- Modify: `internal/cli/outdated.go`

- [ ] **Step 1: Inspect current outdated implementation**

```bash
cat internal/cli/outdated.go
```

- [ ] **Step 2: Wire the bar**

Modify `Outdated`:

```go
func Outdated(p Paths) error {
	repos := /* existing repos load */
	bar := render.Default.Bar("checking remotes")
	prev := gitx.SetOutput(io.Discard)
	defer gitx.SetOutput(prev)

	var stale []staleEntry
	for i, repo := range repos {
		bar.Update(repo.URL)
		sha, err := gitx.LsRemoteHead(repo.URL)
		if err != nil {
			bar.Fail("ls-remote failed", "url", repo.URL, "err", err)
			return err
		}
		if sha != repo.LocalSHA {
			stale = append(stale, staleEntry{URL: repo.URL, Local: repo.LocalSHA, Remote: sha})
		}
		bar.Set(float64(i+1) / float64(len(repos)))
	}

	bar.Done("checked", "repos", len(repos), "stale", len(stale))
	for _, s := range stale {
		render.Info("stale", "url", s.URL, "local", s.Local, "remote", s.Remote)
	}
	return nil
}
```

Use the actual repo struct names from the current implementation. Required
imports:

```go
import (
	"io"

	"github.com/mguilarducci/liszt/internal/gitx"
	"github.com/mguilarducci/liszt/internal/render"
)
```

- [ ] **Step 3: Run tests**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/outdated.go
```

```bash
git commit -m "feat(cli): wire determinate progress bar into outdated"
```

---

## Phase 8: End-to-End testscript Harness

### Task 8.1: testscript TestMain + TestScripts

**Files:**
- Create: `cmd/liszt/script_test.go`
- Create: `cmd/liszt/testdata/script/.gitkeep`

- [ ] **Step 1: Create the harness file**

Create `cmd/liszt/script_test.go`:

```go
package main_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/mguilarducci/liszt/internal/cli"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"liszt": func() int {
			if err := cli.Execute(context.Background()); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			return 0
		},
	}))
}

func TestScripts(t *testing.T) {
	t.Parallel()

	testscript.Run(t, testscript.Params{
		Dir: "testdata/script",
	})
}
```

- [ ] **Step 2: Create testdata directory placeholder**

```bash
mkdir -p cmd/liszt/testdata/script
```

```bash
touch cmd/liszt/testdata/script/.gitkeep
```

- [ ] **Step 3: Run tests**

```bash
go test ./cmd/liszt/...
```

Expected: PASS (zero scripts so far, harness compiles).

- [ ] **Step 4: Commit**

```bash
git add cmd/liszt/script_test.go cmd/liszt/testdata/script/.gitkeep
```

```bash
git commit -m "test(cli): add testscript end-to-end harness"
```

### Task 8.2: testscript scenarios

**Files:**
- Create: `cmd/liszt/testdata/script/version.txtar`
- Create: `cmd/liszt/testdata/script/help.txtar`
- Create: `cmd/liszt/testdata/script/no_color.txtar`

- [ ] **Step 1: Create version script**

Create `cmd/liszt/testdata/script/version.txtar`:

```
# version subcommand prints the build identifier
env NO_COLOR=1
exec liszt version
stderr 'liszt 0\.0\.0-dev'
```

- [ ] **Step 2: Create help script**

Create `cmd/liszt/testdata/script/help.txtar`:

```
# --help prints program name and lists verbs
env NO_COLOR=1
exec liszt --help
stdout 'liszt'
stdout 'repo'
stdout 'plugin'
stdout 'outdated'
```

- [ ] **Step 3: Create no-color script**

Create `cmd/liszt/testdata/script/no_color.txtar`:

```
# --no-color flag strips ANSI from any output
exec liszt --no-color version
! stderr '\x1b\['
stderr 'liszt'
```

- [ ] **Step 4: Run tests**

```bash
go test ./cmd/liszt/...
```

Expected: PASS for all three scripts.

- [ ] **Step 5: Run full test suite once more**

```bash
go test ./...
```

Expected: PASS across every package.

- [ ] **Step 6: Run with race detector**

```bash
go test -race ./...
```

Expected: PASS with no races. If race detected in `render.Bar`, re-read
Task 4.1 and verify all `Bar` mutations use atomics under
`Renderer.mu` boundaries.

- [ ] **Step 7: Commit**

```bash
git add cmd/liszt/testdata/script/version.txtar cmd/liszt/testdata/script/help.txtar cmd/liszt/testdata/script/no_color.txtar
```

```bash
git commit -m "test(cli): add testscript scenarios for version, help, no-color"
```

---

## Self-Review Notes

This plan was reviewed against the spec on 2026-05-18. Coverage map:

| Spec section          | Plan task(s)                                       |
|-----------------------|-----------------------------------------------------|
| §3 Architecture       | All phases                                          |
| §4 Palette            | Task 2.1                                            |
| §5 render package     | Tasks 2.1–4.3                                       |
| §6 cobra migration    | Tasks 5.1–6.4                                       |
| §7 Version package    | Task 1.2                                            |
| §8 Wire bar           | Tasks 7.1–7.4                                       |
| §9 Non-TTY behavior   | Tasks 2.2, 4.1, 4.3                                 |
| §10 Error handling    | Tasks 3.1, 4.1, 7.2, 7.3, 7.4                       |
| §11 Testing           | Tests embedded in every task; Phase 8 e2e           |
| §12 Dependencies      | Task 1.1                                            |
| §13 Delivery order    | Matches phase order                                 |
| §14 Risks             | Mitigations live in Tasks 4.1 (race), 7.1 (writer)  |
| §15 Out of scope      | No tasks for spinner or adaptive palette            |

Type consistency:

- `render.Bar` returns `*Bar`; `*Bar` exposes `Set`, `SetIndeterminate`,
  `Update`, `Stop`, `Done`, `Fail` (same names in spec §5 and plan tasks).
- `RepoAdd`, `PluginInstall`, `ResourceInstall`, `Outdated`, `PluginList`,
  `ResourceList` are the helper names used consistently across Tasks
  6.x and 7.x.
- `gitx.SetOutput(w)` returns the previous `io.Writer` (Task 7.1 + callers
  in Tasks 7.2–7.4).
- `render.Default.Bar(label)` instead of the top-level `render.Bar` because
  the spec keeps the type-vs-function naming separate; package-level
  delegates are `render.Info/Warn/Error/Done/Header/Subheader/Hint`.

No placeholders or TODOs remain.
