// Package plugin provides the public SDK for PaperValet plugins.
// External plugins build against this package.
// Internal packages should use the type aliases in internal/interfaces.
package plugin

import (
	"context"
	"time"

	"github.com/gotd/td/tg"
)

// PluginMetadata holds plugin metadata for external .so plugins.
// External plugins export this as `var Metadata *PluginMetadata`.
type PluginMetadata struct {
	Name        string
	Description string
	Version     string
	Author      string
	MinVersion  string
}

// ============================================================
// Plugin lifecycle
// ============================================================

// Plugin is the interface all plugins must implement.
type Plugin interface {
	Name() string
	Description() string
	Init(ctx context.Context, mgr Manager) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// PluginInfo holds plugin metadata.
type PluginInfo struct {
	Name        string
	Description string
	Status      PluginStatus
}

// PluginStatus represents plugin lifecycle state.
type PluginStatus int

const (
	StatusInactive PluginStatus = iota
	StatusActive
	StatusError
)

// ============================================================
// Manager interface
// ============================================================

// Manager is the plugin manager interface exposed to external plugins.
type Manager interface {
	RegisterPlugin(p Plugin) error
	RegisterCommand(cmd *Command) error
	UnregisterCommand(name string)
	UnregisterPlugin(name string)
	Commands() RegistryProvider
	GetInfo(name string) (PluginInfo, bool)
	GetAllInfo() []PluginInfo
	Emit(ctx context.Context, eventType string, data any) error
	InitAll(ctx context.Context) error
	StartAll(ctx context.Context) error
	StopAll(ctx context.Context) error
}

// RegistryProvider provides command registry access.
type RegistryProvider interface {
	Get(name string) (*Command, bool)
	GetAll() map[string]*Command
	GetByPlugin(plugin string) map[string]*Command
	GetPrefix() string
	GetPrefixes() []string
}

// ============================================================
// Logging
// ============================================================

// Logger is the minimal logging interface for plugins.
type Logger interface {
	Debug(msg string, keysAndValues ...any)
	Info(msg string, keysAndValues ...any)
	Warn(msg string, keysAndValues ...any)
	Error(msg string, keysAndValues ...any)
	Named(name string) Logger
	With(keysAndValues ...any) Logger
}

// ============================================================
// Event system
// ============================================================

// Emitter is the subset of the event bus that plugins/commands need.
type Emitter interface {
	Emit(ctx context.Context, eventType string, data any) error
}

// ============================================================
// Peer resolution
// ============================================================

// PeerResolver resolves chat/user IDs to InputPeer.
type PeerResolver interface {
	ResolveFromChatID(ctx context.Context, chatID int64) (tg.InputPeerClass, error)
	ResolveUserInChannel(ctx context.Context, channelPeer tg.InputChannelClass, userID int64) (tg.InputPeerClass, error)
	ResolveUserFromMessage(ctx context.Context, peer tg.InputPeerClass, msgID int, userID int64) (tg.InputPeerClass, error)
}

// ============================================================
// Message
// ============================================================

// MessageEvent represents a processed message event.
type MessageEvent struct {
	Update    tg.UpdatesClass
	Message   *tg.Message
	Text      string
	UserID    int64
	ChatID    int64
	IsOut     bool
	IsReply   bool
	ReplyToID int
	Entities  []tg.MessageEntityClass
	Media     tg.MessageMediaClass
	Date      int
	PeerID    tg.PeerClass
	Raw       any
}

// ============================================================
// Session
// ============================================================

// Session holds per-(user,chat) conversation state.
type Session struct {
	UserID    int64
	ChatID    int64
	State     string
	Data      map[string]any
	Timestamp int64
}

// SessionContext wraps a Session with a request context.
type SessionContext struct {
	Session *Session
	Context context.Context
	Data    map[string]any
}

// NewSessionContext creates a new SessionContext.
func NewSessionContext(s *Session, ctx context.Context) *SessionContext {
	return &SessionContext{
		Session: s,
		Context: ctx,
		Data:    make(map[string]any),
	}
}

func (s *SessionContext) Ctx() context.Context {
	if s != nil && s.Context != nil {
		return s.Context
	}
	return context.Background()
}

func (s *SessionContext) Get(key string) (any, bool) {
	if s == nil || s.Data == nil {
		return nil, false
	}
	v, ok := s.Data[key]
	return v, ok
}

func (s *SessionContext) Set(key string, value any) {
	if s.Data == nil {
		s.Data = make(map[string]any)
	}
	s.Data[key] = value
}

func (s *SessionContext) Delete(key string) {
	delete(s.Data, key)
}

// ============================================================
// Command system
// ============================================================

// Handler is the signature for command handlers.
type Handler func(ctx *CommandContext) error

// Middleware wraps a handler.
type Middleware func(next Handler) Handler

// Command represents a registered command.
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

// CommandContext is passed to every command handler.
type CommandContext struct {
	Command      string
	Args         []string
	RawArgs      string
	Message      *MessageEvent
	Session      *SessionContext
	API          *tg.Client
	PeerResolver PeerResolver
	Emitter      Emitter
	PluginName   string
	StartTime    time.Time
	Metadata     map[string]any
	Ctx          context.Context
	Logger       Logger
}

func (c *CommandContext) Context() context.Context {
	if c.Ctx != nil {
		return c.Ctx
	}
	if c.Session != nil {
		return c.Session.Ctx()
	}
	return context.Background()
}

func (c *CommandContext) ResolvePeer() (tg.InputPeerClass, error) {
	if c.Message == nil || c.PeerResolver == nil {
		return nil, ErrNoMessage
	}
	return c.PeerResolver.ResolveFromChatID(c.Context(), c.Message.ChatID)
}

func (c *CommandContext) Reply(text string) error {
	if c.Message == nil || c.API == nil || c.Message.Message == nil {
		return ErrNoMessage
	}
	peer, err := c.ResolvePeer()
	if err != nil {
		return err
	}
	_, err = c.API.MessagesSendMessage(c.Context(), &tg.MessagesSendMessageRequest{
		Peer:     peer,
		Message:  text,
		RandomID: time.Now().UnixNano(),
		ReplyTo:  &tg.InputReplyToMessage{ReplyToMsgID: c.Message.Message.ID},
	})
	return err
}

func (c *CommandContext) Edit(text string) error {
	if c.Message == nil || c.API == nil || c.Message.Message == nil {
		return ErrNoMessage
	}
	peer, err := c.ResolvePeer()
	if err != nil {
		return err
	}
	_, err = c.API.MessagesEditMessage(c.Context(), &tg.MessagesEditMessageRequest{
		Peer:    peer,
		ID:      c.Message.Message.ID,
		Message: text,
	})
	return err
}

func (c *CommandContext) Delete() error {
	if c.Message == nil || c.API == nil || c.Message.Message == nil {
		return ErrNoMessage
	}
	_, err := c.API.MessagesDeleteMessages(c.Context(), &tg.MessagesDeleteMessagesRequest{
		ID:     []int{c.Message.Message.ID},
		Revoke: true,
	})
	return err
}

func (c *CommandContext) Typing() error {
	if c.Message == nil || c.API == nil {
		return ErrNoMessage
	}
	peer, err := c.ResolvePeer()
	if err != nil {
		return err
	}
	_, err = c.API.MessagesSetTyping(c.Context(), &tg.MessagesSetTypingRequest{
		Peer:   peer,
		Action: &tg.SendMessageTypingAction{},
	})
	return err
}

func (c *CommandContext) GetArg(index int) string {
	if index < 0 || index >= len(c.Args) {
		return ""
	}
	return c.Args[index]
}

func (c *CommandContext) GetArgs() string { return c.RawArgs }

func (c *CommandContext) ArgCount() int { return len(c.Args) }

func (c *CommandContext) HasArg(arg string) bool {
	for _, a := range c.Args {
		if a == arg {
			return true
		}
	}
	return false
}

// ============================================================
// Errors
// ============================================================

var ErrNoMessage = &CommandError{Code: "NO_MESSAGE", Message: "no message in context"}

type CommandError struct {
	Code    string
	Message string
	Err     error
}

func (e *CommandError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *CommandError) Unwrap() error { return e.Err }