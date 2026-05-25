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
