package builtin

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

const (
	reMaxCount  = 100
	reMaxRepeat = 10
	reDelay     = 300 * time.Millisecond
)

// RePlugin repeats replied messages. Promoted from the external plugin
// repository; forwards without author so media and formatting survive.
type RePlugin struct{}

func NewRe() *RePlugin { return &RePlugin{} }

func (p *RePlugin) Name() string        { return "re" }
func (p *RePlugin) Description() string { return "复读回复的消息" }
func (p *RePlugin) DescEN() string      { return "Repeat the replied message" }

func (p *RePlugin) Init(_ context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "re",
		Description: "复读：回复一条消息发 re，可带条数和次数",
		DescEN:      "Repeat: reply with re, optionally count and times",
		Usage:       "re [条数] [次数]（回复消息）",
		UsageEN:     "re [count] [times] (reply)",
		Plugin:      p.Name(),
		Category:    "tools",
		OwnerOnly:   true,
		Handler:     p.handleRe,
	})
}

func (p *RePlugin) Start(_ context.Context) error { return nil }
func (p *RePlugin) Stop(_ context.Context) error  { return nil }

func (p *RePlugin) help(ctx *interfaces.CommandContext) error {
	return ctx.Edit(ctx.Tlocal(
		`🔁 <b>re 复读</b>

回复一条消息再发：
<code>re</code>  复读这一条
<code>re 3</code>  从这条起往后 3 条一起复读
<code>re 1 5</code>  这一条复读 5 次

图片、贴纸、格式都会保留。上限 100 条 × 10 次。`,
		`🔁 <b>re repeat</b>

Reply to a message, then send:
<code>re</code>  repeat it
<code>re 3</code>  repeat it and the next 2
<code>re 1 5</code>  repeat it 5 times

Media, stickers and formatting are kept. Max 100 messages × 10 times.`))
}

func randomID() int64 {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return int64(binary.LittleEndian.Uint64(b[:]))
}

func (p *RePlugin) handleRe(ctx *interfaces.CommandContext) error {
	if ctx.Message == nil || !ctx.Message.IsReply {
		return p.help(ctx)
	}
	count, repeat := 1, 1
	if ctx.ArgCount() > 0 {
		n, err := parseInt(ctx.GetArg(0))
		if err != nil || n < 1 || n > reMaxCount {
			return ctx.Edit(ctx.Tlocal(
				fmt.Sprintf("条数要写 1–%d", reMaxCount), fmt.Sprintf("Count must be 1-%d", reMaxCount)))
		}
		count = n
	}
	if ctx.ArgCount() > 1 {
		n, err := parseInt(ctx.GetArg(1))
		if err != nil || n < 1 || n > reMaxRepeat {
			return ctx.Edit(ctx.Tlocal(
				fmt.Sprintf("次数要写 1–%d", reMaxRepeat), fmt.Sprintf("Times must be 1-%d", reMaxRepeat)))
		}
		repeat = n
	}

	peer, err := ctx.ResolvePeer()
	if err != nil {
		return ctx.Edit("❌ " + err.Error())
	}

	// Collect the replied message and the following ones, oldest first,
	// stopping before the command itself.
	ids := []int{ctx.Message.ReplyToID}
	if count > 1 {
		page, _, err := pageHistory(ctx, peer, ctx.Message.Message.ID, reMaxCount)
		if err == nil {
			var after []int
			for _, m := range page {
				if m.ID > ctx.Message.ReplyToID {
					after = append(after, m.ID)
				}
			}
			// page is newest first; take the ones right after the reply.
			for i := len(after) - 1; i >= 0 && len(ids) < count; i-- {
				ids = append(ids, after[i])
			}
		}
	}

	_ = ctx.Delete()
	sent := 0
	for r := 0; r < repeat; r++ {
		rids := make([]int64, len(ids))
		for i := range rids {
			rids[i] = randomID()
		}
		req := &tg.MessagesForwardMessagesRequest{
			FromPeer:   peer,
			ToPeer:     peer,
			ID:         ids,
			RandomID:   rids,
			DropAuthor: true,
		}
		if top, ok := topicOf(ctx); ok {
			req.SetTopMsgID(top)
		}
		if _, err := ctx.API.MessagesForwardMessages(ctx.Context(), req); err != nil {
			ctx.Logger.Warn("re forward", "error", err)
			if sent == 0 {
				return p.fallbackText(ctx, peer, repeat)
			}
			return nil
		}
		sent++
		time.Sleep(reDelay)
	}
	return nil
}

// topicOf returns the forum topic the command was sent in, if any.
func topicOf(ctx *interfaces.CommandContext) (int, bool) {
	if ctx.Message == nil || ctx.Message.Message == nil {
		return 0, false
	}
	h, ok := ctx.Message.Message.ReplyTo.(*tg.MessageReplyHeader)
	if !ok || !h.ForumTopic {
		return 0, false
	}
	if top, ok := h.GetReplyToTopID(); ok {
		return top, true
	}
	return h.ReplyToMsgID, true
}

// fallbackText copies plain text when forwarding is restricted
// (protected chats forbid forwards).
func (p *RePlugin) fallbackText(ctx *interfaces.CommandContext, peer tg.InputPeerClass, repeat int) error {
	msg, _, err := fetchMessage(ctx, ctx.Message.ReplyToID)
	if err != nil || msg.Message == "" {
		return nil
	}
	for r := 0; r < repeat; r++ {
		req := &tg.MessagesSendMessageRequest{Peer: peer, Message: msg.Message, RandomID: randomID()}
		if len(msg.Entities) > 0 {
			req.SetEntities(msg.Entities)
		}
		if top, ok := topicOf(ctx); ok {
			req.SetReplyTo(&tg.InputReplyToMessage{ReplyToMsgID: top, TopMsgID: top})
		}
		if _, err := ctx.API.MessagesSendMessage(ctx.Context(), req); err != nil {
			return nil
		}
		time.Sleep(reDelay)
	}
	return nil
}
