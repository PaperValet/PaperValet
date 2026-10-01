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

// RePlugin repeats replied messages: delete the command, forward the
// target into this chat, and copy it verbatim where forwarding is banned.
type RePlugin struct{}

func NewRe() *RePlugin { return &RePlugin{} }

func (p *RePlugin) Name() string        { return "re" }
func (p *RePlugin) Description() string { return "复读消息" }
func (p *RePlugin) DescEN() string      { return "Repeat messages" }

func (p *RePlugin) Init(_ context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "re",
		Description: "复读消息",
		DescEN:      "Repeat messages",
		Usage: "re [条数] [次数]（回复一条消息）\n" +
			"\n" +
			"**示例**\n" +
			"• `re`  复读回复的那条\n" +
			"• `re 3`  从那条起连续 3 条一起复读\n" +
			"• `re 1 5`  那一条复读 5 次\n" +
			"\n" +
			"**机制**\n" +
			"• 先删掉命令消息，再把目标消息转发到当前聊天\n" +
			"• 转发保留图片、贴纸、文件和格式\n" +
			"• 聊天禁止转发时，改为发送一模一样的消息（文字、格式、媒体都照搬）\n" +
			"• 在话题群里发到同一个话题\n" +
			"• 上限 100 条 × 10 次",
		UsageEN: "re [count] [times] (reply to a message)\n" +
			"\n" +
			"**Examples**\n" +
			"• `re`  repeat the replied message\n" +
			"• `re 3`  repeat it and the next 2\n" +
			"• `re 1 5`  repeat it 5 times\n" +
			"\n" +
			"**How it works**\n" +
			"• Deletes the command, then forwards the target into this chat\n" +
			"• Forwarding keeps photos, stickers, files and formatting\n" +
			"• When the chat forbids forwarding, sends an identical copy instead (text, formatting and media)\n" +
			"• In forum groups it posts to the same topic\n" +
			"• Max 100 messages × 10 times",
		Plugin:    p.Name(),
		Category:  "tools",
		OwnerOnly: true,
		Handler:   p.handleRe,
	})
}

func (p *RePlugin) Start(_ context.Context) error { return nil }
func (p *RePlugin) Stop(_ context.Context) error  { return nil }

func randomID() int64 {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return int64(binary.LittleEndian.Uint64(b[:]))
}

func (p *RePlugin) handleRe(ctx *interfaces.CommandContext) error {
	if ctx.Message == nil || !ctx.Message.IsReply {
		return ctx.Edit(ctx.Tlocal(
			"回复一条消息再发 `re`，详细说明见 `help re`",
			"Reply to a message with `re`; see `help re`"))
	}
	count, repeat := 1, 1
	if ctx.ArgCount() > 0 {
		n, err := parseInt(ctx.GetArg(0))
		if err != nil || n < 1 || n > reMaxCount {
			return ctx.Edit(ctx.Tlocal(fmt.Sprintf("条数要写 1–%d", reMaxCount), fmt.Sprintf("Count must be 1-%d", reMaxCount)))
		}
		count = n
	}
	if ctx.ArgCount() > 1 {
		n, err := parseInt(ctx.GetArg(1))
		if err != nil || n < 1 || n > reMaxRepeat {
			return ctx.Edit(ctx.Tlocal(fmt.Sprintf("次数要写 1–%d", reMaxRepeat), fmt.Sprintf("Times must be 1-%d", reMaxRepeat)))
		}
		repeat = n
	}

	peer, err := ctx.ResolvePeer()
	if err != nil {
		return ctx.Edit("❌ " + esc(err.Error()))
	}
	ids := p.targetIDs(ctx, peer, count)
	_ = ctx.Delete()

	top, inTopic := topicOf(ctx)
	for r := 0; r < repeat; r++ {
		if err := p.forward(ctx, peer, ids, top, inTopic); err != nil {
			ctx.Logger.Info("re forward refused, copying", "error", err)
			for _, id := range ids {
				if err := p.copyMessage(ctx, peer, id, top, inTopic); err != nil {
					ctx.Logger.Warn("re copy", "id", id, "error", err)
				}
				time.Sleep(reDelay)
			}
			continue
		}
		time.Sleep(reDelay)
	}
	return nil
}

// targetIDs returns the replied message plus the following count-1, oldest
// first, all before the command.
func (p *RePlugin) targetIDs(ctx *interfaces.CommandContext, peer tg.InputPeerClass, count int) []int {
	ids := []int{ctx.Message.ReplyToID}
	if count <= 1 {
		return ids
	}
	page, _, err := pageHistory(ctx, peer, ctx.Message.Message.ID, reMaxCount)
	if err != nil {
		return ids
	}
	for i := len(page) - 1; i >= 0 && len(ids) < count; i-- {
		if page[i].ID > ctx.Message.ReplyToID {
			ids = append(ids, page[i].ID)
		}
	}
	return ids
}

func (p *RePlugin) forward(ctx *interfaces.CommandContext, peer tg.InputPeerClass, ids []int, top int, inTopic bool) error {
	rids := make([]int64, len(ids))
	for i := range rids {
		rids[i] = randomID()
	}
	req := &tg.MessagesForwardMessagesRequest{FromPeer: peer, ToPeer: peer, ID: ids, RandomID: rids}
	if inTopic {
		req.SetTopMsgID(top)
	}
	_, err := ctx.API.MessagesForwardMessages(ctx.Context(), req)
	return err
}

// copyMessage sends an identical message: same text, entities and media.
func (p *RePlugin) copyMessage(ctx *interfaces.CommandContext, peer tg.InputPeerClass, id, top int, inTopic bool) error {
	msg, _, err := fetchMessage(ctx, id)
	if err != nil {
		return err
	}
	var reply tg.InputReplyToClass
	if inTopic {
		reply = &tg.InputReplyToMessage{ReplyToMsgID: top, TopMsgID: top}
	}
	if media := inputMediaOf(msg.Media); media != nil {
		req := &tg.MessagesSendMediaRequest{Peer: peer, Media: media, Message: msg.Message, RandomID: randomID()}
		if len(msg.Entities) > 0 {
			req.SetEntities(msg.Entities)
		}
		if reply != nil {
			req.SetReplyTo(reply)
		}
		_, err = ctx.API.MessagesSendMedia(ctx.Context(), req)
		return err
	}
	if msg.Message == "" {
		return fmt.Errorf("nothing to copy")
	}
	req := &tg.MessagesSendMessageRequest{Peer: peer, Message: msg.Message, RandomID: randomID()}
	if len(msg.Entities) > 0 {
		req.SetEntities(msg.Entities)
	}
	if reply != nil {
		req.SetReplyTo(reply)
	}
	if _, isWeb := msg.Media.(*tg.MessageMediaWebPage); !isWeb {
		req.NoWebpage = true
	}
	_, err = ctx.API.MessagesSendMessage(ctx.Context(), req)
	return err
}

// inputMediaOf turns received media into sendable media by reference.
// Returns nil for web previews and media that cannot be resent.
func inputMediaOf(m tg.MessageMediaClass) tg.InputMediaClass {
	switch v := m.(type) {
	case *tg.MessageMediaPhoto:
		if ph, ok := v.Photo.(*tg.Photo); ok {
			in := &tg.InputMediaPhoto{ID: &tg.InputPhoto{ID: ph.ID, AccessHash: ph.AccessHash, FileReference: ph.FileReference}}
			in.Spoiler = v.Spoiler
			return in
		}
	case *tg.MessageMediaDocument:
		if d, ok := v.Document.(*tg.Document); ok {
			in := &tg.InputMediaDocument{ID: &tg.InputDocument{ID: d.ID, AccessHash: d.AccessHash, FileReference: d.FileReference}}
			in.Spoiler = v.Spoiler
			return in
		}
	case *tg.MessageMediaGeo:
		if g, ok := v.Geo.(*tg.GeoPoint); ok {
			return &tg.InputMediaGeoPoint{GeoPoint: &tg.InputGeoPoint{Lat: g.Lat, Long: g.Long}}
		}
	case *tg.MessageMediaContact:
		return &tg.InputMediaContact{PhoneNumber: v.PhoneNumber, FirstName: v.FirstName, LastName: v.LastName, Vcard: v.Vcard}
	case *tg.MessageMediaDice:
		return &tg.InputMediaDice{Emoticon: v.Emoticon}
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
