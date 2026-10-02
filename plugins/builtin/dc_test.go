package builtin

import (
	"strings"
	"testing"

	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
)

func TestDCTargets(t *testing.T) {
	ctx := &interfaces.CommandContext{Lang: "en-US"}
	u := userTarget(ctx, &tg.User{ID: 1, FirstName: "A_b", Photo: &tg.UserProfilePhoto{DCID: 5}})
	if u.dc != 5 || u.name != "A_b" || u.kind != "User" {
		t.Fatalf("user = %+v", u)
	}
	if b := userTarget(ctx, &tg.User{ID: 9, Bot: true}); b.dc != 0 || b.name != "9" || b.kind != "Bot" {
		t.Fatalf("bot = %+v", b)
	}
	ch := chatTarget(ctx, &tg.Channel{Title: "News", Broadcast: true, Photo: &tg.ChatPhoto{DCID: 2}})
	if ch.dc != 2 || ch.kind != "Channel" {
		t.Fatalf("channel = %+v", ch)
	}
	if g := chatTarget(ctx, &tg.Chat{Title: "G"}); g.dc != 0 || g.kind != "Group" {
		t.Fatalf("group = %+v", g)
	}
	if chatTarget(ctx, &tg.ChatEmpty{}) != nil {
		t.Fatal("empty chat")
	}
}

func TestDCCard(t *testing.T) {
	ctx := &interfaces.CommandContext{Lang: "zh-CN"}
	out := dcCard(ctx, &dcTarget{name: "x*y", dc: 4, kind: "用户"})
	if !strings.Contains(out, "DC4 · Amsterdam") || !strings.Contains(out, `x\*y`) {
		t.Fatal(out)
	}
	if out := dcCard(ctx, &dcTarget{name: "n", kind: "群组"}); !strings.Contains(out, "没有头像") {
		t.Fatal(out)
	}
	for dc, want := range map[int]string{1: "Miami", 3: "Miami", 2: "Amsterdam", 5: "Singapore", 9: "?"} {
		if dcLocation(dc) != want {
			t.Errorf("dc%d", dc)
		}
	}
}
