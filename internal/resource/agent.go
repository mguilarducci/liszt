package resource

import (
	"path/filepath"
	"strings"
)

func init() {
	register(Kind{Name: "agent", List: listAgents})
}

// agents: <plugin>/agents/<name>.md (Claude) or <name>.agent.md (Copilot).
func listAgents(root string) ([]Item, error) {
	return walkItems(root, "agents", isMarkdownLeaf, agentSlug, identityPath)
}

func agentSlug(rel string) string {
	rel = filepath.ToSlash(rel)
	rel = strings.TrimSuffix(rel, ".agent.md")
	rel = strings.TrimSuffix(rel, ".md")
	return rel
}
