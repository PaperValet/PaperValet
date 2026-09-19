package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/gotd/td/telegram/message/entity"
	"github.com/gotd/td/telegram/message/html"
	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type AtAdminsPlugin struct{}

func New() *AtAdminsPlugin { return &AtAdminsPlugin{} }

func (p *AtAdminsPlugin) Name() string        { return "atadmins" }
func (p *AtAdminsPlugin) Description() string { return "一键艾特全部管理员" }

func (p *AtAdminsPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&plugin.Command{
		Name:        "atadmins",
		Aliases:     []string{"calladmins", "管理员"},
		Description: "艾特群组所有管理员",
		Usage:       "atadmins [消息内容]",
		Plugin:      p.Name(),
		Category:    "group",
		Handler:     p.handleAtAdmins,
	})
}

func (p *AtAdminsPlugin) Start(ctx context.Context) error { return nil }
func (p *AtAdminsPlugin) Stop(ctx context.Context) error  { return nil }

func (p *AtAdminsPlugin) handleAtAdmins(ctx *plugin.CommandContext) error {
	if ctx.Message == nil || ctx.API == nil {
		return plugin.ErrNoMessage
	}
	peer, err := ctx.ResolvePeer()
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 会话解析失败: %v", err))
	}

	users, err := fetchAdmins(ctx, peer)
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 获取管理员列表失败: %v", err))
	}
	if len(users) == 0 {
		return ctx.Edit("❌ 未找到管理员（或当前会话不是群组）")
	}

	msg := strings.Join(ctx.Args, " ")
	if msg == "" {
		msg = "管理员请注意！"
	}

	byID := make(map[int64]*tg.User, len(users))
	var mentions strings.Builder
	for _, u := range users {
		if u.Bot || u.Deleted {
			continue
		}
		byID[u.ID] = u
		name := u.FirstName
		if name == "" {
			name = u.Username
		}
		if name == "" {
			name = fmt.Sprintf("user%d", u.ID)
		}
		fmt.Fprintf(&mentions, `<a href="tg://user?id=%d">%s</a> `, u.ID, name)
	}
	if len(byID) == 0 {
		return ctx.Edit("❌ 可艾特的管理员为空")
	}

	full := "📢 " + msg + "\n\n" + mentions.String()

	// Build entities with a resolver backed by the admin list.
	resolver := func(id int64) (tg.InputUserClass, error) {
		if u, ok := byID[id]; ok {
			return u.AsInput(), nil
		}
		return nil, fmt.Errorf("unknown user %d", id)
	}
	var b entity.Builder
	var text string
	var entities []tg.MessageEntityClass
	if err := html.HTML(strings.NewReader(full), &b, html.Options{UserResolver: resolver}); err != nil {
		text = full // fall back to plain text
	} else {
		text, entities = b.Complete()
	}

	req := &tg.MessagesSendMessageRequest{
		Peer:     peer,
		Message:  text,
		RandomID: int64(ctx.Message.Message.ID)*1e6 + 1,
	}
	if len(entities) > 0 {
		req.SetEntities(entities)
	}
	if _, err := ctx.API.MessagesSendMessage(ctx.Context(), req); err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 发送失败: %v", err))
	}
	return ctx.Delete()
}

func fetchAdmins(ctx *plugin.CommandContext, peer tg.InputPeerClass) ([]*tg.User, error) {
	switch p := peer.(type) {
	case *tg.InputPeerChannel:
		res, err := ctx.API.ChannelsGetParticipants(ctx.Context(), &tg.ChannelsGetParticipantsRequest{
			Channel: &tg.InputChannel{ChannelID: p.ChannelID, AccessHash: p.AccessHash},
			Filter:  &tg.ChannelParticipantsAdmins{},
			Limit:   100,
		})
		if err != nil {
			return nil, err
		}
		participants, ok := res.(*tg.ChannelsChannelParticipants)
		if !ok {
			return nil, fmt.Errorf("unexpected participants type %T", res)
		}
		var out []*tg.User
		for _, u := range participants.Users {
			if user, ok := u.(*tg.User); ok {
				out = append(out, user)
			}
		}
		return out, nil

	case *tg.InputPeerChat:
		full, err := ctx.API.MessagesGetFullChat(ctx.Context(), p.ChatID)
		if err != nil {
			return nil, err
		}
		chatFull, ok := full.FullChat.(*tg.ChatFull)
		if !ok {
			return nil, fmt.Errorf("unexpected full chat type %T", full.FullChat)
		}
		cps, ok := chatFull.Participants.(*tg.ChatParticipants)
		if !ok {
			return nil, fmt.Errorf("unexpected participants type %T", chatFull.Participants)
		}
		adminIDs := map[int64]bool{}
		for _, cp := range cps.Participants {
			switch v := cp.(type) {
			case *tg.ChatParticipantAdmin:
				adminIDs[v.UserID] = true
			case *tg.ChatParticipantCreator:
				adminIDs[v.UserID] = true
			}
		}
		var out []*tg.User
		for _, u := range full.Users {
			if user, ok := u.(*tg.User); ok && adminIDs[user.ID] {
				out = append(out, user)
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("当前会话不是群组")
}
