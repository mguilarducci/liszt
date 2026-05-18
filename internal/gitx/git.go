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
