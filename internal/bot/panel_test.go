package bot

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/TiaraBasori/PaperValet/internal/settings"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	s := New(Options{}, settings.NewRegistry(t.TempDir()), nil)
	s.SetOwner(42)
	return s
}

func hasButton(v *View, data string) bool {
	for _, row := range v.Buttons {
		for _, b := range row {
			if b.Data == data {
				return true
			}
		}
	}
	return false
}

func TestPanelEditsSettings(t *testing.T) {
	s := newTestService(t)
	st, err := s.Settings(&plugin.SettingsSpec{
		Plugin: "demo",
		Settings: []plugin.Setting{
			{Key: "on", Label: "On", Kind: plugin.SettingToggle},
			{Key: "mode", Label: "Mode", Kind: plugin.SettingChoice, Default: "a",
				Choices: []plugin.Choice{{Value: "a", Label: "A"}, {Value: "b", Label: "B"}}},
			{Key: "n", Label: "N", Kind: plugin.SettingNumber, Default: 1, Min: 1, Max: 9},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	menu := s.menuView()
	if !hasButton(menu, "h:demo") {
		t.Fatalf("menu misses plugin: %+v", menu.Buttons)
	}
	v, _, err := s.dispatch(ctx, "h:demo", 1)
	if err != nil || !hasButton(v, "e:demo:on") || !hasButton(v, "m") {
		t.Fatalf("plugin view: %v %+v", err, v)
	}
	if _, _, err := s.dispatch(ctx, "e:demo:on", 1); err != nil || !st.Bool("on") {
		t.Fatal("toggle did not flip")
	}
	v, _, _ = s.dispatch(ctx, "e:demo:mode", 1)
	if !hasButton(v, "c:demo:mode:1") {
		t.Fatalf("choice buttons missing: %+v", v.Buttons)
	}
	if _, _, err := s.dispatch(ctx, "c:demo:mode:1", 1); err != nil || st.String("mode") != "b" {
		t.Fatal("choice not stored")
	}
	if _, _, err := s.dispatch(ctx, "c:demo:mode:9", 1); err == nil {
		t.Fatal("out of range choice accepted")
	}
	v, _, _ = s.dispatch(ctx, "e:demo:mode", 1)
	if !hasButton(v, "r:demo:mode") {
		t.Fatal("reset offered only for changed values")
	}
	if _, _, err := s.dispatch(ctx, "r:demo:mode", 1); err != nil || st.String("mode") != "a" {
		t.Fatal("reset failed")
	}

	// Typed number: edit arms a pending answer for the panel message.
	s.dispatch(ctx, "e:demo:n", 7)
	p := s.currentPending()
	if p == nil || p.key != "n" || p.msgID != 7 || p.page {
		t.Fatalf("pending = %+v", p)
	}
	// Typed answers: a bad one keeps waiting, a good one is stored.
	s.onMessage(42, 100, "99")
	if s.currentPending() == nil || st.Int("n") != 1 {
		t.Fatal("out-of-range answer stored or pending dropped")
	}
	s.onMessage(42, 101, "4")
	if s.currentPending() != nil || st.Int("n") != 4 {
		t.Fatalf("answer not stored: %d", st.Int("n"))
	}
	s.dispatch(ctx, "e:demo:n", 7)
	// Leaving the screen cancels it.
	s.dispatch(ctx, "m", 7)
	if s.currentPending() != nil {
		t.Fatal("pending survived navigation")
	}
}

func TestPageRoutingAndAsk(t *testing.T) {
	s := newTestService(t)
	var got []string
	err := s.For("demo").SetPage(&plugin.Page{
		Title: "Demo",
		Handle: func(c *plugin.BotContext) (*plugin.View, error) {
			got = append(got, c.Data+"|"+c.Input)
			if c.Data == "ask" {
				c.Ask("name")
			}
			if c.Data == "hi" {
				c.Alert("hello")
			}
			return &plugin.View{Text: "page", Buttons: [][]plugin.Button{
				plugin.Row(plugin.Btn("Hi", "hi"), plugin.LinkBtn("Site", "https://example.com")),
				plugin.Row(plugin.Btn("Too long", strings.Repeat("x", 40))),
			}}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	// No settings: the plugin entry opens the page directly.
	v, _, err := s.dispatch(ctx, "h:demo", 3)
	if err != nil || !hasButton(v, "p:demo:hi") || !hasButton(v, "m") {
		t.Fatalf("page view: %v %+v", err, v)
	}
	for _, row := range v.Buttons {
		for _, b := range row {
			if strings.Contains(b.Data, "xxxx") {
				t.Fatal("oversized data kept")
			}
			if b.URL != "" && b.Data != "" {
				t.Fatal("url button got data")
			}
		}
	}
	_, notice, _ := s.dispatch(ctx, "p:demo:hi", 3)
	if notice != "!hello" {
		t.Fatalf("notice = %q", notice)
	}
	s.dispatch(ctx, "p:demo:ask", 3)
	if p := s.currentPending(); p == nil || !p.page || p.key != "name" || p.msgID != 3 {
		t.Fatalf("ask pending = %+v", p)
	}
	if strings.Join(got, ",") != "|,hi|,ask|" {
		t.Fatalf("handle calls = %v", got)
	}

	s.RemovePlugin("demo")
	if s.page("demo") != nil || s.currentPending() != nil {
		t.Fatal("unload left state behind")
	}
}

func TestPinnedFirst(t *testing.T) {
	s := newTestService(t)
	for _, n := range []string{"zeta", "alpha", "lang"} {
		if _, err := s.Settings(&plugin.SettingsSpec{Plugin: n, Settings: []plugin.Setting{{Key: "k", Label: "K"}}}); err != nil {
			t.Fatal(err)
		}
	}
	s.SetPinned([]string{"lang"})
	if got := strings.Join(s.entries(), ","); got != "lang,alpha,zeta" {
		t.Fatalf("order = %s", got)
	}
}

func TestTokenBotID(t *testing.T) {
	if TokenBotID("8863893462:abc") != 8863893462 || TokenBotID("x:y") != 0 || TokenBotID("123") != 0 {
		t.Fatal("TokenBotID")
	}
}

func TestOnlyOwnerRejected(t *testing.T) {
	s := newTestService(t)
	// onMessage from a stranger must not touch pending state or send.
	s.setPending(&pending{plugin: "x", key: "y"})
	s.onMessage(7, 1, "hello")
	if s.currentPending() == nil {
		t.Fatal("stranger cleared owner state")
	}
}

func TestLocalErr(t *testing.T) {
	s := newTestService(t)
	err := plugin.Invalid("中文", "english")
	if got := s.localErr(err); got != "中文" {
		t.Fatalf("zh default: %q", got)
	}
	if got := s.localErr(errors.New("raw")); got != "raw" {
		t.Fatalf("plain error: %q", got)
	}
}
