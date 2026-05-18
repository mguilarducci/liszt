package repos

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// Entry is one row in repos.toml.
type Entry struct {
	Name string `toml:"name"`
	URL  string `toml:"url"`
	SHA  string `toml:"sha"`
}

// Config models repos.toml.
type Config struct {
	Repos []Entry `toml:"repos"`
}

// Load reads path. Missing file returns an empty Config.
func Load(path string) (*Config, error) {
	cfg := &Config{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
}

// Save writes cfg to path. Creates parent directories as needed.
func Save(path string, cfg *Config) error {
	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Upsert replaces an entry with the same Name or appends a new one.
func (c *Config) Upsert(e Entry) {
	for i, r := range c.Repos {
		if r.Name == e.Name {
			c.Repos[i] = e
			return
		}
	}
	c.Repos = append(c.Repos, e)
}
