package xdg

import (
	"os"
	"path/filepath"
)

// DataDir returns the liszt data directory.
// $XDG_DATA_HOME/liszt if XDG_DATA_HOME is set to an absolute path,
// otherwise ~/.local/share/liszt.
func DataDir() string {
	return resolve("XDG_DATA_HOME", filepath.Join(".local", "share"))
}

// CacheDir returns the liszt cache directory.
// $XDG_CACHE_HOME/liszt if XDG_CACHE_HOME is set to an absolute path,
// otherwise ~/.cache/liszt.
func CacheDir() string {
	return resolve("XDG_CACHE_HOME", ".cache")
}

func resolve(envVar, homeRel string) string {
	if v := os.Getenv(envVar); v != "" && filepath.IsAbs(v) {
		return filepath.Join(v, "liszt")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, homeRel, "liszt")
}
