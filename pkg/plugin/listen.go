package plugin

import (
	"context"
	"fmt"

	"github.com/gotd/td/tg"
)

// ============================================================
// Message listeners
// ============================================================

// MessageListener sees every new message, incoming and outgoing, just
// before command parsing (command messages included; check Host.Prefixes
// to skip them). Edited is true for edits. Listeners run one
// after another on the update path, so anything slow belongs in a
// goroutine. Register with Host.Listen.
type MessageListener func(ctx context.Context, msg *MessageEvent, edited bool)

// ============================================================
// Message helpers
// ============================================================

// ChatIDOf converts a peer to the chat id form used by MessageEvent.ChatID:
// users positive, basic groups -id, channels -100… .
func ChatIDOf(p tg.PeerClass) int64 {
	switch v := p.(type) {
	case *tg.PeerUser:
		return v.UserID
	case *tg.PeerChat:
		return -v.ChatID
	case *tg.PeerChannel:
		return ChannelChatID(v.ChannelID)
	}
	return 0
}

// ChatIDOfInput converts an input peer to the MessageEvent.ChatID form.
func ChatIDOfInput(p tg.InputPeerClass) int64 {
	switch v := p.(type) {
	case *tg.InputPeerUser:
		return v.UserID
	case *tg.InputPeerChat:
		return -v.ChatID
	case *tg.InputPeerChannel:
		return ChannelChatID(v.ChannelID)
	}
	return 0
}

// ChannelChatID converts a raw channel id to the -100… chat id form.
func ChannelChatID(channelID int64) int64 { return -1000000000000 - channelID }

// SenderID returns the user id that sent msg, or 0 (channel posts,
// anonymous admins).
func SenderID(msg *tg.Message) int64 {
	if msg == nil {
		return 0
	}
	if u, ok := msg.FromID.(*tg.PeerUser); ok {
		return u.UserID
	}
	if msg.FromID == nil {
		if u, ok := msg.PeerID.(*tg.PeerUser); ok {
			return u.UserID
		}
	}
	return 0
}

// GetMessages fetches messages by id from peer. Channels and supergroups
// use channels.getMessages. Missing ids are absent; service messages are
// skipped. Users and chats returned by Telegram are included so callers can
// read names and access hashes.
func GetMessages(ctx context.Context, api *tg.Client, peer tg.InputPeerClass, ids ...int) ([]*tg.Message, []tg.UserClass, []tg.ChatClass, error) {
	if api == nil {
		return nil, nil, nil, ErrNoMessage
	}
	if len(ids) == 0 {
		return nil, nil, nil, nil
	}
	in := make([]tg.InputMessageClass, len(ids))
	for i, id := range ids {
		in[i] = &tg.InputMessageID{ID: id}
	}
	var (
		res tg.MessagesMessagesClass
		err error
	)
	if ch, ok := peer.(*tg.InputPeerChannel); ok {
		res, err = api.ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
			Channel: &tg.InputChannel{ChannelID: ch.ChannelID, AccessHash: ch.AccessHash},
			ID:      in,
		})
	} else {
		res, err = api.MessagesGetMessages(ctx, in)
	}
	if err != nil {
		return nil, nil, nil, err
	}
	mod, ok := res.AsModified()
	if !ok {
		return nil, nil, nil, nil
	}
	want := ChatIDOfInput(peer)
	_, self := peer.(*tg.InputPeerSelf)
	var out []*tg.Message
	for _, m := range mod.GetMessages() {
		msg, ok := m.(*tg.Message)
		if !ok {
			continue
		}
		// messages.getMessages is global for users and basic groups.
		if !self && want != 0 && ChatIDOf(msg.PeerID) != want {
			continue
		}
		out = append(out, msg)
	}
	return out, mod.GetUsers(), mod.GetChats(), nil
}

// DeleteMessages removes messages in peer, using channels.deleteMessages in
// channels and supergroups (messages.deleteMessages silently does nothing
// there).
func DeleteMessages(ctx context.Context, api *tg.Client, peer tg.InputPeerClass, ids ...int) error {
	if api == nil {
		return ErrNoMessage
	}
	if len(ids) == 0 {
		return nil
	}
	for start := 0; start < len(ids); start += 100 {
		end := min(start+100, len(ids))
		var err error
		if ch, ok := peer.(*tg.InputPeerChannel); ok {
			_, err = api.ChannelsDeleteMessages(ctx, &tg.ChannelsDeleteMessagesRequest{
				Channel: &tg.InputChannel{ChannelID: ch.ChannelID, AccessHash: ch.AccessHash},
				ID:      ids[start:end],
			})
		} else {
			_, err = api.MessagesDeleteMessages(ctx, &tg.MessagesDeleteMessagesRequest{ID: ids[start:end], Revoke: true})
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// ReplyMessage fetches the message the command replies to.
func (c *CommandContext) ReplyMessage() (*tg.Message, error) {
	if c.Message == nil || c.Message.ReplyToID == 0 {
		return nil, fmt.Errorf("no reply")
	}
	peer, err := c.ResolvePeer()
	if err != nil {
		return nil, err
	}
	msgs, _, _, err := GetMessages(c.Context(), c.API, peer, c.Message.ReplyToID)
	if err != nil {
		return nil, err
	}
	for _, m := range msgs {
		if m.ID == c.Message.ReplyToID {
			return m, nil
		}
	}
	return nil, fmt.Errorf("message %d not found", c.Message.ReplyToID)
}

// EventFromMessage builds a MessageEvent for msg (for RunCommand or
// Downloader.DownloadMedia on a fetched message).
func EventFromMessage(msg *tg.Message) *MessageEvent {
	if msg == nil {
		return nil
	}
	ev := &MessageEvent{
		Message:  msg,
		Text:     msg.Message,
		UserID:   SenderID(msg),
		ChatID:   ChatIDOf(msg.PeerID),
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
	return ev
}
