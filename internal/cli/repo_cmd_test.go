package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestRepoCmd_RegisteredOnRoot(t *testing.T) {
	t.Parallel()

	found := false
	for _, c := range rootCmd.Commands() {
		if c.Use == "repo" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("repo subcommand not registered on rootCmd")
	}
}

func TestRepoAddCmd_HasURLArg(t *testing.T) {
	t.Parallel()

	var addCmd *cobra.Command
	for _, c := range rootCmd.Commands() {
		if c.Use != "repo" {
			continue
		}
		for _, sub := range c.Commands() {
			if sub.Use == "add <url>" {
				addCmd = sub
			}
		}
	}
	if addCmd == nil {
		t.Fatal("repo add subcommand not registered")
	}
	if addCmd.Args == nil {
		t.Errorf("repo add must require exactly 1 argument")
	}
}
