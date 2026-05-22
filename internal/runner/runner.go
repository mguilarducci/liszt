// Package runner executes named command groups declared in .liszt/liszt.toml.
package runner

import (
	"fmt"
	"os"

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
