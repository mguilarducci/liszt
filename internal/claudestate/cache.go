package claudestate

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// MaterializePlugin copies srcDir contents into
//
//	claudeHome/plugins/cache/<mp>/<plugin>/<version>/
//
// Removes any existing contents first to avoid stale files.
// Skips ".in_use" directories (Claude-internal session markers).
// Returns the absolute install path.
func MaterializePlugin(claudeHome, mp, plugin, version, srcDir string) (string, error) {
	dest := filepath.Join(claudeHome, "plugins", "cache", mp, plugin, version)
	if err := os.RemoveAll(dest); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dest, 0755); err != nil {
		return "", err
	}
	err := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == ".in_use" && d.IsDir() {
			return fs.SkipDir
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return copyFile(path, target)
	})
	if err != nil {
		return "", err
	}
	return dest, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
