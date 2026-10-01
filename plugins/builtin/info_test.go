package builtin

import (
	"testing"

	"github.com/gotd/td/tg"
)

func TestChatLink(t *testing.T) {
	cases := map[int64]string{
		-1003061608291: "https://t.me/c/3061608291",
		-4567:          "https://t.me/c/4567",
		7041948142:     "",
	}
	for id, want := range cases {
		if got := chatLink(id); got != want {
			t.Errorf("%d: got %q want %q", id, got, want)
		}
	}
}

func TestDisplayName(t *testing.T) {
	if got := displayName(&tg.User{ID: 1, FirstName: "A", LastName: "B", Username: "ab"}); got != "A B (@ab)" {
		t.Fatal(got)
	}
	if got := displayName(&tg.User{ID: 9}); got != "#9" {
		t.Fatal(got)
	}
}

func TestUsersOfChannelMessages(t *testing.T) {
	res := &tg.MessagesChannelMessages{Users: []tg.UserClass{&tg.User{ID: 5, FirstName: "x"}}}
	if u := findUserInChats(res, 5); u == nil || u.FirstName != "x" {
		t.Fatal("channel responses must expose their users")
	}
}

func TestUsernamesIncludeCollectibles(t *testing.T) {
	u := &tg.User{Username: "main", Usernames: []tg.Username{
		{Username: "main", Active: true},
		{Username: "nft", Active: true},
		{Username: "off", Active: false},
	}}
	got := usernamesOf(u)
	if len(got) != 2 || got[0] != "main" || got[1] != "nft" {
		t.Fatalf("got %v", got)
	}
}

func TestEstimateRegDate(t *testing.T) {
	if y := estimateRegDate(7041948142).Year(); y != 2024 {
		t.Fatalf("7041948142 should land in 2024, got %d", y)
	}
	if y := estimateRegDate(100).Year(); y != 2013 {
		t.Fatalf("tiny ids are 2013, got %d", y)
	}
	if y := estimateRegDate(9000000000).Year(); y < 2026 {
		t.Fatalf("ids past the table extrapolate forward, got %d", y)
	}
}
