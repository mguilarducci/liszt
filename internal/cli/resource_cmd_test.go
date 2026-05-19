package cli

import (
	"testing"

	"github.com/mguilarducci/liszt/internal/resource"
)

func TestResourceCmds_AllKindsRegistered(t *testing.T) {
	t.Parallel()

	registered := map[string]bool{}
	for _, c := range rootCmd.Commands() {
		registered[c.Use] = true
	}
	for _, kind := range resource.Kinds() {
		if !registered[kind] {
			t.Errorf("kind %q not registered as subcommand", kind)
		}
	}
}

func TestResourceCmds_ListHasPluginFlag(t *testing.T) {
	t.Parallel()

	for _, c := range rootCmd.Commands() {
		var kind string
		for _, k := range resource.Kinds() {
			if c.Use == k {
				kind = k
			}
		}
		if kind == "" {
			continue
		}
		for _, sub := range c.Commands() {
			if sub.Use != "list" {
				continue
			}
			if sub.Flags().Lookup("plugin") == nil {
				t.Errorf("%s list missing --plugin flag", kind)
			}
		}
	}
}

func TestResourceCmds_InstallHasFlavorFlag(t *testing.T) {
	t.Parallel()

	for _, c := range rootCmd.Commands() {
		var kind string
		for _, k := range resource.Kinds() {
			if c.Use == k {
				kind = k
			}
		}
		if kind == "" {
			continue
		}
		for _, sub := range c.Commands() {
			if sub.Use != "install <slug>" {
				continue
			}
			if sub.Flags().Lookup("flavor") == nil {
				t.Errorf("%s install missing --flavor flag", kind)
			}
		}
	}
}
