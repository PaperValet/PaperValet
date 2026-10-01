package builtin

import (
	"context"
	"fmt"
	"strings"

	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// InfoPlugin shows user, chat and message IDs with jump links.
// It absorbed the former external ids plugin.
type InfoPlugin struct{}

func NewInfo() *InfoPlugin { return &InfoPlugin{} }

func (p *InfoPlugin) Name() string        { return "info" }
func (p *InfoPlugin) Description() string { return "查看用户/群组/消息 ID 和跳转链接" }
func (p *InfoPlugin) DescEN() string      { return "Show user/chat/message IDs with jump links" }

func (p *InfoPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "info",
		Aliases:     []string{"id", "ids", "getid", "whois"},
		Description: "显示 ID 和跳转链接；回复看对方，带 @用户名 查别人",
		DescEN:      "Show IDs and jump links; reply for the sender, @username for others",
		Usage:       "info [@用户名] 或回复",
		UsageEN:     "info [@username] or reply",
		Plugin:      p.Name(),
		Category:    "tools",
		Handler:     p.handleInfo,
	})
}

func (p *InfoPlugin) Start(_ context.Context) error { return nil }
func (p *InfoPlugin) Stop(_ context.Context) error  { return nil }

// chatLink returns the t.me/c base link for groups and channels.
func chatLink(chatID int64) string {
	s := fmt.Sprintf("%d", chatID)
	switch {
	case strings.HasPrefix(s, "-100"):
		return "https://t.me/c/" + s[4:]
	case strings.HasPrefix(s, "-"):
		return "https://t.me/c/" + s[1:]
	}
	return ""
}

func userLine(ctx *interfaces.CommandContext, label string, id int64, name string) string {
	if name == "" {
		name = ctx.Tlocal("打开", "open")
	}
	return fmt.Sprintf("<b>%s:</b> <code>%d</code> (<a href=\"tg://user?id=%d\">%s</a>)\n",
		label, id, id, htmlEscape(name))
}

func (p *InfoPlugin) handleInfo(ctx *interfaces.CommandContext) error {
	msg := ctx.Message
	if msg == nil || msg.Message == nil {
		return plugin.ErrNoMessage
	}
	jump := ctx.Tlocal("跳转", "jump")

	var b strings.Builder
	b.WriteString("🆔 <b>" + ctx.Tlocal("ID 信息", "IDs") + "</b>\n\n")

	// Explicit @username lookup.
	if arg := ctx.GetArg(0); strings.HasPrefix(arg, "@") && len(arg) > 1 && ctx.PeerResolver != nil {
		peer, err := ctx.PeerResolver.ResolveUsername(ctx.Context(), arg[1:])
		if err != nil {
			return ctx.Edit(ctx.Tlocal("❌ 解析不到 "+htmlEscape(arg), "❌ Cannot resolve "+htmlEscape(arg)))
		}
		switch pr := peer.(type) {
		case *tg.InputPeerUser:
			b.WriteString(userLine(ctx, arg, pr.UserID, arg))
		case *tg.InputPeerChat:
			fmt.Fprintf(&b, "<b>%s:</b> <code>%d</code>\n", htmlEscape(arg), -pr.ChatID)
		case *tg.InputPeerChannel:
			id := -1000000000000 - pr.ChannelID
			fmt.Fprintf(&b, "<b>%s:</b> <code>%d</code> (<a href=\"%s\">%s</a>)\n", htmlEscape(arg), id, chatLink(id), jump)
		}
		b.WriteString("\n")
	}

	b.WriteString(userLine(ctx, ctx.Tlocal("你", "You"), msg.UserID, ""))

	chatPart := fmt.Sprintf("<code>%d</code>", msg.ChatID)
	msgPart := fmt.Sprintf("<code>%d</code>", msg.Message.ID)
	if link := chatLink(msg.ChatID); link != "" {
		chatPart += fmt.Sprintf(" (<a href=\"%s\">%s</a>)", link, jump)
		msgPart += fmt.Sprintf(" (<a href=\"%s/%d\">%s</a>)", link, msg.Message.ID, jump)
	}
	kind := ctx.Tlocal("私聊", "private")
	if msg.ChatID < -1000000000000 {
		kind = ctx.Tlocal("频道/超级群", "channel/supergroup")
	} else if msg.ChatID < 0 {
		kind = ctx.Tlocal("群组", "group")
	}
	fmt.Fprintf(&b, "<b>%s:</b> %s · %s\n", ctx.Tlocal("会话", "Chat"), chatPart, kind)
	fmt.Fprintf(&b, "<b>%s:</b> %s\n", ctx.Tlocal("消息", "Message"), msgPart)

	if msg.IsReply && msg.ReplyToID > 0 {
		b.WriteString("\n")
		replyPart := fmt.Sprintf("<code>%d</code>", msg.ReplyToID)
		if link := chatLink(msg.ChatID); link != "" {
			replyPart += fmt.Sprintf(" (<a href=\"%s/%d\">%s</a>)", link, msg.ReplyToID, jump)
		}
		fmt.Fprintf(&b, "<b>%s:</b> %s\n", ctx.Tlocal("回复的消息", "Replied message"), replyPart)
		if rm, res, err := fetchMessage(ctx, msg.ReplyToID); err == nil {
			switch from := rm.FromID.(type) {
			case *tg.PeerUser:
				name := ""
				if u := findUserInChats(res, from.UserID); u != nil {
					name = displayName(u)
				}
				b.WriteString(userLine(ctx, ctx.Tlocal("对方", "Sender"), from.UserID, name))
			case *tg.PeerChannel:
				fmt.Fprintf(&b, "<b>%s:</b> <code>%d</code>\n", ctx.Tlocal("对方（频道身份）", "Sender (as channel)"), -1000000000000-from.ChannelID)
			}
			if fwd, ok := rm.GetFwdFrom(); ok {
				if pu, ok := fwd.FromID.(*tg.PeerUser); ok {
					b.WriteString(userLine(ctx, ctx.Tlocal("原作者", "Forwarded from"), pu.UserID, fwd.FromName))
				} else if pc, ok := fwd.FromID.(*tg.PeerChannel); ok {
					fmt.Fprintf(&b, "<b>%s:</b> <code>%d</code>\n", ctx.Tlocal("转发自频道", "Forwarded from channel"), -1000000000000-pc.ChannelID)
				}
			}
		}
	}
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
