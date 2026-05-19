package render

import (
	"bytes"
	"strings"
	"testing"
)

func TestHeader_PlainText(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	newTestRenderer(&buf).Header("Plugins")
	if !strings.Contains(buf.String(), "Plugins") {
		t.Errorf("Header missing text: %q", buf.String())
	}
}

func TestSubheader_Prefix(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	newTestRenderer(&buf).Subheader("Skills")
	if !strings.Contains(buf.String(), "▸ Skills") {
		t.Errorf("Subheader missing prefix: %q", buf.String())
	}
}

func TestHint_PlainText(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	newTestRenderer(&buf).Hint("3 plugins across 2 repos")
	if !strings.Contains(buf.String(), "3 plugins across 2 repos") {
		t.Errorf("Hint missing text: %q", buf.String())
	}
}
