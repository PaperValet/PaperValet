package peer

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/gotd/td/tg"
)

func TestStoreSurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "peers.json")
	m := NewAccessHashManager(nil, NewStore(path))
	chatID := ChannelChatID(3061608291)
	m.RegisterPeer(chatID, 42, "channel")

	// Put persists asynchronously.
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, _, ok := NewStore(path).Get(chatID); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("peer was not persisted")
		}
		time.Sleep(10 * time.Millisecond)
	}

	fresh := NewAccessHashManager(nil, NewStore(path))
	got, err := fresh.GetInputPeer(context.Background(), chatID)
	if err != nil {
		t.Fatal(err)
	}
	ch, ok := got.(*tg.InputPeerChannel)
	if !ok || ch.ChannelID != 3061608291 || ch.AccessHash != 42 {
		t.Fatalf("got %#v", got)
	}
}

func TestStoreIgnoresZeroHash(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "peers.json"))
	s.Put(1, 0, "user")
	if _, _, ok := s.Get(1); ok {
		t.Fatal("zero access hash must not be stored")
	}
}

func TestFallbackWithoutHash(t *testing.T) {
	m := NewAccessHashManager(nil, NewStore(filepath.Join(t.TempDir(), "p.json")))
	got, _ := m.GetInputPeer(context.Background(), ChannelChatID(1234))
	if ch, ok := got.(*tg.InputPeerChannel); !ok || ch.ChannelID != 1234 {
		t.Fatalf("got %#v", got)
	}
}
