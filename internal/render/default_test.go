package render

import "testing"

func TestEnsureDefaultReturnsSingleton(t *testing.T) {
	a := ensureDefault()
	b := ensureDefault()
	if a != b {
		t.Errorf("ensureDefault returned different instances: %p vs %p", a, b)
	}
	if Default == nil {
		t.Errorf("Default is nil after ensureDefault")
	}
}
