package bot

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type fakeLogger struct {
	errors []string
}

func (l *fakeLogger) Debug(msg string, kv ...any) {}
func (l *fakeLogger) Info(msg string, kv ...any)  {}
func (l *fakeLogger) Warn(msg string, kv ...any)  {}
func (l *fakeLogger) Error(msg string, kv ...any) { l.errors = append(l.errors, msg) }
func (l *fakeLogger) Named(string) plugin.Logger  { return l }
func (l *fakeLogger) With(...any) plugin.Logger   { return l }

func TestSelfErrorHook(t *testing.T) {
	l := &fakeLogger{}
	hook := SelfErrorHook(l)

	// Authorization errors are routine pre-login and must not drop the conn.
	if err := hook(context.Background(), tgerr.New(401, "AUTH_KEY_UNREGISTERED")); err != nil {
		t.Fatalf("401 error must pass through, got %v", err)
	}
	if len(l.errors) != 0 {
		t.Fatalf("401 error must not be logged, got %v", l.errors)
	}

	// Any other failure drops the connection so gotd resubscribes.
	boom := errors.New("connection reset")
	if err := hook(context.Background(), boom); !errors.Is(err, boom) {
		t.Fatalf("want the original error back, got %v", err)
	}
	if len(l.errors) != 1 {
		t.Fatalf("transient error must be logged once, got %v", l.errors)
	}
}

type fakeStateAPI struct {
	failures int
	calls    int
}

func (f *fakeStateAPI) UpdatesGetState(ctx context.Context) (*tg.UpdatesState, error) {
	f.calls++
	if f.calls <= f.failures {
		return nil, errors.New("flood")
	}
	return &tg.UpdatesState{}, nil
}

func TestSubscribeUpdatesRetriesUntilSuccess(t *testing.T) {
	api := &fakeStateAPI{failures: 2}
	var slept []time.Duration
	done := make(chan struct{})
	go func() {
		subscribeUpdates(context.Background(), api, &fakeLogger{}, func(d time.Duration) { slept = append(slept, d) })
		close(done)
	}()
	<-done
	if api.calls != 3 {
		t.Fatalf("want 3 calls, got %d", api.calls)
	}
	if len(slept) != 2 {
		t.Fatalf("want 2 sleeps, got %d", len(slept))
	}
}

func TestSubscribeUpdatesStopsOnCanceledContext(t *testing.T) {
	api := &fakeStateAPI{failures: 100}
	ctx, cancel := context.WithCancel(context.Background())
	var slept int
	done := make(chan struct{})
	go func() {
		subscribeUpdates(ctx, api, &fakeLogger{}, func(time.Duration) {
			slept++
			cancel()
		})
		close(done)
	}()
	<-done
	if slept != 1 {
		t.Fatalf("want exactly 1 sleep before noticing cancellation, got %d", slept)
	}
}

func TestMarkReadyIsIdempotent(t *testing.T) {
	s := &Service{readyCh: make(chan struct{})}
	s.markReady("first")
	s.markReady("second")
	select {
	case <-s.readyCh:
	default:
		t.Fatal("readyCh must be closed")
	}
	s.mu.RLock()
	name := s.username
	ready := s.ready
	s.mu.RUnlock()
	if !ready || name != "second" {
		t.Fatalf("want ready bot named second, got %v %q", ready, name)
	}
}
