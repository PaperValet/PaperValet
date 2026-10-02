package plugin

import (
	"testing"

	"github.com/gotd/td/tg"
)

func TestChatIDs(t *testing.T) {
	cases := []struct {
		peer  tg.PeerClass
		input tg.InputPeerClass
		want  int64
	}{
		{&tg.PeerUser{UserID: 5}, &tg.InputPeerUser{UserID: 5}, 5},
		{&tg.PeerChat{ChatID: 9}, &tg.InputPeerChat{ChatID: 9}, -9},
		{&tg.PeerChannel{ChannelID: 77}, &tg.InputPeerChannel{ChannelID: 77}, -1000000000077},
	}
	for _, c := range cases {
		if got := ChatIDOf(c.peer); got != c.want {
			t.Errorf("ChatIDOf(%T) = %d", c.peer, got)
		}
		if got := ChatIDOfInput(c.input); got != c.want {
			t.Errorf("ChatIDOfInput(%T) = %d", c.input, got)
		}
	}
	if ChatIDOf(nil) != 0 || ChatIDOfInput(&tg.InputPeerSelf{}) != 0 {
		t.Fatal("unknown peers should be 0")
	}
}

func TestSenderAndEvent(t *testing.T) {
	if SenderID(nil) != 0 {
		t.Fatal("nil")
	}
	group := &tg.Message{ID: 4, Message: "hi", FromID: &tg.PeerUser{UserID: 3}, PeerID: &tg.PeerChannel{ChannelID: 1}}
	group.SetReplyTo(&tg.MessageReplyHeader{ReplyToMsgID: 2})
	ev := EventFromMessage(group)
	if ev.UserID != 3 || ev.ChatID != -1000000000001 || !ev.IsReply || ev.ReplyToID != 2 || ev.Text != "hi" {
		t.Fatalf("event = %+v", ev)
	}
	private := &tg.Message{PeerID: &tg.PeerUser{UserID: 8}}
	if SenderID(private) != 8 {
		t.Fatal("private sender")
	}
	post := &tg.Message{PeerID: &tg.PeerChannel{ChannelID: 1}}
	if SenderID(post) != 0 {
		t.Fatal("channel post has no user")
	}
	if EventFromMessage(nil) != nil {
		t.Fatal("nil message")
	}
}
