package command

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/internal/i18n"
	"github.com/TiaraBasori/PaperValet/internal/interfaces"
)

func TestHostDataDir(t *testing.T) {
	t.Chdir(t.TempDir())
	h := NewRegistry(nil, nil, nil, nil, 0, nil).Host()

	dir, err := h.DataDir("sendat")
	if err != nil {
		t.Fatal(err)
	}
	if dir != filepath.Join("data", "sendat") {
		t.Fatalf("dir = %q", dir)
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Fatalf("not created: %v", err)
	}
	for _, bad := range []string{"", "..", "../x", "a/b", "a b"} {
		if _, err := h.DataDir(bad); err == nil {
			t.Errorf("DataDir(%q) accepted", bad)
		}
	}
}

func TestHostIdentityAndLang(t *testing.T) {
	m := i18n.NewManager(i18n.CoreCatalog())
	r := NewRegistry(nil, nil, nil, nil, 42, m)
	h := r.Host()
	if h.SelfID() != 42 {
		t.Fatalf("SelfID = %d", h.SelfID())
	}
	r.SetSelfID(7)
	if h.SelfID() != 7 {
		t.Fatalf("SelfID after login = %d", h.SelfID())
	}
	m.SetUserLang(7, i18n.Lang("en-US"))
	if h.Lang(7) != "en-US" {
		t.Fatalf("Lang = %q", h.Lang(7))
	}
	if NewRegistry(nil, nil, nil, nil, 0, nil).Host().Lang(1) != "zh-CN" {
		t.Fatal("nil i18n should default to zh-CN")
	}
	if h.Logger("x") == nil {
		t.Fatal("nil logger")
	}
}

type fakeMedia struct{}

func (fakeMedia) SendFile(context.Context, int64, string, string, int) error { return nil }
func (fakeMedia) DownloadMedia(context.Context, *interfaces.MessageEvent) (string, error) {
	return "", nil
}

func TestHostMediaFollowsRegistry(t *testing.T) {
	r := NewRegistry(nil, nil, nil, nil, 0, nil)
	h := r.Host()
	if h.Media() != nil || h.Downloader() != nil {
		t.Fatal("media should start nil")
	}
	r.SetMediaSender(fakeMedia{})
	if h.Media() == nil || h.Downloader() == nil {
		t.Fatal("host did not pick up media sender")
	}
}

func TestHostSendWithoutClient(t *testing.T) {
	h := NewRegistry(nil, nil, nil, nil, 0, nil).Host()
	if _, err := h.Send(context.Background(), 1, "hi", 0); !errors.Is(err, interfaces.ErrNoMessage) {
		t.Fatalf("err = %v", err)
	}
}

func TestSentMessageID(t *testing.T) {
	cases := []struct {
		in   tg.UpdatesClass
		want int
	}{
		{&tg.UpdateShortSentMessage{ID: 5}, 5},
		{&tg.Updates{Updates: []tg.UpdateClass{&tg.UpdateMessageID{ID: 9}}}, 9},
		{&tg.Updates{Updates: []tg.UpdateClass{&tg.UpdateNewChannelMessage{Message: &tg.Message{ID: 11}}}}, 11},
		{&tg.UpdatesCombined{Updates: []tg.UpdateClass{&tg.UpdateNewMessage{Message: &tg.Message{ID: 3}}}}, 3},
		{&tg.UpdatesTooLong{}, 0},
	}
	for _, c := range cases {
		if got := sentMessageID(c.in); got != c.want {
			t.Errorf("%T: got %d want %d", c.in, got, c.want)
		}
	}
}
