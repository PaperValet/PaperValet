package eventbus

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/TiaraBasori/PaperValet/pkg/logger"
)

const (
	EventMessage   = "message"
	EventCommand   = "command"
	EventRawUpdate = "raw_update"
	EventShutdown  = "shutdown"
	EventStart     = "start"
	EventError     = "error"
)

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
	logger      *zap.Logger
}

func New() *Bus {
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
		if err := sub.handler(ctx, event); err != nil {
			b.logger.Error("handler failed", zap.String("type", event.Type), zap.Error(err))
		}
	}
	return nil
}

func (b *Bus) EmitAsync(ctx context.Context, eventType string, data any) {
	go func() {
		_ = b.Emit(ctx, eventType, data)
	}()
}

func (b *Bus) Shutdown(ctx context.Context) error {
	b.once.Do(func() {
		close(b.shutdownCh)
		b.mu.Lock()
		b.subscribers = make(map[string][]*Subscription)
		b.mu.Unlock()
	})
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
