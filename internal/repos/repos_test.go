package repos

import "testing"

func TestFind_Hit(t *testing.T) {
	t.Parallel()

	c := &Config{Repos: []Entry{
		{Name: "a/b", URL: "https://x", SHA: "1"},
		{Name: "c/d", URL: "https://y", SHA: "2"},
	}}
	got, ok := c.Find("c/d")
	if !ok {
		t.Fatalf("Find(c/d) returned ok=false")
	}
	if got.URL != "https://y" {
		t.Errorf("Find returned wrong entry: %+v", got)
	}
}

func TestFind_Miss(t *testing.T) {
	t.Parallel()

	c := &Config{Repos: []Entry{{Name: "a/b"}}}
	got, ok := c.Find("z/z")
	if ok {
		t.Errorf("Find(z/z) on absent name should be ok=false, got %+v", got)
	}
}

func TestFind_EmptyConfig(t *testing.T) {
	t.Parallel()

	c := &Config{}
	if _, ok := c.Find("a/b"); ok {
		t.Errorf("Find on empty config should be ok=false")
	}
}
