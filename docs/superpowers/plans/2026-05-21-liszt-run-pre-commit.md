# `liszt run <name>` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `liszt run <name>` subcommand that executes shell commands declared under a `[run.<name>]` table in `.liszt/liszt.toml`.

**Architecture:** A new `internal/runner` package owns TOML parsing and shell execution (pure, returns an exit code, never calls `os.Exit`). A thin `internal/cli/run_cmd.go` wires cobra, resolves the target, streams command output verbatim, and maps the runner's exit code to `os.Exit`.

**Tech Stack:** Go, `github.com/spf13/cobra`, `github.com/pelletier/go-toml/v2`, `github.com/rogpeppe/go-internal/testscript`.

---

## File Structure

| File | Responsibility |
|------|----------------|
| `internal/runner/runner.go` | `Config`, `Target`, `Load`, `Config.Target`, `Target.Run`, exit-code helper. |
| `internal/runner/runner_test.go` | Unit tests for parse + execution semantics. |
| `internal/cli/run_cmd.go` | Cobra `run <name>` command, `--config` flag, exit-code mapping. |
| `cmd/liszt/testdata/script/run.txtar` | testscript end-to-end cases. |

The runner is its own package so the shell-exec and exit-code logic is unit-testable in isolation. `pre-commit` is just the first target name; nothing in the code is pre-commit specific.

---

## Task 1: runner package — types and `Load`

**Files:**
- Create: `internal/runner/runner.go`
- Test: `internal/runner/runner_test.go`

- [ ] **Step 1: Write the failing tests for `Load` and `Target`**

Create `internal/runner/runner_test.go`:

```go
package runner

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTOML(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "liszt.toml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func TestLoad_Valid(t *testing.T) {
	t.Parallel()
	path := writeTOML(t, `
[run.pre-commit]
cmd = ["echo a", "echo b"]
fail_hint = "do the thing"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	target, ok := cfg.Target("pre-commit")
	if !ok {
		t.Fatalf("Target(pre-commit) ok=false")
	}
	if len(target.Cmd) != 2 || target.Cmd[0] != "echo a" {
		t.Errorf("unexpected Cmd: %#v", target.Cmd)
	}
	if target.FailHint != "do the thing" {
		t.Errorf("unexpected FailHint: %q", target.FailHint)
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
	path := writeTOML(t, "this is = = not toml")
	if _, err := Load(path); err == nil {
		t.Fatal("Load on malformed TOML should error")
	}
}

func TestLoad_CmdWrongType(t *testing.T) {
	t.Parallel()
	path := writeTOML(t, "[run.x]\ncmd = \"bare string\"\n")
	if _, err := Load(path); err == nil {
		t.Fatal("Load with string cmd should error (must be array)")
	}
}

func TestTarget_Miss(t *testing.T) {
	t.Parallel()
	path := writeTOML(t, "[run.x]\ncmd = [\"echo hi\"]\n")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := cfg.Target("ghost"); ok {
		t.Error("Target(ghost) should be ok=false")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `rtk go test ./internal/runner/`
Expected: FAIL — `runner.go` does not exist (`undefined: Load`).

- [ ] **Step 3: Write `runner.go` with types, `Load`, and `Target`**

Create `internal/runner/runner.go`:

```go
// Package runner executes named command groups declared in .liszt/liszt.toml.
package runner

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

// Target is one [run.<name>] table: a group of shell commands.
type Target struct {
	Cmd      []string `toml:"cmd"`
	FailHint string   `toml:"fail_hint"`
	Enabled  *bool    `toml:"enabled"` // nil => enabled
}

// Config models a .liszt/liszt.toml run section.
type Config struct {
	Run map[string]Target `toml:"run"`
}

// Load reads and decodes path. A missing or unreadable file, or malformed
// TOML (including a non-array cmd), is an error.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	cfg := &Config{}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
}

// Target returns the named target and whether it exists.
func (c *Config) Target(name string) (Target, bool) {
	t, ok := c.Run[name]
	return t, ok
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `rtk go test ./internal/runner/`
Expected: PASS (5 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/runner/runner.go internal/runner/runner_test.go
git commit -m "feat(runner): add Config/Target types and Load"
```

---

## Task 2: runner package — `Target.Run` execution

**Files:**
- Modify: `internal/runner/runner.go`
- Test: `internal/runner/runner_test.go`

- [ ] **Step 1: Write the failing tests for `Run`**

Append to `internal/runner/runner_test.go`:

```go
import (
	"bytes"
	"strings"
)

func boolPtr(b bool) *bool { return &b }

func TestRun_Disabled(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	tgt := Target{Cmd: []string{"echo nope"}, Enabled: boolPtr(false)}
	if code := tgt.Run("x", &out, &errOut); code != 0 {
		t.Errorf("disabled target should return 0, got %d", code)
	}
	if out.Len() != 0 {
		t.Errorf("disabled target should print nothing, got %q", out.String())
	}
}

func TestRun_EmptyCmd(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	tgt := Target{Cmd: nil}
	if code := tgt.Run("x", &out, &errOut); code != 1 {
		t.Errorf("empty cmd should return 1, got %d", code)
	}
	if !strings.Contains(errOut.String(), "empty cmd") {
		t.Errorf("expected empty cmd message, got %q", errOut.String())
	}
}

func TestRun_AllPass(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	tgt := Target{Cmd: []string{"echo first", "echo second"}}
	if code := tgt.Run("pre-commit", &out, &errOut); code != 0 {
		t.Errorf("all-pass should return 0, got %d", code)
	}
	s := out.String()
	if !strings.Contains(s, "== run pre-commit ==") {
		t.Errorf("missing header in %q", s)
	}
	if !strings.Contains(s, "first") || !strings.Contains(s, "second") {
		t.Errorf("command output missing in %q", s)
	}
}

func TestRun_RetainsFirstFailure(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	// First command exits 3, second exits 4; all run, first failure (3) retained.
	tgt := Target{
		Cmd:      []string{"exit 3", "echo ran-anyway", "exit 4"},
		FailHint: "fix me",
	}
	code := tgt.Run("x", &out, &errOut)
	if code != 3 {
		t.Errorf("expected retained first failure exit 3, got %d", code)
	}
	if !strings.Contains(out.String(), "ran-anyway") {
		t.Errorf("later command should still run; out=%q", out.String())
	}
	es := errOut.String()
	if !strings.Contains(es, "FAILED:") || !strings.Contains(es, "exit 3") {
		t.Errorf("expected FAILED line with exit 3, got %q", es)
	}
	if !strings.Contains(es, "hint: fix me") {
		t.Errorf("expected fail_hint, got %q", es)
	}
}

func TestRun_NoHintWhenUnset(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	tgt := Target{Cmd: []string{"exit 1"}}
	tgt.Run("x", &out, &errOut)
	if strings.Contains(errOut.String(), "hint:") {
		t.Errorf("no fail_hint set, should not print hint line: %q", errOut.String())
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `rtk go test ./internal/runner/`
Expected: FAIL — `tgt.Run undefined (type Target has no field or method Run)`.

- [ ] **Step 3: Implement `Run`, `isEnabled`, and `exitCode`**

Add to `internal/runner/runner.go` — extend the imports and append the methods:

```go
import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/pelletier/go-toml/v2"
)
```

```go
func (t Target) isEnabled() bool {
	return t.Enabled == nil || *t.Enabled
}

// Run executes the target's commands via `bash -c`, streaming each command's
// stdout/stderr to the provided writers. All commands run even if an earlier
// one fails; the exit code of the first failing command is retained and
// returned (0 = all passed). A disabled target returns 0 without output; a
// target with no commands returns 1.
func (t Target) Run(name string, stdout, stderr io.Writer) int {
	if !t.isEnabled() {
		return 0
	}
	if len(t.Cmd) == 0 {
		fmt.Fprintf(stderr, "error: [run.%s] has empty cmd\n", name)
		return 1
	}

	fmt.Fprintf(stdout, "== run %s ==\n", name)

	failCode := 0
	failCmd := ""
	for _, c := range t.Cmd {
		cmd := exec.Command("bash", "-c", c)
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil && failCode == 0 {
			failCode = exitCode(err)
			failCmd = c
		}
	}

	if failCode != 0 {
		fmt.Fprintf(stderr, "FAILED: %s (exit %d)\n", failCmd, failCode)
		if t.FailHint != "" {
			fmt.Fprintf(stderr, "hint: %s\n", t.FailHint)
		}
	}
	return failCode
}

// exitCode extracts the process exit code from a command error. A non-exit
// error (e.g. bash not found) or a signal kill maps to 1.
func exitCode(err error) int {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if code := ee.ExitCode(); code > 0 {
			return code
		}
	}
	return 1
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `rtk go test ./internal/runner/`
Expected: PASS (10 tests).

- [ ] **Step 5: Verify full coverage of the runner package**

Run: `rtk go test -cover ./internal/runner/`
Expected: `coverage: 100.0% of statements`. If below, add the missing case (e.g. the `exitCode` non-`ExitError` branch is covered by `TestRun_EmptyCmd`? no — add a test that runs a non-existent binary if needed: `Target{Cmd: []string{"this-binary-does-not-exist-xyz"}}` expecting code 1).

- [ ] **Step 6: Commit**

```bash
git add internal/runner/runner.go internal/runner/runner_test.go
git commit -m "feat(runner): execute target commands with retained exit code"
```

---

## Task 3: CLI wiring — `run <name>` command

**Files:**
- Create: `internal/cli/run_cmd.go`

- [ ] **Step 1: Write `run_cmd.go`**

Create `internal/cli/run_cmd.go`:

```go
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mguilarducci/liszt/internal/runner"
)

var runConfigPath string

var runCmd = &cobra.Command{
	Use:   "run <name>",
	Short: "Run a named command group from .liszt/liszt.toml",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		cfg, err := runner.Load(runConfigPath)
		if err != nil {
			return err
		}
		target, ok := cfg.Target(args[0])
		if !ok {
			return fmt.Errorf("no [run.%s] target in %s", args[0], runConfigPath)
		}
		os.Exit(target.Run(args[0], os.Stdout, os.Stderr))
		return nil // coverage: unreachable, os.Exit terminates the process
	},
}

func init() {
	runCmd.Flags().StringVar(&runConfigPath, "config", ".liszt/liszt.toml", "config path")
	rootCmd.AddCommand(runCmd)
}
```

- [ ] **Step 2: Verify it builds**

Run: `rtk go build ./...`
Expected: no output, exit 0.

- [ ] **Step 3: Verify the command is registered**

Run: `rtk go run ./cmd/liszt run --help`
Expected: usage text containing `run <name>` and the `--config` flag with default `.liszt/liszt.toml`.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/run_cmd.go
git commit -m "feat(cli): wire run <name> command"
```

---

## Task 4: testscript end-to-end cases

**Files:**
- Create: `cmd/liszt/testdata/script/run.txtar`

The testscript harness (`cmd/liszt/script_test.go`) re-execs the binary per command, so the `os.Exit` in `run_cmd.go` produces an observable exit status. `! exec` asserts a non-zero exit.

- [ ] **Step 1: Write the testscript file**

Create `cmd/liszt/testdata/script/run.txtar`:

```
# all commands pass -> exit 0, header + output present
exec liszt run pre-commit
stdout '== run pre-commit =='
stdout 'hello-pre-commit'

# a failing command -> non-zero exit, FAILED + hint on stderr
! exec liszt run failing
stderr 'FAILED:'
stderr 'hint: fix the thing'

# missing config file -> exit 1
! exec liszt run pre-commit --config does-not-exist.toml
stderr 'read does-not-exist.toml'

# unknown target -> exit 1
! exec liszt run ghost
stderr 'no \[run\.ghost\] target'

# disabled target -> exit 0, no header printed
exec liszt run off
! stdout '== run off =='

# --config override resolves an alternate file
exec liszt run only --config alt/liszt.toml
stdout 'from-alt'

-- .liszt/liszt.toml --
[run.pre-commit]
cmd = ["echo hello-pre-commit"]

[run.failing]
cmd = ["false"]
fail_hint = "fix the thing"

[run.off]
cmd = ["echo should-not-run"]
enabled = false

-- alt/liszt.toml --
[run.only]
cmd = ["echo from-alt"]
```

- [ ] **Step 2: Run the testscript suite to verify it passes**

Run: `rtk go test ./cmd/liszt/ -run TestScripts`
Expected: PASS.

- [ ] **Step 3: Run the full test suite**

Run: `rtk go test ./...`
Expected: all packages PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/liszt/testdata/script/run.txtar
git commit -m "test(cli): cover run command via testscript"
```

---

## Self-Review Notes

- **Spec coverage:** Config schema → Task 1. `cmd` array / wrong-type error → Task 1 (`TestLoad_CmdWrongType`). Execution semantics (all run, retain first failure, fail_hint last) → Task 2. `enabled=false` skip → Task 2 + Task 4. Strict errors (missing file / missing target) → Task 1 + Task 3 + Task 4. Exit-code propagation via `os.Exit` → Task 3 + Task 4. `--config` flag → Task 3 + Task 4. Stream-raw output → Task 2 (writers) + Task 4 (`stdout`/`stderr` matchers).
- **Type consistency:** `Target.Run(name string, stdout, stderr io.Writer) int` is defined in Task 2 and called identically in Task 3. `Config.Target(name) (Target, bool)` defined in Task 1, used in Task 3.
- **Out of scope (per spec non-goals):** removing `pre-commit.sh` and rewiring the marketplace git hook is follow-up work, not in this plan.
```
