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
		return p.handleAll(ctx)
	}

	if !ctx.Message.IsReply {
		return ctx.Edit("用法: 回复消息后发送 dme 删除该消息；dme all [数量] 删除自己最近的消息")
	}

	peer, err := ctx.ResolvePeer()
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 解析失败: %v", err))
	}

	err = deleteMessages(ctx, peer, []int{ctx.Message.ReplyToID})
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 删除失败: %v", err))
	}

	// Remove the command message too so nothing is left behind.
	_ = ctx.Delete()
	return nil
}

func (p *PrunePlugin) handleAll(ctx *interfaces.CommandContext) error {
	count := 1
	if ctx.ArgCount() > 1 {
		n, err := parseInt(ctx.GetArg(1))
		if err != nil || n < 1 || n > 100 {
			return ctx.Edit("数量必须是 1–100")
		}
		count = n
	}

	peer, err := ctx.ResolvePeer()
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 解析失败: %v", err))
	}

	// Page backwards from the command; never delete the status message itself.
	var ids []int
	offset := ctx.Message.Message.ID
	for len(ids) < count {
		history, err := ctx.API.MessagesGetHistory(ctx.Context(), &tg.MessagesGetHistoryRequest{
			Peer: peer, Limit: 100, OffsetID: offset,
		})
		if err != nil {
			return ctx.Edit(fmt.Sprintf("❌ 获取消息失败: %v", err))
		}
		list := historyMessages(history)
		if len(list) == 0 {
			break
		}
		next := offset
		for _, m := range list {
			if id := m.GetID(); id > 0 && id < next {
				next = id
			}
			if msg, ok := m.(*tg.Message); ok && msg.Out && msg.ID < offset {
				ids = append(ids, msg.ID)
				if len(ids) == count {
					break
				}
			}
		}
		if next >= offset {
			break
		}
		offset = next
	}

	if len(ids) == 0 {
		return ctx.Edit("未找到自己的消息")
	}

	err = deleteMessages(ctx, peer, ids)
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 删除失败: %v", err))
	}

	return ctx.Edit(fmt.Sprintf("🗑 已删除 %d 条自己的消息", len(ids)))
}

// historyMessages handles every populated Telegram history response.
func historyMessages(history tg.MessagesMessagesClass) []tg.MessageClass {
	switch h := history.(type) {
	case *tg.MessagesMessages:
		return h.Messages
	case *tg.MessagesMessagesSlice:
		return h.Messages
	case *tg.MessagesChannelMessages:
		return h.Messages
	default:
		return nil
	}
}

func deleteMessages(ctx *interfaces.CommandContext, peer tg.InputPeerClass, ids []int) error {
	if channel, ok := peer.(*tg.InputPeerChannel); ok {
		_, err := ctx.API.ChannelsDeleteMessages(ctx.Context(), &tg.ChannelsDeleteMessagesRequest{
			Channel: &tg.InputChannel{ChannelID: channel.ChannelID, AccessHash: channel.AccessHash}, ID: ids,
		})
		return err
	}
	_, err := ctx.API.MessagesDeleteMessages(ctx.Context(), &tg.MessagesDeleteMessagesRequest{ID: ids, Revoke: true})
	return err
}
