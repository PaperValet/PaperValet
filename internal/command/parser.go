package command

import (
	"context"
	"strings"

	"github.com/TiaraBasori/PaperValet/internal/eventbus"
	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/logger"
)

// Parser listens for message events and dispatches commands.
// It handles the owner's outgoing messages plus, when sudo is enabled,
// messages from delegated users. Other people's traffic is ignored.
type Parser struct {
	registry *Registry
	bus      *eventbus.Bus
	logger   interfaces.Logger
}

func NewParser(registry *Registry, bus *eventbus.Bus) *Parser {
	return &Parser{
		registry: registry,
		bus:      bus,
		logger:   logger.NamedLogger("command_parser"),
	}
}

// Start registers the message listener (userbot: outgoing only).
func (p *Parser) Start() {
	p.bus.Subscribe(eventbus.EventMessage, func(ctx context.Context, event *eventbus.Event) error {
		msg, ok := event.Data.(*interfaces.MessageEvent)
		if !ok || msg == nil || msg.Message == nil {
			return nil
		}
		if !msg.IsOut && !p.registry.CanUseCommands(msg.UserID) {
			return nil
		}
		text := strings.TrimSpace(msg.Text)
		if text == "" {
			return nil
		}
		name, args, isCmd := p.registry.ParseCommand(text)
		if !isCmd {
			return nil
		}
		p.logger.Debug("dispatch", "name", name, "args", args)
		return p.registry.ExecuteCommand(ctx, msg, name, args)
	}, eventbus.WithPriority(100))
}
