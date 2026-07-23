package command

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gotd/td/tg"
	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/logger"
)

// Registry manages command registration, lookup, and middleware execution.
// Supports multi-prefix command parsing.
type Registry struct {
	mu         sync.RWMutex
	commands   map[string]*interfaces.Command
	aliases    map[string]string
	globalMW   []interfaces.Middleware
	emitter    interfaces.Emitter
	resolver   interfaces.PeerResolver
	api        *tg.Client
	ownerID    int64
	prefixes   []string // all supported command prefixes (main first)
	rateLimits map[string]time.Time
	logger     interfaces.Logger
}

func NewRegistry(prefixes []string, emitter interfaces.Emitter, api *tg.Client, resolver interfaces.PeerResolver, ownerID int64) *Registry {
	if len(prefixes) == 0 {
		prefixes = []string{"."}
	}
	r := &Registry{
		commands:   make(map[string]*interfaces.Command),
		aliases:    make(map[string]string),
		globalMW:   make([]interfaces.Middleware, 0),
		emitter:    emitter,
		resolver:   resolver,
		api:        api,
		ownerID:    ownerID,
		prefixes:   prefixes,
		rateLimits: make(map[string]time.Time),
		logger:     logger.NamedLogger("command"),
	}
	r.Use(r.recoveryMiddleware)
	r.Use(r.loggingMiddleware)
	r.Use(r.rateLimitMiddleware)
	go r.rateLimitCleanup()
	return r
}

// SetOwnerID updates the owner ID after initial resolution.
func (r *Registry) SetOwnerID(id int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ownerID = id
}

func (r *Registry) Use(mw interfaces.Middleware) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.globalMW = append(r.globalMW, mw)
}

func (r *Registry) Register(cmd *interfaces.Command) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if cmd.Name == "" {
		return fmt.Errorf("command name is required")
	}
	if _, exists := r.commands[cmd.Name]; exists {
		return fmt.Errorf("command %s already registered", cmd.Name)
	}
	r.commands[cmd.Name] = cmd
	for _, alias := range cmd.Aliases {
		r.aliases[alias] = cmd.Name
	}
	r.logger.Debug("registered", "name", cmd.Name, "plugin", cmd.Plugin)
	return nil
}

func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if cmd, ok := r.commands[name]; ok {
		for _, a := range cmd.Aliases {
			delete(r.aliases, a)
		}
		delete(r.commands, name)
	}
}

func (r *Registry) UnregisterPlugin(plugin string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for name, cmd := range r.commands {
		if cmd.Plugin == plugin {
			for _, a := range cmd.Aliases {
				delete(r.aliases, a)
			}
			delete(r.commands, name)
		}
	}
}

func (r *Registry) Get(name string) (*interfaces.Command, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if cmd, ok := r.commands[name]; ok {
		return cmd, true
	}
	if canonical, ok := r.aliases[name]; ok {
		cmd, ok := r.commands[canonical]
		return cmd, ok
	}
	return nil, false
}

func (r *Registry) GetAll() map[string]*interfaces.Command {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]*interfaces.Command, len(r.commands))
	for k, v := range r.commands {
		out[k] = v
	}
	return out
}

func (r *Registry) GetByPlugin(plugin string) map[string]*interfaces.Command {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]*interfaces.Command)
	for name, cmd := range r.commands {
		if cmd.Plugin == plugin {
			out[name] = cmd
		}
	}
	return out
}

func (r *Registry) GetPrefix() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.prefixes[0]
}

func (r *Registry) GetPrefixes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, len(r.prefixes))
	copy(out, r.prefixes)
	return out
}

func (r *Registry) SetPrefix(prefix string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prefixes = []string{prefix}
}

// SetPrefixes replaces all prefixes.
func (r *Registry) SetPrefixes(prefixes []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(prefixes) == 0 {
		prefixes = []string{"."}
	}
	r.prefixes = prefixes
}

// AddPrefix adds a prefix without removing existing ones.
func (r *Registry) AddPrefix(prefix string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.prefixes {
		if p == prefix {
			return
		}
	}
	r.prefixes = append(r.prefixes, prefix)
}

// RemovePrefix removes a prefix (keeps at least one).
func (r *Registry) RemovePrefix(prefix string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.prefixes) <= 1 {
		return false
	}
	for i, p := range r.prefixes {
		if p == prefix {
			r.prefixes = append(r.prefixes[:i], r.prefixes[i+1:]...)
			return true
		}
	}
	return false
}

// IsCommand checks if text starts with any registered prefix.
func (r *Registry) IsCommand(text string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, p := range r.prefixes {
		if strings.HasPrefix(text, p) {
			return true
		}
	}
	return false
}

// ParseCommand parses a command from text, trying all registered prefixes.
// Returns the first match across all prefixes.
func (r *Registry) ParseCommand(text string) (name string, args []string, ok bool) {
	r.mu.RLock()
	prefixes := r.prefixes
	r.mu.RUnlock()

	for _, p := range prefixes {
		if !strings.HasPrefix(text, p) {
			continue
		}
		trimmed := strings.TrimPrefix(text, p)
		parts := strings.Fields(trimmed)
		if len(parts) == 0 {
			return "", nil, false
		}
		return parts[0], parts[1:], true
	}
	return "", nil, false
}

func (r *Registry) ExecuteCommand(ctx context.Context, msg *interfaces.MessageEvent, name string, args []string) error {
	cmd, exists := r.Get(name)
	if !exists {
		return nil
	}
	if cmd.OwnerOnly && r.ownerID != 0 && msg.UserID != r.ownerID {
		return nil
	}

	cmdCtx := &interfaces.CommandContext{
		Command:      name,
		Args:         args,
		RawArgs:      strings.Join(args, " "),
		Message:      msg,
		Session:      interfaces.NewSessionContext(&interfaces.Session{UserID: msg.UserID, ChatID: msg.ChatID}, ctx),
		API:          r.api,
		PeerResolver: r.resolver,
		Emitter:      r.emitter,
		PluginName:   cmd.Plugin,
		StartTime:    time.Now(),
		Metadata:     make(map[string]any),
		Ctx:          ctx,
		Logger:       r.logger,
	}

	handler := cmd.Handler
	r.mu.RLock()
	mws := append([]interfaces.Middleware(nil), r.globalMW...)
	r.mu.RUnlock()
	for i := len(mws) - 1; i >= 0; i-- {
		handler = mws[i](handler)
	}
	return handler(cmdCtx)
}

func (r *Registry) loggingMiddleware(next interfaces.Handler) interfaces.Handler {
	return func(ctx *interfaces.CommandContext) error {
		r.logger.Info("exec",
			"cmd", ctx.Command,
			"args", ctx.Args,
			"user", ctx.Message.UserID,
			"chat", ctx.Message.ChatID,
		)
		err := next(ctx)
		if err != nil {
			r.logger.Error("failed", "cmd", ctx.Command, "error", err)
		}
		return err
	}
}

func (r *Registry) recoveryMiddleware(next interfaces.Handler) interfaces.Handler {
	return func(ctx *interfaces.CommandContext) (err error) {
		defer func() {
			if rec := recover(); rec != nil {
				err = fmt.Errorf("panic: %v", rec)
				r.logger.Error("panic recovered", "cmd", ctx.Command, "panic", rec)
			}
		}()
		return next(ctx)
	}
}

func (r *Registry) rateLimitMiddleware(next interfaces.Handler) interfaces.Handler {
	return func(ctx *interfaces.CommandContext) error {
		cmd, ok := r.Get(ctx.Command)
		if !ok || cmd.RateLimit <= 0 {
			return next(ctx)
		}
		key := fmt.Sprintf("%d:%s", ctx.Message.UserID, ctx.Command)
		r.mu.Lock()
		if last, exists := r.rateLimits[key]; exists && time.Since(last) < time.Duration(cmd.RateLimit)*time.Second {
			r.mu.Unlock()
			return nil
		}
		r.rateLimits[key] = time.Now()
		r.mu.Unlock()
		return next(ctx)
	}
}

// rateLimitCleanup periodically removes stale rate limit entries.
func (r *Registry) rateLimitCleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		r.mu.Lock()
		now := time.Now()
		for key, last := range r.rateLimits {
			if now.Sub(last) > 10*time.Minute {
				delete(r.rateLimits, key)
			}
		}
		r.mu.Unlock()
	}
}