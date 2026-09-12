package i18n

import "testing"

func TestSubstitute(t *testing.T) {
	got := substitute("Hello {0}, you have {1} messages", []any{"awa", 3})
	want := "Hello awa, you have 3 messages"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFallback(t *testing.T) {
	c := New(ZhCN)
	c.Add(ZhCN, "greet", "你好 {0}")
	c.Add(EnUS, "greet", "Hello {0}")
	c.Add(ZhCN, "only_zh", "仅中文")

	if got := c.T(EnUS, "greet", "awa"); got != "Hello awa" {
		t.Fatalf("en greet: %q", got)
	}
	if got := c.T(ZhCN, "greet", "awa"); got != "你好 awa" {
		t.Fatalf("zh greet: %q", got)
	}
	// Missing in en-US falls back to default zh-CN
	if got := c.T(EnUS, "only_zh"); got != "仅中文" {
		t.Fatalf("fallback: %q", got)
	}
	// Missing everywhere returns key
	if got := c.T(EnUS, "nope"); got != "nope" {
		t.Fatalf("missing key: %q", got)
	}
}

func TestLanguages(t *testing.T) {
	c := New(ZhCN)
	c.Add(ZhCN, "a", "1")
	c.Add(EnUS, "a", "2")
	langs := c.Languages()
	if len(langs) != 2 {
		t.Fatalf("expected 2 languages, got %v", langs)
	}
}