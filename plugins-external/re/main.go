package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
	"github.com/gotd/td/tg"
)

type RePlugin struct{}

func New() (plugin.Plugin, error) {
	return &RePlugin{}, nil
}

var Metadata = &plugin.PluginMetadata{
	Name:        "re",
	Description: "消息复读工具",
	Version:     "1.0.0",
	Author:      "PaperValet",
	MinVersion:  "0.1.0",
}

func (p *RePlugin) Name() string        { return "re" }
func (p *RePlugin) Description() string { return "消息复读工具" }

func (p *RePlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&plugin.Command{
		Name:        "re",
		Description: "复读回复的消息",
		Usage:       "re [数量] [次数]",
		Plugin:      p.Name(),
		Category:    "tools",
		Handler:     p.handleRe,
	})
}

func (p *RePlugin) handleRe(ctx *plugin.CommandContext) error {
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
		if c, err := strconv.Atoi(args[0]); err == nil && c > 0 {
			count = c
		}
	}
	if len(args) > 1 {
		if r, err := strconv.Atoi(args[1]); err == nil && r > 0 {
			repeat = r
		}
	}

	// Limit to reasonable values
	if count > 100 {
		count = 100
	}
	if repeat > 10 {
		repeat = 10
	}

	client := ctx.API
	if client == nil {
		return ctx.Edit("❌ 客户端不可用")
	}

	// Fetch the replied message
	peer, err := ctx.ResolvePeer()
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 解析 Peer 失败: %v", err))
	}

	res, err := client.MessagesGetMessages(ctx.Context(), []tg.InputMessageClass{
		&tg.InputMessageReplyTo{ID: msg.ReplyToID},
	})
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 获取回复消息失败: %v", err))
	}

	var text string
	switch r := res.(type) {
	case *tg.MessagesMessages:
		for _, m := range r.Messages {
			if mm, ok := m.(*tg.Message); ok {
				text = mm.Message
				break
			}
		}
	case *tg.MessagesMessagesSlice:
		for _, m := range r.Messages {
			if mm, ok := m.(*tg.Message); ok {
				text = mm.Message
				break
			}
		}
	case *tg.MessagesChannelMessages:
		for _, m := range r.Messages {
			if mm, ok := m.(*tg.Message); ok {
				text = mm.Message
				break
			}
		}
	}

	if text == "" {
		return ctx.Edit("❌ 回复的消息无文本内容")
	}

	for i := 0; i < repeat; i++ {
		for j := 0; j < count; j++ {
			if _, err := client.MessagesSendMessage(ctx.Context(), &tg.MessagesSendMessageRequest{
				Peer:     peer,
				Message:  text,
				RandomID: time.Now().UnixNano(),
			}); err != nil {
				return ctx.Edit(fmt.Sprintf("❌ 发送失败: %v", err))
			}
		}
	}

	return ctx.Edit(fmt.Sprintf("✅ 复读完成: %d 条 × %d 次", count, repeat))
}

func (p *RePlugin) Start(ctx context.Context) error { return nil }
func (p *RePlugin) Stop(ctx context.Context) error  { return nil }
