package cli

import (
	"strings"
	"testing"
)

func joinOrNil(s []string) string {
	if s == nil {
		return "<nil>"
	}
	return strings.Join(s, ",")
}

func TestSplitHookArgs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		args        []string
		dash        int
		wantName    string
		wantLangs   string
		wantGitArgs string
	}{
		{"no dash, langs only", []string{"pre-commit", "go", "gleam"}, -1, "pre-commit", "go,gleam", "<nil>"},
		{"no dash, name only", []string{"pre-commit"}, -1, "pre-commit", "", "<nil>"},
		{"dash, langs and gitargs", []string{"pre-commit", "gleam", "a", "b"}, 2, "pre-commit", "gleam", "a,b"},
		{"dash, no langs, gitargs", []string{"pre-commit", "a", "b"}, 1, "pre-commit", "", "a,b"},
		{"dash, no langs, no gitargs", []string{"pre-commit"}, 1, "pre-commit", "", ""},
		{"dash zero, name only", []string{"pre-commit"}, 0, "pre-commit", "<nil>", ""},
		{"dash zero, trailing gitargs", []string{"pre-commit", "x"}, 0, "pre-commit", "<nil>", "x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			name, langs, gitArgs := splitHookArgs(tc.args, tc.dash)
			if name != tc.wantName {
				t.Errorf("name = %q, want %q", name, tc.wantName)
			}
			if joinOrNil(langs) != tc.wantLangs {
				t.Errorf("langs = %q, want %q", joinOrNil(langs), tc.wantLangs)
			}
			if joinOrNil(gitArgs) != tc.wantGitArgs {
				t.Errorf("gitArgs = %q, want %q", joinOrNil(gitArgs), tc.wantGitArgs)
			}
		})
	}
}
