package builtin

import (
	"context"
	"fmt"
	"strings"

	"github.com/gotd/td/tg"
	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// InfoPlugin shows user/chat info.
type InfoPlugin struct{}

func NewInfo() *InfoPlugin { return &InfoPlugin{} }

func (p *InfoPlugin) Name() string        { return "info" }
func (p *InfoPlugin) Description() string { return "用户/群组/频道信息查询" }

func (p *InfoPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	cmds := []*interfaces.Command{
		{
			Name:        "info",
			Aliases:     []string{"id", "whois"},
			Description: "显示用户/群组/频道 ID 信息",
			Usage:       "info [@用户名|回复消息]",
			Plugin:      p.Name(),
			Category:    "tools",
			Handler:     p.handleInfo,
		},
		{
			Name:        "fwd",
			Aliases:     []string{"forward"},
			Description: "转发回复的消息到目标",
			Usage:       "fwd <目标>",
			Plugin:      p.Name(),
			Category:    "tools",
			Handler:     p.handleForward,
		},
	}
	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *InfoPlugin) Start(_ context.Context) error { return nil }
func (p *InfoPlugin) Stop(_ context.Context) error  { return nil }

func (p *InfoPlugin) handleInfo(ctx *interfaces.CommandContext) error {
	msg := ctx.Message
	if msg == nil || msg.Message == nil {
		return ctx.Edit("❌ 无消息上下文")
	}

	var targetID int64 = msg.UserID
	var targetName string

	// Check if replying to a message
	if msg.IsReply && msg.Message.ReplyTo != nil {
		if replyHeader, ok := msg.Message.ReplyTo.(*tg.MessageReplyHeader); ok {
			// We can't extract the sender from the reply header alone
			// but we can still show useful info
			_ = replyHeader
		}
	}

	// Try @username resolution
	if ctx.ArgCount() > 0 {
		arg := ctx.GetArg(0)
		if strings.HasPrefix(arg, "@") && len(arg) > 1 {
			username := arg[1:]
			if ctx.PeerResolver != nil {
				peer, err := ctx.PeerResolver.ResolveUsername(ctx.Context(), username)
				if err == nil {
					switch p := peer.(type) {
					case *tg.InputPeerUser:
						targetID = p.UserID
						targetName = arg
					case *tg.InputPeerChat:
						targetID = -p.ChatID
						targetName = arg
					case *tg.InputPeerChannel:
						targetID = -1000000000000 - p.ChannelID
						targetName = arg
					}
				}
			}
		}
	}

	// Build peer type string
	peerType := "👤 用户"
	chatID := msg.ChatID
	if chatID < 0 {
		if chatID > -1000000000000 {
			peerType = "👥 群组"
		} else {
			peerType = "📢 频道/超级群组"
		}
	}

	text := fmt.Sprintf(
		"<b>📋 信息</b>\n\n"+
			"<b>%s:</b> <code>%d</code>\n"+
			"<b>💬 当前对话:</b> <code>%d</code>\n"+
			"<b>📨 消息 ID:</b> <code>%d</code>\n",
		peerType, targetID, chatID, msg.Message.ID,
	)

	if targetName != "" {
		text += fmt.Sprintf("<b>🔗 用户名:</b> %s\n", targetName)
	}

	if msg.IsOut {
		text += "\n<i>（自己发送的消息）</i>"
	}

	return ctx.Edit(text)
}

func (p *InfoPlugin) handleForward(ctx *interfaces.CommandContext) error {
	if !ctx.Message.IsReply {
		return ctx.Edit("❌ 请回复一条消息后再转发")
	}
	if ctx.ArgCount() == 0 {
		return ctx.Edit("用法: fwd <@用户名|chat_id>")
	}

	target := ctx.GetArg(0)
	var destPeer tg.InputPeerClass

	// Parse target
	if strings.HasPrefix(target, "@") && len(target) > 1 {
		username := target[1:]
		peer, err := ctx.PeerResolver.ResolveUsername(ctx.Context(), username)
		if err != nil {
			return ctx.Edit(fmt.Sprintf("❌ 无法解析用户名 %s: %v", target, err))
		}
		destPeer = peer
	} else {
		var chatID int64
		if _, err := fmt.Sscanf(target, "%d", &chatID); err != nil {
			return ctx.Edit(fmt.Sprintf("❌ 无效的目标: %s\n支持 @用户名 或 chat_id 数字", target))
		}
		peer, err := ctx.PeerResolver.ResolveFromChatID(ctx.Context(), chatID)
		if err != nil {
			return ctx.Edit(fmt.Sprintf("❌ 无法解析目标: %v", err))
		}
		destPeer = peer
	}

	// Forward the replied message
	_, err := ctx.API.MessagesForwardMessages(ctx.Context(), &tg.MessagesForwardMessagesRequest{
		FromPeer: destPeer,
		ID:       []int{ctx.Message.Message.ID},
		ToPeer:   destPeer,
	})

	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 转发失败: %v", err))
	}

	return ctx.Edit(fmt.Sprintf("✅ 已转发消息到 %s", target))
}