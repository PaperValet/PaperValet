package builtin

import (
	"testing"

	"github.com/gotd/td/tg"
)

func TestInputMediaOf(t *testing.T) {
	photo := inputMediaOf(&tg.MessageMediaPhoto{Photo: &tg.Photo{ID: 1, AccessHash: 2, FileReference: []byte{3}}, Spoiler: true})
	if p, ok := photo.(*tg.InputMediaPhoto); !ok || !p.Spoiler {
		t.Fatalf("photo: %#v", photo)
	}
	doc := inputMediaOf(&tg.MessageMediaDocument{Document: &tg.Document{ID: 4, AccessHash: 5}})
	if _, ok := doc.(*tg.InputMediaDocument); !ok {
		t.Fatalf("document: %#v", doc)
	}
	if inputMediaOf(&tg.MessageMediaWebPage{}) != nil {
		t.Fatal("web previews are not resendable media")
	}
	if inputMediaOf(nil) != nil {
		t.Fatal("nil media")
	}
	if _, ok := inputMediaOf(&tg.MessageMediaDice{Emoticon: "🎲"}).(*tg.InputMediaDice); !ok {
		t.Fatal("dice")
	}
}
