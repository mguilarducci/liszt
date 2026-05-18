package main

import (
	"fmt"
	"os"

	"github.com/mguilarducci/liszt/internal/cli"
	"github.com/mguilarducci/liszt/internal/resource"
)

const (
	reposFile    = "repos.toml"
	manifestFile = "liszt.toml"
	lockFile     = "liszt.lock"
	cacheDir     = "tmp"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	paths := cli.Paths{Repos: reposFile, Manifest: manifestFile, Lock: lockFile, Cache: cacheDir}

	switch cmd {
	case "repo":
		run(cli.Repo(paths, args))
	case "plugin":
		runPlugin(paths, args)
	case "outdated":
		run(cli.Outdated(paths))
	default:
		if _, ok := resource.Get(cmd); ok {
			runResource(paths, cmd, args)
			return
		}
		usage()
		os.Exit(2)
	}
}

func runPlugin(p cli.Paths, args []string) {
	if len(args) >= 2 && args[0] == "install" {
		slug, flavor := cli.ParseInstallArgs(args[1:])
		run(cli.PluginInstall(p, slug, flavor))
		return
	}
	if len(args) < 1 || args[0] != "list" {
		fmt.Fprintln(os.Stderr, "usage: liszt plugin {list | install <slug> --flavor <claude|copilot>}")
		os.Exit(2)
	}
	run(cli.PluginList(p))
}

func runResource(p cli.Paths, kind string, args []string) {
	if len(args) >= 2 && args[0] == "install" {
		slug, flavor := cli.ParseInstallArgs(args[1:])
		run(cli.ResourceInstall(p, kind, slug, flavor))
		return
	}
	if len(args) < 1 || args[0] != "list" {
		fmt.Fprintf(os.Stderr, "usage: liszt %s {list [--plugin <name>] | install <slug> --flavor <claude|copilot>}\n", kind)
		os.Exit(2)
	}
	var pluginName string
	for i := 1; i < len(args); i++ {
		if args[i] == "--plugin" && i+1 < len(args) {
			pluginName = args[i+1]
			i++
		}
	}
	run(cli.ResourceList(p, kind, pluginName))
}

func run(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  liszt repo add <github-url>             clone marketplace repo
  liszt plugin list                       list plugins across all repos
  liszt plugin install <slug> --flavor <claude|copilot>
  liszt <kind> list [--plugin <name>]     list resources of a kind
  liszt <kind> install <slug> --flavor <claude|copilot>
                                          install: writes liszt.toml (manifest) + liszt.lock
                                          kinds: skill, agent, command, hook, mcp, lsp
  liszt outdated                          compare liszt.lock SHAs vs remote HEAD`)
}
