package bot

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

func TestBarAndElapsed(t *testing.T) {
	if got := bar(0, 4); got != "▱▱▱▱▱▱▱▱▱▱" {
		t.Fatalf("empty bar %q", got)
	}
	if got := bar(1, 2); got != "▰▰▰▰▰▱▱▱▱▱" {
		t.Fatalf("half bar %q", got)
	}
	if got := bar(9, 9); got != "▰▰▰▰▰▰▰▰▰▰" {
		t.Fatalf("full bar %q", got)
	}
	for d, want := range map[time.Duration]string{
		400 * time.Millisecond: "0.4s",
		12 * time.Second:       "12s",
		125 * time.Second:      "2m05s",
	} {
		if got := elapsed(d); got != want {
			t.Fatalf("elapsed(%v) = %q", d, got)
		}
	}
}

func TestProgressFrame(t *testing.T) {
	s := newTestService(t)
	p := s.startProgress(context.Background(), target{}, "全部安装", 3)
	p.step("gt")
	text := p.text(0)
	if !strings.Contains(text, "0/3") || !strings.Contains(text, "▸ `gt`") || !strings.HasPrefix(text, "⏳") {
		t.Fatalf("running frame: %s", text)
	}
	p.result("✅ `gt`")
	p.step("ddg")
	text = p.text(1)
	if !strings.Contains(text, "1/3") || !strings.Contains(text, ">✅ `gt`") || !strings.Contains(text, "▸ `ddg`") || !strings.HasPrefix(text, "⌛") {
		t.Fatalf("second frame: %s", text)
	}
	// The block quote must parse into one entity.
	plain, ents := plugin.ParseMarkdown(text, noUsers)
	quoted := false
	for _, e := range ents {
		if _, ok := e.(*tg.MessageEntityBlockquote); ok {
			quoted = true
		}
	}
	if !quoted || strings.Contains(plain, ">") {
		t.Fatalf("quote not parsed: %q %v", plain, ents)
	}
	p.finish()
}

func TestButtonColors(t *testing.T) {
	m := markup([][]plugin.Button{plugin.Row(
		plugin.Btn("a", "a"),
		plugin.Btn("b", "b").Danger(),
		plugin.LinkBtn("c", "https://example.com").Success(),
	)}).(*tg.ReplyInlineMarkup)
	btns := m.Rows[0].Buttons
	if _, ok := btns[0].(*tg.KeyboardButtonCallback).GetStyle(); ok {
		t.Fatal("plain button got a style")
	}
	if st, ok := btns[1].(*tg.KeyboardButtonCallback).GetStyle(); !ok || !st.BgDanger {
		t.Fatalf("danger style: %+v", st)
	}
	if st, ok := btns[2].(*tg.KeyboardButtonURL).GetStyle(); !ok || !st.BgSuccess {
		t.Fatalf("success style: %+v", st)
	}

	// Panels color their special buttons.
	s, _ := newCatalogService(t)
	ctx := context.Background()
	styleOf := func(v *View, data string) plugin.ButtonStyle {
		for _, row := range v.Buttons {
			for _, b := range row {
				if b.Data == data {
					return b.Style
				}
			}
		}
		t.Fatalf("no button %s", data)
		return 0
	}
	g, _, _ := s.dispatch(ctx, "g:0", target{})
	for data, want := range map[string]plugin.ButtonStyle{
		"a:ia": plugin.StyleSuccess, "a:la": plugin.StylePrimary, "a:ra": plugin.StyleDanger,
		"o:weather": plugin.StyleSuccess, "o:broken": plugin.StyleDanger, "o:gt": plugin.StylePlain,
	} {
		if got := styleOf(g, data); got != want {
			t.Fatalf("%s style %d, want %d", data, got, want)
		}
	}
	c := s.confirmView("rm", "weather")
	if styleOf(c, "y:rm:weather") != plugin.StyleDanger || styleOf(c, "o:weather") != plugin.StylePlain {
		t.Fatalf("confirm colors: %+v", c.Buttons)
	}
}
