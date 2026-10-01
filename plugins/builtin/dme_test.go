package builtin

import (
	"bytes"
	"image/png"
	"testing"
	"time"

	"github.com/gotd/td/tg"
)

func TestIsMine(t *testing.T) {
	if !isMine(&tg.Message{Out: true}, 1) {
		t.Fatal("out message must be mine")
	}
	if !isMine(&tg.Message{FromID: &tg.PeerUser{UserID: 7}}, 7) {
		t.Fatal("from self must be mine")
	}
	if isMine(&tg.Message{FromID: &tg.PeerUser{UserID: 8}}, 7) {
		t.Fatal("other user is not mine")
	}
	if isMine(&tg.Message{FromID: &tg.PeerUser{UserID: 0}}, 0) {
		t.Fatal("unknown self id must not match zero sender")
	}
}

func TestBlankerEligible(t *testing.T) {
	b := &blanker{}
	now := int(time.Now().Unix())
	old := int(time.Now().Add(-72 * time.Hour).Unix())
	cases := []struct {
		name string
		m    *tg.Message
		want bool
	}{
		{"recent text", &tg.Message{Date: now, Message: "hi"}, true},
		{"old text", &tg.Message{Date: old, Message: "hi"}, false},
		{"already blank", &tg.Message{Date: now, Message: dmePlaceholder}, false},
		{"photo", &tg.Message{Date: now, Media: &tg.MessageMediaPhoto{}}, true},
		{"sticker", &tg.Message{Date: now, Media: &tg.MessageMediaDocument{Document: &tg.Document{
			Attributes: []tg.DocumentAttributeClass{&tg.DocumentAttributeSticker{}}}}}, false},
		{"round video", &tg.Message{Date: now, Media: &tg.MessageMediaDocument{Document: &tg.Document{
			Attributes: []tg.DocumentAttributeClass{&tg.DocumentAttributeVideo{RoundMessage: true}}}}}, false},
		{"file", &tg.Message{Date: now, Media: &tg.MessageMediaDocument{Document: &tg.Document{
			Attributes: []tg.DocumentAttributeClass{&tg.DocumentAttributeFilename{FileName: "a.zip"}}}}}, true},
		{"poll", &tg.Message{Date: now, Media: &tg.MessageMediaPoll{}}, false},
	}
	for _, c := range cases {
		if got := b.eligible(c.m); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestBlankPNGDecodes(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(blankPNG()))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 64 {
		t.Fatalf("size %v", img.Bounds())
	}
}
