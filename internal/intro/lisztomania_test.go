package intro

import (
	"bytes"
	"testing"
)

func TestNew_DefaultsToDarkBackground(t *testing.T) {
	t.Parallel()

	m := NewWithDefaults()
	if !m.hasDarkBackground {
		t.Errorf("NewWithDefaults() should set hasDarkBackground=true")
	}
	if m.TotalFrames() == 0 {
		t.Errorf("model has zero frames; generated frame data missing")
	}
}

func TestNew_LightBackground(t *testing.T) {
	t.Parallel()

	m := New(false)
	if m.hasDarkBackground {
		t.Errorf("New(false) should set hasDarkBackground=false")
	}
}

func TestView_NonEmptyForFirstFrame(t *testing.T) {
	t.Parallel()

	m := NewWithDefaults()
	if got := m.View(); got == "" {
		t.Errorf("View() returned empty string for frame 0")
	}
}

func TestPlay_NonTTYIsNoOp(t *testing.T) {
	t.Parallel()

	// bytes.Buffer is not an *os.File, so Play should short-circuit.
	if err := Play(&bytes.Buffer{}, true); err != nil {
		t.Errorf("Play on non-TTY writer returned error: %v", err)
	}
}

func TestPlay_NonTTYFileIsNoOp(t *testing.T) {
	t.Parallel()

	// An *os.File that is not a terminal (regular file) should also be a no-op.
	f, err := osCreateTemp(t)
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer f.Close()
	if err := Play(f, true); err != nil {
		t.Errorf("Play on non-TTY file returned error: %v", err)
	}
}
