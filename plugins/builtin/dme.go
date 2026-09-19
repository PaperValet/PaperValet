package builtin

import (
	"context"
	"fmt"

	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// PrunePlugin provides message deletion utilities.
type PrunePlugin struct{}

func NewPrune() *PrunePlugin { return &PrunePlugin{} }

func (p *PrunePlugin) Name() string        { return "dme" }
func (p *PrunePlugin) Description() string { return "消息清理" }

func (p *PrunePlugin) Init(_ context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "dme",
		Description: "删除消息",
		Usage:       "dme（回复删除）| dme all [数量]（删除自己最近的消息）",
		Plugin:      p.Name(),
		Category:    "tools",
		OwnerOnly:   true,
		Handler:     p.handleDme,
	})
}

func (p *PrunePlugin) Start(_ context.Context) error { return nil }
func (p *PrunePlugin) Stop(_ context.Context) error  { return nil }

func (p *PrunePlugin) handleDme(ctx *interfaces.CommandContext) error {
	if ctx.ArgCount() > 0 && ctx.GetArg(0) == "all" {
		return p.handleSelfPrune(ctx)
	}

	if !ctx.Message.IsReply {
		return ctx.Edit("用法: 回复消息后发送 dme 删除该消息；dme all [数量] 删除自己最近的消息")
	}

	_, err := ctx.ResolvePeer()
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 解析失败: %v", err))
	}

	_, err = ctx.API.MessagesDeleteMessages(ctx.Context(), &tg.MessagesDeleteMessagesRequest{
		ID:     []int{ctx.Message.ReplyToID},
		Revoke: true,
	})
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 删除失败: %v", err))
	}

	// Remove the command message too so nothing is left behind.
	_ = ctx.Delete()
	return nil
}

func (p *PrunePlugin) handleSelfPrune(ctx *interfaces.CommandContext) error {
	count := 1
	if ctx.ArgCount() > 1 {
		if n, err := parseInt(ctx.GetArg(1)); err == nil && n > 0 && n <= 100 {
			count = n
		}
	}

	peer, err := ctx.ResolvePeer()
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 解析失败: %v", err))
	}

	// Fetch recent messages and delete own ones.
	history, err := ctx.API.MessagesGetHistory(ctx.Context(), &tg.MessagesGetHistoryRequest{
		Peer:  peer,
		Limit: count * 3, // fetch more to filter
	})
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 获取消息失败: %v", err))
	}

	var ids []int
	collect := func(list []tg.MessageClass) {
		for _, m := range list {
			if msg, ok := m.(*tg.Message); ok && msg.Out {
				ids = append(ids, msg.ID)
				if len(ids) >= count {
					return
				}
			}
		}
	}
	switch msgs := history.(type) {
	case *tg.MessagesMessages:
		collect(msgs.Messages)
	case *tg.MessagesChannelMessages:
		collect(msgs.Messages)
	}

	if len(ids) == 0 {
		return ctx.Edit("未找到自己的消息")
	}

	_, err = ctx.API.MessagesDeleteMessages(ctx.Context(), &tg.MessagesDeleteMessagesRequest{
		ID:     ids,
		Revoke: true,
	})
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 删除失败: %v", err))
	}

	return ctx.Edit(fmt.Sprintf("🗑 已删除 %d 条自己的消息", len(ids)))
}
