package render

import (
	"bytes"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

func TestDetectProfile_NoColorEnv(t *testing.T) {
	t.Parallel()

	got, isTTY := detectProfile(&bytes.Buffer{}, []string{"NO_COLOR=1", "TERM=xterm-256color"})
	if got != colorprofile.NoTTY {
		t.Errorf("with NO_COLOR set: got profile=%v, want NoTTY", got)
	}
	if isTTY {
		t.Errorf("with bytes.Buffer writer: got isTTY=true, want false")
	}
}

func TestDetectProfile_DumbTerm(t *testing.T) {
	t.Parallel()

	got, _ := detectProfile(&bytes.Buffer{}, []string{"TERM=dumb"})
	if got != colorprofile.NoTTY {
		t.Errorf("with TERM=dumb: got profile=%v, want NoTTY", got)
	}
}

func TestDetectProfile_NonTTYWriter(t *testing.T) {
	t.Parallel()

	_, isTTY := detectProfile(&bytes.Buffer{}, []string{"TERM=xterm-256color", "COLORTERM=truecolor"})
	if isTTY {
		t.Errorf("with bytes.Buffer writer: got isTTY=true, want false")
	}
}
