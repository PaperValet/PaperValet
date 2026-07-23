package eventbus

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
)

const (
	EventMessage   = "message"
	EventCommand   = "command"
	EventRawUpdate = "raw_update"
	EventShutdown  = "shutdown"
	EventStart     = "start"
	EventError     = "error"
)

// ErrHandlerTimeout is returned when a handler exceeds its execution deadline.
var ErrHandlerTimeout = errors.New("eventbus: handler timed out")

type Event struct {
	Type      string
	Timestamp time.Time
	Data      any
	Context   context.Context
	Metadata  map[string]any
}

type Handler func(ctx context.Context, event *Event) error

type Subscription struct {
	id       int
	handler  Handler
	priority int
	filter   func(*Event) bool
}

type Bus struct {
	mu          sync.RWMutex
	nextID      int
	subscribers map[string][]*Subscription
	shutdownCh  chan struct{}
	once        sync.Once
	wg          sync.WaitGroup // tracks active async emissions
	logger      interfaces.Logger
}

func New(logger interfaces.Logger) *Bus {
	if logger == nil {
		logger = &noopLogger{}
	}
	return &Bus{
		subscribers: make(map[string][]*Subscription),
		shutdownCh:  make(chan struct{}),
		logger:      logger.Named("eventbus"),
	}
}

type Option func(*Subscription)

func WithPriority(p int) Option {
	return func(s *Subscription) { s.priority = p }
}

func WithFilter(f func(*Event) bool) Option {
	return func(s *Subscription) { s.filter = f }
}

func (b *Bus) Subscribe(eventType string, handler Handler, opts ...Option) *Subscription {
	s := &Subscription{handler: handler}
	for _, opt := range opts {
		opt(s)
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	s.id = b.nextID
	list := append(b.subscribers[eventType], s)
	sortByPriority(list)
	b.subscribers[eventType] = list
	return s
}

func (b *Bus) Unsubscribe(sub *Subscription) {
	if sub == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for et, list := range b.subscribers {
		for i, s := range list {
			if s.id == sub.id {
				b.subscribers[et] = append(list[:i], list[i+1:]...)
				return
			}
		}
	}
}

func (b *Bus) Emit(ctx context.Context, eventType string, data any) error {
	event := &Event{
		Type:      eventType,
		Timestamp: time.Now(),
		Data:      data,
		Context:   ctx,
	}
	return b.emitEvent(ctx, event)
}

func (b *Bus) EmitEvent(ctx context.Context, event *Event) error {
	if event.Context == nil {
		event.Context = ctx
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	return b.emitEvent(ctx, event)
}

func (b *Bus) emitEvent(ctx context.Context, event *Event) error {
	b.mu.RLock()
	subs := append([]*Subscription(nil), b.subscribers[event.Type]...)
	b.mu.RUnlock()

	for _, sub := range subs {
		// Respect shutdown and caller context before each handler
		select {
		case <-b.shutdownCh:
			return context.Canceled
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if sub.filter != nil && !sub.filter(event) {
			continue
		}

		// Run each handler with a per-handler timeout (30s) derived from the parent ctx
		handlerCtx, cancel := context.WithTimeoutCause(ctx, 30*time.Second, ErrHandlerTimeout)
		err := sub.handler(handlerCtx, event)
		cancel()
		if err != nil {
			b.logger.Error("handler failed", "type", event.Type, "error", err)
		}
	}
	return nil
}

func (b *Bus) EmitAsync(ctx context.Context, eventType string, data any) {
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		defer func() {
			if rec := recover(); rec != nil {
				b.logger.Error("async emit panic", "type", eventType, "panic", rec)
			}
		}()
		_ = b.Emit(ctx, eventType, data)
	}()
}

func (b *Bus) Shutdown(ctx context.Context) error {
	b.once.Do(func() {
		close(b.shutdownCh)
	})

	// Wait for all async emissions to complete (with timeout)
	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}

	b.mu.Lock()
	b.subscribers = make(map[string][]*Subscription)
	b.mu.Unlock()
	return nil
}

func (b *Bus) WaitForShutdown() <-chan struct{} {
	return b.shutdownCh
}

func sortByPriority(subs []*Subscription) {
	for i := 1; i < len(subs); i++ {
		key := subs[i]
		j := i - 1
		for j >= 0 && subs[j].priority < key.priority {
			subs[j+1] = subs[j]
			j--
		}
		subs[j+1] = key
	}
}

type noopLogger struct{}

func (n *noopLogger) Debug(msg string, keysAndValues ...any) {}
func (n *noopLogger) Info(msg string, keysAndValues ...any)  {}
func (n *noopLogger) Warn(msg string, keysAndValues ...any)  {}
func (n *noopLogger) Error(msg string, keysAndValues ...any) {}
func (n *noopLogger) With(keysAndValues ...any) interfaces.Logger { return n }
func (n *noopLogger) Named(name string) interfaces.Logger      { return n }