// Package runner executes named command groups declared in .liszt/liszt.toml.
package runner

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/pelletier/go-toml/v2"
)

// Target is one [run.<name>] table: a group of shell commands.
type Target struct {
	Cmd      []string `toml:"cmd"`
	FailHint string   `toml:"fail_hint"`
	Enabled  *bool    `toml:"enabled"` // nil => enabled
}

// Config models a .liszt/liszt.toml run section.
type Config struct {
	Run map[string]Target `toml:"run"`
}

// Load reads and decodes path. A missing or unreadable file, or malformed
// TOML (including a non-array cmd), is an error.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	cfg := &Config{}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
}

// Target returns the named target and whether it exists.
func (c *Config) Target(name string) (Target, bool) {
	t, ok := c.Run[name]
	return t, ok
}

func (t Target) isEnabled() bool {
	return t.Enabled == nil || *t.Enabled
}

// Run executes the target's commands via `bash -c`, streaming each command's
// stdout/stderr to the provided writers. All commands run even if an earlier
// one fails; the first failing command is retained and its exit code returned
// (0 = all passed). A command that never started (e.g. bash missing) is
// reported distinctly so its failure is not misattributed to the command's own
// exit status. A disabled target returns 0 without output; a target with no
// commands returns 1.
func (t Target) Run(name string, stdout, stderr io.Writer) int {
	if !t.isEnabled() {
		return 0
	}
	if len(t.Cmd) == 0 {
		fmt.Fprintf(stderr, "error: [run.%s] has empty cmd\n", name)
		return 1
	}

	fmt.Fprintf(stdout, "== run %s ==\n", name)

	failCode := 0
	failCmd := ""
	var failErr error
	for _, c := range t.Cmd {
		cmd := exec.Command("bash", "-c", c)
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil && failCode == 0 {
			failCode = exitCode(err)
			failCmd = c
			failErr = err
		}
	}

	if failCode != 0 {
		fmt.Fprint(stderr, failureLine(failCmd, failCode, failErr))
		if t.FailHint != "" {
			fmt.Fprintf(stderr, "hint: %s\n", t.FailHint)
		}
	}
	return failCode
}

// failureLine formats the FAILED line. A command that ran and exited non-zero
// reports its exit code; a command that never started (non-ExitError, e.g. bash
// not found) reports the underlying error so the cause is not misattributed to
// the command's own exit status.
func failureLine(cmd string, code int, err error) string {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return fmt.Sprintf("FAILED: %s (exit %d)\n", cmd, code)
	}
	return fmt.Sprintf("FAILED: %s (could not start: %v)\n", cmd, err)
}

// exitCode extracts the process exit code from a command error. A non-exit
// error (e.g. bash not found) or a signal kill maps to 1.
func exitCode(err error) int {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		// code > 0 folds signal kills (ExitCode() == -1) into the 1 fallback.
		if code := ee.ExitCode(); code > 0 {
			return code
		}
	}
	return 1
}
