package app

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type fakeLogger struct {
	mu     sync.Mutex
	errors []string
}

func (l *fakeLogger) Debug(msg string, kv ...any) {}
func (l *fakeLogger) Info(msg string, kv ...any)  {}
func (l *fakeLogger) Warn(msg string, kv ...any)  {}
func (l *fakeLogger) Error(msg string, kv ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errors = append(l.errors, msg)
}
func (l *fakeLogger) Named(string) plugin.Logger { return l }
func (l *fakeLogger) With(...any) plugin.Logger  { return l }

func (l *fakeLogger) errorCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.errors)
}

func TestRetryWhileAliveRestartsUntilContextDies(t *testing.T) {
	l := &fakeLogger{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var runs int
	sleeps := make(chan time.Duration, 10)
	sleepCalls := 0
	go func() {
		retryWhileAlive(ctx, l, func(d time.Duration) {
			sleepCalls++
			sleeps <- d
			if sleepCalls == 3 {
				cancel()
			}
		}, func(context.Context) error {
			runs++
			return errors.New("bot login failed")
		})
	}()
	// Three failures, the third one stops the world; no fourth run happens.
	for i := 0; i < 3; i++ {
		select {
		case <-sleeps:
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for retry sleep")
		}
	}
	time.Sleep(200 * time.Millisecond)
	if runs != 3 {
		t.Fatalf("want 3 runs total, got %d", runs)
	}
	if l.errorCount() != 3 {
		t.Fatalf("want 3 logged failures, got %d", l.errorCount())
	}
}

func TestRetryWhileAliveSwallowsContextCanceled(t *testing.T) {
	l := &fakeLogger{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() {
		retryWhileAlive(ctx, l, func(time.Duration) {}, func(context.Context) error {
			return context.Canceled
		})
		close(done)
	}()
	<-done
	if l.errorCount() != 0 {
		t.Fatalf("context.Canceled must not be logged as a failure, got %d", l.errorCount())
	}
}

func TestRetryWhileAliveExitsOnCleanShutdown(t *testing.T) {
	l := &fakeLogger{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() {
		retryWhileAlive(ctx, l, func(time.Duration) {}, func(context.Context) error {
			return nil
		})
		close(done)
	}()
	<-done
	// A cancelled context must stop the loop even when Run reports success.
	if l.errorCount() != 0 {
		t.Fatalf("clean shutdown must not log, got %d", l.errorCount())
	}
}
