package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type IDsPlugin struct{}

func New() *IDsPlugin { return &IDsPlugin{} }

func (p *IDsPlugin) Name() string        { return "ids" }
func (p *IDsPlugin) Description() string { return "获取用户/群组/消息 ID" }

func (p *IDsPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&plugin.Command{
		Name:        "ids",
		Aliases:     []string{"id", "getid"},
		Description: "显示当前用户、群组、消息 ID；回复消息时显示对方信息",
		Usage:       "ids",
		Plugin:      p.Name(),
		Category:    "tools",
		Handler:     p.handleIDs,
	})
}

func (p *IDsPlugin) Start(ctx context.Context) error { return nil }
func (p *IDsPlugin) Stop(ctx context.Context) error  { return nil }

func (p *IDsPlugin) handleIDs(ctx *plugin.CommandContext) error {
	if ctx.Message == nil || ctx.Message.Message == nil {
		return plugin.ErrNoMessage
	}
	msg := ctx.Message

	userID := msg.UserID
	msgID := msg.Message.ID
	replyInfo := ""

	if msg.IsReply && msg.ReplyToID > 0 {
		if sender, err := p.fetchReplySender(ctx, msg); err == nil && sender != nil {
			replyInfo = fmt.Sprintf("\n<b>回复对象:</b> <code>%d</code> (<a href=\"tg://user?id=%d\">%s</a>)",
				sender.ID, sender.ID, strings.TrimSpace(sender.FirstName+" "+sender.LastName))
		} else {
			replyInfo = fmt.Sprintf("\n<b>回复消息:</b> <code>%d</code>", msg.ReplyToID)
		}
	}

	chatLink := ""
	chatID := msg.ChatID
	internal := fmt.Sprintf("%d", chatID)
	if strings.HasPrefix(internal, "-100") {
		chatLink = fmt.Sprintf("https://t.me/c/%s", internal[4:])
	} else if strings.HasPrefix(internal, "-") {
		chatLink = fmt.Sprintf("https://t.me/c/%s", internal[1:])
	}

	chatPart := fmt.Sprintf("<code>%d</code>", chatID)
	msgPart := fmt.Sprintf("<code>%d</code>", msgID)
	if chatLink != "" {
		chatPart += fmt.Sprintf(" (<a href=\"%s\">跳转</a>)", chatLink)
		msgPart += fmt.Sprintf(" (<a href=\"%s/%d\">跳转</a>)", chatLink, msgID)
	}

	return ctx.Edit(fmt.Sprintf(`🆔 <b>ID 信息</b>

<b>用户:</b> <code>%d</code> (<a href="tg://user?id=%d">点击跳转</a>)
<b>会话:</b> %s
<b>消息:</b> %s%s`,
		userID, userID, chatPart, msgPart, replyInfo))
}

// fetchReplySender loads the replied-to message and returns its sender user.
func (p *IDsPlugin) fetchReplySender(ctx *plugin.CommandContext, msg *plugin.MessageEvent) (*tg.User, error) {
	peer, err := ctx.ResolvePeer()
	if err != nil {
		return nil, err
	}

	var msgs []tg.MessageClass
	switch peer.(type) {
	case *tg.InputPeerChannel:
		ch, ok := peer.(*tg.InputPeerChannel)
		if !ok {
			return nil, fmt.Errorf("bad channel peer")
		}
		res, err := ctx.API.ChannelsGetMessages(ctx.Context(), &tg.ChannelsGetMessagesRequest{
			Channel: &tg.InputChannel{ChannelID: ch.ChannelID, AccessHash: ch.AccessHash},
			ID:      []tg.InputMessageClass{&tg.InputMessageID{ID: msg.ReplyToID}},
		})
		if err != nil {
			return nil, err
		}
		if m, ok := res.(*tg.MessagesChannelMessages); ok {
			msgs = m.Messages
		}
	default:
		res, err := ctx.API.MessagesGetMessages(ctx.Context(),
			[]tg.InputMessageClass{&tg.InputMessageID{ID: msg.ReplyToID}})
		if err != nil {
			return nil, err
		}
		if m, ok := res.(*tg.MessagesMessages); ok {
			msgs = m.Messages
		}
	}

	for _, mc := range msgs {
		m, ok := mc.(*tg.Message)
		if !ok {
			continue
		}
		if pu, ok := m.FromID.(*tg.PeerUser); ok {
			users, err := ctx.API.UsersGetUsers(ctx.Context(),
				[]tg.InputUserClass{&tg.InputUser{UserID: pu.UserID}})
			if err == nil && len(users) > 0 {
				if u, ok := users[0].(*tg.User); ok {
					return u, nil
				}
			}
			return &tg.User{ID: pu.UserID}, nil
		}
	}
	return nil, fmt.Errorf("reply sender not found")
}
