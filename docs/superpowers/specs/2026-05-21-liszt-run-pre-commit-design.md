# `liszt run <name>` — Design

A generic command runner for the `liszt` CLI. `liszt run <name>` executes the
shell commands declared under a `[run.<name>]` table in `.liszt/liszt.toml`.
The first target is `pre-commit`; the command replaces the pure-bash
`plugins/insurance/scripts/pre-commit.sh` runner from the marketplace.

## Goals

- Run a named group of shell commands declared in `.liszt/liszt.toml`.
- Stream command output verbatim (no wrapping) so tool errors stay readable.
- Propagate a meaningful exit code so a git pre-commit hook can block commits.
- Strict, fail-loud behavior: missing config or missing target is an error.

## Non-goals

- Removing `pre-commit.sh` and rewiring the git hook to call `liszt run
  pre-commit`. That is follow-up work in the marketplace repo, out of scope here.
- Parallel execution, command timeouts, environment injection, or per-command
  working directories. YAGNI.

## Config file

Path: `.liszt/liszt.toml`, resolved relative to the current working directory.
A `--config <path>` flag overrides the default.

This file uses a `[run.<name>]` namespace, distinct from the root `liszt.toml`
manifest (`[[items]]` with `kind`/`slug`/`flavor`). The two files share a name
but never share a schema and are never merged.

```toml
[run.pre-commit]
cmd       = ["gleam format --check src test", "gleam check"]
fail_hint = "run `gleam format src test` to fix formatting"
enabled   = true            # optional, defaults to true

[run.lint]
cmd = ["golangci-lint run"]
```

`<name>` matches the TOML bare-key rules (the marketplace uses
`[A-Za-z0-9_-]+`, e.g. `pre-commit`). `liszt run pre-commit` runs the
`[run.pre-commit]` table; `liszt run lint` runs `[run.lint]`.

### Target keys

| Key | Type | Required | Default | Meaning |
|-----|------|----------|---------|---------|
| `cmd` | array of strings | **yes** | — | Shell commands to run, in array order. |
| `fail_hint` | string | no | (none) | Printed as the last line to stderr if the target fails. |
| `enabled` | boolean | no | `true` | When `false`, the target is skipped and the run exits `0`. |

- `cmd` must be an array of strings. A bare string (`cmd = "..."`) is a TOML
  type mismatch and fails to decode — this is an error, not a silent skip.
- An empty array (`cmd = []`) is an error.

## Package `internal/runner`

The parser and executor live in their own package so the shell-exec and exit-code
logic is unit-testable in isolation. `internal/cli` is thin wiring.

```go
package runner

type Target struct {
    Cmd      []string `toml:"cmd"`
    FailHint string   `toml:"fail_hint"`
    Enabled  *bool    `toml:"enabled"` // nil => treated as true
}

type Config struct {
    Run map[string]Target `toml:"run"`
}

// Load reads and decodes path. A missing file is an error (strict).
func Load(path string) (*Config, error)

// Target returns the named target and whether it exists.
func (c *Config) Target(name string) (Target, bool)

// Run executes the target's commands and returns the aggregate exit code
// (0 = all passed). It never calls os.Exit; the caller maps the code.
func (t Target) Run(stdout, stderr io.Writer) int
```

- `Load`: missing file, unreadable file, or malformed TOML (including `cmd` of
  the wrong type) returns an error wrapped with the path.
- `Enabled` is `*bool` so an unset value defaults to `true`; only an explicit
  `enabled = false` skips.

## Execution semantics

For `liszt run <name>`:

1. `Load(.liszt/liszt.toml)`. Error → exit `1`.
2. `Target(name)`. Missing → error, exit `1`.
3. If `enabled = false`: print nothing, exit `0`.
4. If `cmd` is empty: error, exit `1`.
5. Print the header `== run <name> ==` to stdout.
6. Run each `cmd` element via `bash -c "<elem>"` in array order. Each command's
   stdout/stderr is streamed straight to the process stdout/stderr (verbatim).
7. **Every command runs even if an earlier one fails.** The exit code of the
   **first** failing command is retained.
8. After all commands run, if the target failed:
   - Print `FAILED: <cmd> (exit <code>)` to stderr (the first failing command).
   - If `fail_hint` is set, print `hint: <fail_hint>` to stderr as the last line.
   - Exit with the retained code.
9. If all commands passed, exit `0`.

`bash -c` is used (matching the existing script and its documented limitation
that escaped quotes inside a command are not handled). Commands run in the
process's current working directory.

### Exit-code propagation

`runner` returns the aggregate exit code as an `int` and never calls `os.Exit`,
keeping it pure and unit-testable. The thin wiring in `internal/cli/run_cmd.go`
calls `os.Exit(code)` after the runner prints — the standard subprocess-wrapper
pattern (cf. `xargs`, `time`). Routing the failure through cobra/fang as an
`error` instead would double-print, since fang renders a styled error box and we
already stream our own `FAILED`/`hint` output. The `os.Exit` path is exercised
end-to-end by the testscript harness (which runs the real binary).

## CLI wiring (`internal/cli/run_cmd.go`)

```go
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
        os.Exit(target.Run(os.Stdout, os.Stderr))
        return nil // unreachable
    },
}

func init() {
    runCmd.Flags().StringVar(&runConfigPath, "config", ".liszt/liszt.toml", "config path")
    rootCmd.AddCommand(runCmd)
}
```

Operational failures (missing config, missing target) return through `RunE` and
are rendered by fang, exiting `1` via `main.go`. Check failures exit via
`os.Exit` with the retained command code.

## Testing (100% coverage target)

**Unit — `internal/runner`:**

- `Load`: valid file; missing file (error); malformed TOML (error); `cmd` as a
  bare string (decode error); `cmd = []` (error surfaced at run).
- `Target`: present; absent.
- `Enabled`: nil → true; explicit `true`; explicit `false` → skip path.
- `Run`: single command success (exit 0); multiple commands where a middle one
  fails (all run, first failing exit retained); `fail_hint` printed last on
  failure; output streamed to the provided writers (assert via buffers).

**testscript — `cmd/liszt/script_test.go`:**

- `liszt run pre-commit` success → exit 0, command output present.
- failure → exit non-zero, `FAILED` + `hint` on stderr.
- missing `.liszt/liszt.toml` → exit 1.
- unknown target → exit 1.
- `enabled = false` → exit 0, no command output.
- `--config <path>` override resolves an alternate file.

## Files

| File | Purpose |
|------|---------|
| `internal/runner/runner.go` | `Config`, `Target`, `Load`, `Target`, `Run`. |
| `internal/runner/runner_test.go` | Unit tests. |
| `internal/cli/run_cmd.go` | Cobra wiring, `--config` flag, exit-code mapping. |
| `cmd/liszt/script_test.go` | testscript cases (extend existing file). |
