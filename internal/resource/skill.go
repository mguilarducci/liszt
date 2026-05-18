package resource

import (
	"io/fs"
	"path/filepath"
	"strings"
)

func init() {
	register(Kind{Name: "skill", List: listSkills})
}

// skills: <plugin>/skills/<name>/SKILL.md (recursive). Artifact = the skill dir.
func listSkills(root string) ([]Item, error) {
	return walkItems(root, "skills",
		func(d fs.DirEntry) bool { return !d.IsDir() && strings.EqualFold(d.Name(), "SKILL.md") },
		func(rel string) string { return filepath.ToSlash(filepath.Dir(rel)) },
		filepath.Dir,
	)
}
