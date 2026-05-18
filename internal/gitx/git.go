package gitx

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// EnsureClone clones url into dest if .git is absent. No-op if already cloned.
func EnsureClone(url, dest string) error {
	if _, err := os.Stat(filepath.Join(dest, ".git")); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	cmd := exec.Command("git", "clone", "--depth=1", url, dest)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}
	return nil
}

// HeadSHA returns the local repo's HEAD commit SHA.
func HeadSHA(dir string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// CloneAtSHA shallow-clones url into dest and checks out sha.
// Idempotent: if dest already has HEAD == sha, returns nil.
// On failure, leaves no partial directory behind.
func CloneAtSHA(url, sha, dest string) error {
	if head, err := HeadSHA(dest); err == nil && head == sha {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(filepath.Dir(dest), ".liszt-clone-*")
	if err != nil {
		return err
	}
	if err := cloneInto(url, sha, tmp); err != nil {
		os.RemoveAll(tmp)
		return err
	}
	if err := os.RemoveAll(dest); err != nil {
		os.RemoveAll(tmp)
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		os.RemoveAll(tmp)
		return err
	}
	return nil
}

func cloneInto(url, sha, dir string) error {
	run := func(args ...string) error {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	if err := run("init", "-q"); err != nil {
		return fmt.Errorf("git init: %w", err)
	}
	if err := run("remote", "add", "origin", url); err != nil {
		return fmt.Errorf("git remote add: %w", err)
	}
	if err := run("fetch", "--depth=1", "origin", sha); err != nil {
		if err2 := run("fetch", "origin"); err2 != nil {
			return fmt.Errorf("git fetch: %w (after shallow: %v)", err2, err)
		}
	}
	if err := run("checkout", "-q", sha); err != nil {
		return fmt.Errorf("git checkout %s: %w", sha, err)
	}
	return nil
}

// LsRemoteHead returns the remote's HEAD SHA without cloning.
func LsRemoteHead(url string) (string, error) {
	out, err := exec.Command("git", "ls-remote", url, "HEAD").Output()
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(string(out))
	if i := strings.IndexAny(line, " \t"); i > 0 {
		return line[:i], nil
	}
	return "", fmt.Errorf("unexpected ls-remote output: %q", line)
}
