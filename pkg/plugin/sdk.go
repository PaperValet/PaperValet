// Package plugin provides the public SDK for PaperValet plugins.
// External plugins build against this package.
// Internal packages should use the type aliases in internal/interfaces.
package plugin

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gotd/td/telegram/message/entity"
	"github.com/gotd/td/telegram/message/html"
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

func (s PluginStatus) String() string {
	switch s {
	case StatusActive:
		return "Active"
	case StatusError:
		return "Error"
	default:
		return "Inactive"
	}
}

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
	GetPlugin(name string) (Plugin, bool)
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
	// SetPrefixes replaces all command prefixes (main prefix first).
	SetPrefixes(prefixes []string)
	// AddUserAlias registers a runtime alias expanded once during parsing.
	// The value is a full command line, e.g. "exec ./deploy.sh".
	AddUserAlias(name, cmd string)
	RemoveUserAlias(name string)
	UserAliases() map[string]string
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
	ResolveUsername(ctx context.Context, username string) (tg.InputPeerClass, error)
}

// ============================================================
// Message
// ============================================================

// MediaSender sends local files to chats. Implemented by internal/media.Manager.
type MediaSender interface {
	SendFile(ctx context.Context, chatID int64, path string, caption string, replyTo int) error
}

// MediaDownloader downloads media from a message into a local regular file.
type MediaDownloader interface {
	DownloadMedia(ctx context.Context, msg *MessageEvent) (string, error)
}

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
	Media        MediaSender
	Downloader   MediaDownloader
	PluginName   string
	StartTime    time.Time
	Metadata     map[string]any
	Ctx          context.Context
	Logger       Logger
	I18n         func(key string, args ...any) string
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

// T translates a message key using the command's i18n resolver.
// Falls back to the raw key when no resolver is wired.
func (c *CommandContext) T(key string, args ...any) string {
	if c.I18n != nil {
		return c.I18n(key, args...)
	}
	return key
}

func (c *CommandContext) Reply(text string) error {
	if c.Message == nil || c.API == nil || c.Message.Message == nil {
		return ErrNoMessage
	}
	peer, err := c.ResolvePeer()
	if err != nil {
		return err
	}
	plain, entities := c.parseHTML(text)
	_, err = c.API.MessagesSendMessage(c.Context(), &tg.MessagesSendMessageRequest{
		Peer:     peer,
		Message:  plain,
		Entities: entities,
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
	plain, entities := c.parseHTML(text)
	req := &tg.MessagesEditMessageRequest{
		Peer:    peer,
		ID:      c.Message.Message.ID,
		Message: plain,
	}
	if len(entities) > 0 {
		req.SetEntities(entities)
	}
	_, err = c.API.MessagesEditMessage(c.Context(), req)
	return err
}

// ReplyMedia sends a local file (photo/document) as a reply and deletes nothing.
func (c *CommandContext) ReplyMedia(path, caption string) error {
	if c.Message == nil || c.Media == nil {
		return ErrNoMessage
	}
	return c.Media.SendFile(c.Context(), c.Message.ChatID, path, caption, c.Message.Message.ID)
}

// parseHTML converts a limited HTML string (b/i/u/s/a/code/pre/blockquote)
// into plain text plus Telegram entities. On parse failure it falls back to
// the raw text with no entities. tg://user?id= links resolve via the API and
// degrade to plain text when the user cannot be resolved.
func (c *CommandContext) parseHTML(text string) (string, []tg.MessageEntityClass) {
	var b entity.Builder
	if err := html.HTML(strings.NewReader(text), &b, html.Options{UserResolver: c.resolveInputUser}); err != nil {
		return text, nil
	}
	return b.Complete()
}

func (c *CommandContext) resolveInputUser(id int64) (tg.InputUserClass, error) {
	if c.API == nil {
		return nil, fmt.Errorf("no api client")
	}
	users, err := c.API.UsersGetUsers(c.Context(), []tg.InputUserClass{&tg.InputUser{UserID: id}})
	if err != nil || len(users) == 0 {
		return nil, fmt.Errorf("resolve user %d failed", id)
	}
	u, ok := users[0].(*tg.User)
	if !ok {
		return nil, fmt.Errorf("unexpected user type %T", users[0])
	}
	return u.AsInput(), nil
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
