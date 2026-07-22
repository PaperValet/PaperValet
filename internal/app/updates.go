package app

import (
	"context"

	"github.com/gotd/td/tg"
	"go.uber.org/zap"

	"github.com/TiaraBasori/PaperValet/internal/core"
	"github.com/TiaraBasori/PaperValet/internal/eventbus"
	"github.com/TiaraBasori/PaperValet/pkg/logger"
)

// UpdateHandler implements telegram.UpdateHandler and fans out to the event bus.
type UpdateHandler struct {
	bus        *eventbus.Bus
	selfUserID int64
	logger     *zap.Logger
}

func NewUpdateHandler(bus *eventbus.Bus) *UpdateHandler {
	return &UpdateHandler{
		bus:    bus,
		logger: logger.Named("updates"),
	}
}

func (h *UpdateHandler) SetSelfUserID(id int64) { h.selfUserID = id }

func (h *UpdateHandler) Handle(ctx context.Context, u tg.UpdatesClass) error {
	_ = h.bus.Emit(ctx, eventbus.EventRawUpdate, u)

	switch updates := u.(type) {
	case *tg.Updates:
		for _, upd := range updates.Updates {
			if err := h.handleOne(ctx, upd, updates); err != nil {
				h.logger.Error("update failed", zap.Error(err))
			}
		}
	case *tg.UpdatesCombined:
		for _, upd := range updates.Updates {
			if err := h.handleOne(ctx, upd, updates); err != nil {
				h.logger.Error("update failed", zap.Error(err))
			}
		}
	case *tg.UpdateShort:
		return h.handleOne(ctx, updates.Update, updates)
	case *tg.UpdateShortMessage:
		msg := &tg.Message{
			ID:      updates.ID,
			Message: updates.Message,
			Date:    updates.Date,
			Out:     updates.Out,
			PeerID:  &tg.PeerUser{UserID: updates.UserID},
		}
		return h.dispatchMessage(ctx, msg, updates)
	case *tg.UpdateShortChatMessage:
		msg := &tg.Message{
			ID:      updates.ID,
			Message: updates.Message,
			Date:    updates.Date,
			Out:     updates.Out,
			PeerID:  &tg.PeerChat{ChatID: updates.ChatID},
			FromID:  &tg.PeerUser{UserID: updates.FromID},
		}
		return h.dispatchMessage(ctx, msg, updates)
	}
	return nil
}

func (h *UpdateHandler) handleOne(ctx context.Context, upd tg.UpdateClass, raw tg.UpdatesClass) error {
	switch u := upd.(type) {
	case *tg.UpdateNewMessage:
		if msg, ok := u.Message.(*tg.Message); ok {
			return h.dispatchMessage(ctx, msg, raw)
		}
	case *tg.UpdateNewChannelMessage:
		if msg, ok := u.Message.(*tg.Message); ok {
			return h.dispatchMessage(ctx, msg, raw)
		}
	}
	return nil
}

func (h *UpdateHandler) dispatchMessage(ctx context.Context, msg *tg.Message, raw tg.UpdatesClass) error {
	userID := extractUserID(msg)
	if userID == 0 && msg.Out && h.selfUserID != 0 {
		userID = h.selfUserID
	}

	ev := &core.MessageEvent{
		Update:   raw,
		Message:  msg,
		Text:     msg.Message,
		UserID:   userID,
		ChatID:   extractChatID(msg),
		IsOut:    msg.Out,
		Entities: msg.Entities,
		Media:    msg.Media,
		Date:     msg.Date,
		PeerID:   msg.PeerID,
		Raw:      msg,
	}
	if reply, ok := msg.ReplyTo.(*tg.MessageReplyHeader); ok {
		ev.IsReply = true
		ev.ReplyToID = reply.ReplyToMsgID
	}
	return h.bus.Emit(ctx, eventbus.EventMessage, ev)
}

func extractUserID(msg *tg.Message) int64 {
	if msg.FromID != nil {
		if u, ok := msg.FromID.(*tg.PeerUser); ok {
			return u.UserID
		}
	}
	if u, ok := msg.PeerID.(*tg.PeerUser); ok {
		return u.UserID
	}
	return 0
}

func extractChatID(msg *tg.Message) int64 {
	switch p := msg.PeerID.(type) {
	case *tg.PeerUser:
		return p.UserID
	case *tg.PeerChat:
		return -p.ChatID
	case *tg.PeerChannel:
		return -1000000000000 - p.ChannelID
	}
	return 0
}
