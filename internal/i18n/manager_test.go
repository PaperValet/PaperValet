package i18n

import "testing"

func TestManagerUserLang(t *testing.T) {
	cat := CoreCatalog()
	m := NewManager(cat)

	if got := m.UserLang(1); got != ZhCN {
		t.Fatalf("default should be zh-CN, got %s", got)
	}
	m.SetUserLang(1, EnUS)
	if got := m.UserLang(1); got != EnUS {
		t.Fatalf("user lang should be en-US, got %s", got)
	}
	if got := m.T(1, "core.ping_start"); got != "🏓 Pong!" {
		t.Fatalf("en ping: %q", got)
	}
	if got := m.T(2, "core.ping_start"); got != "🏓 Pong!" {
		t.Fatalf("zh ping: %q", got)
	}
}

func TestContextLang(t *testing.T) {
	cat := CoreCatalog()
	m := NewManager(cat)
	ctx := WithLang(t.Context(), EnUS)
	if got := m.TContext(ctx, 0, "core.ping_start"); got != "🏓 Pong!" {
		t.Fatalf("ctx lang: %q", got)
	}
}

func TestClearUserLangs(t *testing.T) {
	cat := New(ZhCN)
	m := NewManager(cat)
	m.SetUserLang(1, EnUS)
	cat.SetDefault(EnUS)
	m.ClearUserLangs()
	if m.UserLang(1) != EnUS {
		t.Fatal("cleared user must follow the default")
	}
	cat.SetDefault(ZhCN)
	if m.UserLang(1) != ZhCN {
		t.Fatal("default switch must apply after clear")
	}
}
