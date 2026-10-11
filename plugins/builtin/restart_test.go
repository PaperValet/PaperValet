package builtin

import (
	"context"
	"errors"
	"testing"

	"github.com/gotd/td/tg"
)

type fakeRestartAPI struct {
	editErr error
	edited  *tg.MessagesEditMessageRequest
	sent    *tg.MessagesSendMessageRequest
}

func (f *fakeRestartAPI) MessagesEditMessage(ctx context.Context, r *tg.MessagesEditMessageRequest) (tg.UpdatesClass, error) {
	f.edited = r
	if f.editErr != nil {
		return nil, f.editErr
	}
	return &tg.Updates{}, nil
}

func (f *fakeRestartAPI) MessagesSendMessage(ctx context.Context, r *tg.MessagesSendMessageRequest) (tg.UpdatesClass, error) {
	f.sent = r
	return &tg.Updates{}, nil
}

func TestNotifyRestartDoneEdits(t *testing.T) {
	api := &fakeRestartAPI{}
	notifyRestartDone(context.Background(), api, &tg.InputPeerSelf{}, 42, "✅ done")
	if api.edited == nil || api.edited.ID != 42 || api.edited.Message != "✅ done" {
		t.Fatalf("want the message edited in place, got %+v", api.edited)
	}
	if api.sent != nil {
		t.Fatalf("edit succeeded, no new message expected")
	}
}

func TestNotifyRestartDoneFallsBackToSend(t *testing.T) {
	api := &fakeRestartAPI{editErr: errors.New("MESSAGE_ID_INVALID")}
	notifyRestartDone(context.Background(), api, &tg.InputPeerSelf{}, 42, "✅ done")
	if api.edited == nil {
		t.Fatalf("edit must be attempted first")
	}
	if api.sent == nil || api.sent.Message != "✅ done" {
		t.Fatalf("failed edit must fall back to sending the notice, got %+v", api.sent)
	}
}
