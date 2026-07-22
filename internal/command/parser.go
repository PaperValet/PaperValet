package command

import (
	"context"
	"strings"

	"go.uber.org/zap"

	"github.com/TiaraBasori/PaperValet/internal/core"
	"github.com/TiaraBasori/PaperValet/internal/eventbus"
	"github.com/TiaraBasori/PaperValet/pkg/logger"
)

// Parser listens for outgoing message events and dispatches commands.
type Parser struct {
	registry *Registry
	bus      *eventbus.Bus
	logger   *zap.Logger
}

func NewParser(registry *Registry, bus *eventbus.Bus) *Parser {
	return &Parser{
		registry: registry,
		bus:      bus,
		logger:   logger.Named("command_parser"),
	}
}

// Start registers the message listener (userbot: outgoing only).
func (p *Parser) Start() {
	p.bus.Subscribe(eventbus.EventMessage, func(ctx context.Context, event *eventbus.Event) error {
		msg, ok := event.Data.(*core.MessageEvent)
		if !ok || msg == nil || msg.Message == nil {
			return nil
		}
		if !msg.IsOut {
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
		p.logger.Debug("dispatch", zap.String("name", name), zap.Strings("args", args))
		return p.registry.ExecuteCommand(ctx, msg, name, args)
	}, eventbus.WithPriority(100))
}
