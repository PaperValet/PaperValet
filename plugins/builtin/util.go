package builtin

import (
	"fmt"
	"strconv"

	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
)

func parseInt(s string) (int, error) {
	return strconv.Atoi(s)
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// fetchMessage loads one message from the current chat. Channels and
// supergroups need channels.getMessages; messages.getMessages only sees
// private chats and basic groups.
func fetchMessage(ctx *interfaces.CommandContext, id int) (*tg.Message, tg.MessagesMessagesClass, error) {
	peer, err := ctx.ResolvePeer()
	if err != nil {
		return nil, nil, err
	}
	ref := []tg.InputMessageClass{&tg.InputMessageID{ID: id}}
	var res tg.MessagesMessagesClass
	if ch, ok := peer.(*tg.InputPeerChannel); ok {
		res, err = ctx.API.ChannelsGetMessages(ctx.Context(), &tg.ChannelsGetMessagesRequest{
			Channel: &tg.InputChannel{ChannelID: ch.ChannelID, AccessHash: ch.AccessHash},
			ID:      ref,
		})
	} else {
		res, err = ctx.API.MessagesGetMessages(ctx.Context(), ref)
	}
	if err != nil {
		return nil, nil, err
	}
	for _, m := range historyMessages(res) {
		if msg, ok := m.(*tg.Message); ok && msg.ID == id {
			return msg, res, nil
		}
	}
	return nil, res, fmt.Errorf("message %d not found", id)
}

// usersOf returns the users attached to a messages response.
func usersOf(res tg.MessagesMessagesClass) []tg.UserClass {
	switch m := res.(type) {
	case *tg.MessagesMessages:
		return m.Users
	case *tg.MessagesMessagesSlice:
		return m.Users
	case *tg.MessagesChannelMessages:
		return m.Users
	}
	return nil
}
