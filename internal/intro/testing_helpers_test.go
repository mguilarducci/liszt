package intro

import (
	"os"
	"testing"
)

func osCreateTemp(t *testing.T) (*os.File, error) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "intro-test-*")
	if err != nil {
		return nil, err
	}
	t.Cleanup(func() { os.Remove(f.Name()) })
	return f, nil
}
