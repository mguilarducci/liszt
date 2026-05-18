package manifest

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// Entry is one declarative want.
type Entry struct {
	Kind   string `toml:"kind"`
	Slug   string `toml:"slug"`   // may be qualified "<plugin>:<slug>"
	Flavor string `toml:"flavor"` // claude | copilot
}

// Config models liszt.toml.
type Config struct {
	Items []Entry `toml:"items"`
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

// Upsert replaces an entry with the same (Kind, Slug, Flavor) or appends.
func (c *Config) Upsert(e Entry) {
	for i, x := range c.Items {
		if x.Kind == e.Kind && x.Slug == e.Slug && x.Flavor == e.Flavor {
			c.Items[i] = e
			return
		}
	}
	c.Items = append(c.Items, e)
}
