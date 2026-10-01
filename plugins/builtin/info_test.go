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
