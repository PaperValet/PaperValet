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
func (p *InfoPlugin) Description() string { return "查看用户/群组/频道 ID 信息" }
func (p *InfoPlugin) DescEN() string      { return "Show user/group/channel IDs" }

func (p *InfoPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "info",
		Aliases:     []string{"id", "whois"},
		Description: "显示当前聊天或 @用户 的 ID（回复可看对方）",
		DescEN:      "Show IDs for this chat or @user (reply targets the sender)",
		Usage:       "info [@用户名] 或回复",
		UsageEN:     "info [@username] or reply",
		Plugin:      p.Name(),
		Category:    "tools",
		Handler:     p.handleInfo,
	})
}

func (p *InfoPlugin) Start(_ context.Context) error { return nil }
func (p *InfoPlugin) Stop(_ context.Context) error  { return nil }

func (p *InfoPlugin) handleInfo(ctx *interfaces.CommandContext) error {
	msg := ctx.Message
	if msg == nil || msg.Message == nil {
		return ctx.Edit("❌ no message")
	}

	targetID := msg.UserID
	targetName := ""

	// Reply shows the replied sender; an explicit @username wins.
	if ctx.ArgCount() > 0 && strings.HasPrefix(ctx.GetArg(0), "@") && ctx.PeerResolver != nil {
		peer, err := ctx.PeerResolver.ResolveUsername(ctx.Context(), ctx.GetArg(0)[1:])
		if err != nil {
			return ctx.Edit(ctx.Tlocal("❌ 解析不到这个用户名", "❌ Cannot resolve that username"))
		}
		switch pr := peer.(type) {
		case *tg.InputPeerUser:
			targetID, targetName = pr.UserID, ctx.GetArg(0)
		case *tg.InputPeerChat:
			targetID, targetName = -pr.ChatID, ctx.GetArg(0)
		case *tg.InputPeerChannel:
			targetID, targetName = -1000000000000-pr.ChannelID, ctx.GetArg(0)
		}
	} else if msg.IsReply && ctx.API != nil {
		if msgs, err := ctx.API.MessagesGetMessages(ctx.Context(),
			[]tg.InputMessageClass{&tg.InputMessageID{ID: msg.ReplyToID}}); err == nil {
			for _, m := range historyMessages(msgs) {
				if rm, ok := m.(*tg.Message); ok && rm.ID == msg.ReplyToID {
					if pu, ok := rm.FromID.(*tg.PeerUser); ok {
						targetID = pu.UserID
						if u := findUserInChats(msgs, pu.UserID); u != nil {
							targetName = displayName(u)
						}
					}
					break
				}
			}
		}
	}

	chatID := msg.ChatID
	kind := ctx.Tlocal("👤 用户", "👤 User")
	if chatID < -1000000000000 {
		kind = ctx.Tlocal("📢 频道/超级群组", "📢 Channel/Supergroup")
	} else if chatID < 0 {
		kind = ctx.Tlocal("👥 群组", "👥 Group")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "📋 <b>%s</b>\n\n", ctx.Tlocal("信息", "Info"))
	fmt.Fprintf(&b, "%s ID: <code>%d</code>\n", ctx.Tlocal("目标", "Target"), targetID)
	if targetName != "" {
		fmt.Fprintf(&b, "%s: %s\n", ctx.Tlocal("名字", "Name"), htmlEscape(targetName))
	}
	fmt.Fprintf(&b, "%s: <code>%d</code>\n", ctx.Tlocal("本聊天 ID", "This chat"), chatID)
	fmt.Fprintf(&b, "%s: <code>%d</code> (%s)\n", ctx.Tlocal("消息 ID", "Message"), msg.Message.ID, kind)
	return ctx.Edit(b.String())
}

func displayName(u *tg.User) string {
	name := strings.TrimSpace(strings.Join([]string{u.FirstName, u.LastName}, " "))
	if name == "" {
		return fmt.Sprintf("#%d", u.ID)
	}
	if u.Username != "" {
		return fmt.Sprintf("%s (@%s)", name, u.Username)
	}
	return name
}
