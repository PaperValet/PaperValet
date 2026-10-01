package builtin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

const (
	dmeFile         = "data/dme.json"
	dmePageSize     = 100
	dmeBatchSize    = 100
	dmeMaxCount     = 5000
	dmeEditWindow   = 48 * time.Hour // Telegram refuses edits after this
	dmePlaceholder  = "‎"            // invisible LRM; edit needs non-empty text
	dmeBatchDelay   = 300 * time.Millisecond
	dmeEditDelay    = 50 * time.Millisecond
	dmeProgressStep = 3 * time.Second
	dmeAll          = -1
)

// PrunePlugin deletes messages in the current chat, TeleBox-dme style.
//
// Own messages still inside the 48h edit window are overwritten before
// deletion (text → blank, media → blank image), so clients that keep a
// local copy of deleted messages (anti-recall mods) only keep the blank.
type PrunePlugin struct {
	mu     sync.Mutex
	active map[int64]bool
	others bool
}

func NewPrune() *PrunePlugin { return &PrunePlugin{active: make(map[int64]bool)} }

func (p *PrunePlugin) Name() string { return "dme" }
func (p *PrunePlugin) Description() string {
	return "批量删除当前聊天的消息，内建防撤回"
}
func (p *PrunePlugin) DescEN() string {
	return "Bulk-delete messages in this chat with anti-recall overwrite"
}

func (p *PrunePlugin) Init(_ context.Context, mgr plugin.Manager) error {
	p.load()
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "dme",
		Description: "删除当前聊天里自己最近 N 条或全部消息",
		DescEN:      "Delete your last N (or all) messages in this chat",
		Usage:       "dme <数量|all> | dme others on|off",
		UsageEN:     "dme <count|all> | dme others on|off",
		Plugin:      p.Name(),
		Category:    "tools",
		OwnerOnly:   true,
		Handler:     p.handleDme,
	})
}

func (p *PrunePlugin) Start(_ context.Context) error { return nil }
func (p *PrunePlugin) Stop(_ context.Context) error  { return nil }

func (p *PrunePlugin) load() {
	data, err := os.ReadFile(dmeFile)
	if err != nil {
		return
	}
	var st struct {
		Others bool `json:"others"`
	}
	if json.Unmarshal(data, &st) == nil {
		p.others = st.Others
	}
}

func (p *PrunePlugin) save() {
	_ = os.MkdirAll(filepath.Dir(dmeFile), 0o700)
	data, _ := json.Marshal(map[string]bool{"others": p.others})
	_ = os.WriteFile(dmeFile, data, 0o600)
}

func (p *PrunePlugin) othersOn() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.others
}

func (p *PrunePlugin) help(ctx *interfaces.CommandContext) error {
	state := ctx.Tlocal("关", "off")
	if p.othersOn() {
		state = ctx.Tlocal("开", "on")
	}
	return ctx.Edit(ctx.Tlocal(
		`🗑 <b>dme</b> 删除当前聊天的消息

<code>dme 10</code>  删最近 10 条
<code>dme all</code>  删全部
<code>dme others on</code>  连别人的消息一起删（需要管理员权限）
<code>dme others off</code>  只删自己的

删除他人: <b>`+state+`</b>
自己 48 小时内的消息会先被改成空白再删，对方的防撤回客户端也只能留下空白。`,
		`🗑 <b>dme</b> delete messages in this chat

<code>dme 10</code>  delete the last 10
<code>dme all</code>  delete everything
<code>dme others on</code>  include other people's messages (needs admin)
<code>dme others off</code>  only your own

Delete others: <b>`+state+`</b>
Your messages from the last 48h are blanked before deletion, so anti-recall clients only keep a blank.`))
}

func (p *PrunePlugin) handleDme(ctx *interfaces.CommandContext) error {
	arg := strings.ToLower(ctx.GetArg(0))
	switch arg {
	case "", "help", "h":
		return p.help(ctx)
	case "others":
		return p.setOthers(ctx)
	}

	count := dmeAll
	if arg != "all" {
		n, err := parseInt(arg)
		if err != nil || n < 1 || n > dmeMaxCount {
			return ctx.Edit(ctx.Tlocal(
				fmt.Sprintf("数量要写 1–%d，或者 <code>dme all</code>", dmeMaxCount),
				fmt.Sprintf("Count must be 1-%d, or <code>dme all</code>", dmeMaxCount)))
		}
		count = n
	}

	chatID := ctx.Message.ChatID
	p.mu.Lock()
	if p.active[chatID] {
		p.mu.Unlock()
		return ctx.Edit(ctx.Tlocal("⏳ 这个聊天已经在删了，等它结束", "⏳ Already deleting in this chat"))
	}
	p.active[chatID] = true
	others := p.others
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		delete(p.active, chatID)
		p.mu.Unlock()
	}()

	return p.run(ctx, count, others)
}

func (p *PrunePlugin) setOthers(ctx *interfaces.CommandContext) error {
	switch strings.ToLower(ctx.GetArg(1)) {
	case "on":
		p.mu.Lock()
		p.others = true
		p.save()
		p.mu.Unlock()
		return ctx.Edit(ctx.Tlocal(
			"✅ 删除他人消息已开启，之后 dme 会连别人的消息一起删（需要管理员权限）",
			"✅ Deleting others is on; dme now includes other people's messages (needs admin)"))
	case "off":
		p.mu.Lock()
		p.others = false
		p.save()
		p.mu.Unlock()
		return ctx.Edit(ctx.Tlocal("⏸️ 删除他人消息已关闭，只删自己的", "⏸️ Deleting others is off; only your own"))
	default:
		return p.help(ctx)
	}
}

type dmeStats struct{ deleted, blanked, failed int }

func (p *PrunePlugin) run(ctx *interfaces.CommandContext, count int, others bool) error {
	peer, err := ctx.ResolvePeer()
	if err != nil {
		return ctx.Edit("❌ " + err.Error())
	}
	cmdID := ctx.Message.Message.ID
	// The command message goes first so it never shows the progress.
	_ = deleteMessages(ctx, peer, []int{cmdID})

	blank := &blanker{ctx: ctx, peer: peer}
	var st dmeStats
	offset := cmdID
	for count == dmeAll || st.deleted < count {
		page, next, err := pageHistory(ctx, peer, offset, dmePageSize)
		if err != nil {
			ctx.Logger.Warn("dme history", "error", err)
			break
		}
		if len(page) == 0 {
			break
		}
		var ids []int
		for _, m := range page {
			if count != dmeAll && st.deleted+len(ids) >= count {
				break
			}
			mine := isMine(m, ctx.SelfID)
			if !mine && !others {
				continue
			}
			if mine && blank.eligible(m) {
				if blank.wipe(m) {
					st.blanked++
				}
				time.Sleep(dmeEditDelay)
			}
			ids = append(ids, m.ID)
		}
		for i := 0; i < len(ids); i += dmeBatchSize {
			end := min(i+dmeBatchSize, len(ids))
			if err := deleteMessages(ctx, peer, ids[i:end]); err != nil {
				ctx.Logger.Warn("dme delete", "error", err)
				st.failed += end - i
				continue
			}
			st.deleted += end - i
			time.Sleep(dmeBatchDelay)
		}
		if next == 0 {
			break
		}
		offset = next
	}
	ctx.Logger.Info("dme done", "chat", ctx.Message.ChatID, "deleted", st.deleted, "blanked", st.blanked, "failed", st.failed)
	return nil
}

// pageHistory returns one page of messages older than offsetID plus the
// next offset (0 when history is exhausted).
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
		if msg, ok := m.(*tg.Message); ok && msg.ID < offsetID {
			msgs = append(msgs, msg)
		}
	}
	if next >= offsetID || len(list) == 0 {
		next = 0
	}
	return msgs, next, nil
}

// isMine reports whether the account sent m. Out covers channel-identity
// posts in supergroups too.
func isMine(m *tg.Message, selfID int64) bool {
	if m.Out {
		return true
	}
	pu, ok := m.FromID.(*tg.PeerUser)
	return ok && selfID != 0 && pu.UserID == selfID
}

// blanker overwrites own messages before deletion.
type blanker struct {
	ctx   *interfaces.CommandContext
	peer  tg.InputPeerClass
	photo tg.InputPhotoClass // reused after the first upload
	off   bool               // upload failed; skip media
}

func (b *blanker) eligible(m *tg.Message) bool {
	if m.Date > 0 && time.Since(time.Unix(int64(m.Date), 0)) > dmeEditWindow {
		return false
	}
	switch media := m.Media.(type) {
	case nil, *tg.MessageMediaWebPage:
		return m.Message != "" && m.Message != dmePlaceholder
	case *tg.MessageMediaPhoto:
		return true
	case *tg.MessageMediaDocument:
		// Stickers, voice and round videos cannot be edited into a photo.
		if doc, ok := media.Document.(*tg.Document); ok {
			for _, a := range doc.Attributes {
				switch a.(type) {
				case *tg.DocumentAttributeSticker, *tg.DocumentAttributeAudio:
					return false
				case *tg.DocumentAttributeVideo:
					if v := a.(*tg.DocumentAttributeVideo); v.RoundMessage {
						return false
					}
				}
			}
		}
		return true
	default:
		return false
	}
}

func (b *blanker) wipe(m *tg.Message) bool {
	req := &tg.MessagesEditMessageRequest{Peer: b.peer, ID: m.ID, Message: dmePlaceholder}
	if m.Media != nil {
		if _, isWeb := m.Media.(*tg.MessageMediaWebPage); !isWeb {
			media := b.media()
			if media == nil {
				return false
			}
			req.SetMedia(media)
		}
	}
	upd, err := b.ctx.API.MessagesEditMessage(b.ctx.Context(), req)
	if err != nil {
		return false
	}
	if b.photo == nil {
		b.photo = photoFromUpdates(upd)
	}
	return true
}

// media returns the blank image, uploading it once per run.
func (b *blanker) media() tg.InputMediaClass {
	if b.photo != nil {
		return &tg.InputMediaPhoto{ID: b.photo}
	}
	if b.off {
		return nil
	}
	file, err := uploader.NewUploader(b.ctx.API).FromBytes(b.ctx.Context(), "blank.png", blankPNG())
	if err != nil {
		b.off = true
		return nil
	}
	return &tg.InputMediaUploadedPhoto{File: file}
}

// photoFromUpdates extracts the edited photo so later edits reuse it
// instead of uploading again.
func photoFromUpdates(u tg.UpdatesClass) tg.InputPhotoClass {
	upds, ok := u.(*tg.Updates)
	if !ok {
		return nil
	}
	for _, up := range upds.Updates {
		var msg tg.MessageClass
		switch e := up.(type) {
		case *tg.UpdateEditMessage:
			msg = e.Message
		case *tg.UpdateEditChannelMessage:
			msg = e.Message
		}
		if m, ok := msg.(*tg.Message); ok {
			if mp, ok := m.Media.(*tg.MessageMediaPhoto); ok {
				if ph, ok := mp.Photo.(*tg.Photo); ok {
					return &tg.InputPhoto{ID: ph.ID, AccessHash: ph.AccessHash, FileReference: ph.FileReference}
				}
			}
		}
	}
	return nil
}

var (
	blankOnce sync.Once
	blankData []byte
)

// blankPNG is a small white square used to overwrite media.
func blankPNG() []byte {
	blankOnce.Do(func() {
		img := image.NewGray(image.Rect(0, 0, 64, 64))
		for i := range img.Pix {
			img.Pix[i] = 0xff
		}
		var buf bytes.Buffer
		_ = png.Encode(&buf, img)
		blankData = buf.Bytes()
	})
	return blankData
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
