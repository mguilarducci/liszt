package resource

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func isMarkdownLeaf(d fs.DirEntry) bool {
	return !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".md")
}

func trimExt(rel string) string {
	rel = filepath.ToSlash(rel)
	return strings.TrimSuffix(rel, filepath.Ext(rel))
}

func identityPath(p string) string { return p }

// walkItems walks <pluginRoot>/<subdir>. For each leaf matching match:
//   - slugOf(rel)         → Item.Slug   (rel = path under subdir)
//   - pathOf(pathInPlugin) → Item.Path  (pathInPlugin = subdir-prefixed path within plugin root)
func walkItems(pluginRoot, subdir string, match func(fs.DirEntry) bool, slugOf func(rel string) string, pathOf func(pathInPlugin string) string) ([]Item, error) {
	base := filepath.Join(pluginRoot, subdir)
	if _, err := os.Stat(base); os.IsNotExist(err) {
		return nil, nil
	}
	var out []Item
	err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if match(d) {
			rel, _ := filepath.Rel(base, path)
			pathInPlugin, _ := filepath.Rel(pluginRoot, path)
			out = append(out, Item{
				Slug: slugOf(rel),
				Path: filepath.ToSlash(pathOf(pathInPlugin)),
			})
		}
		return nil
	})
	return out, err
}

// readFirstWithSource returns the first existing file under root and the relative path that matched.
func readFirstWithSource(root string, paths ...string) ([]byte, string, bool, error) {
	for _, p := range paths {
		data, err := os.ReadFile(filepath.Join(root, p))
		if err == nil {
			return data, p, true, nil
		}
		if !os.IsNotExist(err) {
			return nil, "", false, err
		}
	}
	return nil, "", false, nil
}
