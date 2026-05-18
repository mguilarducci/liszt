package lock

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

// Entry is one resolved install record.
type Entry struct {
	Kind   string `toml:"kind"`
	Flavor string `toml:"flavor"`
	Slug   string `toml:"slug"`
	Plugin string `toml:"plugin"`
	Repo   string `toml:"repo"`
	SHA    string `toml:"sha"`
	Path   string `toml:"path"`
}

// Config models liszt.lock.
type Config struct {
	Locked []Entry `toml:"locked"`
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

// Save writes cfg to path.
func Save(path string, cfg *Config) error {
	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Upsert replaces an entry with the same (Kind, Slug, Plugin, Flavor) or appends.
func (c *Config) Upsert(e Entry) {
	for i, x := range c.Locked {
		if x.Kind == e.Kind && x.Slug == e.Slug && x.Plugin == e.Plugin && x.Flavor == e.Flavor {
			c.Locked[i] = e
			return
		}
	}
	c.Locked = append(c.Locked, e)
}
