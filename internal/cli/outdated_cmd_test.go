package cli

import "testing"

func TestOutdatedCmd_Registered(t *testing.T) {
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Use == "outdated" {
			found = true
		}
	}
	if !found {
		t.Errorf("outdated subcommand not registered")
	}
}
