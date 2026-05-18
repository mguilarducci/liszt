package claudehome

import (
	"os"
	"path/filepath"
)

// Dir returns the Claude Code home directory.
// $CLAUDE_HOME if set to an absolute path, otherwise ~/.claude.
func Dir() string {
	if v := os.Getenv("CLAUDE_HOME"); v != "" && filepath.IsAbs(v) {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".claude")
}
