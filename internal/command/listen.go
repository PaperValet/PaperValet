package command

import (
	"context"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type listener struct {
	id     int
	plugin string
	fn     plugin.MessageListener
}

// AddListener registers a plugin message listener and returns its remover.
func (r *Registry) AddListener(pluginName string, fn plugin.MessageListener) func() {
	if fn == nil {
		return func() {}
	}
	r.mu.Lock()
	r.nextListener++
	id := r.nextListener
	r.listeners = append(r.listeners, listener{id: id, plugin: pluginName, fn: fn})
	r.mu.Unlock()
	return func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		for i, l := range r.listeners {
			if l.id == id {
				r.listeners = append(r.listeners[:i:i], r.listeners[i+1:]...)
				return
			}
		}
	}
}

// NotifyListeners hands msg to every plugin listener. A panicking listener
// is logged and skipped.
func (r *Registry) NotifyListeners(ctx context.Context, msg *interfaces.MessageEvent, edited bool) {
	r.mu.RLock()
	list := append([]listener(nil), r.listeners...)
	r.mu.RUnlock()
	for _, l := range list {
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					r.logger.Error("listener panic", "plugin", l.plugin, "panic", rec)
				}
			}()
			l.fn(ctx, msg, edited)
		}()
	}
}

// RunAsOwner parses msg.Text and runs the matching command with owner
// rights. It reports whether a command matched.
func (r *Registry) RunAsOwner(ctx context.Context, msg *interfaces.MessageEvent) (bool, error) {
	if msg == nil || msg.Message == nil {
		return false, interfaces.ErrNoMessage
	}
	name, args, ok := r.ParseCommand(msg.Text)
	if !ok {
		return false, nil
	}
	if _, exists := r.Get(name); !exists {
		return false, nil
	}
	ev := *msg
	r.mu.RLock()
	ev.UserID = r.ownerID
	r.mu.RUnlock()
	if ev.UserID == 0 {
		ev.UserID = r.getSelfID()
	}
	ev.IsOut = true
	return true, r.ExecuteCommand(ctx, &ev, name, args)
}
