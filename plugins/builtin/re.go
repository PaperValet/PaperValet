package builtin

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
	"github.com/gotd/td/tg"
)

// RePlugin repeats messages (复读机).
type RePlugin struct{}

func NewRe() *RePlugin { return &RePlugin{} }

func (p *RePlugin) Name() string        { return "re" }
func (p *RePlugin) Description() string { return "消息复读工具" }

func (p *RePlugin) Init(_ context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "re",
		Description: "复读回复的消息",
		Usage:       "re [数量] [次数]",
		Plugin:      p.Name(),
		Category:    "tools",
		Handler:     p.handleRe,
	})
}

func (p *RePlugin) Start(_ context.Context) error { return nil }
func (p *RePlugin) Stop(_ context.Context) error  { return nil }

func (p *RePlugin) handleRe(ctx *interfaces.CommandContext) error {
	msg := ctx.Message
	if msg == nil {
		return ctx.Edit("❌ 无消息上下文")
	}

	if !msg.IsReply {
		return ctx.Edit("❌ 你必须回复一条消息才能复读")
	}

	args := ctx.Args
	count := 1
	repeat := 1

	if len(args) > 0 {
		if c, err := strconv.Atoi(args[0]); err == nil && c > 0 && c <= 50 {
			count = c
		}
	}
	if len(args) > 1 {
		if r, err := strconv.Atoi(args[1]); err == nil && r > 0 && r <= 10 {
			repeat = r
		}
	}

	replyMsg := msg.Message
	if replyMsg == nil {
		return ctx.Edit("❌ 无法获取回复的消息")
	}

	// Build content to repeat
	var content strings.Builder
	if replyMsg.Message != "" {
		content.WriteString(replyMsg.Message)
	}
	// Could also handle media, but text is primary

	text := content.String()
	if text == "" {
		return ctx.Edit("❌ 回复的消息无文本内容")
	}

	// Send repeated messages
	peer, err := ctx.PeerResolver.ResolveFromChatID(ctx.Context(), msg.ChatID)
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 解析 Peer 失败: %v", err))
	}

	for i := 0; i < repeat; i++ {
		for j := 0; j < count; j++ {
			_, err := ctx.API.MessagesSendMessage(ctx.Context(), &tg.MessagesSendMessageRequest{
				Peer:     peer,
				Message:  text,
				RandomID: time.Now().UnixNano(),
			})
			if err != nil {
				return ctx.Edit(fmt.Sprintf("❌ 发送失败: %v", err))
			}
		}
	}

	return ctx.Edit(fmt.Sprintf("✅ 复读完成: %d 条 × %d 次", count, repeat))
}