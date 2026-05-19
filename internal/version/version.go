// Package version holds build-time identification strings injected via
// -ldflags. The .goreleaser.yaml file wires Version, Commit, and Date for
// release builds; local builds fall back to the dev defaults below.
package version

var (
	Version = "0.0.0-dev"
	Commit  = "none"
	Date    = "unknown"
)

// Full returns the user-facing version string used by the `liszt version`
// subcommand and cobra's `--version` flag.
func Full() string {
	return "liszt " + Version
}
