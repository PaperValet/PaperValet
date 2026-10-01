package builtin

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

const (
	dmeBatchSize    = 50
	dmeSearchLimit  = 100
	dmeEditWindow   = 48 * time.Hour
	dmePlaceholder  = "."
	dmeMaxCount     = 2000
	dmeAllCount     = 999999
	dmeActiveWait   = 200 * time.Millisecond
	dmeBatchDelay   = 200 * time.Millisecond
	dmeSearchDelay  = 100 * time.Millisecond
	dmeEditWait     = 700 * time.Millisecond
	dmeEmptyBatches = 3
)

// PrunePlugin deletes the owner's own messages, TeleBox-dme style.
type PrunePlugin struct {
	mu       sync.Mutex
	active   map[int64]bool
	othersMu sync.RWMutex
	delOther bool
}

func NewPrune() *PrunePlugin {
	return &PrunePlugin{active: make(map[int64]bool)}
}

func (p *PrunePlugin) Name() string          { return "dme" }
func (p *PrunePlugin) Description() string   { return "删除消息（自己的为主，可选删他人）" }
func (p *PrunePlugin) DescEN() string        { return "Delete messages (own by default, others optional)" }

func (p *PrunePlugin) Init(_ context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&interfaces.Command{
		Name: "dme",
		Description: "删消息：回复删除那条；dme 5 删自己最近 5 条；dme all 全删；dme -f 防撤回",
		DescEN:      "Delete: reply deletes that message; dme 5 deletes my last 5; dme all wipes; dme -f anti-recall",
		Usage:       "dme [all|N|-f [N]|others on|off] 或回复",
		UsageEN:     "dme [all|N|-f [N]|others on|off] or reply",
		Plugin:      p.Name(),
		Category:    "tools",
		OwnerOnly:   true,
		Handler:     p.handleDme,
	})
}

func (p *PrunePlugin) Start(_ context.Context) error  { return nil }
func (p *PrunePlugin) Stop(_ context.Context) error   { return nil }

func (p *PrunePlugin) help(ctx *interfaces.CommandContext) error {
	return ctx.Edit(ctx.Tlocal(
		`🗑 <b>dme 删除消息</b>

<b>用法</b>
• 回复一条消息 + <code>dme</code> — 删除那条（自己或他人的）
• <code>dme 5</code> — 删除自己最近 5 条
• <code>dme all</code> — 删除本聊天里自己全部消息
• <code>dme -f 10</code> — 防撤回删除：先把内容改成占位符再删
• <code>dme others on|off</code> — 回复删除时是否允许删别人的消息（默认关）

命令消息本身也会一起删掉。`,
		`🗑 <b>dme delete messages</b>

<b>Usage</b>
• Reply + <code>dme</code> — delete that message (yours or others')
• <code>dme 5</code> — delete your last 5 messages
• <code>dme all</code> — delete all of your messages here
• <code>dme -f 10</code> — anti-recall: overwrite content first, then delete
• <code>dme others on|off</code> — allow deleting others' messages by reply (default off)

The command message itself is deleted too.`))
}

func (p *PrunePlugin) handleDme(ctx *interfaces.CommandContext) error {
	if ctx.ArgCount() == 0 && !ctx.Message.IsReply {
		return p.help(ctx)
	}

	// dme others on/off
	if ctx.ArgCount() >= 2 && ctx.GetArg(0) == "others" {
		switch strings.ToLower(ctx.GetArg(1)) {
		case "on", "true", "1":
			p.othersMu.Lock()
			p.delOther = true
			p.othersMu.Unlock()
			return ctx.Edit(ctx.Tlocal("✅ 回复删除他人消息：已开启", "✅ Deleting others' messages by reply: on"))
		case "off", "false", "0":
			p.othersMu.Lock()
			p.delOther = false
			p.othersMu.Unlock()
			return ctx.Edit(ctx.Tlocal("⏸️ 回复删除他人消息：已关闭", "⏸️ Deleting others' messages by reply: off"))
		}
		return ctx.Edit(ctx.Tlocal("用法: <code>dme others on|off</code>", "Usage: <code>dme others on|off</code>"))
	}
	if ctx.ArgCount() == 1 && ctx.GetArg(0) == "others" {
		p.othersMu.RLock()
		on := p.delOther
		p.othersMu.RUnlock()
		state := ctx.Tlocal("关闭", "off")
		if on {
			state = ctx.Tlocal("开启", "on")
		}
		return ctx.Edit(ctx.Tlocal(fmt.Sprintf("删除他人消息: %s（<code>dme others on</code> 打开）", state),
			fmt.Sprintf("Deleting others' messages: %s (<code>dme others on</code> to enable)", state)))
	}

	// Reply mode: delete exactly the replied message.
	if ctx.Message.IsReply {
		return p.deleteReplied(ctx)
	}

	// Count mode: dme [N] | dme all | dme -f [N]
	antiRecall := false
	arg := ctx.GetArg(0)
	if arg == "-f" {
		antiRecall = true
		if ctx.ArgCount() > 1 {
			arg = ctx.GetArg(1)
		} else {
			arg = "all"
		}
	}

	count := 1
	switch {
	case arg == "all":
		count = dmeAllCount
	default:
		n, err := parseInt(arg)
		if err != nil || n < 1 || n > dmeMaxCount {
			return ctx.Edit(ctx.Tlocal(
				"数量要写 1–2000，或者用 <code>all</code>", "Count must be 1-2000, or use <code>all</code>"))
		}
		count = n
	}

	if !p.tryLock(ctx.Message.ChatID) {
		return ctx.Edit(ctx.Tlocal("⏳ 这个聊天里已有 dme 在跑，等它结束", "⏳ A dme run is already active in this chat"))
	}
	defer p.unlock(ctx.Message.ChatID)

	return p.run(ctx, count, antiRecall)
}

// deleteReplied removes the replied message (and the command).
func (p *PrunePlugin) deleteReplied(ctx *interfaces.CommandContext) error {
	peer, err := ctx.ResolvePeer()
	if err != nil {
		return ctx.Edit("❌ " + err.Error())
	}

	// Own message: always allowed. Others': only with the switch on.
	target := &tg.Message{ID: ctx.Message.ReplyToID}
	if msgs, err := ctx.API.MessagesGetMessages(ctx.Context(),
		[]tg.InputMessageClass{&tg.InputMessageID{ID: ctx.Message.ReplyToID}}); err == nil {
		for _, m := range historyMessages(msgs) {
			if msg, ok := m.(*tg.Message); ok && msg.ID == ctx.Message.ReplyToID {
				target = msg
				break
			}
		}
	}
	if !target.Out {
		if pu, ok := target.FromID.(*tg.PeerUser); ok && pu.UserID == ctx.SelfID {
			target.Out = true
		}
	}
	if !target.Out {
		p.othersMu.RLock()
		allowed := p.delOther
		p.othersMu.RUnlock()
		if !allowed {
			return ctx.Edit(ctx.Tlocal(
				"这条不是你发的。要删别人的消息先 <code>dme others on</code>",
				"That message isn't yours. Enable <code>dme others on</code> first"))
		}
	}

	if err := deleteMessages(ctx, peer, []int{target.ID}); err != nil {
		return ctx.Edit(ctx.Tlocal("❌ 删除失败: "+err.Error(), "❌ Delete failed: "+err.Error()))
	}
	_ = ctx.Delete()
	return nil
}

func (p *PrunePlugin) tryLock(chatID int64) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.active[chatID] {
		return false
	}
	p.active[chatID] = true
	return true
}

func (p *PrunePlugin) unlock(chatID int64) {
	p.mu.Lock()
	delete(p.active, chatID)
	p.mu.Unlock()
}

// run finds and deletes the owner's messages, newest first.
func (p *PrunePlugin) run(ctx *interfaces.CommandContext, count int, antiRecall bool) error {
	cmdMsgID := ctx.Message.Message.ID
	peer, err := ctx.ResolvePeer()
	if err != nil {
		return ctx.Edit("❌ " + err.Error())
	}
	_ = ctx.Edit("⏳ …")

	total, edited := 0, 0
	target := count
	offsetID := cmdMsgID // only touch messages sent before the command
	empty := 0

	for total < target && empty < dmeEmptyBatches {
		msgs, next, err := pageHistory(ctx, peer, offsetID, dmeSearchLimit)
		if err != nil {
			return ctx.Edit(ctx.Tlocal("❌ 获取历史失败: "+err.Error(), "❌ History failed: "+err.Error()))
		}
		if len(msgs) == 0 {
			break
		}

		mine := make([]*tg.Message, 0, len(msgs))
		for _, m := range msgs {
			if m.ID >= cmdMsgID {
				continue
			}
			if isMine(m, ctx.SelfID) {
				mine = append(mine, m)
			}
		}

		if len(mine) == 0 {
			empty++
			offsetID = next
			time.Sleep(dmeSearchDelay)
			continue
		}
		empty = 0

		if total+len(mine) > target {
			mine = mine[:target-total]
		}

		if antiRecall {
			edited += p.antiRecall(ctx, peer, mine)
			time.Sleep(dmeEditWait)
		}

		ids := make([]int, 0, len(mine))
		for _, m := range mine {
			ids = append(ids, m.ID)
		}
		for i := 0; i < len(ids); i += dmeBatchSize {
			end := i + dmeBatchSize
			if end > len(ids) {
				end = len(ids)
			}
			if err := deleteMessages(ctx, peer, ids[i:end]); err != nil {
				return ctx.Edit(ctx.Tlocal(
					fmt.Sprintf("⚠️ 删除中断: %v\n已删除 %d 条", err, total),
					fmt.Sprintf("⚠️ Delete interrupted: %v\nDeleted %d", err, total)))
			}
			total += end - i
			time.Sleep(dmeBatchDelay)
		}

		offsetID = next
		_ = ctx.Edit(fmt.Sprintf("🗑 %d", total))
		time.Sleep(dmeSearchDelay)
	}

	if total == 0 {
		_ = ctx.Delete()
		return nil
	}

	// Silent finish: the command message is gone with the batch.
	_ = ctx.Delete()
	result := ctx.Tlocal(fmt.Sprintf("🗑 已删除 %d 条", total), fmt.Sprintf("🗑 Deleted %d", total))
	if edited > 0 {
		result += ctx.Tlocal(fmt.Sprintf("（防撤回处理 %d 条）", edited), fmt.Sprintf(" (%d anti-recalled)", edited))
	}
	_ = result
	return nil
}

// pageHistory returns one page of plain messages plus the next offset.
func pageHistory(ctx *interfaces.CommandContext, peer tg.InputPeerClass, offsetID, limit int) ([]*tg.Message, int, error) {
	history, err := ctx.API.MessagesGetHistory(ctx.Context(), &tg.MessagesGetHistoryRequest{
		Peer: peer, Limit: limit, OffsetID: offsetID,
	})
	if err != nil {
		return nil, 0, err
	}
	list := historyMessages(history)
	msgs := make([]*tg.Message, 0, len(list))
	next := offsetID
	for _, m := range list {
		if id := m.GetID(); id > 0 && id < next {
			next = id
		}
		if msg, ok := m.(*tg.Message); ok {
			msgs = append(msgs, msg)
		}
	}
	if next >= offsetID {
		next = 0
	}
	return msgs, next, nil
}

func isMine(m *tg.Message, selfID int64) bool {
	if m.Out {
		return true
	}
	if pu, ok := m.FromID.(*tg.PeerUser); ok {
		return pu.UserID == selfID
	}
	return false
}

// antiRecall overwrites content so "deleted" media cannot be recovered from
// the CDN cache: text becomes a dot, media is dropped via Edit with no media
// is not possible, so we blank the caption/text and rely on deletion.
func (p *PrunePlugin) antiRecall(ctx *interfaces.CommandContext, peer tg.InputPeerClass, msgs []*tg.Message) int {
	done := 0
	now := time.Now()
	for _, m := range msgs {
		if m.Date > 0 && now.Sub(time.Unix(int64(m.Date), 0)) > dmeEditWindow {
			continue
		}
		if strings.TrimSpace(m.Message) == "" {
			continue
		}
		_, err := ctx.API.MessagesEditMessage(ctx.Context(), &tg.MessagesEditMessageRequest{
			Peer: peer, ID: m.ID, Message: dmePlaceholder,
		})
		if err == nil {
			done++
		}
	}
	return done
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
