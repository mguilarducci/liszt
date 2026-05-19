package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

func TestDetail_SuppressedByDefault(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := New(&buf, WithProfile(colorprofile.NoTTY))
	r.Detail("loading", "path", "/tmp/x")
	if buf.Len() != 0 {
		t.Errorf("Detail must be silent when verbose=false: %q", buf.String())
	}
}

func TestDetail_EmittedWhenVerbose(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := New(&buf, WithProfile(colorprofile.NoTTY))
	r.SetVerbose(true)
	r.Detail("loading", "path", "/tmp/x")
	got := buf.String()
	if !strings.HasPrefix(got, "· loading") {
		t.Errorf("Detail missing `· ` prefix or message: %q", got)
	}
	if !strings.Contains(got, "path=/tmp/x") {
		t.Errorf("Detail missing kv pair: %q", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("Detail missing trailing newline: %q", got)
	}
}

func TestDetail_NoKVOmitsPayload(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := New(&buf, WithProfile(colorprofile.NoTTY))
	r.SetVerbose(true)
	r.Detail("ping")
	got := buf.String()
	if got != "· ping\n" {
		t.Errorf("Detail with no kv should be `· ping\\n`, got %q", got)
	}
}
