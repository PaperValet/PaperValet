package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// SavePlugin forwards messages to a configurable target, bypassing
// forward restrictions via direct API calls.
type SavePlugin struct {
	dbFile string
	db     *SaveDB
}

type SaveDB struct {
	Users map[string]UserConfig `json:"users"`
}

type UserConfig struct {
	Target     string `json:"target"`      // @user, chatID, "me"
	ShowSource bool   `json:"show_source"` // Whether to append a source link
}

func New() (plugin.Plugin, error) {
	return &SavePlugin{
		dbFile: "data/save_db.json",
		db:     &SaveDB{Users: make(map[string]UserConfig)},
	}, nil
}

var Metadata = &plugin.PluginMetadata{
	Name:        "save",
	Description: "突破限制保存/转发消息",
	Version:     "1.0.0",
	Author:      "PaperValet",
	MinVersion:  "0.1.0",
}

func (p *SavePlugin) Name() string        { return "save" }
func (p *SavePlugin) Description() string { return "突破限制保存/转发消息" }

func (p *SavePlugin) Init(_ context.Context, mgr plugin.Manager) error {
	p.loadDB()
	return mgr.RegisterCommand(&plugin.Command{
		Name:        "save",
		Description: "保存/转发消息到指定目标",
		Usage:       "save（回复）| save <链接…> | save to <目标> | save source on|off",
		Plugin:      p.Name(),
		Category:    "tools",
		OwnerOnly:   true,
		Handler:     p.handleSave,
	})
}

func (p *SavePlugin) Start(_ context.Context) error { return nil }
func (p *SavePlugin) Stop(_ context.Context) error  { return nil }

func (p *SavePlugin) loadDB() {
	data, err := os.ReadFile(p.dbFile)
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, &p.db)
	if p.db.Users == nil {
		p.db.Users = make(map[string]UserConfig)
	}
}

func (p *SavePlugin) saveDB() {
	_ = os.MkdirAll(filepath.Dir(p.dbFile), 0o755)
	data, _ := json.MarshalIndent(p.db, "", "  ")
	_ = os.WriteFile(p.dbFile, data, 0o600)
}

func (p *SavePlugin) handleSave(ctx *plugin.CommandContext) error {
	args := ctx.Args
	if len(args) == 0 {
		if ctx.Message != nil && ctx.Message.IsReply {
			return p.forwardMessage(ctx, "", ctx.Message.ChatID, []int{ctx.Message.ReplyToID})
		}
		return ctx.Edit(p.helpText())
	}

	sub := args[0]
	switch sub {
	case "to", "target":
		if len(args) < 2 {
			return p.showTarget(ctx)
		}
		return p.setTarget(ctx, args[1])
	case "source":
		if len(args) < 2 {
			return p.showSource(ctx)
		}
		return p.setSource(ctx, args[1])
	case "help", "h":
		return ctx.Edit(p.helpText())
	default:
		if isTmeLink(sub) {
			return p.handleLinks(ctx, args)
		}
		// Reply + custom target override.
		if ctx.Message != nil && ctx.Message.IsReply {
			return p.forwardMessage(ctx, args[0], ctx.Message.ChatID, []int{ctx.Message.ReplyToID})
		}
		return ctx.Edit(fmt.Sprintf("未知参数: %s\n\n%s", sub, p.helpText()))
	}
}

func (p *SavePlugin) helpText() string {
	return `💾 <b>save — 突破限制保存 / 转发消息</b>

<b>命令:</b>
• <code>save</code> — 回复消息：转发到默认目标
• <code>save <@user|chatID></code> — 回复消息：临时改目标
• <code>save <链接…></code> — 批量转发链接
• <code>save <链接1> <链接2></code> — 转发两链接之间的消息范围（上限100）

<b>设置:</b>
• <code>save to <@user|chatID|me></code> — 默认目标
• <code>save target</code> — 查看默认目标
• <code>save source on|off</code> — 转发后来源链接`
}

func (p *SavePlugin) getUserConfig(userID string) UserConfig {
	if config, ok := p.db.Users[userID]; ok {
		return config
	}
	return UserConfig{Target: "me", ShowSource: false}
}

func (p *SavePlugin) setUserConfig(userID string, config UserConfig) {
	p.db.Users[userID] = config
	p.saveDB()
}

func (p *SavePlugin) setTarget(ctx *plugin.CommandContext, target string) error {
	if target != "me" && !strings.HasPrefix(target, "@") {
		if _, err := strconv.ParseInt(target, 10, 64); err != nil {
			return ctx.Edit("❌ 目标无效: 支持 @用户名、数字 chatID 或 me")
		}
	}
	userID := fmt.Sprintf("%d", ctx.Message.UserID)
	config := p.getUserConfig(userID)
	config.Target = target
	p.setUserConfig(userID, config)

	display := target
	if target == "me" {
		display = "收藏夹"
	}
	return ctx.Edit(fmt.Sprintf("✅ 默认保存目标已设为: <code>%s</code>", display))
}

func (p *SavePlugin) showTarget(ctx *plugin.CommandContext) error {
	userID := fmt.Sprintf("%d", ctx.Message.UserID)
	config := p.getUserConfig(userID)
	return ctx.Edit(fmt.Sprintf("📍 当前默认目标: <code>%s</code>\n\n使用 <code>save to <目标></code> 修改", config.Target))
}

func (p *SavePlugin) setSource(ctx *plugin.CommandContext, value string) error {
	userID := fmt.Sprintf("%d", ctx.Message.UserID)
	config := p.getUserConfig(userID)
	switch strings.ToLower(value) {
	case "on", "true", "1", "yes":
		config.ShowSource = true
	case "off", "false", "0", "no":
		config.ShowSource = false
	default:
		return ctx.Edit("用法: save source on|off")
	}
	p.setUserConfig(userID, config)
	state := "关闭"
	if config.ShowSource {
		state = "开启"
	}
	return ctx.Edit(fmt.Sprintf("✅ 来源链接显示: %s", state))
}

func (p *SavePlugin) showSource(ctx *plugin.CommandContext) error {
	userID := fmt.Sprintf("%d", ctx.Message.UserID)
	config := p.getUserConfig(userID)
	state := "关闭"
	if config.ShowSource {
		state = "开启"
	}
	return ctx.Edit(fmt.Sprintf("🔗 来源链接显示: %s\n\n使用 <code>save source on|off</code> 修改", state))
}

// resolveTarget converts "me" / @username / numeric chatID into an InputPeer.
func (p *SavePlugin) resolveTarget(ctx *plugin.CommandContext, target string) (tg.InputPeerClass, error) {
	if target == "" || target == "me" {
		return ctx.PeerResolver.ResolveFromChatID(ctx.Context(), ctx.Message.UserID)
	}
	if strings.HasPrefix(target, "@") && len(target) > 1 {
		return ctx.PeerResolver.ResolveUsername(ctx.Context(), target[1:])
	}
	id, err := strconv.ParseInt(target, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("无效的目标: %s", target)
	}
	return ctx.PeerResolver.ResolveFromChatID(ctx.Context(), id)
}

// forwardMessage forwards message IDs from sourceChat to the target.
func (p *SavePlugin) forwardMessage(ctx *plugin.CommandContext, targetOverride string, sourceChatID int64, ids []int) error {
	userID := fmt.Sprintf("%d", ctx.Message.UserID)
	config := p.getUserConfig(userID)

	target := config.Target
	if targetOverride != "" {
		target = targetOverride
	}

	destPeer, err := p.resolveTarget(ctx, target)
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 解析目标失败: %v", err))
	}
	fromPeer, err := ctx.PeerResolver.ResolveFromChatID(ctx.Context(), sourceChatID)
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 解析来源失败: %v", err))
	}

	_, err = ctx.API.MessagesForwardMessages(ctx.Context(), &tg.MessagesForwardMessagesRequest{
		FromPeer:   fromPeer,
		ID:         ids,
		ToPeer:     destPeer,
		DropAuthor: true,
	})
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 转发失败: %v", err))
	}

	if config.ShowSource && len(ids) > 0 {
		_, _ = ctx.API.MessagesSendMessage(ctx.Context(), &tg.MessagesSendMessageRequest{
			Peer:     destPeer,
			Message:  sourceLink(sourceChatID, ids[0]),
			RandomID: randomID(),
		})
	}

	display := target
	if target == "me" || target == "" {
		display = "收藏夹"
	}
	return ctx.Edit(fmt.Sprintf("✅ 已转发 %d 条消息到 <code>%s</code>", len(ids), display))
}

// sourceLink builds a t.me/c/ link for channel/supergroup sources.
func sourceLink(chatID int64, msgID int) string {
	if chatID <= -1000000000000 {
		return fmt.Sprintf("📎 来源: https://t.me/c/%d/%d", -1000000000000-chatID, msgID)
	}
	return fmt.Sprintf("📎 来源: chat %d, msg %d", chatID, msgID)
}

// msgLink is one parsed t.me message link.
type msgLink struct {
	username string // public chat username, if any
	chatID   int64  // private channel ID (raw, without -100 prefix)
	msgID    int
}

func isTmeLink(s string) bool {
	return strings.HasPrefix(s, "https://t.me/") || strings.HasPrefix(s, "t.me/")
}

func parseTmeLink(s string) (*msgLink, error) {
	s = strings.TrimPrefix(s, "https://t.me/")
	s = strings.TrimPrefix(s, "t.me/")
	parts := strings.Split(strings.Trim(s, "/"), "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("无法解析链接")
	}
	msgID, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil || msgID <= 0 {
		return nil, fmt.Errorf("链接中缺少有效的消息 ID")
	}
	if parts[0] == "c" && len(parts) >= 3 {
		chatID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("无效的频道 ID")
		}
		return &msgLink{chatID: chatID, msgID: msgID}, nil
	}
	return &msgLink{username: parts[0], msgID: msgID}, nil
}

// resolveLinkPeer resolves the chat a link points to.
func (p *SavePlugin) resolveLinkPeer(ctx *plugin.CommandContext, l *msgLink) (tg.InputPeerClass, error) {
	if l.username != "" {
		return ctx.PeerResolver.ResolveUsername(ctx.Context(), l.username)
	}
	return ctx.PeerResolver.ResolveFromChatID(ctx.Context(), -1000000000000-l.chatID)
}

func (p *SavePlugin) handleLinks(ctx *plugin.CommandContext, args []string) error {
	var links []*msgLink
	for _, arg := range args {
		if !isTmeLink(arg) {
			continue
		}
		l, err := parseTmeLink(arg)
		if err != nil {
			return ctx.Edit(fmt.Sprintf("❌ %s: %v", arg, err))
		}
		links = append(links, l)
	}
	if len(links) == 0 {
		return ctx.Edit("❌ 未识别到有效的 t.me 链接")
	}

	// Range mode: two links in the same chat.
	if len(links) == 2 && links[0].username == links[1].username && links[0].chatID == links[1].chatID && links[0].msgID != links[1].msgID {
		lo, hi := links[0].msgID, links[1].msgID
		if lo > hi {
			lo, hi = hi, lo
		}
		if hi-lo > 100 {
			return ctx.Edit("❌ 范围过大，最多 100 条")
		}
		peer, err := p.resolveLinkPeer(ctx, links[0])
		if err != nil {
			return ctx.Edit(fmt.Sprintf("❌ 解析来源失败: %v", err))
		}
		var ids []int
		for id := lo; id <= hi; id++ {
			ids = append(ids, id)
		}
		return p.forwardFromPeer(ctx, "", peer, ids)
	}

	// Single / batch mode.
	sent := 0
	for _, l := range links {
		peer, err := p.resolveLinkPeer(ctx, l)
		if err != nil {
			continue
		}
		if err := p.forwardFromPeer(ctx, "", peer, []int{l.msgID}); err != nil {
			return err
		}
		sent++
	}
	if sent == 0 {
		return ctx.Edit("❌ 没有链接转发成功")
	}
	return nil
}

// forwardFromPeer forwards by resolved peer (used by link mode).
func (p *SavePlugin) forwardFromPeer(ctx *plugin.CommandContext, targetOverride string, fromPeer tg.InputPeerClass, ids []int) error {
	userID := fmt.Sprintf("%d", ctx.Message.UserID)
	config := p.getUserConfig(userID)
	target := config.Target
	if targetOverride != "" {
		target = targetOverride
	}

	destPeer, err := p.resolveTarget(ctx, target)
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 解析目标失败: %v", err))
	}
	_, err = ctx.API.MessagesForwardMessages(ctx.Context(), &tg.MessagesForwardMessagesRequest{
		FromPeer:   fromPeer,
		ID:         ids,
		ToPeer:     destPeer,
		DropAuthor: true,
	})
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 转发失败: %v", err))
	}
	display := target
	if target == "me" || target == "" {
		display = "收藏夹"
	}
	return ctx.Edit(fmt.Sprintf("✅ 已转发 %d 条消息到 <code>%s</code>", len(ids), display))
}

func randomID() int64 {
	return time.Now().UnixNano()
}
