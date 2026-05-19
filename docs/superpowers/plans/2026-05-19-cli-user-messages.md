# CLI User-Facing Messages Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace debug-style `render.Info` logs with a semantic verb vocabulary (Step / StepDone / StepFail / Done / Fail / Hint / Warn / Detail) gated by `-v`, wrap the existing styled `render.Bar` with a `Progress` driver, and refactor `repo add` so it preflights the registry and refuses already-registered repos with a clear hint.

**Architecture:** All changes land in three packages.

- `internal/render` gains new verbs with new glyph-prefixed line formats (`⠇`, `✓`, `✗`, `✔`, `✖`, `·`), a `Progress` wrapper around the existing `Bar`, a verbose toggle (`SetVerbose`), and a renamed `Error → Fail`. `Info` is removed.
- `internal/repos` gains a `Find(name)` helper used by the preflight check.
- `internal/cli` adds a persistent `-v`/`--verbose` flag wired to `render.SetVerbose`, refactors `RepoAdd` to fail fast with a new `ErrAlreadyAdded` sentinel, drives the 5-step flow through `Progress`, and mechanically migrates other commands' `Info`/`Warn` payloads to `Detail`.

**Tech Stack:** Go 1.24, `github.com/spf13/cobra`, `charm.land/lipgloss/v2`, `github.com/charmbracelet/colorprofile`, `github.com/charmbracelet/fang`, `github.com/pelletier/go-toml/v2`, stdlib `testing`, `github.com/rogpeppe/go-internal/testscript`.

**Spec:** `docs/superpowers/specs/2026-05-19-cli-user-messages-design.md`.

**Visual reference (after this plan ships):**

`repo add` happy path (no `-v`):

```
✓ Resolved obra/superpowers
✓ Not yet registered
✓ Cloned obra/superpowers
✓ Read marketplace.json
✓ Saved to repos.toml
✔ Added obra/superpowers
  marketplace: superpowers-dev
  plugins:     1
→ Run `liszt plugin list` to see available plugins
→ Run `liszt plugin install <name>` to install
```

`repo add` mid-clone (one frame):

```
✓ Resolved obra/superpowers
✓ Not yet registered
▌ step  ♬♪♩♫♫♪♬♩♪♫·············  40%  Cloning obra/superpowers
```

`repo add` on already-registered repo:

```
✓ Resolved obra/superpowers
✖ obra/superpowers already added
→ Run `liszt repo update obra/superpowers` to refresh
```

**Line format conventions locked by this plan:**

| Verb | Format | Notes |
|---|---|---|
| `Step(msg)` | `⠇ <msg>` | persistent line; static glyph (not animated) |
| `StepDone(msg)` | `✓ <msg>` | dim |
| `StepFail(msg, err)` | `✗ <msg>: <err>` | error color |
| `Done(msg, kv...)` | `✔ <msg>\n  key: value\n` | done color; kv rendered as indented `key: value` block |
| `Fail(msg, kv...)` | `✖ <msg>\n  key: value\n` | error color; replaces former `Error` |
| `Warn(msg)` | `! <msg>` | warn color; **no kv** — payload moves to `Detail` |
| `Detail(msg, kv...)` | `· <msg> key=value` dim | suppressed when verbose=false |
| `Hint(msg)` | unchanged (dim italic, no prefix) | caller embeds `→ ` in `msg` for post-Done CTAs |
| Bar repaint | `▌ step  ♬♪♩♫…  40%  label` | bar's internal prefix label changes from `info` to `step` |

**Decomposition note:** Tasks 1–6 build the render layer in isolation. Each task ships green tests against new API surface without touching CLI callers. Tasks 7–13 wire and migrate callers. Task 14 removes `Info`. Task 15 adds integration coverage. The CLI compiles and tests pass after every task.

---

## Task 1: Render — `Detail` verb + `SetVerbose` toggle

**Files:**
- Create: `internal/render/detail.go`
- Create: `internal/render/detail_test.go`
- Modify: `internal/render/renderer.go` (add `verbose` field)
- Modify: `internal/render/default.go` (add `SetVerbose` + `Detail` delegates)

**Context:** `Detail` is the dim, verbose-gated diagnostic verb. Format: `· <msg> k=v` rendered in `styDim`. When `verbose=false` (default), `Detail` returns immediately without writing. `SetVerbose` sets the toggle on the package-level `Default` renderer.

### Steps

- [ ] **Step 1: Write the failing tests**

Create `internal/render/detail_test.go`:

```go
package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

func TestDetail_SuppressedByDefault(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := New(&buf, WithProfile(colorprofile.NoTTY))
	r.Detail("loading", "path", "/tmp/x")
	if buf.Len() != 0 {
		t.Errorf("Detail must be silent when verbose=false: %q", buf.String())
	}
}

func TestDetail_EmittedWhenVerbose(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := New(&buf, WithProfile(colorprofile.NoTTY))
	r.SetVerbose(true)
	r.Detail("loading", "path", "/tmp/x")
	got := buf.String()
	if !strings.HasPrefix(got, "· loading") {
		t.Errorf("Detail missing `· ` prefix or message: %q", got)
	}
	if !strings.Contains(got, "path=/tmp/x") {
		t.Errorf("Detail missing kv pair: %q", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("Detail missing trailing newline: %q", got)
	}
}

func TestDetail_NoKVOmitsPayload(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := New(&buf, WithProfile(colorprofile.NoTTY))
	r.SetVerbose(true)
	r.Detail("ping")
	got := buf.String()
	if got != "· ping\n" {
		t.Errorf("Detail with no kv should be `· ping\\n`, got %q", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/render -run TestDetail -v`
Expected: FAIL with `r.Detail undefined` (compile error).

- [ ] **Step 3: Add `verbose` field + `SetVerbose` to Renderer**

Modify `internal/render/renderer.go`. Inside the `Renderer` struct add the field:

```go
type Renderer struct {
	w       io.Writer
	profile colorprofile.Profile
	isTTY   bool
	mu      sync.Mutex
	rng     *rand.Rand
	active  anim
	verbose bool
}
```

Append a setter at the end of the file:

```go
// SetVerbose toggles emission of Detail lines. Default false.
func (r *Renderer) SetVerbose(on bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.verbose = on
}
```

- [ ] **Step 4: Implement `Detail`**

Create `internal/render/detail.go`:

```go
package render

import (
	"strings"
)

// Detail prints a dim diagnostic line of the form `· msg k=v ...`. It is
// suppressed unless SetVerbose(true) has been called on the receiver.
//
// Detail is the home for technical payload (paths, SHAs, URLs) that used
// to ride along on Info lines in earlier versions of the CLI.
func (r *Renderer) Detail(msg string, kv ...any) {
	r.mu.Lock()
	if !r.verbose {
		r.mu.Unlock()
		return
	}
	r.mu.Unlock()

	var sb strings.Builder
	sb.WriteString("· ")
	sb.WriteString(msg)
	if len(kv) > 0 {
		sb.WriteString(" ")
		sb.WriteString(formatKV(kv))
	}
	sb.WriteString("\n")
	line := styDim.Render(sb.String())

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active != nil {
		r.eraseLine()
	}
	r.writeString(line)
	if r.active != nil {
		active := r.active
		r.mu.Unlock()
		active.repaint()
		r.mu.Lock()
	}
}
```

- [ ] **Step 5: Add package-level delegates**

Modify `internal/render/default.go`. Append:

```go
// Detail delegates to Default.Detail.
func Detail(msg string, kv ...any) { ensureDefault().Detail(msg, kv...) }

// SetVerbose toggles Detail emission on the package-level Default renderer.
func SetVerbose(on bool) { ensureDefault().SetVerbose(on) }
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/render -run TestDetail -v`
Expected: PASS for all three tests.

- [ ] **Step 7: Run full render test suite to confirm no regression**

Run: `go test ./internal/render -v`
Expected: PASS (existing tests remain green).

- [ ] **Step 8: Commit**

```bash
git add internal/render/detail.go internal/render/detail_test.go \
        internal/render/renderer.go internal/render/default.go
git commit -m "feat(render): add Detail verb gated by SetVerbose"
```

---

## Task 2: Render — `Step` / `StepDone` / `StepFail` verbs

**Files:**
- Create: `internal/render/step.go`
- Create: `internal/render/step_test.go`
- Modify: `internal/render/default.go` (add delegates)

**Context:** These three verbs emit persistent glyph-prefixed lines that sit above the live bar (when a bar is active, the existing `writeLine`-style mid-print erase/repaint logic handles redraw). `Step` is the in-flight indicator (rare standalone use; `Progress` is the common driver). `StepDone` is the dim ✓ that marks a finished phase. `StepFail` carries the error inline.

### Steps

- [ ] **Step 1: Write the failing tests**

Create `internal/render/step_test.go`:

```go
package render

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

func plainRenderer(buf *bytes.Buffer) *Renderer {
	return New(buf, WithProfile(colorprofile.NoTTY))
}

func TestStep_PrintsGlyphAndMessage(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	plainRenderer(&buf).Step("Cloning obra/superpowers")
	got := buf.String()
	if got != "⠇ Cloning obra/superpowers\n" {
		t.Errorf("Step output mismatch: %q", got)
	}
}

func TestStepDone_PrintsCheckAndMessage(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	plainRenderer(&buf).StepDone("Cloned obra/superpowers")
	got := buf.String()
	if got != "✓ Cloned obra/superpowers\n" {
		t.Errorf("StepDone output mismatch: %q", got)
	}
}

func TestStepFail_IncludesErrorInline(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	plainRenderer(&buf).StepFail("Cloning obra/superpowers", errors.New("permission denied"))
	got := buf.String()
	want := "✗ Cloning obra/superpowers: permission denied\n"
	if got != want {
		t.Errorf("StepFail mismatch:\nwant %q\ngot  %q", want, got)
	}
}

func TestStepFail_NilErrorOmitsColon(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	plainRenderer(&buf).StepFail("Cloning", nil)
	got := buf.String()
	if !strings.Contains(got, "✗ Cloning") {
		t.Errorf("StepFail nil error missing message: %q", got)
	}
	if strings.Contains(got, ":") {
		t.Errorf("StepFail nil error should not include colon: %q", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/render -run "TestStep" -v`
Expected: FAIL with `r.Step undefined`, `r.StepDone undefined`, `r.StepFail undefined`.

- [ ] **Step 3: Implement the verbs**

Create `internal/render/step.go`:

```go
package render

import "strings"

// Step emits `⠇ <msg>` as a persistent line above any active bar. The glyph
// is static; the bar (when present) carries the live animation.
func (r *Renderer) Step(msg string) {
	r.writeGlyphLine("⠇ "+msg+"\n", nil)
}

// StepDone emits `✓ <msg>` dim. Used to mark a finished progress phase.
func (r *Renderer) StepDone(msg string) {
	r.writeGlyphLine("✓ "+msg+"\n", &styDim)
}

// StepFail emits `✗ <msg>: <err>` in the error color. A nil err drops the
// trailing `: <err>` portion so a bare failed-step line is still legible.
func (r *Renderer) StepFail(msg string, err error) {
	var b strings.Builder
	b.WriteString("✗ ")
	b.WriteString(msg)
	if err != nil {
		b.WriteString(": ")
		b.WriteString(err.Error())
	}
	b.WriteString("\n")
	r.writeGlyphLine(b.String(), &styErrorLbl)
}

// writeGlyphLine writes a glyph-prefixed verb line through the same
// mid-print erase/repaint dance writeLine uses. If sty is non-nil the
// raw string is wrapped in that style before write.
func (r *Renderer) writeGlyphLine(raw string, sty *lipglossStyle) {
	out := raw
	if sty != nil {
		out = sty.Render(raw)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active != nil {
		r.eraseLine()
	}
	r.writeString(out)
	if r.active != nil {
		active := r.active
		r.mu.Unlock()
		active.repaint()
		r.mu.Lock()
	}
}
```

The `lipglossStyle` alias keeps the helper's signature short; add it at the top of `step.go` below the import line:

```go
import (
	"strings"

	"charm.land/lipgloss/v2"
)

type lipglossStyle = lipgloss.Style
```

- [ ] **Step 4: Add package-level delegates**

Modify `internal/render/default.go`. Append:

```go
// Step delegates to Default.Step.
func Step(msg string) { ensureDefault().Step(msg) }

// StepDone delegates to Default.StepDone.
func StepDone(msg string) { ensureDefault().StepDone(msg) }

// StepFail delegates to Default.StepFail.
func StepFail(msg string, err error) { ensureDefault().StepFail(msg, err) }
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/render -run "TestStep" -v`
Expected: PASS (all 4 tests).

- [ ] **Step 6: Run full render test suite**

Run: `go test ./internal/render -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/render/step.go internal/render/step_test.go internal/render/default.go
git commit -m "feat(render): add Step, StepDone, StepFail verbs"
```

---

## Task 3: Render — Bar prefix label `info` → `step`

**Files:**
- Modify: `internal/render/theme.go` (add `lblStep`)
- Modify: `internal/render/bar.go` (use `lblStep` in `repaint`)
- Modify: `internal/render/bar_test.go` (update assertions that match `"info"`)

**Context:** The animated bar currently labels itself `▌ info` via `styInfoLbl.Render(lblInfo)` at `internal/render/bar.go:85`. With `Info` removed and `Step` introduced, the bar reads `▌ step  ♬♪♩♫…  40%  <label>`. The pink-on-Gleam visual stays the same — only the literal string and which constant it references change. Two existing bar tests assert the literal `"info"` substring and need to flip to `"step"`.

### Steps

- [ ] **Step 1: Add the new label constant + style alias**

Modify `internal/render/theme.go`. In the styles block, add a step-tinted style aliased to the existing info color (visual continuity — same blue):

```go
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
	styStepBar  = lipgloss.NewStyle().Foreground(cInfo)

	styInfoLbl  = lipgloss.NewStyle().Foreground(cInfo).Bold(true)
	styDoneLbl  = lipgloss.NewStyle().Foreground(cDone).Bold(true)
	styWarnLbl  = lipgloss.NewStyle().Foreground(cWarn).Bold(true)
	styErrorLbl = lipgloss.NewStyle().Foreground(cError).Bold(true)
	styStepLbl  = lipgloss.NewStyle().Foreground(cInfo).Bold(true)
)
```

In the constants block, append `lblStep`:

```go
const (
	lblInfo  = "info "
	lblDone  = "done "
	lblWarn  = "warn "
	lblError = "error"
	lblStep  = "step "
)
```

- [ ] **Step 2: Switch bar repaint to the new label**

Modify `internal/render/bar.go`. In `repaint` (currently line 84-85), change the bar prefix and label style references:

```go
bar := styStepBar.Render("▌")
lbl := styStepLbl.Render(lblStep)
```

Leave the rest of `repaint` unchanged.

Also update the non-TTY one-shot branch at `internal/render/bar.go:44`:

```go
line := r.formatLine(styStepBar, styStepLbl, lblStep, label, nil)
```

- [ ] **Step 3: Update bar tests that assert the literal `"info"`**

Modify `internal/render/bar_test.go`. Two test functions reference the literal string `"info"`:

`TestBar_NonTTYSingleLineOnConstruction` (around line 108):

```go
func TestBar_NonTTYSingleLineOnConstruction(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := New(&buf) // bytes.Buffer is non-TTY
	r.Bar("hello")
	got := buf.String()
	if !strings.Contains(got, "step") {
		t.Errorf("non-TTY bar should emit step line: %q", got)
	}
	if !strings.Contains(got, "hello") {
		t.Errorf("non-TTY bar missing label text: %q", got)
	}
}
```

`TestBar_MidPrintInterruptRepaints` (around line 148): the test prints an `r.Info(...)` line then asserts the bar repainted. Replace the `Info` call with `Step` and adjust the assertion:

```go
func TestBar_MidPrintInterruptRepaints(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := newTTYRenderer(&buf)
	b := r.Bar("running")
	b.Set(0.5)
	r.Step("interrupting message")
	got := buf.String()
	if !strings.Contains(got, "interrupting message") {
		t.Errorf("Step line missing: %q", got)
	}
	if !strings.Contains(got, "running") {
		t.Errorf("bar did not repaint after Step: %q", got)
	}
	b.Stop()
}
```

- [ ] **Step 4: Run bar tests**

Run: `go test ./internal/render -run TestBar -v`
Expected: PASS (all bar tests, including the updated two).

- [ ] **Step 5: Run full render suite**

Run: `go test ./internal/render -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/render/theme.go internal/render/bar.go internal/render/bar_test.go
git commit -m "refactor(render): relabel bar prefix info -> step"
```

---

## Task 4: Render — rename `Error` → `Fail` (atomic across callers)

**Files:**
- Modify: `internal/render/message.go` (rename `Error` method to `Fail`, change line format)
- Modify: `internal/render/default.go` (rename `Error` delegate to `Fail`)
- Modify: `internal/render/message_test.go` (rename `error` case to `fail`)
- Modify: `internal/render/bar.go` (`Bar.Fail` calls `r.Fail`, not `r.Error`)
- Modify: `internal/render/bar_test.go` (assertion updated)
- Modify: `internal/cli/repo.go` (4 `render.Error` call sites → `render.Fail`)
- Modify: `internal/cli/root.go` (doc comment mentioning `render.Info`)

**Context:** Spec rename. No alias kept — `render` is `internal`, no external consumers. The line format also changes: `Fail` now emits `✖ <msg>\n  key: value\n` (summary block) instead of the bar-prefixed `▌ error  msg  kv` format. This is a coordinated edit; render and all callers move in one commit so the build stays green.

### Steps

- [ ] **Step 1: Update the failing-level test first**

Modify `internal/render/message_test.go`. In `TestEachLevelLabel` drop the `error` row and add a separate `TestFail_SummaryBlock`:

```go
func TestEachLevelLabel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		fn    func(*Renderer, string, ...any)
		label string
	}{
		{"info", (*Renderer).Info, "info "},
		{"warn", (*Renderer).Warn, "warn "},
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

func TestFail_GlyphAndSummaryBlock(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	newTestRenderer(&buf).Fail("repos save failed", "path", "/tmp/x", "err", "permission denied")
	got := buf.String()
	if !strings.HasPrefix(got, "✖ repos save failed\n") {
		t.Errorf("Fail missing glyph header: %q", got)
	}
	if !strings.Contains(got, "  path: /tmp/x\n") {
		t.Errorf("Fail missing indented kv path: %q", got)
	}
	if !strings.Contains(got, "  err:  permission denied\n") {
		t.Errorf("Fail missing indented kv err: %q", got)
	}
}

func TestFail_NoKVOmitsSummary(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	newTestRenderer(&buf).Fail("boom")
	got := buf.String()
	if got != "✖ boom\n" {
		t.Errorf("Fail with no kv mismatch: %q", got)
	}
}
```

Run: `go test ./internal/render -run "TestFail|TestEachLevelLabel" -v`
Expected: FAIL (`Fail` undefined; `error` case removed but `Error` still on Renderer — that's fine, we're about to remove it).

- [ ] **Step 2: Add `Fail` (new format) + a `Done` upgrade to the same summary-block format**

Modify `internal/render/message.go`. Replace the file contents with:

```go
package render

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// Info prints an informational line. Kept temporarily during the migration
// to the new vocabulary — Task 14 removes this method.
func (r *Renderer) Info(msg string, kv ...any) {
	r.writeLine(styInfoBar, styInfoLbl, lblInfo, msg, kv)
}

// Warn prints `! <msg>` in the warn color. Kv payload is dropped (callers
// pair Warn with Detail for technical context).
func (r *Renderer) Warn(msg string, _ ...any) {
	r.writeGlyphLine(styWarnLbl.Render("! "+msg)+"\n", nil)
}

// Done prints `✔ <msg>` followed by an indented `key: value` summary block.
func (r *Renderer) Done(msg string, kv ...any) {
	r.writeSummaryBlock(styDoneLbl, "✔ "+msg, kv)
}

// Fail prints `✖ <msg>` followed by an indented `key: value` summary block.
// Callers should still return the underlying error from RunE so fang prints
// its own styled terminal error.
func (r *Renderer) Fail(msg string, kv ...any) {
	r.writeSummaryBlock(styErrorLbl, "✖ "+msg, kv)
}

func (r *Renderer) writeSummaryBlock(headerSty lipgloss.Style, header string, kv []any) {
	var sb strings.Builder
	sb.WriteString(headerSty.Render(header))
	sb.WriteString("\n")
	if len(kv) > 0 {
		sb.WriteString(formatSummary(kv))
	}
	r.writeGlyphLine(sb.String(), nil)
}

// formatSummary renders kv pairs as aligned `  key: value\n` lines.
func formatSummary(kv []any) string {
	keys := make([]string, 0, (len(kv)+1)/2)
	vals := make([]string, 0, (len(kv)+1)/2)
	width := 0
	for i := 0; i < len(kv); i += 2 {
		k := fmt.Sprint(kv[i])
		var v string
		if i+1 >= len(kv) {
			v = "<missing>"
		} else {
			v = fmt.Sprint(kv[i+1])
		}
		if len(k) > width {
			width = len(k)
		}
		keys = append(keys, k)
		vals = append(vals, v)
	}
	var sb strings.Builder
	for i, k := range keys {
		pad := strings.Repeat(" ", width-len(k))
		sb.WriteString("  ")
		sb.WriteString(k)
		sb.WriteString(":")
		sb.WriteString(pad)
		sb.WriteString(" ")
		sb.WriteString(vals[i])
		sb.WriteString("\n")
	}
	return sb.String()
}

// writeLine emits the bar-prefixed `▌ label  msg  kv` format used by Info
// (transitional) and the underlying Bar repaint.
func (r *Renderer) writeLine(barSty, lblSty lipgloss.Style, label, msg string, kv []any) {
	line := r.formatLine(barSty, lblSty, label, msg, kv)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active != nil {
		r.eraseLine()
	}
	r.writeString(line)
	if r.active != nil {
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

This file now defines: `Info` (transitional), `Warn` (new format, drops kv), `Done` (new summary-block format), `Fail` (new), the internal `writeSummaryBlock`/`formatSummary` helpers, and the existing `writeLine`/`formatLine`/`formatKV` retained because Bar repaint still needs them. `Error` is removed.

- [ ] **Step 3: Update `default.go` — rename `Error` delegate to `Fail`**

Modify `internal/render/default.go`. Replace the `Error` function with `Fail`:

```go
// Fail delegates to Default.Fail.
func Fail(msg string, kv ...any) { ensureDefault().Fail(msg, kv...) }
```

Delete the prior `Error` delegate (the 3-line block at `default.go:30-32`).

- [ ] **Step 4: Update `Bar.Fail` to call `r.Fail`**

Modify `internal/render/bar.go:158`. Replace:

```go
// Fail stops the bar and prints a Fail line.
func (b *Bar) Fail(msg string, kv ...any) {
	b.Stop()
	b.r.Fail(msg, kv...)
}
```

- [ ] **Step 5: Update `TestBar_FailEmitsErrorLine`**

The test currently asserts `"error"` substring (from the old `▌ error` format). New format emits `✖`. Modify `internal/render/bar_test.go`:

```go
func TestBar_FailEmitsErrorLine(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	b := newTTYRenderer(&buf).Bar("label")
	b.Fail("boom", "err", "network")
	got := buf.String()
	if !strings.Contains(got, "✖ boom") {
		t.Errorf("Fail did not emit ✖ glyph: %q", got)
	}
	if !strings.Contains(got, "  err: network") {
		t.Errorf("Fail did not include summary kv: %q", got)
	}
}
```

Also update `TestBar_DoneEmitsSuccessLine` which asserts the old `done ` substring:

```go
func TestBar_DoneEmitsSuccessLine(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	b := newTTYRenderer(&buf).Bar("label")
	b.Done("installed", "slug", "x")
	got := buf.String()
	if !strings.Contains(got, "✔ installed") {
		t.Errorf("Done did not emit ✔ glyph: %q", got)
	}
	if !strings.Contains(got, "  slug: x") {
		t.Errorf("Done did not include summary kv: %q", got)
	}
}
```

- [ ] **Step 6: Update the 4 `render.Error` call sites in `cli/repo.go`**

Modify `internal/cli/repo.go`. Rename each `render.Error(` to `render.Fail(`:

- Line 23: `render.Fail("parse url failed", "url", url, "err", err)`
- Line 40: `render.Fail("head-sha failed", "dest", dest, "err", err)`
- Line 56: `render.Fail("repos load failed", "path", p.Repos, "err", err)`
- Line 61: `render.Fail("repos save failed", "path", p.Repos, "err", err)`

(The wider RepoAdd refactor lands in Task 9 — this step only renames.)

- [ ] **Step 7: Update the doc comment in `cli/root.go`**

Modify `internal/cli/root.go:57`. The comment references `render.Info`; update for accuracy:

```go
// gleamColorScheme maps the Gleam palette onto fang's ColorScheme so help,
// version, and error output share the same look-and-feel as render.Step,
// render.Bar, etc. The function signature matches fang.ColorSchemeFunc and
```

- [ ] **Step 8: Run render tests**

Run: `go test ./internal/render -v`
Expected: PASS (all existing + new `TestFail_*` tests).

- [ ] **Step 9: Run full test suite to catch any straggler caller**

Run: `go test ./...`
Expected: PASS. Build errors would surface any missed `render.Error` reference; fix any in this commit before proceeding.

- [ ] **Step 10: Commit**

```bash
git add internal/render/message.go internal/render/message_test.go \
        internal/render/default.go internal/render/bar.go internal/render/bar_test.go \
        internal/cli/repo.go internal/cli/root.go
git commit -m "refactor(render): rename Error to Fail, switch Done/Fail to summary block"
```

---

## Task 5: Render — `Progress` wrapper

**Files:**
- Create: `internal/render/progress.go`
- Create: `internal/render/progress_test.go`
- Modify: `internal/render/default.go` (add `NewProgress` delegate)

**Context:** `Progress` orchestrates the bar across N steps: each `Step` call advances `current`, updates the bar's label, and emits a `✓ <prev label>` persistent line for the prior step. Behind the scenes it owns one `*Bar` from `Renderer.Bar`. `Done` emits the final `✓` for the in-flight step then calls `Bar.Done` (which now uses the summary-block format from Task 4). `StepFail` short-circuits with `✗` + bar.Fail.

### Steps

- [ ] **Step 1: Write the failing tests**

Create `internal/render/progress_test.go`:

```go
package render

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestProgress_StepEmitsPriorCheckmark(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := newTTYRenderer(&buf)
	p := r.NewProgress(3)

	p.Step("Resolving")
	p.Step("Cloning")

	got := buf.String()
	if !strings.Contains(got, "✓ Resolving") {
		t.Errorf("second Step should print prior ✓ Resolving line: %q", got)
	}
	if !strings.Contains(got, "Cloning") {
		t.Errorf("bar should now carry Cloning label: %q", got)
	}
	p.Done("ok")
}

func TestProgress_DoneEmitsFinalCheckAndSummary(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := newTTYRenderer(&buf)
	p := r.NewProgress(2)
	p.Step("Resolving")
	p.Step("Saving")
	p.Done("Added obra/superpowers", "plugins", 1)

	got := buf.String()
	if !strings.Contains(got, "✓ Saving") {
		t.Errorf("Done should emit final ✓ for current step: %q", got)
	}
	if !strings.Contains(got, "✔ Added obra/superpowers") {
		t.Errorf("Done should emit success header: %q", got)
	}
	if !strings.Contains(got, "  plugins: 1") {
		t.Errorf("Done should emit summary kv: %q", got)
	}
}

func TestProgress_StepFailMarksFailure(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := newTTYRenderer(&buf)
	p := r.NewProgress(3)
	p.Step("Resolving")
	p.Step("Cloning")
	p.StepFail(errors.New("network down"))

	got := buf.String()
	if !strings.Contains(got, "✓ Resolving") {
		t.Errorf("prior step should still be ✓: %q", got)
	}
	if !strings.Contains(got, "✗ Cloning: network down") {
		t.Errorf("failing step should emit ✗ with err: %q", got)
	}
}

func TestProgress_NonTTYEmitsOneLinePerStep(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := New(&buf) // bytes.Buffer is non-TTY
	p := r.NewProgress(2)
	p.Step("Resolving")
	p.Step("Saving")
	p.Done("ok")

	got := buf.String()
	if !strings.Contains(got, "Resolving") || !strings.Contains(got, "Saving") {
		t.Errorf("non-TTY progress should print both step labels: %q", got)
	}
}
```

Run: `go test ./internal/render -run TestProgress -v`
Expected: FAIL — `r.NewProgress` undefined.

- [ ] **Step 2: Implement `Progress`**

Create `internal/render/progress.go`:

```go
package render

// Progress drives a single Bar across a known number of phases. Step
// advances the bar percentage, updates its label, and emits a persistent
// `✓ <previous label>` line for the just-finished phase. The first Step
// has no previous label and only starts the bar.
type Progress struct {
	r       *Renderer
	bar     *Bar
	total   int
	current int
	label   string
	failed  bool
}

// NewProgress constructs a determinate Progress with total steps. The bar
// starts at 0%. Use Step to advance, Done or StepFail to terminate.
func (r *Renderer) NewProgress(total int) *Progress {
	return &Progress{
		r:     r,
		bar:   r.Bar(""),
		total: total,
	}
}

// Step advances the bar to the next phase. If a previous step is in
// flight, its label is committed as a `✓` line. Calling Step more times
// than `total` is tolerated (bar caps at 100%) but indicates a caller
// mismatch worth fixing.
func (p *Progress) Step(label string) {
	if p.failed {
		return
	}
	if p.current > 0 && p.label != "" {
		p.r.StepDone(p.label)
	}
	p.current++
	if p.total > 0 {
		pct := float64(p.current) / float64(p.total)
		if pct > 1 {
			pct = 1
		}
		p.bar.Set(pct)
	}
	p.bar.Update(label)
	p.label = label
}

// Done emits a final `✓ <current label>` line, then calls Bar.Done which
// prints the `✔ msg` summary block.
func (p *Progress) Done(msg string, kv ...any) {
	if p.failed {
		return
	}
	if p.label != "" {
		p.r.StepDone(p.label)
	}
	p.bar.Set(1)
	p.bar.Done(msg, kv...)
}

// StepFail stops the bar with an `✗ <current label>: <err>` line. Any
// subsequent Step / Done call on this Progress is a no-op.
func (p *Progress) StepFail(err error) {
	if p.failed {
		return
	}
	p.failed = true
	p.bar.Stop()
	p.r.StepFail(p.label, err)
}
```

- [ ] **Step 3: Add the package-level delegate**

Modify `internal/render/default.go`. Append:

```go
// NewProgress delegates to Default.NewProgress.
func NewProgress(total int) *Progress { return ensureDefault().NewProgress(total) }
```

- [ ] **Step 4: Run progress tests**

Run: `go test ./internal/render -run TestProgress -v`
Expected: PASS.

- [ ] **Step 5: Run full render suite**

Run: `go test ./internal/render -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/render/progress.go internal/render/progress_test.go internal/render/default.go
git commit -m "feat(render): add Progress wrapper over Bar"
```

---

## Task 6: Repos — `Find(name)` helper

**Files:**
- Modify: `internal/repos/repos.go`
- Create: `internal/repos/repos_test.go` (if it does not yet exist) or modify it

**Context:** Preflight needs to ask "is this name already in the registry?" Add a lookup that returns the pointer and an `ok` flag — standard Go map-style return.

### Steps

- [ ] **Step 1: Check whether a repos_test.go already exists**

Run: `ls internal/repos`
Expected: outputs `repos.go` (and possibly a test file). If no test file exists, create one in Step 2; otherwise append.

- [ ] **Step 2: Write the failing tests**

Create or append to `internal/repos/repos_test.go`:

```go
package repos

import "testing"

func TestFind_Hit(t *testing.T) {
	t.Parallel()

	c := &Config{Repos: []Entry{
		{Name: "a/b", URL: "https://x", SHA: "1"},
		{Name: "c/d", URL: "https://y", SHA: "2"},
	}}
	got, ok := c.Find("c/d")
	if !ok {
		t.Fatalf("Find(c/d) returned ok=false")
	}
	if got.URL != "https://y" {
		t.Errorf("Find returned wrong entry: %+v", got)
	}
}

func TestFind_Miss(t *testing.T) {
	t.Parallel()

	c := &Config{Repos: []Entry{{Name: "a/b"}}}
	got, ok := c.Find("z/z")
	if ok {
		t.Errorf("Find(z/z) on absent name should be ok=false, got %+v", got)
	}
}

func TestFind_EmptyConfig(t *testing.T) {
	t.Parallel()

	c := &Config{}
	if _, ok := c.Find("a/b"); ok {
		t.Errorf("Find on empty config should be ok=false")
	}
}
```

Run: `go test ./internal/repos -run TestFind -v`
Expected: FAIL — `c.Find` undefined.

- [ ] **Step 3: Implement `Find`**

Modify `internal/repos/repos.go`. Append:

```go
// Find returns the entry with the given name and true if present.
// The returned pointer aliases the slice element; callers must not
// retain it across mutations of c.Repos.
func (c *Config) Find(name string) (*Entry, bool) {
	for i, r := range c.Repos {
		if r.Name == name {
			return &c.Repos[i], true
		}
	}
	return nil, false
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/repos -v`
Expected: PASS (all 3 Find cases + any prior tests).

- [ ] **Step 5: Commit**

```bash
git add internal/repos/repos.go internal/repos/repos_test.go
git commit -m "feat(repos): add Config.Find lookup helper"
```

---

## Task 7: CLI root — `-v`/`--verbose` PersistentFlag

**Files:**
- Modify: `internal/cli/root.go`
- Modify: `internal/cli/root_test.go`

**Context:** `cobra` PersistentFlag attached to `rootCmd` so every subcommand inherits it. The existing `PersistentPreRun` handles `--no-color`; extend it to wire `render.SetVerbose`. Cobra's flag-binding requires reading the flag value via `cmd.Flags().GetBool("verbose")`.

### Steps

- [ ] **Step 1: Write the failing test**

Modify `internal/cli/root_test.go`. Append:

```go
func TestRoot_VerboseFlagWiresRender(t *testing.T) {
	// Reset render.Default verbose state at end of test so we do not
	// leak into other tests sharing the package-level renderer.
	t.Cleanup(func() { render.SetVerbose(false) })

	rootCmd.SetArgs([]string{"--verbose", "version"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	// Smoke: verbose flag exists and is wired without error. Coverage of
	// the actual Detail suppression behaviour lives in the render package
	// and in the testscript scenarios.
	if v, _ := rootCmd.PersistentFlags().GetBool("verbose"); !v {
		t.Errorf("verbose flag should be true after --verbose parse")
	}
}
```

The test needs `render` imported in `internal/cli/root_test.go`. Add the import:

```go
import (
	"testing"

	"github.com/mguilarducci/liszt/internal/render"
)
```

Run: `go test ./internal/cli -run TestRoot_VerboseFlagWiresRender -v`
Expected: FAIL — `verbose` flag not registered.

- [ ] **Step 2: Add the flag + wire `render.SetVerbose`**

Modify `internal/cli/root.go`. Replace the `init` function:

```go
var (
	noColor bool
	verbose bool
)

func init() {
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable color output")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "show technical detail")
	rootCmd.PersistentPreRun = func(_ *cobra.Command, _ []string) {
		if noColor {
			// Setenv only fails on platforms without env support; failure
			// here means colors stay on, which is harmless.
			_ = os.Setenv("NO_COLOR", "1")
		}
		render.SetVerbose(verbose)
	}
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/cli -run TestRoot -v`
Expected: PASS (all root tests).

- [ ] **Step 4: Run full suite**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/root.go internal/cli/root_test.go
git commit -m "feat(cli): add --verbose flag wired to render.SetVerbose"
```

---

## Task 8: CLI repo — `ErrAlreadyAdded` sentinel + preflight check

**Files:**
- Modify: `internal/cli/repo.go`
- Modify: `internal/cli/repo_cmd_test.go` (rename if needed) or create `internal/cli/repo_test.go`

**Context:** Smallest behavioural change: before clone, load the registry and call `Find(name)`. If found, return `ErrAlreadyAdded` without invoking `gitx.EnsureClone`. The legacy `render.Info`/`render.Fail` lines stay in place — Task 9 swaps them for `Progress`. Splitting the change keeps each diff readable.

### Steps

- [ ] **Step 1: Check the current test file layout**

Run: `ls internal/cli/repo*`
Expected: `repo.go  repo_cmd.go  repo_cmd_test.go`. There is no `repo_test.go` for the `RepoAdd` implementation; create one.

- [ ] **Step 2: Write the failing test**

Create `internal/cli/repo_test.go`:

```go
package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mguilarducci/liszt/internal/repos"
)

func TestRepoAdd_AlreadyAddedReturnsSentinel(t *testing.T) {
	dir := t.TempDir()
	reposPath := filepath.Join(dir, "repos.toml")
	cache := filepath.Join(dir, "cache")

	// Seed registry with the target repo so the preflight trips.
	if err := repos.Save(reposPath, &repos.Config{Repos: []repos.Entry{
		{Name: "obra/superpowers", URL: "https://github.com/obra/superpowers", SHA: "deadbeef"},
	}}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	err := RepoAdd(Paths{Repos: reposPath, Cache: cache}, "https://github.com/obra/superpowers")
	if !errors.Is(err, ErrAlreadyAdded) {
		t.Fatalf("expected ErrAlreadyAdded, got %v", err)
	}

	// Clone must not have happened — the cache dir was never created.
	if _, statErr := os.Stat(filepath.Join(cache, "obra", "superpowers")); !os.IsNotExist(statErr) {
		t.Errorf("preflight should refuse before clone, but cache dir exists: %v", statErr)
	}
}
```

Run: `go test ./internal/cli -run TestRepoAdd_AlreadyAdded -v`
Expected: FAIL — `ErrAlreadyAdded` undefined.

- [ ] **Step 3: Add sentinel + preflight**

Modify `internal/cli/repo.go`. Replace the file body with:

```go
package cli

import (
	"errors"
	"io"

	"github.com/mguilarducci/liszt/internal/gitx"
	"github.com/mguilarducci/liszt/internal/marketplace"
	"github.com/mguilarducci/liszt/internal/render"
	"github.com/mguilarducci/liszt/internal/repos"
)

// ErrAlreadyAdded is returned by RepoAdd when the registry already contains
// an entry for the resolved repo name. Callers (CLI / tests) match on this
// sentinel via errors.Is.
var ErrAlreadyAdded = errors.New("repo already added")

// RepoAdd clones url into p.Cache and appends the entry to p.Repos.
// Preflight: if the registry already lists the resolved name, RepoAdd
// returns ErrAlreadyAdded without cloning. Refresh is the future
// `liszt repo update` command's responsibility.
func RepoAdd(p Paths, url string) error {
	prev := gitx.SetOutput(io.Discard)
	defer gitx.SetOutput(prev)

	render.Info("resolving repo", "url", url)
	owner, repo, err := gitx.ParseGitHubURL(url)
	if err != nil {
		render.Fail("parse url failed", "url", url, "err", err)
		return err
	}
	name := owner + "/" + repo
	dest := gitx.RepoPath(p.Cache, owner, repo)

	cfg, err := repos.Load(p.Repos)
	if err != nil {
		render.Fail("repos load failed", "path", p.Repos, "err", err)
		return err
	}
	if _, ok := cfg.Find(name); ok {
		render.Fail(name+" already added",
			"hint", "Run `liszt repo update "+name+"` to refresh")
		return ErrAlreadyAdded
	}

	render.Info("cloning", "name", name, "dest", dest)
	bar := render.NewBar("cloning " + name)
	bar.SetIndeterminate(true)
	if err := gitx.EnsureClone(url, dest); err != nil {
		bar.Fail("clone failed", "url", url, "err", err)
		return err
	}
	bar.Stop()

	sha, err := gitx.HeadSHA(dest)
	if err != nil {
		render.Fail("head-sha failed", "dest", dest, "err", err)
		return err
	}
	render.Info("cloned", "sha", sha[:12])

	render.Info("reading marketplace.json")
	mp, flavor, mpErr := marketplace.Read(dest)
	if mpErr != nil {
		render.Warn("marketplace.json failed to read")
	} else {
		render.Info("marketplace", "name", mp.Name, "flavor", flavor, "plugins", len(mp.Plugins))
	}

	render.Info("saving repos.toml", "path", p.Repos)
	cfg.Repos = append(cfg.Repos, repos.Entry{Name: name, URL: url, SHA: sha})
	if err := repos.Save(p.Repos, cfg); err != nil {
		render.Fail("repos save failed", "path", p.Repos, "err", err)
		return err
	}

	render.Done("repo added", "name", name)
	return nil
}
```

The structural changes vs. the prior version:

- Sentinel `ErrAlreadyAdded` declared.
- `repos.Load` moves *before* `gitx.EnsureClone` so we can preflight.
- `cfg.Find(name)` short-circuits with `ErrAlreadyAdded`.
- The append on line just before `repos.Save` replaces the prior `cfg.Upsert(...)` (the spec mandates a pure append now that conflict is rejected upstream).
- `marketplace.Read` failure path drops the kv payload from the `Warn` line (the new `Warn` ignores kv).

- [ ] **Step 4: Run tests**

Run: `go test ./internal/cli -run TestRepoAdd_AlreadyAdded -v`
Expected: PASS.

- [ ] **Step 5: Run full cli + repos suites**

Run: `go test ./internal/cli ./internal/repos`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/repo.go internal/cli/repo_test.go
git commit -m "feat(cli): repo add preflights registry and refuses duplicates"
```

---

## Task 9: CLI repo — refactor `RepoAdd` to use `Progress` + new vocabulary

**Files:**
- Modify: `internal/cli/repo.go`
- Modify: `internal/cli/repo_test.go` (add happy-path output assertion)

**Context:** Now swap all `render.Info` / mid-flow `render.Fail` calls for the 5-step `Progress` flow described in the spec, and move technical payload (URL, dest, SHA, paths) onto `render.Detail` calls. The conflict path emits a `render.Fail` (`✖`) + `render.Hint` line per the spec visual. The post-Done hints are emitted as plain Hint calls with `→ ` embedded in the message.

### Steps

- [ ] **Step 1: Write an output-shape assertion**

Modify `internal/cli/repo_test.go`. Append:

```go
import (
	"bytes"
	// ...existing imports

	"github.com/mguilarducci/liszt/internal/render"
)

func TestRepoAdd_AlreadyAddedEmitsFailAndHint(t *testing.T) {
	var buf bytes.Buffer
	prev := render.Default
	render.Default = render.New(&buf)
	t.Cleanup(func() { render.Default = prev })

	dir := t.TempDir()
	reposPath := filepath.Join(dir, "repos.toml")
	if err := repos.Save(reposPath, &repos.Config{Repos: []repos.Entry{
		{Name: "obra/superpowers", URL: "https://github.com/obra/superpowers", SHA: "1"},
	}}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	_ = RepoAdd(Paths{Repos: reposPath, Cache: dir + "/cache"},
		"https://github.com/obra/superpowers")

	got := buf.String()
	if !strings.Contains(got, "✓ Resolved obra/superpowers") {
		t.Errorf("missing resolve step: %q", got)
	}
	if !strings.Contains(got, "✖ obra/superpowers already added") {
		t.Errorf("missing fail line: %q", got)
	}
	if !strings.Contains(got, "→ Run `liszt repo update obra/superpowers`") {
		t.Errorf("missing hint: %q", got)
	}
}
```

Add the missing imports (`bytes`, `strings`) to the test file if not present.

Run: `go test ./internal/cli -run TestRepoAdd -v`
Expected: FAIL (the legacy code still uses `render.Info("resolving repo", …)` rather than the new step format).

- [ ] **Step 2: Rewrite `RepoAdd` against the new vocabulary**

Modify `internal/cli/repo.go`. Replace the body:

```go
package cli

import (
	"errors"
	"io"

	"github.com/mguilarducci/liszt/internal/gitx"
	"github.com/mguilarducci/liszt/internal/marketplace"
	"github.com/mguilarducci/liszt/internal/render"
	"github.com/mguilarducci/liszt/internal/repos"
)

// ErrAlreadyAdded is returned by RepoAdd when the registry already contains
// an entry for the resolved repo name. Callers (CLI / tests) match on this
// sentinel via errors.Is.
var ErrAlreadyAdded = errors.New("repo already added")

// RepoAdd clones url into p.Cache and appends the entry to p.Repos.
// Five steps drive a single Progress bar: resolve URL, preflight registry,
// clone, read manifest, persist. Technical payload (URL, dest, SHA, path)
// rides on Detail and is gated by --verbose.
func RepoAdd(p Paths, url string) error {
	prev := gitx.SetOutput(io.Discard)
	defer gitx.SetOutput(prev)

	render.Detail("url=" + url)

	progress := render.NewProgress(5)

	progress.Step("Resolving " + url)
	owner, repo, err := gitx.ParseGitHubURL(url)
	if err != nil {
		progress.StepFail(err)
		return err
	}
	name := owner + "/" + repo
	dest := gitx.RepoPath(p.Cache, owner, repo)
	render.Detail("resolved", "name", name, "dest", dest)

	progress.Step("Checking registry")
	cfg, err := repos.Load(p.Repos)
	if err != nil {
		progress.StepFail(err)
		render.Detail("repos load failed", "path", p.Repos, "err", err)
		return err
	}
	if _, ok := cfg.Find(name); ok {
		progress.StepFail(ErrAlreadyAdded)
		render.Fail(name + " already added")
		render.Hint("→ Run `liszt repo update " + name + "` to refresh")
		return ErrAlreadyAdded
	}

	progress.Step("Cloning " + name)
	if err := gitx.EnsureClone(url, dest); err != nil {
		progress.StepFail(err)
		render.Detail("clone failed", "url", url, "err", err)
		return err
	}
	sha, err := gitx.HeadSHA(dest)
	if err != nil {
		progress.StepFail(err)
		render.Detail("head-sha failed", "dest", dest, "err", err)
		return err
	}
	render.Detail("cloned", "sha", sha[:12])

	progress.Step("Reading marketplace.json")
	mp, flavor, mpErr := marketplace.Read(dest)
	if mpErr != nil {
		render.Warn("marketplace.json missing or invalid")
		render.Detail("marketplace.json", "err", mpErr)
	} else {
		render.Detail("marketplace", "name", mp.Name, "flavor", flavor, "plugins", len(mp.Plugins))
	}

	progress.Step("Saving to repos.toml")
	cfg.Repos = append(cfg.Repos, repos.Entry{Name: name, URL: url, SHA: sha})
	if err := repos.Save(p.Repos, cfg); err != nil {
		progress.StepFail(err)
		render.Detail("repos save failed", "path", p.Repos, "err", err)
		return err
	}
	render.Detail("repos.toml", "path", p.Repos)

	switch {
	case mpErr != nil:
		progress.Done("Added " + name)
	default:
		progress.Done("Added "+name,
			"marketplace", mp.Name,
			"plugins", len(mp.Plugins))
	}
	render.Hint("→ Run `liszt plugin list` to see available plugins")
	render.Hint("→ Run `liszt plugin install <name>` to install")
	return nil
}
```

- [ ] **Step 3: Run repo tests**

Run: `go test ./internal/cli -run TestRepoAdd -v`
Expected: PASS — both `TestRepoAdd_AlreadyAddedReturnsSentinel` and `TestRepoAdd_AlreadyAddedEmitsFailAndHint`.

- [ ] **Step 4: Run full suite**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 5: Smoke against a real repo (manual sanity)**

Run: `go build -o bin/liszt ./cmd/liszt && rm -rf /tmp/liszt-smoke && XDG_DATA_HOME=/tmp/liszt-smoke XDG_CACHE_HOME=/tmp/liszt-smoke ./bin/liszt repo add https://github.com/obra/superpowers`

Expected output (TTY): bar animates during clone, 5 ✓ persistent lines emit, a ✔ summary block appears, two `→` hint lines follow.

Re-run the same command without removing the data dir to confirm the duplicate path:

Run: `XDG_DATA_HOME=/tmp/liszt-smoke XDG_CACHE_HOME=/tmp/liszt-smoke ./bin/liszt repo add https://github.com/obra/superpowers`

Expected: `✖ obra/superpowers already added` + `→ Run \`liszt repo update obra/superpowers\` to refresh`. Exit code non-zero.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/repo.go internal/cli/repo_test.go
git commit -m "feat(cli): drive repo add through Progress with 5 steps"
```

---

## Task 10: CLI outdated — migrate `Info` and `Warn`-payload to new verbs

**Files:**
- Modify: `internal/cli/outdated.go`

**Context:** `Outdated` currently calls `render.Warn("ls-remote failed", "repo", e.Repo, "err", err)` and a `render.Info(...)` line per drift. Under the new vocabulary, `Warn` takes no kv (drop the payload to `Detail`), and the drift list should use `Hint` (descriptive output) plus `Detail` for the technical SHA/plugin context. The existing `Bar` use stays — `Outdated` already drives one bar across N repos.

### Steps

- [ ] **Step 1: Locate the call sites**

Run: `grep -n "render\." internal/cli/outdated.go`
Expected: matches at lines 34 (`NewBar`), 58 (`Warn`), 84 (`Info`).

- [ ] **Step 2: Update the Warn payload at line 58**

Modify `internal/cli/outdated.go`. Replace:

```go
sha, err := gitx.LsRemoteHead(url)
if err != nil {
	render.Warn("could not reach remote")
	render.Detail("ls-remote failed", "repo", e.Repo, "err", err)
	unknown++
	if total > 0 {
		bar.Set(float64(i+1) / float64(total))
	}
	continue
}
```

- [ ] **Step 3: Update the drift list at line 83**

Replace the `for _, d := range drifts { render.Info(...) }` block with:

```go
for _, d := range drifts {
	render.Hint(fmt.Sprintf("- %s %s [%s]  %s..%s",
		d.entry.Kind, d.entry.Slug, d.entry.Flavor,
		d.entry.SHA[:12], d.latest[:12]))
	render.Detail("drift",
		"plugin", d.entry.Plugin,
		"repo", d.entry.Repo,
		"locked", d.entry.SHA[:12],
		"remote", d.latest[:12],
	)
}
```

- [ ] **Step 4: Run tests + build**

Run: `go test ./internal/cli ./internal/render`
Expected: PASS.

Run: `go build ./...`
Expected: success.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/outdated.go
git commit -m "refactor(cli): outdated emits Hint per drift, Detail for technical payload"
```

---

## Task 11: CLI plugin — migrate Warn payloads to Detail

**Files:**
- Modify: `internal/cli/plugin.go`

**Context:** `PluginList` uses `render.Warn("skip", "name", r.Name, "err", err)` twice. With the new `Warn` ignoring kv, move payload onto `Detail`. `PluginInstall` uses `bar.Fail(...)` calls; those route through `Bar.Fail` which now calls `r.Fail` (summary block) — no source-level change needed for the install path, but verify it compiles.

### Steps

- [ ] **Step 1: Update the two Warn calls in `PluginList`**

Modify `internal/cli/plugin.go`. Lines 31 and 36 currently read:

```go
render.Warn("skip", "name", r.Name, "err", err)
```

Replace each with:

```go
render.Warn("Skipped " + r.Name)
render.Detail("skip", "name", r.Name, "err", err)
```

- [ ] **Step 2: Verify the rest of the file compiles**

Run: `go build ./internal/cli`
Expected: success. No source-level changes needed in `PluginInstall` or `claudeInstallWithBar` — the renamed `render.Fail` is reached through `Bar.Fail` which was already updated in Task 4.

- [ ] **Step 3: Run tests**

Run: `go test ./internal/cli`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/plugin.go
git commit -m "refactor(cli): plugin list emits Warn + Detail instead of Warn-with-payload"
```

---

## Task 12: CLI resource — migrate Warn payload to Detail

**Files:**
- Modify: `internal/cli/resource.go`

**Context:** Single Warn call at `internal/cli/resource.go:91` carries a `"repo"` kv. Split into Warn (human) + Detail (technical).

### Steps

- [ ] **Step 1: Update the Warn call**

Modify `internal/cli/resource.go`. Replace lines 91-94:

```go
if len(matches) > 1 {
	render.Warn(fmt.Sprintf("%d sources for %q; picking %s:%s",
		len(matches), slug, m.pluginName, m.slug))
	render.Detail("disambiguation", "repo", m.repoName)
}
```

- [ ] **Step 2: Build + test**

Run: `go build ./...`
Run: `go test ./internal/cli`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/cli/resource.go
git commit -m "refactor(cli): resource install splits Warn message from Detail payload"
```

---

## Task 13: CLI version — replace `render.Info` with `fmt.Println`

**Files:**
- Modify: `internal/cli/version_cmd.go`

**Context:** `version_cmd.go:21` calls `render.Info(version.Full())` to print the version. The new vocabulary has no `Info`, and the version string is plain output, not a verb-categorised line. Use `fmt.Println` directly.

### Steps

- [ ] **Step 1: Read the current implementation**

Run: `cat internal/cli/version_cmd.go`
Expected: shows a cobra command whose `RunE` calls `render.Info(version.Full())`.

- [ ] **Step 2: Replace `render.Info` with `fmt.Println`**

Modify `internal/cli/version_cmd.go`. Replace the line `render.Info(version.Full())` with:

```go
fmt.Println(version.Full())
```

Adjust the imports: drop `render` if no other reference remains in the file; add `fmt` if it is not already imported.

- [ ] **Step 3: Build + test**

Run: `go build ./...`
Run: `go test ./internal/cli -run TestVersion -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/version_cmd.go
git commit -m "refactor(cli): version prints via fmt.Println"
```

---

## Task 14: Render — remove `Info` verb and its tests

**Files:**
- Modify: `internal/render/message.go` (delete `Info` method)
- Modify: `internal/render/default.go` (delete `Info` delegate)
- Modify: `internal/render/message_test.go` (delete `Info` tests + `info` case)

**Context:** With every CLI caller migrated, `Info` is dead. Remove the method, the delegate, and the four `TestInfo_*` / `TestMultilineMessageIndents` tests that exercised it. `TestEachLevelLabel` already dropped its `info` row in Task 4; keep the remaining `warn` / `done` cases.

### Steps

- [ ] **Step 1: Verify no caller remains**

Run: `grep -rn "render\.Info\|\.Info(" internal/ cmd/ --include="*.go"`
Expected: zero matches outside of doc comments and test files about to be deleted. If anything else surfaces, migrate it before continuing.

- [ ] **Step 2: Delete `Info` from `message.go`**

Modify `internal/render/message.go`. Remove the `Info` method and the doc comment above it. Keep `Warn`, `Done`, `Fail`, and the helpers.

- [ ] **Step 3: Delete `Info` delegate from `default.go`**

Modify `internal/render/default.go`. Remove the lines:

```go
// Info delegates to Default.Info.
func Info(msg string, kv ...any) { ensureDefault().Info(msg, kv...) }
```

- [ ] **Step 4: Delete `Info`-specific tests from `message_test.go`**

Modify `internal/render/message_test.go`. Delete:

- `TestInfo_PlainOutput`
- `TestInfo_WithKV`
- `TestInfo_OddKV`
- `TestMultilineMessageIndents` (it exercises `Info` continuation indent; the format is no longer reachable through a public API)

- [ ] **Step 5: Run render tests**

Run: `go test ./internal/render -v`
Expected: PASS.

- [ ] **Step 6: Run full suite**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/render/message.go internal/render/default.go internal/render/message_test.go
git commit -m "refactor(render): remove Info verb after migration"
```

---

## Task 15: Testscript scenarios — verbose + duplicate-repo

**Files:**
- Modify: `internal/cli/script_test.go` and/or testdata under `internal/cli/testdata`

**Context:** The repo already runs `testscript` against the built binary (see `internal/cli/script_test.go` + commit `df83e86`). Add two scenarios:

1. `verbose.txtar`: invoke `liszt version --verbose` against a fixture and assert the exit code is zero (sanity that `--verbose` parses on every subcommand). For a more meaningful assertion, add a `liszt` invocation that produces a `Detail` line and confirm it appears with `-v` and disappears without it. The plugin-list path is a good candidate: `liszt plugin list` against a tempdir with a malformed repo entry emits a `Warn` + `Detail` pair; `-v` toggles visibility of the `Detail` line.
2. `repo_add_duplicate.txtar`: seed a `repos.toml` with one entry and invoke `liszt repo add <same-url>`; assert `! exec` (non-zero exit) and stderr `contains "already added"`.

### Steps

- [ ] **Step 1: Inspect the existing scenarios**

Run: `ls internal/cli/testdata`
Expected: a list of `.txtar` files. Read one of them (e.g. the existing version smoke) to confirm the layout this repo uses.

- [ ] **Step 2: Add `repo_add_duplicate.txtar`**

Create `internal/cli/testdata/repo_add_duplicate.txtar`:

```
# repo add should refuse to clone when the repo is already registered.
env XDG_DATA_HOME=$HOME/.local/share
env XDG_CACHE_HOME=$HOME/.cache

! liszt repo add https://github.com/obra/superpowers
stderr 'already added'
stderr 'repo update obra/superpowers'

-- home/.local/share/liszt/repos.toml --
[[repos]]
name = "obra/superpowers"
url = "https://github.com/obra/superpowers"
sha = "deadbeef"
```

Adjust the env / path layout to match what existing scenarios use (read one for the exact pattern).

- [ ] **Step 3: Add `verbose.txtar`**

Create `internal/cli/testdata/verbose.txtar`:

```
# --verbose surfaces Detail lines; without it they stay hidden.
env XDG_DATA_HOME=$HOME/.local/share
env XDG_CACHE_HOME=$HOME/.cache

# Seed a corrupt repo entry so PluginList emits a Warn + Detail pair.
-- home/.local/share/liszt/repos.toml --
[[repos]]
name = "broken/repo"
url = "not-a-real-url"
sha = "0"
--

liszt plugin list
stdout 'Skipped broken/repo' OR stderr 'Skipped broken/repo'
! stderr 'name=broken/repo'

liszt plugin list --verbose
stderr 'name=broken/repo'
```

Adjust `stdout`/`stderr` and the exact `OR` syntax to match the testscript style used by other scenarios in this repo.

- [ ] **Step 4: Run testscript**

Run: `go test ./internal/cli -run TestScript -v`
Expected: PASS for both new scenarios plus existing ones.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/testdata/repo_add_duplicate.txtar internal/cli/testdata/verbose.txtar
git commit -m "test(cli): cover verbose flag and duplicate repo add via testscript"
```

---

## Self-Review Notes

The plan covers every requirement in the spec:

- Vocabulary (Step / StepDone / StepFail / Done / Fail / Warn / Detail / Hint) — Tasks 1, 2, 4. `Hint` unchanged by deliberate scope decision (see Line-format conventions block above).
- Bar prefix rename `info → step` — Task 3.
- `Error → Fail` rename — Task 4.
- `Progress` wrapper — Task 5.
- `repos.Find` — Task 6.
- `-v` flag wiring — Task 7.
- `ErrAlreadyAdded` + preflight — Tasks 8, 9.
- New `repo add` flow with Progress + Detail — Task 9.
- Mechanical migration of other commands — Tasks 10, 11, 12, 13.
- `Info` removal — Task 14.
- Testscript coverage of `--verbose` + duplicate path — Task 15.

Spec deferrals (per-command copy rewrites, `repo update` subcommand) remain out of scope; the duplicate-error hint references the future command as text only.
