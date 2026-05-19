package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestPluginCmd_RegisteredOnRoot(t *testing.T) {
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Use == "plugin" {
			found = true
		}
	}
	if !found {
		t.Errorf("plugin subcommand not registered on rootCmd")
	}
}

func TestPluginListAndInstallRegistered(t *testing.T) {
	wantSubs := map[string]bool{"list": false, "install <slug>": false}
	for _, c := range rootCmd.Commands() {
		if c.Use != "plugin" {
			continue
		}
		for _, sub := range c.Commands() {
			if _, ok := wantSubs[sub.Use]; ok {
				wantSubs[sub.Use] = true
			}
		}
	}
	for name, found := range wantSubs {
		if !found {
			t.Errorf("plugin %s not registered", name)
		}
	}
}

func TestPluginInstall_HasFlavorFlag(t *testing.T) {
	var installCmd *cobra.Command
	for _, c := range rootCmd.Commands() {
		if c.Use != "plugin" {
			continue
		}
		for _, sub := range c.Commands() {
			if sub.Use == "install <slug>" {
				installCmd = sub
			}
		}
	}
	if installCmd == nil {
		t.Fatal("plugin install subcommand not found")
	}
	if installCmd.Flags().Lookup("flavor") == nil {
		t.Errorf("plugin install missing --flavor flag")
	}
}
