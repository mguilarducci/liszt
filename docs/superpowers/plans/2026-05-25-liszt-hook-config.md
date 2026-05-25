# Liszt Hook Config Separation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the flat `[tasks.*]` runner and `liszt run` verb with a hook-oriented model — `liszt hook <name> [lang...] [-- gitargs...]` — backed by `.liszt/hooks.toml` whose nested `[<hook>.<segment>]` tables separate a hook's `general` steps from its per-language steps.

**Architecture:** `internal/runner` exposes `Config map[string]Hook` / `Hook map[string]Segment`. `Config.Resolve(name, langs)` returns the ordered segment names (`general` first when present, then named langs; errors on missing hook/segment or empty result). `Config.RunHook` runs those segments in order, each segment running its `run` commands via `bash -c "<cmd>" bash <gitargs...>` (args become `$1`/`$@`), retaining the first failing exit code. The CLI splits positional args at `--` (via cobra `ArgsLenAtDash`): before `--` are segment selectors, at/after are git args.

**Tech Stack:** Go, cobra, `github.com/pelletier/go-toml/v2` v2.3.1, `rogpeppe/go-internal/testscript`.

**Verified before planning:** the named-map decode (`type Config map[string]Hook`; `toml.Unmarshal(data, &cfg)`) decodes top-level `[pre-commit.general]` tables, `fail_hint`, and `enabled *bool` correctly; a missing hook key yields `ok=false`. No wrapper struct needed.

---

## File Structure

| File | Responsibility |
|------|----------------|
| `internal/runner/runner.go` | Rewritten: `Segment`, `Hook`, `Config` types; `Load`, `Resolve`, `RunHook`, segment exec, `failureLine`, `exitCode`. |
| `internal/runner/runner_test.go` | Rewritten: unit tests for `Load`, `Resolve`, `RunHook`, segment exec. |
| `internal/cli/hook_cmd.go` | New (replaces `run_cmd.go`): `hook` command, `splitHookArgs`, `--config` default `.liszt/hooks.toml`. |
| `internal/cli/run_cmd.go` | Deleted. |
| `internal/cli/hook_cmd_test.go` | New: unit tests for `splitHookArgs`. |
| `cmd/liszt/testdata/script/hook.txtar` | New (replaces `run.txtar`): end-to-end CLI behavior. |
| `cmd/liszt/testdata/script/run.txtar` | Deleted. |
| `cmd/liszt/testdata/script/help.txtar`, `bare_help.txtar` | `run` → `hook` in expected output. |
| `.liszt/hooks.toml` | New: this repo's own pre-commit hook (dogfood). |

---

## Task 1: Rewrite `internal/runner` (types, Load, Resolve, exec)

**Files:**
- Modify (rewrite): `internal/runner/runner.go`
- Modify (rewrite): `internal/runner/runner_test.go`

- [ ] **Step 1: Write failing tests for `Load` + new types**

Replace the entire contents of `internal/runner/runner_test.go` with the test file built across Steps 1, 4, 7 below. For this step, start the file with:

```go
package runner

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeTOML(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.toml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func TestLoad_Valid(t *testing.T) {
	t.Parallel()
	path := writeTOML(t, `
[pre-commit.general]
run = ["echo a", "echo b"]
fail_hint = "do the thing"

[pre-commit.gleam]
run = ["gleam test"]
enabled = false
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	hook, ok := cfg["pre-commit"]
	if !ok {
		t.Fatalf(`cfg["pre-commit"] ok=false`)
	}
	gen := hook["general"]
	if len(gen.Commands) != 2 || gen.Commands[0] != "echo a" {
		t.Errorf("unexpected general.Commands: %#v", gen.Commands)
	}
	if gen.FailHint != "do the thing" {
		t.Errorf("unexpected FailHint: %q", gen.FailHint)
	}
	if g := hook["gleam"]; g.Enabled == nil || *g.Enabled != false {
		t.Errorf("expected gleam enabled=false to decode, got %v", g.Enabled)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	t.Parallel()
	if _, err := Load(filepath.Join(t.TempDir(), "nope.toml")); err == nil {
		t.Fatal("Load on missing file should error")
	}
}

func TestLoad_MalformedTOML(t *testing.T) {
	t.Parallel()
	if _, err := Load(writeTOML(t, "this is = = not toml")); err == nil {
		t.Fatal("Load on malformed TOML should error")
	}
}

func TestLoad_RunWrongType(t *testing.T) {
	t.Parallel()
	if _, err := Load(writeTOML(t, "[h.general]\nrun = \"bare string\"\n")); err == nil {
		t.Fatal("Load with string run should error (must be array)")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `rtk proxy go test ./internal/runner/ -run TestLoad -v`
Expected: compile failure — `Load` now returns a map type the new tests index, but old `runner.go` still defines `Config` as a struct with `Tasks`. Build error / FAIL.

- [ ] **Step 3: Rewrite `runner.go` types + `Load`**

Replace the entire contents of `internal/runner/runner.go` down through `Load` with:

```go
// Package runner executes git-hook task segments declared in .liszt/hooks.toml.
package runner

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/pelletier/go-toml/v2"
)

// generalSegment is the reserved segment name that always runs first.
const generalSegment = "general"

// Segment is one [<hook>.<segment>] table: a group of shell commands plus
// failure metadata.
type Segment struct {
	Commands []string `toml:"run"`
	FailHint string   `toml:"fail_hint"`
	Enabled  *bool    `toml:"enabled"` // nil => enabled
}

func (s Segment) isEnabled() bool {
	return s.Enabled == nil || *s.Enabled
}

// Hook is one [<hook>.*] table: segments keyed by name. "general" is reserved.
type Hook map[string]Segment

// Config is every hook in the file; top-level TOML keys decode into this map.
type Config map[string]Hook

// Load reads and decodes path. A missing or unreadable file, or malformed TOML,
// is an error.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	cfg := Config{}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
}
```

(Steps 6 and 9 append `Resolve`, `RunHook`, segment exec, `failureLine`, `exitCode` to this same file. Leave the rest of the old file deleted for now — it will not compile until Step 9 adds the exec helpers; that is expected and resolved within this task before any commit.)

- [ ] **Step 4: Write failing tests for `Resolve`**

Append to `internal/runner/runner_test.go`:

```go
func loadCfg(t *testing.T, body string) Config {
	t.Helper()
	cfg, err := Load(writeTOML(t, body))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cfg
}

const resolveFixture = `
[pre-commit.general]
run = ["echo g"]
[pre-commit.gleam]
run = ["echo gl"]
[pre-commit.go]
run = ["echo go"]
[no-general.gleam]
run = ["echo x"]
`

func TestResolve_GeneralFirstThenLangsInOrder(t *testing.T) {
	t.Parallel()
	cfg := loadCfg(t, resolveFixture)
	got, err := cfg.Resolve("pre-commit", []string{"go", "gleam"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := []string{"general", "go", "gleam"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("order = %v, want %v", got, want)
	}
}

func TestResolve_GeneralOnlyWhenNoLangs(t *testing.T) {
	t.Parallel()
	cfg := loadCfg(t, resolveFixture)
	got, err := cfg.Resolve("pre-commit", nil)
	if err != nil || len(got) != 1 || got[0] != "general" {
		t.Errorf("got %v err %v, want [general]", got, err)
	}
}

func TestResolve_NoGeneralUsesLangsOnly(t *testing.T) {
	t.Parallel()
	cfg := loadCfg(t, resolveFixture)
	got, err := cfg.Resolve("no-general", []string{"gleam"})
	if err != nil || len(got) != 1 || got[0] != "gleam" {
		t.Errorf("got %v err %v, want [gleam]", got, err)
	}
}

func TestResolve_NamingGeneralIsNoop(t *testing.T) {
	t.Parallel()
	cfg := loadCfg(t, resolveFixture)
	got, err := cfg.Resolve("pre-commit", []string{"general", "go"})
	want := []string{"general", "go"}
	if err != nil || strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("got %v err %v, want %v (general not duplicated)", got, err, want)
	}
}

func TestResolve_MissingHook(t *testing.T) {
	t.Parallel()
	cfg := loadCfg(t, resolveFixture)
	if _, err := cfg.Resolve("ghost", nil); err == nil {
		t.Fatal("Resolve on missing hook should error")
	}
}

func TestResolve_MissingSegment(t *testing.T) {
	t.Parallel()
	cfg := loadCfg(t, resolveFixture)
	if _, err := cfg.Resolve("pre-commit", []string{"rust"}); err == nil {
		t.Fatal("Resolve on missing segment should error")
	}
}

func TestResolve_NothingToRun(t *testing.T) {
	t.Parallel()
	cfg := loadCfg(t, resolveFixture)
	if _, err := cfg.Resolve("no-general", nil); err == nil {
		t.Fatal("Resolve with no general and no langs should error")
	}
}
```

- [ ] **Step 5: Run tests to verify they fail**

Run: `rtk proxy go test ./internal/runner/ -run TestResolve -v`
Expected: compile failure — `Resolve` undefined.

- [ ] **Step 6: Implement `Resolve`**

Append to `internal/runner/runner.go`:

```go
// Resolve returns the ordered segment names to run for a hook: "general" first
// when present, then each requested lang in order. Naming "general" explicitly
// is a no-op (it is already included). Errors if the hook is absent, a requested
// lang is absent, or nothing would run.
func (c Config) Resolve(name string, langs []string) ([]string, error) {
	hook, ok := c[name]
	if !ok {
		return nil, fmt.Errorf("no [%s] hook in config", name)
	}
	var order []string
	if _, ok := hook[generalSegment]; ok {
		order = append(order, generalSegment)
	}
	for _, l := range langs {
		if l == generalSegment {
			continue
		}
		if _, ok := hook[l]; !ok {
			return nil, fmt.Errorf("no [%s.%s] segment in config", name, l)
		}
		order = append(order, l)
	}
	if len(order) == 0 {
		return nil, fmt.Errorf("[%s] has nothing to run", name)
	}
	return order, nil
}
```

- [ ] **Step 7: Write failing tests for `RunHook` + segment exec**

Append to `internal/runner/runner_test.go`:

```go
func boolPtr(b bool) *bool { return &b }

func TestRunHook_RunsSegmentsInOrderWithHeaders(t *testing.T) {
	t.Parallel()
	cfg := Config{"pre-commit": Hook{
		"general": {Commands: []string{"echo from-general"}},
		"gleam":   {Commands: []string{"echo from-gleam"}},
	}}
	var out, errOut bytes.Buffer
	code := cfg.RunHook("pre-commit", []string{"general", "gleam"}, nil, &out, &errOut)
	if code != 0 {
		t.Fatalf("all-pass should return 0, got %d", code)
	}
	s := out.String()
	if !strings.Contains(s, "== pre-commit.general ==") || !strings.Contains(s, "from-general") {
		t.Errorf("missing general output/header in %q", s)
	}
	if !strings.Contains(s, "== pre-commit.gleam ==") || !strings.Contains(s, "from-gleam") {
		t.Errorf("missing gleam output/header in %q", s)
	}
	if strings.Index(s, "from-general") > strings.Index(s, "from-gleam") {
		t.Errorf("general should run before gleam; out=%q", s)
	}
}

func TestRunHook_ForwardsArgsToEverySegment(t *testing.T) {
	t.Parallel()
	cfg := Config{"h": Hook{
		"general": {Commands: []string{`printf 'g:%s\n' "$1"`}},
		"go":      {Commands: []string{`printf 'go:%s\n' "$@"`}},
	}}
	var out, errOut bytes.Buffer
	if code := cfg.RunHook("h", []string{"general", "go"}, []string{"X", "Y"}, &out, &errOut); code != 0 {
		t.Fatalf("expected 0, got %d", code)
	}
	s := out.String()
	if !strings.Contains(s, "g:X") || !strings.Contains(s, "go:X") || !strings.Contains(s, "Y") {
		t.Errorf("args not forwarded to every segment: %q", s)
	}
}

func TestRunHook_RetainsFirstFailureAcrossSegments(t *testing.T) {
	t.Parallel()
	cfg := Config{"h": Hook{
		"general": {Commands: []string{"exit 3", "echo ran-anyway"}, FailHint: "fix me"},
		"go":      {Commands: []string{"exit 4"}},
	}}
	var out, errOut bytes.Buffer
	code := cfg.RunHook("h", []string{"general", "go"}, nil, &out, &errOut)
	if code != 3 {
		t.Errorf("expected retained first failure exit 3, got %d", code)
	}
	if !strings.Contains(out.String(), "ran-anyway") {
		t.Errorf("later command should still run; out=%q", out.String())
	}
	es := errOut.String()
	if !strings.Contains(es, "FAILED:") || !strings.Contains(es, "exit 3") || !strings.Contains(es, "hint: fix me") {
		t.Errorf("expected FAILED+exit 3+hint, got %q", es)
	}
}

func TestRunHook_DisabledSegmentSkipped(t *testing.T) {
	t.Parallel()
	cfg := Config{"h": Hook{
		"general": {Commands: []string{"echo nope"}, Enabled: boolPtr(false)},
		"go":      {Commands: []string{"echo yes"}},
	}}
	var out, errOut bytes.Buffer
	if code := cfg.RunHook("h", []string{"general", "go"}, nil, &out, &errOut); code != 0 {
		t.Fatalf("expected 0, got %d", code)
	}
	s := out.String()
	if strings.Contains(s, "nope") || strings.Contains(s, "== h.general ==") {
		t.Errorf("disabled segment should produce no output: %q", s)
	}
	if !strings.Contains(s, "yes") {
		t.Errorf("enabled segment should still run: %q", s)
	}
}

func TestRunHook_EmptyRunErrors(t *testing.T) {
	t.Parallel()
	cfg := Config{"h": Hook{"general": {Commands: nil}}}
	var out, errOut bytes.Buffer
	if code := cfg.RunHook("h", []string{"general"}, nil, &out, &errOut); code != 1 {
		t.Errorf("empty run should return 1, got %d", code)
	}
	if !strings.Contains(errOut.String(), "empty run") {
		t.Errorf("expected empty-run message, got %q", errOut.String())
	}
}

func TestRunHook_NoHintWhenUnset(t *testing.T) {
	t.Parallel()
	cfg := Config{"h": Hook{"general": {Commands: []string{"exit 1"}}}}
	var out, errOut bytes.Buffer
	cfg.RunHook("h", []string{"general"}, nil, &out, &errOut)
	if strings.Contains(errOut.String(), "hint:") {
		t.Errorf("no fail_hint set, should not print hint: %q", errOut.String())
	}
}

func TestRunHook_CommandNotFound(t *testing.T) {
	t.Parallel()
	cfg := Config{"h": Hook{"general": {Commands: []string{"this-binary-does-not-exist-xyz"}}}}
	var out, errOut bytes.Buffer
	if code := cfg.RunHook("h", []string{"general"}, nil, &out, &errOut); code != 127 {
		t.Errorf("command-not-found should map to 127, got %d", code)
	}
}

func TestExitCode_NonExitError(t *testing.T) {
	t.Parallel()
	if got := exitCode(errors.New("not an exit error")); got != 1 {
		t.Errorf("non-ExitError should map to 1, got %d", got)
	}
}

func TestFailureLine_ExitError(t *testing.T) {
	t.Parallel()
	err := exec.Command("bash", "-c", "exit 2").Run()
	if line := failureLine("exit 2", 2, err); !strings.Contains(line, "FAILED: exit 2 (exit 2)") {
		t.Errorf("expected exit-code form, got %q", line)
	}
}

func TestFailureLine_StartError(t *testing.T) {
	t.Parallel()
	if line := failureLine("foo", 1, errors.New("boom")); !strings.Contains(line, "could not start: boom") {
		t.Errorf("expected start-failure form, got %q", line)
	}
}
```

- [ ] **Step 8: Run tests to verify they fail**

Run: `rtk proxy go test ./internal/runner/ -v`
Expected: compile failure — `RunHook`, `exitCode`, `failureLine` undefined.

- [ ] **Step 9: Implement `RunHook`, segment exec, helpers**

Append to `internal/runner/runner.go`:

```go
// RunHook runs the named segments of the hook in order. Within each segment,
// commands run via `bash -c` with args forwarded as positional parameters
// ($1, $@) to every command. All commands run even on failure; the first
// failing command's exit code (across all segments) is retained and returned
// (0 = all passed). Disabled segments are skipped. The named segments are
// assumed present (the caller resolved them via Resolve).
func (c Config) RunHook(name string, segments, args []string, stdout, stderr io.Writer) int {
	hook := c[name]
	failCode := 0
	for _, seg := range segments {
		if code := hook[seg].run(name, seg, args, stdout, stderr); code != 0 && failCode == 0 {
			failCode = code
		}
	}
	return failCode
}

// run executes one segment's commands. A disabled segment returns 0 without
// output; a segment with no commands returns 1 with an error.
func (s Segment) run(hookName, segName string, args []string, stdout, stderr io.Writer) int {
	if !s.isEnabled() {
		return 0
	}
	if len(s.Commands) == 0 {
		_, _ = fmt.Fprintf(stderr, "error: [%s.%s] has empty run\n", hookName, segName)
		return 1
	}

	_, _ = fmt.Fprintf(stdout, "== %s.%s ==\n", hookName, segName)

	failCode := 0
	failCmd := ""
	var failErr error
	for _, c := range s.Commands {
		cmd := exec.Command("bash", append([]string{"-c", c, "bash"}, args...)...)
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil && failCode == 0 {
			failCode = exitCode(err)
			failCmd = c
			failErr = err
		}
	}

	if failCode != 0 {
		_, _ = fmt.Fprint(stderr, failureLine(failCmd, failCode, failErr))
		if s.FailHint != "" {
			_, _ = fmt.Fprintf(stderr, "hint: %s\n", s.FailHint)
		}
	}
	return failCode
}

// failureLine formats the FAILED line. A command that ran and exited non-zero
// reports its exit code; a command that never started (non-ExitError, e.g. bash
// not found) reports the underlying error so the cause is not misattributed.
func failureLine(cmd string, code int, err error) string {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return fmt.Sprintf("FAILED: %s (exit %d)\n", cmd, code)
	}
	return fmt.Sprintf("FAILED: %s (could not start: %v)\n", cmd, err)
}

// exitCode extracts the process exit code from a command error. A non-exit
// error or a signal kill maps to 1.
func exitCode(err error) int {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		// code > 0 folds signal kills (ExitCode() == -1) into the 1 fallback.
		if code := ee.ExitCode(); code > 0 {
			return code
		}
	}
	return 1
}
```

- [ ] **Step 10: Run the runner tests, verify all pass**

Run: `rtk proxy go test ./internal/runner/ -v`
Expected: PASS (all `TestLoad_*`, `TestResolve_*`, `TestRunHook_*`, `TestExitCode_*`, `TestFailureLine_*`).

- [ ] **Step 11: Confirm 100% coverage of the package**

Run: `rtk proxy go test ./internal/runner/ -covermode=atomic -coverprofile=/tmp/runner.cover`
Then run: `rtk proxy go tool cover -func=/tmp/runner.cover`
Expected: `internal/runner/runner.go` functions all at `100.0%`. If any line is uncovered, add a focused test before committing.

- [ ] **Step 12: Commit**

Note: `internal/cli/run_cmd.go` still references the old runner API and will not build yet — that is fixed in Task 2. Commit the runner package on its own; do not run a repo-wide build here.

```bash
rtk proxy git add internal/runner/runner.go internal/runner/runner_test.go
```
```bash
rtk proxy git commit -m "feat(runner): hook/segment model with general + per-language composition"
```

---

## Task 2: Replace `run` command with `hook` command

**Files:**
- Create: `internal/cli/hook_cmd.go`
- Delete: `internal/cli/run_cmd.go`
- Create: `internal/cli/hook_cmd_test.go`

- [ ] **Step 1: Write failing test for `splitHookArgs`**

Create `internal/cli/hook_cmd_test.go`:

```go
package cli

import (
	"strings"
	"testing"
)

func joinOrNil(s []string) string {
	if s == nil {
		return "<nil>"
	}
	return strings.Join(s, ",")
}

func TestSplitHookArgs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name              string
		args              []string
		dash              int
		wantName          string
		wantLangs         string
		wantGitArgs       string
	}{
		{"no dash, langs only", []string{"pre-commit", "go", "gleam"}, -1, "pre-commit", "go,gleam", "<nil>"},
		{"no dash, name only", []string{"pre-commit"}, -1, "pre-commit", "", "<nil>"},
		{"dash, langs and gitargs", []string{"pre-commit", "gleam", "a", "b"}, 2, "pre-commit", "gleam", "a,b"},
		{"dash, no langs, gitargs", []string{"pre-commit", "a", "b"}, 1, "pre-commit", "", "a,b"},
		{"dash, no langs, no gitargs", []string{"pre-commit"}, 1, "pre-commit", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			name, langs, gitArgs := splitHookArgs(tc.args, tc.dash)
			if name != tc.wantName {
				t.Errorf("name = %q, want %q", name, tc.wantName)
			}
			if joinOrNil(langs) != tc.wantLangs {
				t.Errorf("langs = %q, want %q", joinOrNil(langs), tc.wantLangs)
			}
			if joinOrNil(gitArgs) != tc.wantGitArgs {
				t.Errorf("gitArgs = %q, want %q", joinOrNil(gitArgs), tc.wantGitArgs)
			}
		})
	}
}
```

Note on the empty-vs-nil expectations: slicing a non-nil slice to zero length yields a non-nil empty slice, so `joinOrNil` returns `""` for it. With `dash >= 0`, `args[1:dash]` and `args[dash:]` are such slices (return `""` when empty). With `dash == -1`, langs is `args[1:]` (`""` when only the name is present) and gitArgs is the literal `nil` returned by `splitHookArgs` (`<nil>`). The table above matches these exactly.

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk proxy go test ./internal/cli/ -run TestSplitHookArgs -v`
Expected: compile failure — `splitHookArgs` undefined.

- [ ] **Step 3: Create `hook_cmd.go`, delete `run_cmd.go`**

Delete `internal/cli/run_cmd.go`.

Create `internal/cli/hook_cmd.go`:

```go
package cli

import (
	"os"

	"github.com/mguilarducci/liszt/internal/runner"
	"github.com/spf13/cobra"
)

var hookConfigPath string

var hookCmd = &cobra.Command{
	Use:   "hook <name> [lang...] [-- gitargs...]",
	Short: "Run a git hook from .liszt/hooks.toml",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, langs, gitArgs := splitHookArgs(args, cmd.ArgsLenAtDash())
		cfg, err := runner.Load(hookConfigPath)
		if err != nil {
			return err
		}
		segments, err := cfg.Resolve(name, langs)
		if err != nil {
			return err
		}
		os.Exit(cfg.RunHook(name, segments, gitArgs, os.Stdout, os.Stderr))
		return nil // coverage: unreachable, os.Exit terminates the process
	},
}

// splitHookArgs partitions cobra positional args using the index of "--" (dash,
// as reported by cobra.Command.ArgsLenAtDash; -1 when absent). args[0] is the
// hook name. With no "--", every remaining arg is a lang selector. With "--" at
// index d, args[1:d] are lang selectors and args[d:] are git args forwarded to
// the commands.
func splitHookArgs(args []string, dash int) (name string, langs, gitArgs []string) {
	name = args[0]
	if dash < 0 {
		return name, args[1:], nil
	}
	return name, args[1:dash], args[dash:]
}

func init() {
	hookCmd.Flags().StringVar(&hookConfigPath, "config", ".liszt/hooks.toml", "config path")
	rootCmd.AddCommand(hookCmd)
}
```

- [ ] **Step 4: Run the unit test, verify it passes**

Run: `rtk proxy go test ./internal/cli/ -run TestSplitHookArgs -v`
Expected: PASS (all five subtests).

- [ ] **Step 5: Build the whole module to confirm no dangling references**

Run: `rtk proxy go build ./...`
Expected: success — no remaining references to the deleted `run` command or old `Target`/`Tasks` API.

- [ ] **Step 6: Commit**

```bash
rtk proxy git add internal/cli/hook_cmd.go internal/cli/hook_cmd_test.go
```
```bash
rtk proxy git rm internal/cli/run_cmd.go
```
```bash
rtk proxy git commit -m "feat(cli): replace run with hook command (-- splits selectors from git args)"
```

---

## Task 3: Update CLI end-to-end scripts

**Files:**
- Create: `cmd/liszt/testdata/script/hook.txtar`
- Delete: `cmd/liszt/testdata/script/run.txtar`
- Modify: `cmd/liszt/testdata/script/help.txtar`
- Modify: `cmd/liszt/testdata/script/bare_help.txtar`

- [ ] **Step 1: Write the new `hook.txtar`**

Create `cmd/liszt/testdata/script/hook.txtar`:

```
# general only -> header + general output, exit 0
exec liszt hook pre-commit
stdout '== pre-commit.general =='
stdout 'hello-general'
! stdout '== pre-commit.gleam =='

# general + named lang, in order
exec liszt hook pre-commit gleam
stdout '== pre-commit.general =='
stdout '== pre-commit.gleam =='
stdout 'hello-gleam'

# git args after -- reach commands as positional params
exec liszt hook echoarg -- WORLD
stdout 'got:WORLD'

# a failing command -> non-zero exit, FAILED + hint on stderr
! exec liszt hook failing
stderr 'FAILED:'
stderr 'hint: fix the thing'

# missing config file -> exit 1
! exec liszt hook pre-commit --config does-not-exist.toml
stderr 'read does-not-exist.toml'

# unknown hook -> exit 1
! exec liszt hook ghost
stderr 'no \[ghost\] hook'

# unknown segment -> exit 1
! exec liszt hook pre-commit rust
stderr 'no \[pre-commit\.rust\] segment'

# disabled segment -> exit 0, no header for it
exec liszt hook withoff
! stdout '== withoff.general =='
stdout 'on-runs'

# --config override resolves an alternate file
exec liszt hook only --config alt/hooks.toml
stdout 'from-alt'

-- .liszt/hooks.toml --
[pre-commit.general]
run = ["echo hello-general"]

[pre-commit.gleam]
run = ["echo hello-gleam"]

[failing.general]
run = ["false"]
fail_hint = "fix the thing"

[echoarg.general]
run = ["printf 'got:%s' \"$1\""]

[withoff.general]
run = ["echo should-not-run"]
enabled = false

[withoff.go]
run = ["echo on-runs"]

-- alt/hooks.toml --
[only.general]
run = ["echo from-alt"]
```

- [ ] **Step 2: Delete the old `run.txtar`**

Delete `cmd/liszt/testdata/script/run.txtar`.

- [ ] **Step 3: Update `help.txtar` and `bare_help.txtar`**

In `cmd/liszt/testdata/script/help.txtar`, change the line `stdout 'run'` to `stdout 'hook'`.

In `cmd/liszt/testdata/script/bare_help.txtar`, change the line `stdout 'run'` to `stdout 'hook'`.

- [ ] **Step 4: Run the script tests, verify they pass**

Run: `rtk proxy go test ./cmd/liszt/ -v`
Expected: PASS (`TestScripts` including `hook`, `help`, `bare_help`, `version`, `no_color`).

- [ ] **Step 5: Commit**

```bash
rtk proxy git add cmd/liszt/testdata/script/hook.txtar cmd/liszt/testdata/script/help.txtar cmd/liszt/testdata/script/bare_help.txtar
```
```bash
rtk proxy git rm cmd/liszt/testdata/script/run.txtar
```
```bash
rtk proxy git commit -m "test(cli): hook command end-to-end scripts"
```

---

## Task 4: Add this repo's own `.liszt/hooks.toml` (dogfood)

**Files:**
- Create: `.liszt/hooks.toml`

This repo is Go; its pre-commit hook runs the same checks as the Makefile (`vet`, `lint`, `test`) plus a whitespace check. `general` holds language-agnostic checks; `go` holds the Go toolchain checks.

- [ ] **Step 1: Create `.liszt/hooks.toml`**

Create `.liszt/hooks.toml`:

```toml
[pre-commit.general]
run = ["git diff --check"]
fail_hint = "remove trailing whitespace / conflict markers"

[pre-commit.go]
run = [
  "go vet ./cmd/liszt ./internal/...",
  "golangci-lint run",
  "go test ./... -race -covermode=atomic -coverpkg=./...",
]
fail_hint = "run 'make vet lint test' to reproduce locally"
```

- [ ] **Step 2: Verify it loads and resolves**

Run: `rtk proxy go run ./cmd/liszt hook pre-commit go -- ignored`
Expected: runs `== pre-commit.general ==` (git diff --check), then `== pre-commit.go ==` (vet, lint, test). Exit 0 if the tree is clean and tests pass; non-zero with a `FAILED:`/`hint:` line otherwise. Either outcome confirms loading, resolution, and segment ordering work end-to-end.

- [ ] **Step 3: Commit**

```bash
rtk proxy git add .liszt/hooks.toml
```
```bash
rtk proxy git commit -m "chore: dogfood liszt hook config for this repo's pre-commit"
```

---

## Task 5: Full verification

**Files:** none (verification only)

- [ ] **Step 1: Run the full test suite with race + coverage**

Run: `rtk proxy go test ./... -race -covermode=atomic -coverpkg=./... -coverprofile=cover.out`
Expected: PASS, no race warnings.

- [ ] **Step 2: Inspect coverage**

Run: `rtk proxy go tool cover -func=cover.out`
Expected: `internal/runner` at 100%; `internal/cli` hook command paths covered (success via testscript, error branches via the missing-hook/missing-segment/missing-file scripts; the `return nil` after `os.Exit` is the only intentionally-unreachable line). Add tests for any unexpected gap.

- [ ] **Step 3: Lint**

Run: `rtk proxy golangci-lint run`
Expected: no findings.

- [ ] **Step 4: Confirm the old verb is gone**

Run: `rtk proxy go run ./cmd/liszt --help`
Expected: lists `hook` and `version`; no `run`.

---

## Self-Review

**Spec coverage:**
- Hook grammar `hook <name> [lang...] [-- gitargs]` → Task 2 (`splitHookArgs`, `ArgsLenAtDash`).
- `general` always-first, named langs in order, missing-segment/hook/nothing-to-run errors → Task 1 `Resolve` + tests.
- Args forwarded as `$1`/`$@` to every command across segments → Task 1 `RunHook`/`run` + tests.
- First-failure retention, disabled skip, empty-run error, fail_hint, 127 mapping → Task 1 tests.
- `.liszt/hooks.toml`, nested `[<hook>.<segment>]`, removal of `run`/`[tasks.*]` → Tasks 2 (delete run_cmd), 3 (scripts), default flag path; dogfood config Task 4.
- Top-level named-map decode → Task 1 `Load` (pre-verified).
- Testing 100% target → Tasks 1, 2, 5.

**Placeholder scan:** No TBD/TODO; every code step shows full content.

**Type consistency:** `Segment`/`Hook`/`Config`, `Load` (returns `Config`), `Config.Resolve(name, langs) ([]string, error)`, `Config.RunHook(name, segments, args, stdout, stderr) int`, `Segment.run(...)`, `splitHookArgs(args, dash) (name, langs, gitArgs)`, `failureLine`, `exitCode` — names and signatures match across Tasks 1–4 and the spec.
