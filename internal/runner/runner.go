// Package runner executes git-hook task segments declared in .liszt/hooks.toml.
package runner

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/pelletier/go-toml/v2"
)

// generalSegment is the reserved segment name that always runs first.
const generalSegment = "general"

// Segment is one [<hook>.<segment>] table: a group of shell commands plus
// failure metadata.
type Segment struct {
	Commands []string `toml:"run"`
	FailHint string   `toml:"fail_hint"`
	Enabled  *bool    `toml:"enabled"` // nil => enabled
}

func (s Segment) isEnabled() bool {
	return s.Enabled == nil || *s.Enabled
}

// Hook is one [<hook>.*] table: segments keyed by name. "general" is reserved.
type Hook map[string]Segment

// Config is every hook in the file; top-level TOML keys decode into this map.
type Config map[string]Hook

// Load reads and decodes path. A missing or unreadable file, or malformed TOML,
// is an error.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	cfg := Config{}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
}

// Resolve returns the ordered segment names to run for a hook: "general" first
// when present, then each requested lang in order. Naming "general" explicitly
// is a no-op (it is already included). Errors if the hook is absent, a requested
// lang is absent, or no segment is selected at all (no general segment and no
// langs requested). A hook whose only selected segments are disabled still
// resolves successfully and RunHook returns 0: disabling a segment is how it is
// turned off, so an all-disabled hook is a successful no-op, not an error.
func (c Config) Resolve(name string, langs []string) ([]string, error) {
	hook, ok := c[name]
	if !ok {
		return nil, fmt.Errorf("no [%s] hook in config", name)
	}
	var order []string
	if _, ok := hook[generalSegment]; ok {
		order = append(order, generalSegment)
	}
	for _, l := range langs {
		if l == generalSegment {
			continue
		}
		if _, ok := hook[l]; !ok {
			return nil, fmt.Errorf("no [%s.%s] segment in config", name, l)
		}
		order = append(order, l)
	}
	if len(order) == 0 {
		return nil, fmt.Errorf("[%s] has nothing to run", name)
	}
	return order, nil
}

// RunHook runs the named segments of the hook in order. Within each segment,
// commands run via `bash -c` with args forwarded as positional parameters
// ($1, $@) to every command. All commands run even on failure; the first
// failing command's exit code (across all segments) is retained and returned
// (0 = all passed). Disabled segments are skipped. The named segments are
// assumed present (the caller resolved them via Resolve).
func (c Config) RunHook(name string, segments, args []string, stdout, stderr io.Writer) int {
	hook := c[name]
	failCode := 0
	for _, seg := range segments {
		if code := hook[seg].run(name, seg, args, stdout, stderr); code != 0 && failCode == 0 {
			failCode = code
		}
	}
	return failCode
}

// run executes one segment's commands. A disabled segment returns 0 without
// output; a segment with no commands returns 1 with an error.
func (s Segment) run(hookName, segName string, args []string, stdout, stderr io.Writer) int {
	if !s.isEnabled() {
		return 0
	}
	if len(s.Commands) == 0 {
		_, _ = fmt.Fprintf(stderr, "error: [%s.%s] has empty run\n", hookName, segName)
		return 1
	}

	_, _ = fmt.Fprintf(stdout, "== %s.%s ==\n", hookName, segName)

	failCode := 0
	failCmd := ""
	var failErr error
	for _, c := range s.Commands {
		cmd := exec.Command("bash", append([]string{"-c", c, "bash"}, args...)...)
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil && failCode == 0 {
			failCode = exitCode(err)
			failCmd = c
			failErr = err
		}
	}

	if failCode != 0 {
		_, _ = fmt.Fprint(stderr, failureLine(failCmd, failCode, failErr))
		if s.FailHint != "" {
			_, _ = fmt.Fprintf(stderr, "hint: %s\n", s.FailHint)
		}
	}
	return failCode
}

// failureLine formats the FAILED line. A command that ran and exited non-zero
// reports its exit code; a command that never started (non-ExitError, e.g. bash
// not found) reports the underlying error so the cause is not misattributed.
func failureLine(cmd string, code int, err error) string {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return fmt.Sprintf("FAILED: %s (exit %d)\n", cmd, code)
	}
	return fmt.Sprintf("FAILED: %s (could not start: %v)\n", cmd, err)
}

// exitCode extracts the process exit code from a command error. A non-exit
// error or a signal kill maps to 1.
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
