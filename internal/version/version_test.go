package version_test

import (
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/mguilarducci/liszt/internal/version"
)

func TestFullSnapshot(t *testing.T) {
	t.Parallel()

	autogold.Expect("liszt 0.0.0-dev").Equal(t, version.Full())
}

func TestFullUsesVersionVar(t *testing.T) {
	orig := version.Version
	t.Cleanup(func() { version.Version = orig })
	version.Version = "9.9.9"

	if got := version.Full(); got != "liszt 9.9.9" {
		t.Errorf("Full() = %q; want %q", got, "liszt 9.9.9")
	}
}
