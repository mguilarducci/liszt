package resource

func init() {
	register(Kind{Name: "command", List: listCommands})
}

// commands: <plugin>/commands/<name>.md (flat).
func listCommands(root string) ([]Item, error) {
	return walkItems(root, "commands", isMarkdownLeaf, trimExt, identityPath)
}
