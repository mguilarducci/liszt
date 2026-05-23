# World-Class CI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a security-hardened GitHub Actions CI (lint, 3-OS test matrix, 90% coverage gate, build) plus CodeQL scanning and Dependabot to the `liszt` Go CLI.

**Architecture:** Two workflows under `.github/workflows/` (`ci.yml` for validation, `codeql.yml` for security — separated by permission boundary), `.github/dependabot.yml` for gomod + github-actions updates, and two repo-root config files (`.golangci.yml`, `codecov.yml`). Coverage is gated by Codecov (the de-facto community standard) — the `test` job uploads the profile and Codecov posts a `codecov/project` status check failing below 90%. Makefile gains `test`/`lint`/`cover` so local == CI. Every third-party action is pinned to a full commit SHA.

**Tech Stack:** GitHub Actions; Go 1.26.3 (from `go.mod`); golangci-lint v2.12.2; Codecov (codecov-action v6.0.1, tokenless via OIDC — repo is public); github/codeql-action v4.

---

## Pinned action SHAs (verified 2026-05-22)

Use these EXACT `uses:` strings verbatim. The `# vX.Y.Z` comment is required (Dependabot reads it).

```
actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd          # v6.0.2
actions/setup-go@4a3601121dd01d1626a1e23e37211e3254c1c06c          # v6.4.0
golangci/golangci-lint-action@82606bf257cbaff209d206a39f5134f0cfbfd2ee # v9.2.1
codecov/codecov-action@e79a6962e0d4c0c17b229090214935d2e33f8354     # v6.0.1
github/codeql-action/init@7211b7c8077ea37d8641b6271f6a365a22a5fbfa    # v4
github/codeql-action/analyze@7211b7c8077ea37d8641b6271f6a365a22a5fbfa # v4
```

## Local tooling

CI tools (golangci-lint, Codecov, CodeQL) run **in GitHub Actions** — do NOT
`go install` them on the dev machine. Local coverage uses the Go toolchain only:
`make cover` runs `go test ... -coverpkg=./...` then `go tool cover -func`.
Workflow YAML is validated by pushing to CI (no local actionlint install).

---

## Task 1: Makefile test/lint/cover targets

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Rewrite Makefile with new targets**

Replace the entire file contents with:

```makefile
BIN := bin/liszt
PKG := ./cmd/liszt
COVER := cover.out

.PHONY: build vet test lint cover clean

build:
	go build -o $(BIN) $(PKG)

vet:
	go vet ./cmd/liszt ./internal/...

test:
	go test ./... -race -covermode=atomic -coverpkg=./... -coverprofile=$(COVER)

lint:
	golangci-lint run

cover: test
	go-test-coverage --config=.testcoverage.yml

clean:
	rm -rf bin $(COVER)
```

- [ ] **Step 2: Verify the test target runs**

Run: `make test`
Expected: all packages build and tests PASS; `cover.out` is written. (Coverage gate not enforced yet — that is Task 3.)

- [ ] **Step 3: Commit**

```
git add Makefile
git commit -m "build: add test, lint, and cover make targets"
```

---

## Task 2: golangci-lint v2 config

**Files:**
- Create: `.golangci.yml`

> Note: golangci-lint is NOT installed locally. The `lint` CI job runs it. Steps 3–4 below assume it is already present in the dev environment; if it is not, skip them and let CI run the linter on push.

- [ ] **Step 1: Create `.golangci.yml`**

```yaml
version: "2"

linters:
  default: standard
  enable:
    - bodyclose
    - errorlint
    - misspell
    - unconvert

formatters:
  enable:
    - gofumpt
    - gci
```

- [ ] **Step 3: Auto-format the codebase**

Run: `golangci-lint fmt`
Expected: gofumpt + gci rewrite any unformatted files in place. Review the diff with `git diff` — changes should be formatting-only.

- [ ] **Step 4: Run the linter**

Run: `golangci-lint run`
Expected: PASS (exit 0, no issues). If a linter reports issues, fix the flagged code (the four added linters — bodyclose, errorlint, misspell, unconvert — surface real defects, not style preferences) and re-run until clean.

- [ ] **Step 5: Verify tests still pass after formatting**

Run: `make test`
Expected: PASS.

- [ ] **Step 6: Commit**

```
git add .golangci.yml
git add -u
git commit -m "ci: add golangci-lint v2 config with gofumpt/gci formatters"
```

---

## Task 3: Codecov coverage gate config

**Files:**
- Create: `codecov.yml`

Coverage is gated by Codecov (community standard), not a third-party action.
The `test` CI job uploads `cover.out`; Codecov posts a `codecov/project` status
check that fails when total coverage falls below 90%.

- [ ] **Step 1: Create `codecov.yml`** at the repo root

```yaml
coverage:
  status:
    project:
      default:
        target: 90%
        threshold: 0%
    patch:
      default:
        target: 90%
```

`patch` is enabled: new/changed lines in a PR must be >=90% covered, so untested
new code is blocked even when the project total stays high. `project` gates the
overall total at 90%.

- [ ] **Step 2: Sanity-check coverage locally (Go toolchain only — no installs)**

Run: `make cover`
Expected: prints per-function coverage ending in `total: (statements) 96.6%`,
comfortably above the 90 floor. (`-coverpkg=./...` is what makes this number
correct — see notes.)

- [ ] **Step 3: Commit**

```
git add codecov.yml
git commit -m "ci: add Codecov project coverage gate (90%)"
```

---

## Task 4: CI workflow (lint, test matrix + Codecov upload, build)

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Create `.github/workflows/ci.yml`**

The `test` job uploads coverage to Codecov from the ubuntu runner using OIDC
(`use_oidc: true`), which needs `id-token: write` on that job. No artifact
hand-off and no separate gate job — Codecov enforces the 90% threshold via the
`codecov/project` status check (configured in `codecov.yml`).

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

permissions:
  contents: read

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  # only cancel superseded PR runs; let every main commit finish (CodeQL/coverage baseline)
  cancel-in-progress: ${{ github.event_name == 'pull_request' }}

jobs:
  lint:
    name: lint
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2
      - uses: actions/setup-go@4a3601121dd01d1626a1e23e37211e3254c1c06c # v6.4.0
        with:
          go-version-file: go.mod
      - uses: golangci/golangci-lint-action@82606bf257cbaff209d206a39f5134f0cfbfd2ee # v9.2.1
        with:
          version: v2.12.2

  test:
    name: test (${{ matrix.os }})
    runs-on: ${{ matrix.os }}
    permissions:
      contents: read
      id-token: write
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2
      - uses: actions/setup-go@4a3601121dd01d1626a1e23e37211e3254c1c06c # v6.4.0
        with:
          go-version-file: go.mod
      - name: test
        run: go test ./... -race -covermode=atomic -coverpkg=./... -coverprofile=cover.out
      - name: upload coverage to codecov
        if: matrix.os == 'ubuntu-latest'
        uses: codecov/codecov-action@e79a6962e0d4c0c17b229090214935d2e33f8354 # v6.0.1
        with:
          use_oidc: true
          files: cover.out
          # gate is the codecov/project status check, not this upload step
          fail_ci_if_error: false

  build:
    name: build (${{ matrix.os }})
    runs-on: ${{ matrix.os }}
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2
      - uses: actions/setup-go@4a3601121dd01d1626a1e23e37211e3254c1c06c # v6.4.0
        with:
          go-version-file: go.mod
      - name: build
        run: go build ./cmd/liszt
```

- [ ] **Step 2: Verify the file structure**

Use the Read tool on `.github/workflows/ci.yml`. Confirm: three jobs (`lint`,
`test`, `build`); the `test` job has `permissions: id-token: write`; the codecov
step is gated by `if: matrix.os == 'ubuntu-latest'`. YAML is validated by CI on
push (Task 7) — no local actionlint install.

- [ ] **Step 3: Commit**

```
git add .github/workflows/ci.yml
git commit -m "ci: add CI workflow with lint, 3-OS test matrix, Codecov upload, build"
```

---

## Task 5: CodeQL workflow

**Files:**
- Create: `.github/workflows/codeql.yml`

- [ ] **Step 1: Create `.github/workflows/codeql.yml`**

```yaml
name: CodeQL

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
  schedule:
    - cron: "27 3 * * 1"

permissions:
  contents: read

jobs:
  analyze:
    name: analyze (go)
    runs-on: ubuntu-latest
    permissions:
      contents: read
      actions: read
      security-events: write
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2
      - name: init
        uses: github/codeql-action/init@7211b7c8077ea37d8641b6271f6a365a22a5fbfa # v4
        with:
          languages: go
          build-mode: autobuild
          dependency-caching: true
      - name: analyze
        uses: github/codeql-action/analyze@7211b7c8077ea37d8641b6271f6a365a22a5fbfa # v4
```

- [ ] **Step 2: Verify the file structure**

Use the Read tool on `.github/workflows/codeql.yml`. Confirm the `analyze` job
has `security-events: write` and both `init` and `analyze` use the codeql-action
v4 SHA. YAML is validated by CI on push (Task 7) — no local actionlint install.

- [ ] **Step 3: Commit**

```
git add .github/workflows/codeql.yml
git commit -m "ci: add CodeQL security scanning workflow for Go"
```

---

## Task 6: Dependabot config

**Files:**
- Create: `.github/dependabot.yml`

- [ ] **Step 1: Create `.github/dependabot.yml`**

```yaml
version: 2
updates:
  - package-ecosystem: gomod
    directory: "/"
    schedule:
      interval: weekly
    groups:
      go-deps:
        update-types:
          - minor
          - patch

  - package-ecosystem: github-actions
    directory: "/"
    schedule:
      interval: weekly
    groups:
      actions:
        update-types:
          - minor
          - patch
```

- [ ] **Step 2: Validate YAML syntax**

Read the file back and confirm two `package-ecosystem` entries (`gomod`, `github-actions`), both `directory: "/"`, both weekly with a `groups` block. (Dependabot has no offline validator; GitHub validates it server-side after push — surfaced under Insights → Dependency graph → Dependabot.)

- [ ] **Step 3: Commit**

```
git add .github/dependabot.yml
git commit -m "ci: add Dependabot for gomod and github-actions"
```

---

## Task 7: Trigger and verify on a branch

**Files:** none (verification + external setup).

- [ ] **Step 1: Enable Codecov for the repo (one-time, user action)**

The repo (`mguilarducci/liszt`) is public, so coverage upload is tokenless via
OIDC — but for the `codecov/project` status check to post on PRs, the Codecov
GitHub App must be installed and the repo activated at https://codecov.io.
Confirm this is done before relying on the gate.

- [ ] **Step 2: Push the branch and open a PR**

Push the feature branch and open a PR against `main` so the workflows run.
(Confirm the branch name and remote with the user before pushing.)

- [ ] **Step 3: Verify CI checks**

On the PR, confirm every check is green:
- `lint`
- `test (ubuntu-latest)`, `test (macos-latest)`, `test (windows-latest)`
- `build (ubuntu-latest)`, `build (macos-latest)`, `build (windows-latest)`
- CodeQL `analyze (go)`
- `codecov/project` (posted by Codecov; fails if total < 90%)

Expected: all PASS. If `codecov/project` is red, the suite dropped below 90% —
investigate the uncovered lines, do not lower the threshold.

- [ ] **Step 4: Verify Dependabot registered**

In the repo: Insights → Dependency graph → Dependabot. Expected: both `gomod`
and `github-actions` ecosystems listed with no config errors.

---

## Notes for the implementer

- **SHA pin trap:** copy the `uses:` lines verbatim from the pinned-SHA block at the top. For golangci-lint-action and codeql-action these are the *dereferenced commit* SHAs, not the annotated-tag object SHAs.
- **rtk:** run all commands through `rtk` (the shell hook rewrites transparently); do not invoke absolute binary paths.
- **No compound commands:** one command per shell invocation — `git add` and `git commit` are separate calls.
- **Coverage is a floor, not a target:** the gate is 90; the project culture is ~100%. Never lower the gate to make a red build green.
- **`-coverpkg=./...` is required:** the testscript harness in `cmd/liszt` exercises the `internal/*` packages in-process. Without `-coverpkg`, that coverage is not attributed (e.g. `internal/cli` reads as ~56%). With it, the merged profile reports ~96.6% (only `cmd/liszt/main.go`, the entry point, is uncovered). Keep `-coverpkg=./...` in both the Makefile `test` target and the CI `test` job.
