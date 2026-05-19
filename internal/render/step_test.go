package render

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

func plainRenderer(buf *bytes.Buffer) *Renderer {
	return New(buf, WithProfile(colorprofile.NoTTY))
}

func TestStep_PrintsGlyphAndMessage(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	plainRenderer(&buf).Step("Cloning obra/superpowers")
	got := buf.String()
	if got != "⠇ Cloning obra/superpowers\n" {
		t.Errorf("Step output mismatch: %q", got)
	}
}

func TestStepDone_PrintsCheckAndMessage(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	plainRenderer(&buf).StepDone("Cloned obra/superpowers")
	got := buf.String()
	if got != "✓ Cloned obra/superpowers\n" {
		t.Errorf("StepDone output mismatch: %q", got)
	}
}

func TestStepFail_IncludesErrorInline(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	plainRenderer(&buf).StepFail("Cloning obra/superpowers", errors.New("permission denied"))
	got := buf.String()
	want := "✗ Cloning obra/superpowers: permission denied\n"
	if got != want {
		t.Errorf("StepFail mismatch:\nwant %q\ngot  %q", want, got)
	}
}

func TestStepFail_NilErrorOmitsColon(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	plainRenderer(&buf).StepFail("Cloning", nil)
	got := buf.String()
	if !strings.Contains(got, "✗ Cloning") {
		t.Errorf("StepFail nil error missing message: %q", got)
	}
	if strings.Contains(got, ":") {
		t.Errorf("StepFail nil error should not include colon: %q", got)
	}
}
