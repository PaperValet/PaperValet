package core

import (
	"context"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
)

// MessageEvent represents a processed message event.
type MessageEvent = interfaces.MessageEvent

// PeerResolver resolves chat/user IDs to InputPeer.
type PeerResolver = interfaces.PeerResolver

// Emitter is the subset of the event bus that plugins/commands need.
type Emitter = interfaces.Emitter

// Session holds per-(user,chat) conversation state.
type Session = interfaces.Session

// SessionContext wraps a Session with a request context.
type SessionContext = interfaces.SessionContext

func NewSessionContext(s *Session, ctx context.Context) *SessionContext {
	return interfaces.NewSessionContext(s, ctx)
}

// CommandContext is passed to every command handler.
type CommandContext = interfaces.CommandContext

// Handler is the signature for command handlers.
type Handler = interfaces.Handler

// Middleware wraps a handler.
type Middleware = interfaces.Middleware

// Command represents a registered command.
type Command = interfaces.Command

var ErrNoMessage = interfaces.ErrNoMessage
type Error = interfaces.Error

// Logger is the minimal logging interface.
type Logger = interfaces.Logger