package command

import (
	"context"
	"testing"

	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
)

func TestListenersAddRemoveAndUnload(t *testing.T) {
	r := NewRegistry(nil, nil, nil, nil, 1, nil)
	var a, b, edits int
	removeA := r.Host().Listen("a", func(_ context.Context, _ *interfaces.MessageEvent, edited bool) {
		a++
		if edited {
			edits++
		}
	})
	r.Host().Listen("b", func(context.Context, *interfaces.MessageEvent, bool) { b++ })
	r.Host().Listen("p", func(context.Context, *interfaces.MessageEvent, bool) { panic("boom") })

	msg := &interfaces.MessageEvent{Message: &tg.Message{ID: 1}}
	r.NotifyListeners(context.Background(), msg, false)
	r.NotifyListeners(context.Background(), msg, true)
	if a != 2 || b != 2 || edits != 1 {
		t.Fatalf("a=%d b=%d edits=%d", a, b, edits)
	}
	removeA()
	removeA() // idempotent
	r.UnregisterPlugin("b")
	r.NotifyListeners(context.Background(), msg, false)
	if a != 2 || b != 2 {
		t.Fatalf("after removal a=%d b=%d", a, b)
	}
}

func TestRunAsOwner(t *testing.T) {
	r := NewRegistry([]string{"."}, nil, nil, nil, 42, nil)
	var got *interfaces.CommandContext
	if err := r.Register(&interfaces.Command{Name: "echo", OwnerOnly: true, Handler: func(c *interfaces.CommandContext) error {
		got = c
		return nil
	}}); err != nil {
		t.Fatal(err)
	}
	ev := &interfaces.MessageEvent{Text: ".echo hi there", UserID: 7, ChatID: -5, Message: &tg.Message{ID: 3}}
	ok, err := r.RunAsOwner(context.Background(), ev)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if got == nil || got.Message.UserID != 42 || got.RawArgs != "hi there" || got.Message.ChatID != -5 {
		t.Fatalf("ctx = %+v", got)
	}
	if ev.UserID != 7 {
		t.Fatal("caller event mutated")
	}
	for _, text := range []string{"hello", ".nope", "."} {
		ok, err := r.RunAsOwner(context.Background(), &interfaces.MessageEvent{Text: text, Message: &tg.Message{}})
		if ok || err != nil {
			t.Errorf("%q: ok=%v err=%v", text, ok, err)
		}
	}
	if _, err := r.RunAsOwner(context.Background(), nil); err == nil {
		t.Fatal("nil event accepted")
	}
	if p := r.Host().Prefixes(); len(p) != 1 || p[0] != "." {
		t.Fatalf("prefixes = %v", p)
	}
}
