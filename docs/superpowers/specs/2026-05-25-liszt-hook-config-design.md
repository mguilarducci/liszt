# `liszt hook <name> [lang...] [-- gitargs...]` — hook config separation — Design

Replace the flat `[tasks.<name>]` runner with a hook-oriented model that
separates a hook's **general** steps from its **per-language** steps. A hook is
invoked as `liszt hook <name> [lang...] [-- gitargs...]`: the `general` segment
always runs, named language segments run on request, and everything after `--`
is forwarded to every command as bash positional parameters (`$1`, `$@`) — the
channel git uses to hand a hook its arguments.

## Motivation

`.liszt/liszt.toml` currently mixes every task into one flat namespace
(`[tasks.gleam-pre-commit]`, `[tasks.commit-msg]`, …). There is no structural
distinction between work that runs regardless of language and work that only
applies to one language, so the split lives in ad-hoc task names and in the git
hook scripts that call them.

This design makes the split first-class in the config: each hook is a table
whose subtables are segments — one reserved `general` segment plus any number of
named language segments. The caller composes a run by naming the languages it
wants; `general` is implicit and always included. No language auto-detection
(a missing/misdetected marker file failing silently is the failure mode we are
avoiding) — the git hook script names the languages explicitly, because it is
the thing that knows the repo.

## Goals

- `liszt hook pre-commit gleam -- "$@"` runs `[pre-commit.general]` then
  `[pre-commit.gleam]`, forwarding the args after `--` to every command.
- `general` is a reserved segment name that always runs, first, when present.
- Named language segments run after `general`, in the order given on the CLI.
- A named segment that does not exist is a hard error (no silent skip).
- Config lives in `.liszt/hooks.toml`, structurally separating each hook and its
  language segments. The root `liszt.toml` (plugins) is untouched.
- Single config file, nested tables (chosen layout; see Alternatives).

## Non-goals

The runner does not grow these:

- Language auto-detection from marker files (`gleam.toml`, `go.mod`).
- Conditional execution by changed/staged file globs.
- Segment-to-segment composition beyond the `general` + named-langs rule.
- stdin passthrough, parallelism, per-segment env or working directory,
  `fail_fast`.
- Backward compatibility with `liszt run` / `[tasks.*]` — both are removed
  outright, not aliased.

## CLI grammar and semantics

```
liszt hook <name> [lang...] [-- gitargs...]
```

| Input | Behavior |
|------|----------|
| `<name>` | Hook table to run. Absent from config → error. |
| `lang...` | Segments to run after `general`, in order. Each missing → error. |
| `-- gitargs...` | Positional params forwarded to **every** command (`$1`…, `$@`). |
| (no lang, `general` present) | Runs `general` only. |
| (no lang, no `general`) | Error: nothing to run. |
| `enabled = false` on a segment | That segment is skipped (no output). |
| failure | All selected segments and their commands run; the **first** failing command's exit code is retained and returned. |

`liszt run` is deleted. `hook` is the only task-execution verb.

`general` + named-lang ordering is resolved on the CLI side into an ordered list
of segment names; the runner receives that list and executes the segments in
order. Positional args before `--` are segment selectors; args at/after `--`
(detected via cobra's `ArgsLenAtDash`) are git args. This keeps selectors and
git args on separate, unambiguous channels.

## Config schema (`.liszt/hooks.toml`)

```toml
[pre-commit.general]
run = ["typos", "git diff --check"]
fail_hint = "run 'typos -w' to autofix"

[pre-commit.gleam]
run = ["gleam format --check src test", "gleam test"]

[commit-msg.general]
run = ['commitlint --edit "$1"']   # $1 = commit-msg file path (git)
```

- Top-level key (`pre-commit`, `commit-msg`) = hook name.
- Second-level key (`general`, `gleam`) = segment name. `general` is reserved.
- `run` (array, required), `fail_hint` (string, optional), `enabled` (bool,
  optional, default true) live on each segment — same fields as today's
  `Target`, renamed `Segment`.

Scalar-before-subtable TOML rule: making `general` a subtable
(`[pre-commit.general]`) rather than a loose `general = [...]` array avoids the
ordering pitfall where a scalar key would bind to the wrong table.

## Package `internal/runner`

```go
// Segment is one [<hook>.<segment>] table: a group of shell commands plus
// failure metadata. (Was: Target.)
type Segment struct {
    Commands []string `toml:"run"`
    FailHint string   `toml:"fail_hint"`
    Enabled  *bool    `toml:"enabled"` // nil => enabled
}

// Hook is one [<hook>.*] table: segments keyed by name. "general" is reserved
// and always runs first when present.
type Hook map[string]Segment

// Config is every hook in the file. Top-level TOML keys decode directly into
// this map (no wrapper table), so Config is a named map type.
type Config map[string]Hook
```

`Load` reads `.liszt/hooks.toml` and unmarshals into `Config`. Top-level
arbitrary keys decode into a `map[string]...` directly with
`github.com/pelletier/go-toml/v2`; the exact decode shape (named map type vs.
`map[string]map[string]Segment` with conversion) is the one technical risk and
will be confirmed against the library in the planning phase before coding.

Execution API (segment-level run logic is preserved from today's `Target.Run`):

```go
// Run executes the named segments in order. Within each segment, commands run
// via `bash -c`, with args forwarded as positional parameters ($1, $@) to every
// command. All commands run even on failure; the first failing command's exit
// code is retained and returned (0 = all passed). A disabled segment is
// skipped. Returns a distinct error/code if no segment was runnable.
func (c Config) RunHook(name string, segments []string, args []string, stdout, stderr io.Writer) int
```

`segments` is the resolved ordered list (`general` first if present, then the
requested langs). The per-command `bash -c "<cmd>" bash <args...>` mechanism,
first-failure retention, `fail_hint` printing, and exit-code extraction are
carried over unchanged from the current runner.

## CLI wiring (`internal/cli/hook_cmd.go`, replaces `run_cmd.go`)

```go
var hookCmd = &cobra.Command{
    Use:   "hook <name> [lang...] [-- gitargs...]",
    Short: "Run a git hook from .liszt/hooks.toml",
    Args:  cobra.MinimumNArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        // dash = index where "--" appeared; -1 if absent.
        dash := cmd.ArgsLenAtDash()
        name, langs, gitArgs := splitArgs(args, dash)
        cfg, err := runner.Load(hookConfigPath)
        if err != nil { return err }
        segments, err := cfg.Resolve(name, langs) // general + langs, validates existence
        if err != nil { return err }
        os.Exit(cfg.RunHook(name, segments, gitArgs, os.Stdout, os.Stderr))
        return nil // unreachable: os.Exit terminates
    },
}
// flag default: ".liszt/hooks.toml"
```

`Resolve` is where "hook missing", "segment missing", and "nothing to run"
become errors, and where `general` is prepended. Splitting it from `RunHook`
keeps validation (pure, easily tested) separate from process execution.

## Hook consumer (illustrative, out of scope)

The git hook script names the languages and forwards git's args:

```sh
#!/bin/sh
# .git/hooks/pre-commit — polyglot repo
exec liszt hook pre-commit go gleam -- "$@"
```

```sh
#!/bin/sh
# .git/hooks/commit-msg — $1 is the message file path
exec liszt hook commit-msg -- "$@"
```

## Alternatives considered

- **File per hook** (`.liszt/hooks/pre-commit.toml`): maximal separation but
  needs directory-walking load. Deferred (YAGNI) until the single file is too
  large; migration is mechanical.
- **Axis split** (`hooks.toml` for general + `lang/<x>.toml` for languages):
  mirrors the mental model but spreads one hook across files, hurting
  end-to-end readability.
- **Overload `liszt run`** with shape-dependent positional meaning: rejected —
  the meaning of a positional would depend on the TOML shape, and git args
  already own the positionals.

## Testing (100% coverage target)

**Unit — `internal/runner`:**

- `Resolve`: `general` prepended when present; omitted when absent but langs
  given; error when hook missing; error when a named lang missing; error when
  neither general nor any lang is runnable; order preserved (general, then langs
  in CLI order).
- `RunHook`: runs segments in resolved order; within a segment runs commands in
  order; args forwarded as `$1`/`$@` to every command across segments;
  disabled segment skipped (no output, does not block others); first failure
  across all segments retained with its exit code; `fail_hint` of the failing
  segment printed; command-not-found maps to 127.
- `Load`: valid nested file decodes; missing file errors; malformed TOML errors;
  `run` of wrong type errors; `enabled = false` decodes.

**testscript — `cmd/liszt/testdata/script/hook.txtar` (replaces `run.txtar`):**

- `liszt hook pre-commit gleam -- arg` with segments echoing markers + `$1` →
  general output, then gleam output, then `arg`; exit 0.
- `liszt hook pre-commit` (general only) → general output, exit 0.
- `liszt hook pre-commit rust` (missing segment) → error, exit non-zero.
- `liszt hook ghost` (missing hook) → error, exit non-zero.
- zero args → usage error (MinimumNArgs(1)).

## Files

| File | Change |
|------|--------|
| `internal/runner/runner.go` | `Target`→`Segment`; add `Hook`, `Config` map types; `Resolve`; `RunHook`. Per-command exec logic preserved. |
| `internal/runner/runner_test.go` | Rewrite for `Resolve` + `RunHook` cases above. |
| `internal/cli/run_cmd.go` → `internal/cli/hook_cmd.go` | New `hook` command; `ArgsLenAtDash` split; `--config` default `.liszt/hooks.toml`. `run` removed. |
| `cmd/liszt/testdata/script/run.txtar` → `hook.txtar` | Rewrite for `hook` grammar. |
| `cmd/liszt/testdata/script/help.txtar`, `bare_help.txtar` | Update expected help text (`run`→`hook`). |
| `.liszt/liszt.toml` → `.liszt/hooks.toml` | Rename + restructure into `[<hook>.<segment>]` (repo's own config, if present). |
