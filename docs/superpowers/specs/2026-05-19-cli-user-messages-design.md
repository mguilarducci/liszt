# CLI User-Facing Messages — Design

**Date:** 2026-05-19
**Status:** Approved
**Scope:** `internal/render`, `internal/repos`, `internal/cli`

## Problem

Current CLI output reads like developer logs. Every stage emits `info`
lines with technical key/value payloads (paths, SHAs, marketplace
counts):

```
❯ ./bin/liszt repo add https://github.com/obra/superpowers
▌ info   resolving repo  url=https://github.com/obra/superpowers
▌ info   cloning  name=obra/superpowers dest=/Users/.../repos/obra/superpowers
▌ info   cloned  sha=f2cbfbefebbf
▌ info   reading marketplace.json
▌ info   marketplace  name=superpowers-dev flavor=claude plugins=1
▌ info   saving repos.toml  path=/Users/.../repos.toml
▌ done   repo added  name=obra/superpowers
```

Two failures:

1. **Tone.** `info` is the default verb for every step. The user gets
   a debug feed, not a narrative.
2. **Behaviour.** `repo add` clones before checking the registry, and
   `repos.Config.Upsert` silently overwrites existing entries. There
   is no signal that the repo is already known.

## Goals

- Default CLI output is short, narrative, user-facing.
- Technical payload (paths, SHAs, URLs) hidden behind `-v` / `--verbose`.
- Progress shown as a single determinate bar across explicit steps,
  reusing the existing styled `render.Bar` (Gleam palette, ♬♪♩♫
  notes).
- `repo add` on an already-registered repo fails with a clear hint,
  before any clone happens.
- A reusable message vocabulary so every subcommand renders the same
  way without ad-hoc formatting.

## Non-Goals

- `repo update` subcommand — hint references it, implementation
  deferred.
- Per-command copy rewrites (the human text in each `Step`/`Warn`/
  `Hint`). Structural migration only; copy is a separate session.
- Changes to `Bar` visuals (notes set, palette, animation cadence).
- Verbose levels beyond bool (no `--quiet`, no `--trace`).
- Structured logging to file.
- i18n.

## Approach

**Approach A — Semantic verbs.** Replace the single `Info` verb with
a small vocabulary that carries semantics (step, success, failure,
hint, detail). `-v` toggles detail output. `Progress` wraps the
existing `Bar` to coordinate step-by-step advancement.

Rejected: stateful `Render.Run` runner (too rigid for branching
flows like the conflict-detect path), and verbosity-flag-only (does
not fix the tone problem — `Info` remains a catch-all).

## Vocabulary

New verbs in `internal/render`:

| Verb | Use | Visual | Verbosity |
|---|---|---|---|
| `Step(label)` | Start a progress phase | `⠇ <label>` with spinner active | always |
| `StepDone(label)` | Successful step end | `✓ <label>` persistent dim line | always |
| `StepFail(label, err)` | Failed step end | `✗ <label>: <err>` | always |
| `Done(msg, kv...)` | Command-level success | `✔ <msg>` + summary block | always |
| `Fail(msg, kv...)` | Command-level failure | `✖ <msg>` (fang renders the cobra error separately) | always |
| `Hint(msg)` | Suggested next step | `→ <msg>` dim | always |
| `Warn(msg, kv...)` | Non-fatal anomaly | `! <msg>` | always |
| `Detail(msg, kv...)` | Technical diagnostic | `· <msg> k=v` dim | only when `-v` |

`Info` is removed. Existing call sites migrate to `Detail` (paths,
SHAs, payload) or `Step`/`StepDone` (phases).

## Progress

`Progress` is a thin wrapper over `render.Bar`. The underlying
`Bar` is unchanged: it keeps the Gleam pink ♬♪♩♫ filled cells, the
`▌` prefix, the percentage column, and the existing erase/repaint
machinery in `internal/render/bar.go`.

```go
type Progress struct {
    bar     *Bar
    total   int
    current int
    label   string // current step label, set by Step
}

func NewProgress(total int) *Progress
func (p *Progress) Step(label string)
func (p *Progress) StepFail(err error)
func (p *Progress) Done(msg string, kv ...any)
```

Semantics:

- `NewProgress(total)` constructs the bar in determinate mode at 0%.
- `Step(label)`:
  - If a previous step was active, emit its `✓ <previous label>`
    persistent line.
  - Increment `current`, call `bar.Set(current/total)`,
    `bar.Update(label)`.
- `StepFail(err)`: emit `✗ <current label>: <err>`, call `bar.Fail`.
- `Done(msg, kv...)`: emit `✓ <current label>` for the last step,
  call `bar.Done(msg, kv...)`, then render the summary block from
  `kv`.

Determinate bar throughout. No indeterminate mode within a step. A
slow opaque operation (e.g. `git clone`) leaves the bar parked at
its current percentage; the note flicker keeps the bar visibly
alive without claiming false progress.

Non-TTY behaviour is inherited from `Bar` (one-shot info lines per
step, no animation).

## Verbosity

`internal/cli/root.go` adds a persistent flag:

```go
rootCmd.PersistentFlags().BoolP("verbose", "v", false, "show technical detail")
```

`PersistentPreRun` (or `PersistentPreRunE`) on `rootCmd` reads the
flag and calls `render.SetVerbose(true)`. `Detail` is a no-op when
verbose is false. All other verbs ignore verbosity.

`render.SetVerbose` is package-level state. Acceptable because a
single CLI invocation runs one command end-to-end.

## `repo add` Flow

```
Step 1: Resolving URL
  ParseGitHubURL(url)
  ✓ Resolved <owner>/<repo>

Step 2: Checking registry           // preflight, before clone
  repos.Load + Find(name)
  if found:
    Fail("<name> already added")
    Hint("Run `liszt repo update <name>` to refresh")
    return ErrAlreadyAdded
  ✓ Not yet registered

Step 3: Cloning
  gitx.EnsureClone(url, dest)       // slow; bar holds at 3/5
  gitx.HeadSHA(dest)
  ✓ Cloned <owner>/<repo>

Step 4: Reading manifest
  marketplace.Read(dest)
  if err:
    Warn("marketplace.json missing or invalid")  // non-fatal
  else:
    ✓ Read marketplace.json

Step 5: Saving registry
  cfg.Repos = append(cfg.Repos, Entry{...})       // pure append; no upsert
  repos.Save(p.Repos, cfg)
  ✓ Saved to repos.toml

Done("Added <name>",
  "marketplace", mp.Name,
  "plugins",     len(mp.Plugins))
Hint("Run `liszt plugin list` to see available plugins")
Hint("Run `liszt plugin install <name>` to install")
```

Detail block (only with `-v`):

```
· url=https://github.com/obra/superpowers
· dest=/Users/.../.cache/liszt/repos/obra/superpowers
· sha=f2cbfbefebbf
· path=/Users/.../.local/share/liszt/repos.toml
```

## API Changes

### `internal/render`

- Add: `Step`, `StepDone`, `StepFail`, `Detail`, `Progress`,
  `NewProgress`, `SetVerbose`.
- Keep: `Done`, `Hint`, `Warn`.
- Rename: `Error` → `Fail`. `Error` is removed; no alias (internal
  package, no external consumers).
- Remove: `Info`. Call sites migrate.
- Internal: `Renderer.verbose bool`. `Detail` checks it.
- Bar prefix label: `internal/render/bar.go:85` currently hardcodes
  `lblInfo` ("info"). Replace with a new `lblStep` ("step") so the
  bar reads `▌ step  ♬♪♩♫…  40%  <label>` during `Progress` runs.
  The underlying bar code and styles (`styInfoBar`, `styInfoLbl`,
  pink notes) stay unchanged; only the label string and the
  reference in `repaint` change.

### `internal/repos`

- Add: `func (c *Config) Find(name string) (*Entry, bool)`.
- `Upsert` remains (unused by `RepoAdd` after refactor; preserved
  for the future `repo update` command).

### `internal/cli`

- Add: `var ErrAlreadyAdded = errors.New("repo already added")` in
  `internal/cli/repo.go`. Tests assert error identity, not string.
- `RepoAdd` refactored per the flow above.
- `root.go`: `-v` persistent flag + `PersistentPreRun` wiring.
- Other commands: mechanical migration of `Info`→`Detail`,
  `Warn`/`Error` kv payload moves to `Detail`. Human-facing copy
  unchanged in this spec — left for a follow-up session.

## Visual Reference

After `repo add` succeeds (default, no `-v`):

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

During step 3 (clone running):

```
✓ Resolved obra/superpowers
✓ Not yet registered
▌ step  ♬♪♩♫♫♪♬♩♪♫·············  40%  Cloning obra/superpowers
```

After `repo add` of an already-registered repo:

```
✓ Resolved obra/superpowers
✖ obra/superpowers already added
→ Run `liszt repo update obra/superpowers` to refresh
```

## Testing

- `internal/render/progress_test.go` (new): cover `NewProgress`,
  `Step` advancement, `StepFail`, `Done`, verbose toggle behaviour
  of `Detail`. 100% of new code.
- `internal/render/message_test.go`: drop `Info` tests, add
  `Step`/`StepDone`/`StepFail`/`Detail`. Keep `Warn`/`Done`/`Hint`
  coverage.
- `internal/repos/repos_test.go`: add `Find` cases (hit, miss,
  empty config).
- `internal/cli/repo_test.go`: add already-added case asserting
  `errors.Is(err, ErrAlreadyAdded)` and that `gitx.EnsureClone`
  was not invoked. Existing happy-path test updated for new step
  output.
- `internal/cli/script_test.go` (testscript): add scenario
  exercising `-v` (detail lines visible) and absence of `-v`
  (detail lines hidden).

Coverage target: 100% on new code and new branches; existing
target on touched files preserved.

## Migration of Other Commands

Structural mapping for the mechanical migration. Final human copy
deferred.

| File | Current | Becomes |
|---|---|---|
| `cli/install.go:25` | `render.NewBar("installing " + label)` | `render.NewProgress(N).Step("Installing " + label)` |
| `cli/outdated.go:34` | `render.NewBar("checking remotes")` | `render.NewProgress(len(repos)).Step("Checking " + repo)` per repo |
| `cli/outdated.go:58` | `render.Warn("ls-remote failed", "repo", r, "err", err)` | `render.Warn("Could not reach " + r)` + `render.Detail("err", err)` |
| `cli/outdated.go:84` | `render.Info(...)` summary | `render.Done(...)` + `render.Detail(...)` |
| `cli/plugin.go:25` | `render.Hint(...)` | unchanged |
| `cli/plugin.go:31,36` | `render.Warn("skip", "name", n, "err", err)` | `render.Warn("Skipped " + n)` + `render.Detail("err", err)` |
| `cli/plugin.go:42` | `render.Header(...)` | unchanged |
| `cli/plugin.go:50` | `render.Hint(...)` | unchanged |
| `cli/resource.go:91` | `render.Warn(...)` | `render.Warn(<human msg>)` + `render.Detail(<payload>)` |
| `cli/repo.go:20-65` | `Info` everywhere | per the new flow above |

## Risks

- **Mechanical migration churn.** 11+ call sites plus their tests.
  Compile failures from removing `Info` drive the work; grep
  catches the rest.
- **Global verbose state.** `render.SetVerbose` is package-level.
  Safe for current single-cmd CLI invocations; revisit if
  parallel cmds appear.
- **Breaking `render.Info` API.** Package is internal — no external
  consumers. No deprecation period needed.

## Open Questions

None. All clarifications resolved during brainstorming.
