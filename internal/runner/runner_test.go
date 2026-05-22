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
