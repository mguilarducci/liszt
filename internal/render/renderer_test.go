package render

import (
	"bytes"
	"math/rand/v2"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

func TestNew_DefaultProfileDetected(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := New(&buf)
	if r == nil {
		t.Fatal("New returned nil")
	}
	if r.isTTY {
		t.Errorf("bytes.Buffer is not a TTY")
	}
}

func TestNew_WithProfileOverride(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := New(&buf, WithProfile(colorprofile.TrueColor))
	if r.profile != colorprofile.TrueColor {
		t.Errorf("WithProfile: got %v, want TrueColor", r.profile)
	}
}

func TestNew_WithNoColor(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := New(&buf, WithProfile(colorprofile.TrueColor), WithNoColor())
	if r.profile != colorprofile.NoTTY {
		t.Errorf("WithNoColor: got %v, want NoTTY", r.profile)
	}
}

func TestNew_WithTTY(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := New(&buf, WithTTY(true))
	if !r.isTTY {
		t.Errorf("WithTTY(true): got false, want true")
	}
}

func TestNew_WithRand(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	rng := rand.New(rand.NewPCG(1, 2))
	r := New(&buf, WithRand(rng))
	if r.rng != rng {
		t.Errorf("WithRand: did not store provided rand")
	}
}

func TestWriteString_StripsANSIOnNoTTY(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := New(&buf, WithProfile(colorprofile.NoTTY))
	r.writeString("\x1b[31mred\x1b[0m")
	if got := buf.String(); got != "red" {
		t.Errorf("NoTTY profile did not strip ANSI: got %q", got)
	}
}

func TestEraseLine_BypassesProfile(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := New(&buf, WithProfile(colorprofile.NoTTY))
	r.eraseLine()
	if got := buf.String(); got != "\r\x1b[K" {
		t.Errorf("eraseLine emitted %q; want %q", got, "\r\x1b[K")
	}
}
