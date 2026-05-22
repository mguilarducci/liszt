# World-Class CI for liszt — Design

**Date:** 2026-05-22
**Status:** Approved (design)
**Scope:** Continuous Integration validation + Dependabot + CodeQL. **No** release/CD.

## Goal

Add a community-aligned, security-hardened GitHub Actions CI to the `liszt`
Go CLI. The pipeline validates every push to `main` and every pull request:
lint, cross-platform tests, coverage gate, and build. Supply-chain security via
SHA-pinned actions, CodeQL scanning, and Dependabot.

## Constraints & Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Scope | CI validation + Dependabot + CodeQL (no release) | Per request |
| Test matrix | 3 OS (ubuntu, macos, windows) × Go `stable` | CLI depends on terminal/TUI libs (charm, x/termios, x/windows) — platform bugs are real |
| Coverage gate | Fail if **total < 90%** | Project targets ~100%; 90 is the hard floor |
| Coverage tool | **Codecov** (`codecov-action` v6.0.1, OIDC tokenless) | De-facto community standard for Go OSS; repo is public so no token. Gate enforced via `codecov/project` status check |
| Action pinning | Full commit SHA + `# vX.Y.Z` comment | OpenSSF Scorecard / StepSecurity consensus; matches repo "pin exact versions" rule |
| Pin freshness | Dependabot (`github-actions` ecosystem) | Auto-bumps SHAs |
| Go version source | `go-version-file: go.mod` | Single source of truth (currently 1.26.3) |
| Lint tool | golangci-lint v2 via official action | Includes gofumpt formatter (already a dep) |

## File Layout

```
.github/
  workflows/
    ci.yml          # lint, test (matrix) + Codecov upload, build
    codeql.yml      # CodeQL security scan (separate: needs elevated perms)
  dependabot.yml    # gomod + github-actions ecosystems
.golangci.yml       # golangci-lint v2 config (linters + formatters)
codecov.yml         # Codecov project gate (total 90%)
Makefile            # + test / lint / cover targets (CI mirrors local)
```

## Pinned Versions (verified 2026-05-22 via GitHub API)

| Action | Tag | Commit SHA |
|---|---|---|
| actions/checkout | v6.0.2 | `de0fac2e4500dabe0009e67214ff5f5447ce83dd` |
| actions/setup-go | v6.4.0 | `4a3601121dd01d1626a1e23e37211e3254c1c06c` |
| golangci/golangci-lint-action | v9.2.1 | `82606bf257cbaff209d206a39f5134f0cfbfd2ee` |
| codecov/codecov-action | v6.0.1 | `e79a6962e0d4c0c17b229090214935d2e33f8354` |
| github/codeql-action (init + analyze) | v4 | `7211b7c8077ea37d8641b6271f6a365a22a5fbfa` |

Tool versions: golangci-lint `v2.12.2` (passed to action `version:`), Go from
`go.mod` (1.26.3), CodeQL bundle managed by action v4 (currently v2.25.5).

> Annotated tags (golangci-lint-action, codeql-action) were dereferenced to the
> underlying **commit** SHA (`tag^{}`); the annotated-tag object SHA must NOT be
> used in `uses:` — it does not resolve.

## ci.yml

**Triggers:** `push` to `main`, `pull_request`.
**Top-level permissions:** `contents: read` (least privilege).
**Concurrency:** group by `${{ github.workflow }}-${{ github.ref }}`,
`cancel-in-progress: true` — stale PR runs are cancelled.

### Job: `lint` (ubuntu-latest)
Separate job per golangci-lint-action's official recommendation (parallelism).
1. checkout (SHA-pinned)
2. setup-go — `go-version-file: go.mod`; setup-go v4+ caches GOCACHE/GOMODCACHE keyed on `go.sum`
3. golangci/golangci-lint-action — `version: v2.12.2`

golangci-lint also runs the **formatters** (gofumpt, gci) in check mode, so a
separate `gofmt`/`gofumpt` step is unnecessary. `go vet` is covered by the
`govet` linter.

### Job: `test` (matrix)
```yaml
strategy:
  fail-fast: false
  matrix:
    os: [ubuntu-latest, macos-latest, windows-latest]
```
The `test` job declares `permissions: { contents: read, id-token: write }` for OIDC.

1. checkout
2. setup-go (`go-version-file: go.mod`, cache on)
3. `go test ./... -race -covermode=atomic -coverpkg=./... -coverprofile=cover.out`
4. **On ubuntu only:** `codecov/codecov-action` with `use_oidc: true`,
   `files: cover.out`, `fail_ci_if_error: false`

Race detector on across all platforms. `fail-fast: false` so one OS failing
still reports the others.

`-coverpkg=./...` is mandatory: the `cmd/liszt` testscript harness drives the
`internal/*` packages in-process, so without it their coverage is unattributed
(`internal/cli` reads ~56%). With it the merged profile is ~96.6% — only
`cmd/liszt/main.go` (entry point, justified inline) is uncovered — clearing the
90 floor.

### Coverage gate (Codecov, not a CI job)
No separate gate job and no artifact hand-off. Codecov ingests the uploaded
profile and posts a `codecov/project` status check that fails when total
coverage drops below 90% (configured in `codecov.yml`). To block merges, add
`codecov/project` as a required status check in branch protection. The upload
step uses `fail_ci_if_error: false` so a Codecov outage (or a not-yet-activated
repo) does not redden the `test` job — coverage enforcement lives entirely in
the `codecov/project` check, keeping concerns separate.

### Job: `build` (matrix, 3 OS)
1. checkout
2. setup-go
3. `go build ./cmd/liszt`

Confirms the binary compiles on every target OS.

## codeql.yml

Separate file: CodeQL needs `security-events: write`, which must NOT leak into
the rest of CI.

**Triggers:** `push` to `main`, `pull_request` to `main`, `schedule` (weekly cron).
**Permissions:** `security-events: write`, `contents: read`, `actions: read`.

Steps:
1. checkout (SHA-pinned)
2. `codeql-action/init@<sha>` — `languages: go`, `build-mode: autobuild`,
   `dependency-caching: true`
3. `codeql-action/analyze@<sha>`

(v3 is deprecated December 2026; v4 is current.)

## dependabot.yml

```yaml
version: 2
updates:
  - package-ecosystem: gomod
    directory: "/"
    schedule: { interval: weekly }
    groups:
      go-deps: { update-types: [minor, patch] }
  - package-ecosystem: github-actions
    directory: "/"
    schedule: { interval: weekly }
    groups:
      actions: { update-types: [minor, patch] }
```
Grouped minor/patch PRs reduce noise. The `github-actions` ecosystem keeps the
SHA pins current.

## .golangci.yml (v2 schema)

`version: "2"`. Curated linter set on top of defaults (govet, staticcheck,
errcheck, revive, ineffassign, unconvert, misspell …). `formatters:` enables
`gofumpt` and `gci`. Format issues fail lint in CI.

## codecov.yml

```yaml
coverage:
  status:
    project:
      default:
        target: 90%
        threshold: 0%
    patch: false
```
`patch: false` disables per-PR patch gating — only the project total is gated,
per spec. File/package thresholds omitted.

## Makefile additions

Add targets so local == CI:
- `test`: `go test ./... -race -covermode=atomic -coverpkg=./... -coverprofile=cover.out`
- `lint`: `golangci-lint run`
- `cover`: `make test` then `go tool cover -func=cover.out` (Go toolchain only — no extra tool)

## Error Handling / Edge Cases

- **Windows line endings / paths:** `go test` matrix surfaces these; no extra config.
- **Annotated-tag SHA trap:** pin commit SHA (`tag^{}`), not tag-object SHA. Documented above.
- **Coverage upload failure:** `fail_ci_if_error: false` — a Codecov outage or not-yet-activated repo must not break the `test` job; the `codecov/project` check is the gate.
- **Test failure skips upload:** the codecov step runs after `go test`; if tests fail the job stops before upload (no false green).
- **Cron noise:** CodeQL weekly schedule only; PR/push cover the hot path.

## Verification Plan

1. Read each workflow/config file to confirm structure; YAML is validated by CI on push (no local actionlint install).
2. One-time: activate the repo on codecov.io + install the Codecov GitHub App so the status check can post.
3. Open a PR; confirm `lint`, `test` (×3 OS), `build`, CodeQL, and the `codecov/project` check all pass against the existing ~100%-coverage suite.
4. Confirm Dependabot config validates (Insights → Dependency graph → Dependabot).

## Out of Scope (YAGNI)

- Release / goreleaser / binary publishing
- Coverage badge in README (Codecov can provide one later if wanted)
- Multi-Go-version matrix (oldstable)
- Container images, Docker, deployment
- Local installs of CI tools (golangci-lint / actionlint / coverage) — they run in Actions
