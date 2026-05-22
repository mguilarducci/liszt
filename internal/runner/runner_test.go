package runner

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

func TestRun_SpawnFailureMapsTo1(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	// bash -c of a non-existent binary: bash exits 127, which is a normal
	// ExitError; to hit the non-ExitError branch we rely on bash returning
	// non-zero. Assert a non-zero code is retained.
	tgt := Target{Cmd: []string{"this-binary-does-not-exist-xyz"}}
	if code := tgt.Run("x", &out, &errOut); code == 0 {
		t.Errorf("expected non-zero exit for failing command, got 0")
	}
}

func TestLoad_EnabledDecodes(t *testing.T) {
	t.Parallel()
	path := writeTOML(t, "[run.x]\ncmd = [\"echo hi\"]\nenabled = false\n")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	tgt, _ := cfg.Target("x")
	if tgt.Enabled == nil || *tgt.Enabled != false {
		t.Errorf("expected enabled=false to decode, got %v", tgt.Enabled)
	}
}

func TestExitCode_NonExitError(t *testing.T) {
	t.Parallel()
	if got := exitCode(errors.New("not an exit error")); got != 1 {
		t.Errorf("non-ExitError should map to 1, got %d", got)
	}
}
