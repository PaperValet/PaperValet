package builtin

import (
	"context"
	"fmt"
	"strings"

	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// PrunePlugin provides message deletion utilities.
// Ported from PagerMaid's prune.py.
type PrunePlugin struct{}

func NewPrune() *PrunePlugin { return &PrunePlugin{} }

func (p *PrunePlugin) Name() string        { return "prune" }
func (p *PrunePlugin) Description() string { return "消息清理" }

func (p *PrunePlugin) Init(_ context.Context, mgr plugin.Manager) error {
	cmds := []*interfaces.Command{
		{Name: "prune", Aliases: []string{"del", "purge"}, Description: "删除消息", Usage: "prune [数量] | selfprune [数量] | yourprune [数量]", Plugin: p.Name(), Category: "tools", OwnerOnly: true, Handler: p.handlePrune},
		{Name: "selfprune", Aliases: []string{"sp"}, Description: "删除自己发送的消息", Usage: "selfprune [数量]", Plugin: p.Name(), Category: "tools", OwnerOnly: true, Handler: p.handleSelfPrune},
	}
	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *PrunePlugin) Start(_ context.Context) error { return nil }
func (p *PrunePlugin) Stop(_ context.Context) error  { return nil }

func (p *PrunePlugin) handlePrune(ctx *interfaces.CommandContext) error {
	if !ctx.Message.IsReply {
		return ctx.Edit("❌ 请回复一条消息作为起始点")
	}

	_, err := ctx.ResolvePeer()
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 解析失败: %v", err))
	}

	// Delete the replied-to message.
	ids := []int{ctx.Message.ReplyToID}
	_, err = ctx.API.MessagesDeleteMessages(ctx.Context(), &tg.MessagesDeleteMessagesRequest{
		ID:     ids,
		Revoke: true,
	})
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 删除失败: %v", err))
	}

	return ctx.Edit(fmt.Sprintf("🗑 已删除 %d 条消息", len(ids)))
}

func (p *PrunePlugin) handleSelfPrune(ctx *interfaces.CommandContext) error {
	count := 1
	if ctx.ArgCount() > 0 {
		if n, err := parseInt(ctx.GetArg(0)); err == nil && n > 0 && n <= 100 {
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
	if msgs, ok := history.(*tg.MessagesMessages); ok {
		for _, m := range msgs.Messages {
			if msg, ok := m.(*tg.Message); ok && msg.Out {
				ids = append(ids, msg.ID)
				if len(ids) >= count {
					break
				}
			}
		}
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

var _ = strings.TrimSpace