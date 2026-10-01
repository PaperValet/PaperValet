package builtin

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

//go:embed assets/logo.jpg
var logoJPG []byte

const (
	dmePageSize    = 100
	dmeBatchSize   = 100
	dmeMaxCount    = 5000
	dmeEditWindow  = 48 * time.Hour // Telegram refuses edits after this
	dmeBatchDelay  = 300 * time.Millisecond
	dmeEditDelay   = 50 * time.Millisecond
	dmeSummaryHold = 5 * time.Second
	dmeAll         = -1
)

// Anti-recall placeholder written over own messages before deletion.
const (
	dmeNoticeZH = "这条消息在删除前被替换为该内容以反制防撤回"
	dmeNoticeEN = "This message was replaced with this text before deletion to defeat anti-recall"
)

// PrunePlugin deletes messages in the current chat, TeleBox-dme style.
type PrunePlugin struct {
	mu     sync.Mutex
	active map[int64]bool
}

func NewPrune() *PrunePlugin { return &PrunePlugin{active: make(map[int64]bool)} }

func (p *PrunePlugin) Name() string        { return "dme" }
func (p *PrunePlugin) Description() string { return "批量删除消息" }
func (p *PrunePlugin) DescEN() string      { return "Bulk-delete messages" }

func (p *PrunePlugin) Init(_ context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "dme",
		Description: "批量删除消息",
		DescEN:      "Bulk-delete messages",
		Usage: "dme <数量|all> [-f]\n" +
			"\n" +
			"**参数**\n" +
			"• `数量`  删除最近 N 条（1–5000）\n" +
			"• `all`  删除本聊天全部可见消息\n" +
			"• `-f`  连别人的消息一起删（需要删除权限）\n" +
			"\n" +
			"**示例**\n" +
			"• `dme 10`  删自己最近 10 条\n" +
			"• `dme all`  删自己在这里的全部消息\n" +
			"• `dme 50 -f`  删最近 50 条，不分是谁发的\n" +
			"\n" +
			"**机制**\n" +
			"• 从命令往前翻历史，只处理命令之前的消息\n" +
			"• 自己的消息（48 小时内）先改写再删：文字替换为一句提示，图片和文件换成 PaperValet logo，对方防撤回客户端只能留下替换后的内容\n" +
			"• 超过 48 小时的消息 Telegram 不允许编辑，直接删除\n" +
			"• 贴纸、语音、圆视频无法改写，直接删除\n" +
			"• 每 100 条一批删除，失败的计入失败数继续往下\n" +
			"• 完成后命令消息变成总结（删了多少、用时多少），5 秒后自动删除\n" +
			"• 同一聊天同时只跑一个 dme",
		UsageEN: "dme <count|all> [-f]\n" +
			"\n" +
			"**Arguments**\n" +
			"• `count`  delete the last N (1-5000)\n" +
			"• `all`  delete everything visible in this chat\n" +
			"• `-f`  include other people's messages (needs delete rights)\n" +
			"\n" +
			"**Examples**\n" +
			"• `dme 10`  delete your last 10\n" +
			"• `dme all`  delete all of your messages here\n" +
			"• `dme 50 -f`  delete the last 50 regardless of sender\n" +
			"\n" +
			"**How it works**\n" +
			"• Walks history backwards from the command; only earlier messages are touched\n" +
			"• Your own messages (last 48h) are overwritten first: text becomes a notice, photos and files become the PaperValet logo, so anti-recall clients only keep the replacement\n" +
			"• Messages older than 48h cannot be edited and are deleted directly\n" +
			"• Stickers, voice and round videos cannot be rewritten and are deleted directly\n" +
			"• Deletes in batches of 100; failures are counted and skipped\n" +
			"• When done, the command turns into a summary (count and time), removed after 5 seconds\n" +
			"• One dme run per chat at a time",
		Plugin:    p.Name(),
		Category:  "tools",
		OwnerOnly: true,
		Handler:   p.handleDme,
	})
}

func (p *PrunePlugin) Start(_ context.Context) error { return nil }
func (p *PrunePlugin) Stop(_ context.Context) error  { return nil }

// parseDmeArgs reads "<count|all> [-f]" in any order.
func parseDmeArgs(args []string) (count int, force bool, err error) {
	count = 0
	for _, a := range args {
		switch strings.ToLower(a) {
		case "-f", "--force":
			force = true
		case "all":
			count = dmeAll
		default:
			n, e := parseInt(a)
			if e != nil || n < 1 || n > dmeMaxCount {
				return 0, false, fmt.Errorf("bad count")
			}
			count = n
		}
	}
	if count == 0 {
		return 0, false, fmt.Errorf("missing count")
	}
	return count, force, nil
}

func (p *PrunePlugin) handleDme(ctx *interfaces.CommandContext) error {
	count, force, err := parseDmeArgs(ctx.Args)
	if err != nil {
		return ctx.Edit(ctx.Tlocal(
			fmt.Sprintf("用法: `dme 10`、`dme all`，加 `-f` 连别人的一起删\n数量 1–%d，详细说明见 `help dme`", dmeMaxCount),
			fmt.Sprintf("Usage: `dme 10`, `dme all`; add `-f` to include others\nCount 1-%d; see `help dme`", dmeMaxCount)))
	}

	chatID := ctx.Message.ChatID
	p.mu.Lock()
	if p.active[chatID] {
		p.mu.Unlock()
		return ctx.Edit(ctx.Tlocal("⏳ 这个聊天已经在删了，等它结束", "⏳ Already deleting in this chat"))
	}
	p.active[chatID] = true
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		delete(p.active, chatID)
		p.mu.Unlock()
	}()

	return p.run(ctx, count, force)
}

type dmeStats struct{ deleted, rewritten, failed int }

func (p *PrunePlugin) run(ctx *interfaces.CommandContext, count int, force bool) error {
	peer, err := ctx.ResolvePeer()
	if err != nil {
		return ctx.Edit("❌ " + esc(err.Error()))
	}
	start := time.Now()
	cmdID := ctx.Message.Message.ID
	notice := ctx.Tlocal(dmeNoticeZH, dmeNoticeEN)
	rw := &rewriter{ctx: ctx, peer: peer, notice: notice}

	var st dmeStats
	offset := cmdID
	for count == dmeAll || st.deleted+st.failed < count {
		page, next, err := pageHistory(ctx, peer, offset, dmePageSize)
		if err != nil {
			ctx.Logger.Warn("dme history", "error", err)
			break
		}
		var ids []int
		for _, m := range page {
			if count != dmeAll && st.deleted+st.failed+len(ids) >= count {
				break
			}
			mine := isMine(m, ctx.SelfID)
			if !mine && !force {
				continue
			}
			if mine && rw.eligible(m) {
				if rw.rewrite(m) {
					st.rewritten++
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

	elapsed := time.Since(start).Round(100 * time.Millisecond)
	summary := ctx.Tlocal(
		fmt.Sprintf("🗑 已删除 %d 条，用时 %s", st.deleted, elapsed),
		fmt.Sprintf("🗑 Deleted %d in %s", st.deleted, elapsed))
	if st.failed > 0 {
		summary += ctx.Tlocal(fmt.Sprintf("，%d 条删不掉", st.failed), fmt.Sprintf(", %d failed", st.failed))
	}
	_ = ctx.Edit(summary)
	time.Sleep(dmeSummaryHold)
	_ = deleteMessages(ctx, peer, []int{cmdID})
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

// rewriter overwrites own messages before deletion so anti-recall clients
// only keep the replacement.
type rewriter struct {
	ctx    *interfaces.CommandContext
	peer   tg.InputPeerClass
	notice string
	photo  tg.InputPhotoClass // reused after the first upload
	off    bool               // upload failed; skip media
}

func (r *rewriter) eligible(m *tg.Message) bool {
	if m.Date > 0 && time.Since(time.Unix(int64(m.Date), 0)) > dmeEditWindow {
		return false
	}
	switch media := m.Media.(type) {
	case nil, *tg.MessageMediaWebPage:
		return m.Message != "" && m.Message != r.notice
	case *tg.MessageMediaPhoto:
		return true
	case *tg.MessageMediaDocument:
		// Stickers, voice and round videos cannot become a photo.
		if doc, ok := media.Document.(*tg.Document); ok {
			for _, a := range doc.Attributes {
				switch v := a.(type) {
				case *tg.DocumentAttributeSticker:
					return false
				case *tg.DocumentAttributeAudio:
					if v.Voice {
						return false
					}
				case *tg.DocumentAttributeVideo:
					if v.RoundMessage {
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

func (r *rewriter) rewrite(m *tg.Message) bool {
	req := &tg.MessagesEditMessageRequest{Peer: r.peer, ID: m.ID, Message: r.notice}
	if m.Media != nil {
		if _, isWeb := m.Media.(*tg.MessageMediaWebPage); !isWeb {
			media := r.media()
			if media == nil {
				return false
			}
			req.SetMedia(media)
		}
	}
	upd, err := r.ctx.API.MessagesEditMessage(r.ctx.Context(), req)
	if err != nil {
		return false
	}
	if r.photo == nil {
		r.photo = photoFromUpdates(upd)
	}
	return true
}

// media returns the logo, uploading it once per run.
func (r *rewriter) media() tg.InputMediaClass {
	if r.photo != nil {
		return &tg.InputMediaPhoto{ID: r.photo}
	}
	if r.off {
		return nil
	}
	file, err := uploader.NewUploader(r.ctx.API).FromBytes(r.ctx.Context(), "papervalet.jpg", logoJPG)
	if err != nil {
		r.off = true
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
