package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mguilarducci/liszt/internal/version"
)

func TestVersionCmd_EmitsFullString(t *testing.T) {
	var buf bytes.Buffer
	cmd := newVersionCmd()
	cmd.SetOut(&buf)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE error: %v", err)
	}
	if !strings.Contains(buf.String(), version.Full()) {
		t.Errorf("version output missing Full(): %q", buf.String())
	}
}

func TestVersionCmd_RegisteredOnRoot(t *testing.T) {
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Use == "version" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("version subcommand not registered on rootCmd")
	}
}
