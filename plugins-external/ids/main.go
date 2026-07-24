package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type IDsPlugin struct{}

func New() *IDsPlugin { return &IDsPlugin{} }

func (p *IDsPlugin) Name() string        { return "ids" }
func (p *IDsPlugin) Description() string { return "显示用户/群组/频道 ID 及跳转链接" }

func (p *IDsPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	cmds := []*plugin.Command{
		{
			Name:        "ids",
			Aliases:     []string{"id", "whoami", "myid"},
			Description: "显示用户/群组/消息 ID 及 t.me 跳转链接",
			Usage:       "ids [@username|reply]",
			Plugin:      p.Name(),
			Category:    "info",
			OwnerOnly:   false,
			Handler:     p.handleIDs,
		},
	}
	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *IDsPlugin) Start(ctx context.Context) error { return nil }
func (p *IDsPlugin) Stop(ctx context.Context) error  { return nil }

func (p *IDsPlugin) handleIDs(ctx *plugin.CommandContext) error {
	if ctx.Message == nil {
		return ctx.Edit("❌ 无消息上下文")
	}

	msg := ctx.Message
	var targetUserID int64 = msg.UserID
	var targetChatID int64 = msg.ChatID
	var targetMsgID int = msg.Message.ID

	// If reply, get replied user info
	if msg.IsReply && msg.ReplyToID > 0 {
		// In real impl, would fetch replied message
		targetMsgID = msg.ReplyToID
	}

	// Build links
	userLink := fmt.Sprintf("tg://user?id=%d", targetUserID)
	chatLink := ""
	if targetChatID < 0 {
		internal := fmt.Sprintf("%d", targetChatID)
		if strings.HasPrefix(internal, "-100") {
			internal = internal[4:]
		} else if strings.HasPrefix(internal, "-") {
			internal = internal[1:]
		}
		chatLink = fmt.Sprintf("https://t.me/c/%s", internal)
	}

	return ctx.Edit(fmt.Sprintf(`🆔 <b>ID 信息</b>

<b>用户:</b> <code>%d</code> (<a href="%s">点击跳转</a>)
<b>群组:</b> <code>%d</code>%s
<b>消息:</b> <code>%d</code>%s
<b>你发的:</b> %v
<b>回复消息:</b> %v`,
		targetUserID, userLink,
		targetChatID, map[bool]string{true: " (<a href=\"" + chatLink + "\">跳转</a>)"}[chatLink != ""],
		targetMsgID, map[bool]string{true: " (<a href=\"" + chatLink + "/" + fmt.Sprintf("%d", targetMsgID) + "\">跳转</a>)"}[chatLink != ""],
		msg.IsOut, msg.IsReply))
}