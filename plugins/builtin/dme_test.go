package builtin

import (
	"bytes"
	"image/jpeg"
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

func TestParseDmeArgs(t *testing.T) {
	cases := []struct {
		args  []string
		count int
		force bool
		bad   bool
	}{
		{[]string{"5"}, 5, false, false},
		{[]string{"5", "-f"}, 5, true, false},
		{[]string{"-f", "5"}, 5, true, false},
		{[]string{"all"}, dmeAll, false, false},
		{[]string{"all", "-f"}, dmeAll, true, false},
		{nil, 0, false, true},
		{[]string{"-f"}, 0, false, true},
		{[]string{"0"}, 0, false, true},
		{[]string{"9999"}, 0, false, true},
		{[]string{"others", "on"}, 0, false, true},
	}
	for _, c := range cases {
		n, f, err := parseDmeArgs(c.args)
		if c.bad {
			if err == nil {
				t.Errorf("%v: want error", c.args)
			}
			continue
		}
		if err != nil || n != c.count || f != c.force {
			t.Errorf("%v: got %d %v %v", c.args, n, f, err)
		}
	}
}

func TestRewriterEligible(t *testing.T) {
	r := &rewriter{notice: dmeNoticeZH}
	now := int(time.Now().Unix())
	old := int(time.Now().Add(-72 * time.Hour).Unix())
	doc := func(a ...tg.DocumentAttributeClass) *tg.MessageMediaDocument {
		return &tg.MessageMediaDocument{Document: &tg.Document{Attributes: a}}
	}
	cases := []struct {
		name string
		m    *tg.Message
		want bool
	}{
		{"recent text", &tg.Message{Date: now, Message: "hi"}, true},
		{"old text", &tg.Message{Date: old, Message: "hi"}, false},
		{"already rewritten", &tg.Message{Date: now, Message: dmeNoticeZH}, false},
		{"photo", &tg.Message{Date: now, Media: &tg.MessageMediaPhoto{}}, true},
		{"sticker", &tg.Message{Date: now, Media: doc(&tg.DocumentAttributeSticker{})}, false},
		{"voice", &tg.Message{Date: now, Media: doc(&tg.DocumentAttributeAudio{Voice: true})}, false},
		{"music", &tg.Message{Date: now, Media: doc(&tg.DocumentAttributeAudio{})}, true},
		{"round video", &tg.Message{Date: now, Media: doc(&tg.DocumentAttributeVideo{RoundMessage: true})}, false},
		{"file", &tg.Message{Date: now, Media: doc(&tg.DocumentAttributeFilename{FileName: "a.zip"})}, true},
		{"poll", &tg.Message{Date: now, Media: &tg.MessageMediaPoll{}}, false},
	}
	for _, c := range cases {
		if got := r.eligible(c.m); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestLogoEmbedded(t *testing.T) {
	img, err := jpeg.Decode(bytes.NewReader(logoJPG))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() < 100 {
		t.Fatalf("logo too small: %v", img.Bounds())
	}
}
