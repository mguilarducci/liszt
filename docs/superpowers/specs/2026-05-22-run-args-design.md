# `liszt run <name> [args...]` — argument passthrough — Design

`liszt run` forwards extra CLI arguments to the task's commands as bash
positional parameters (`$1`, `$2`, `$@`). This lets git hooks that receive
arguments — most importantly `commit-msg`, whose `$1` is the commit-message
file path — be backed by a plain `[tasks.<name>]` task, without the runner
gaining any per-hook knowledge.

## Motivation

The runner stays minimal: `liszt run <name>` executes the `run` array of that
task, nothing more. The one thing it could not do is hand a command the value
git passes to a hook. Reading a fixed path (`.git/COMMIT_EDITMSG`) was the only
workaround, and it only happens to work for `commit-msg`. Forwarding positional
arguments is the smallest, idiomatic mechanism that covers the general case
while keeping the runner dumb.

## Goals

- `liszt run <name> a b c` runs each command in `[tasks.<name>].run` with `a`,
  `b`, `c` available as bash positional parameters `$1`, `$2`, `$3` (`$@`).
- A command that does not reference any positional parameter is unaffected —
  existing tasks keep working unchanged.
- No new TOML syntax. The command string uses ordinary shell positional refs.

## Non-goals

These remain out of scope (the runner does not grow them):

- stdin passthrough (e.g. `pre-push` refs on stdin).
- conditional execution by changed/staged file globs.
- task composition (a task invoking another task).
- `fail_fast`, parallelism, per-task env or working directory.

## Mechanism

Commands already run via `bash -c "<cmd>"`. `bash -c` accepts trailing operands
that become `$0`, `$1`, … inside the script. The runner forwards the extra args
there:

```go
// before
cmd := exec.Command("bash", "-c", c)
// after
cmd := exec.Command("bash", append([]string{"-c", c, "bash"}, args...)...)
```

`bash -c "<script>" bash a b` sets `$0`="bash" (a conventional placeholder),
`$1`="a", `$2`="b", `$@`="a b" inside `<script>`.

- Every command in the `run` array receives the **same** positional parameters;
  each command references `$1`/`$@` only if it needs them.
- Empty args (`liszt run lint`) leave `$1` unset/empty — identical to today's
  behavior for tasks that reference nothing.

## Package `internal/runner`

`Target.Run` gains an `args []string` parameter, threaded into each
`exec.Command`. Everything else (enabled check, empty-`run` error, run-all,
first-failure retention, `fail_hint`, exit-code extraction) is unchanged.

```go
// Run executes the target's commands via `bash -c`, forwarding args as bash
// positional parameters ($1, $@) to every command. Behavior is otherwise
// unchanged.
func (t Target) Run(name string, args []string, stdout, stderr io.Writer) int
```

## CLI wiring (`internal/cli/run_cmd.go`)

- `Args: cobra.ExactArgs(1)` → `cobra.MinimumNArgs(1)`.
- `args[0]` is the task name; `args[1:]` are forwarded.

```go
target.Run(args[0], args[1:], os.Stdout, os.Stderr)
```

The `--config` flag and exit-code mapping (`os.Exit`) are unchanged.

## Hook consumer (illustrative, out of scope)

The git hook script forwards git's arguments to liszt; liszt forwards them to
bash. Wiring lives in the marketplace repo, not here.

```sh
#!/bin/sh
exec liszt run commit-msg "$@"
```

```toml
[tasks.commit-msg]
run = ["conventional-commit-check $1"]
fail_hint = "format: <type>(<scope>): <description> — see GIT.md"
```

## Testing (100% coverage target)

**Unit — `internal/runner`:**

- args reach a single command: `run = ["printf '%s' \"$1\""]` with
  `args=["hello"]` writes `hello` to stdout.
- `$@` expands all args, in order, for a command that echoes `"$@"`.
- args reach **every** command in a multi-command array.
- no args (`args=nil`): `$1` empty; an existing reference-free command still
  succeeds (exit 0). Confirms backward compatibility.
- args are still forwarded on the failure path (a failing command that
  references `$1`), so the failure-retention logic is exercised with args.

**testscript — `cmd/liszt/script_test.go`:**

- `liszt run <name> somearg` where the task's command echoes `$1` → output
  contains `somearg`, exit 0.
- existing no-arg cases (`MinimumNArgs(1)` still requires the task name; zero
  args → usage error, exit non-zero) remain green.

## Files

| File | Change |
|------|--------|
| `internal/runner/runner.go` | `Run` gains `args []string`; forward to `bash -c` operands. |
| `internal/runner/runner_test.go` | Arg-forwarding unit cases. |
| `internal/cli/run_cmd.go` | `MinimumNArgs(1)`; pass `args[1:]` to `Run`. |
| `cmd/liszt/script_test.go` | testscript arg-passthrough case. |
