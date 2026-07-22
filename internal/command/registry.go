package command

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gotd/td/tg"
	"go.uber.org/zap"

	"github.com/TiaraBasori/PaperValet/internal/core"
	"github.com/TiaraBasori/PaperValet/pkg/logger"
)

type Handler func(ctx *core.CommandContext) error

type Middleware func(next Handler) Handler

type Command struct {
	Name        string
	Aliases     []string
	Description string
	Usage       string
	Plugin      string
	Category    string
	OwnerOnly   bool
	Hidden      bool
	RateLimit   int
	Handler     Handler
}

type Registry struct {
	mu         sync.RWMutex
	commands   map[string]*Command
	aliases    map[string]string
	globalMW   []Middleware
	emitter    core.Emitter
	resolver   core.PeerResolver
	api        *tg.Client
	ownerID    int64
	prefix     string
	rateLimits map[string]time.Time
	logger     *zap.Logger
}

func NewRegistry(prefix string, emitter core.Emitter, api *tg.Client, resolver core.PeerResolver, ownerID int64) *Registry {
	r := &Registry{
		commands:   make(map[string]*Command),
		aliases:    make(map[string]string),
		globalMW:   make([]Middleware, 0),
		emitter:    emitter,
		resolver:   resolver,
		api:        api,
		ownerID:    ownerID,
		prefix:     prefix,
		rateLimits: make(map[string]time.Time),
		logger:     logger.Named("command"),
	}
	r.Use(r.recoveryMiddleware)
	r.Use(r.loggingMiddleware)
	r.Use(r.rateLimitMiddleware)
	return r
}

func (r *Registry) Use(mw Middleware) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.globalMW = append(r.globalMW, mw)
}

func (r *Registry) Register(cmd *Command) error {
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
	r.logger.Debug("registered", zap.String("name", cmd.Name), zap.String("plugin", cmd.Plugin))
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

func (r *Registry) Get(name string) (*Command, bool) {
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

func (r *Registry) GetAll() map[string]*Command {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]*Command, len(r.commands))
	for k, v := range r.commands {
		out[k] = v
	}
	return out
}

func (r *Registry) GetByPlugin(plugin string) map[string]*Command {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]*Command)
	for name, cmd := range r.commands {
		if cmd.Plugin == plugin {
			out[name] = cmd
		}
	}
	return out
}

func (r *Registry) GetPrefix() string { return r.prefix }

func (r *Registry) SetPrefix(prefix string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prefix = prefix
}

func (r *Registry) IsCommand(text string) bool {
	return strings.HasPrefix(text, r.prefix)
}

func (r *Registry) ParseCommand(text string) (name string, args []string, ok bool) {
	if !strings.HasPrefix(text, r.prefix) {
		return "", nil, false
	}
	parts := strings.Fields(strings.TrimPrefix(text, r.prefix))
	if len(parts) == 0 {
		return "", nil, false
	}
	return parts[0], parts[1:], true
}

func (r *Registry) ExecuteCommand(ctx context.Context, msg *core.MessageEvent, name string, args []string) error {
	cmd, exists := r.Get(name)
	if !exists {
		return nil
	}
	if cmd.OwnerOnly && r.ownerID != 0 && msg.UserID != r.ownerID {
		return nil
	}

	cmdCtx := &core.CommandContext{
		Command:      name,
		Args:         args,
		RawArgs:      strings.Join(args, " "),
		Message:      msg,
		Session:      core.NewSessionContext(&core.Session{UserID: msg.UserID, ChatID: msg.ChatID}, ctx),
		API:          r.api,
		PeerResolver: r.resolver,
		Emitter:      r.emitter,
		PluginName:   cmd.Plugin,
		StartTime:    time.Now(),
		Metadata:     make(map[string]any),
		Ctx:          ctx,
	}

	handler := cmd.Handler
	r.mu.RLock()
	mws := append([]Middleware(nil), r.globalMW...)
	r.mu.RUnlock()
	for i := len(mws) - 1; i >= 0; i-- {
		handler = mws[i](handler)
	}
	return handler(cmdCtx)
}

func (r *Registry) loggingMiddleware(next Handler) Handler {
	return func(ctx *core.CommandContext) error {
		r.logger.Info("exec",
			zap.String("cmd", ctx.Command),
			zap.Strings("args", ctx.Args),
			zap.Int64("user", ctx.Message.UserID),
			zap.Int64("chat", ctx.Message.ChatID),
		)
		err := next(ctx)
		if err != nil {
			r.logger.Error("failed", zap.String("cmd", ctx.Command), zap.Error(err))
		}
		return err
	}
}

func (r *Registry) recoveryMiddleware(next Handler) Handler {
	return func(ctx *core.CommandContext) (err error) {
		defer func() {
			if rec := recover(); rec != nil {
				err = fmt.Errorf("panic: %v", rec)
				r.logger.Error("panic recovered", zap.String("cmd", ctx.Command), zap.Any("panic", rec))
			}
		}()
		return next(ctx)
	}
}

func (r *Registry) rateLimitMiddleware(next Handler) Handler {
	return func(ctx *core.CommandContext) error {
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
