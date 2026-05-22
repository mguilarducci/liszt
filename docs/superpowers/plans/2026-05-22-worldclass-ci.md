# World-Class CI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a security-hardened GitHub Actions CI (lint, 3-OS test matrix, 90% coverage gate, build) plus CodeQL scanning and Dependabot to the `liszt` Go CLI.

**Architecture:** Two workflows under `.github/workflows/` (`ci.yml` for validation, `codeql.yml` for security — separated by permission boundary), `.github/dependabot.yml` for gomod + github-actions updates, and two repo-root config files (`.golangci.yml`, `.testcoverage.yml`). Makefile gains `test`/`lint`/`cover` so local == CI. Every third-party action is pinned to a full commit SHA.

**Tech Stack:** GitHub Actions; Go 1.26.3 (from `go.mod`); golangci-lint v2.12.2; vladopajic/go-test-coverage v2.18.8; github/codeql-action v4; actionlint v1.7.12 (local YAML validation).

---

## Pinned action SHAs (verified 2026-05-22)

Use these EXACT `uses:` strings verbatim. The `# vX.Y.Z` comment is required (Dependabot reads it).

```
actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd          # v6.0.2
actions/setup-go@4a3601121dd01d1626a1e23e37211e3254c1c06c          # v6.4.0
actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a   # v7.0.1
actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c # v8.0.1
golangci/golangci-lint-action@82606bf257cbaff209d206a39f5134f0cfbfd2ee # v9.2.1
github/codeql-action/init@7211b7c8077ea37d8641b6271f6a365a22a5fbfa    # v4
github/codeql-action/analyze@7211b7c8077ea37d8641b6271f6a365a22a5fbfa # v4
vladopajic/go-test-coverage@a93b868a4cbcbf18dc3781650fad241f0020e609 # v2.18.8
```

## Local tooling (pinned)

```
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
go install github.com/vladopajic/go-test-coverage/v2@v2.18.8
go install github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
```

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

- [ ] **Step 1: Install golangci-lint locally**

Run: `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2`
Expected: binary installed to `$(go env GOPATH)/bin/golangci-lint`. Confirm with `golangci-lint version` → reports `2.12.2`.

- [ ] **Step 2: Create `.golangci.yml`**

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

## Task 3: Coverage threshold config + gate

**Files:**
- Create: `.testcoverage.yml`

- [ ] **Step 1: Install go-test-coverage locally**

Run: `go install github.com/vladopajic/go-test-coverage/v2@v2.18.8`
Expected: `go-test-coverage` on PATH.

- [ ] **Step 2: Create `.testcoverage.yml`**

```yaml
profile: cover.out

threshold:
  total: 90
```

- [ ] **Step 3: Generate a fresh coverage profile**

Run: `make test`
Expected: `cover.out` written, tests PASS.

- [ ] **Step 4: Run the coverage gate**

Run: `go-test-coverage --config=.testcoverage.yml`
Expected: PASS — prints the total coverage and exits 0 because the suite is well above 90%. (`make cover` runs steps 3–4 together.)

- [ ] **Step 5: Commit**

```
git add .testcoverage.yml
git commit -m "ci: add coverage threshold config (total 90%)"
```

---

## Task 4: CI workflow (lint, test matrix, coverage gate, build)

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Install actionlint locally**

Run: `go install github.com/rhysd/actionlint/cmd/actionlint@v1.7.12`
Expected: `actionlint` on PATH.

- [ ] **Step 2: Create `.github/workflows/ci.yml`**

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:

permissions:
  contents: read

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true

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
      - name: upload coverage
        if: matrix.os == 'ubuntu-latest'
        uses: actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a # v7.0.1
        with:
          name: coverage
          path: cover.out
          if-no-files-found: error

  coverage:
    name: coverage gate
    runs-on: ubuntu-latest
    needs: test
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2
      - uses: actions/setup-go@4a3601121dd01d1626a1e23e37211e3254c1c06c # v6.4.0
        with:
          go-version-file: go.mod
      - name: download coverage
        uses: actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c # v8.0.1
        with:
          name: coverage
      - name: check coverage
        uses: vladopajic/go-test-coverage@a93b868a4cbcbf18dc3781650fad241f0020e609 # v2.18.8
        with:
          config: .testcoverage.yml

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

- [ ] **Step 3: Validate the workflow YAML**

Run: `actionlint .github/workflows/ci.yml`
Expected: no output, exit 0. (actionlint also shellchecks the `run:` scripts.)

- [ ] **Step 4: Commit**

```
git add .github/workflows/ci.yml
git commit -m "ci: add CI workflow with lint, 3-OS test matrix, coverage gate, build"
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

- [ ] **Step 2: Validate the workflow YAML**

Run: `actionlint .github/workflows/codeql.yml`
Expected: no output, exit 0.

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

**Files:** none (verification only).

- [ ] **Step 1: Push the branch and open a PR**

Push the feature branch and open a PR against `main` so the workflows run. (Confirm the branch name and remote with the user before pushing.)

- [ ] **Step 2: Verify CI checks**

On the PR, confirm every check is green:
- `lint`
- `test (ubuntu-latest)`, `test (macos-latest)`, `test (windows-latest)`
- `coverage gate`
- `build (ubuntu-latest)`, `build (macos-latest)`, `build (windows-latest)`
- CodeQL `analyze (go)`

Expected: all PASS. If `coverage gate` fails, the suite dropped below 90% — investigate the uncovered lines, do not lower the threshold.

- [ ] **Step 3: Verify Dependabot registered**

In the repo: Insights → Dependency graph → Dependabot. Expected: both `gomod` and `github-actions` ecosystems listed with no config errors.

---

## Notes for the implementer

- **SHA pin trap:** copy the `uses:` lines verbatim from the pinned-SHA block at the top. For golangci-lint-action and codeql-action these are the *dereferenced commit* SHAs, not the annotated-tag object SHAs.
- **rtk:** run all commands through `rtk` (the shell hook rewrites transparently); do not invoke absolute binary paths.
- **No compound commands:** one command per shell invocation — `git add` and `git commit` are separate calls.
- **Coverage is a floor, not a target:** the gate is 90; the project culture is ~100%. Never lower the gate to make a red build green.
- **`-coverpkg=./...` is required:** the testscript harness in `cmd/liszt` exercises the `internal/*` packages in-process. Without `-coverpkg`, that coverage is not attributed (e.g. `internal/cli` reads as ~56%). With it, the merged profile reports ~96.6% (only `cmd/liszt/main.go`, the entry point, is uncovered). Keep `-coverpkg=./...` in both the Makefile `test` target and the CI `test` job.
