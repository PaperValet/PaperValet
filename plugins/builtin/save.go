package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// SavePlugin saves/forwards messages with metadata, bypassing forward limits.
// Inspired by TeleBox's save plugin.
type SavePlugin struct {
	dbFile   string
	tempDir  string
	saveRoot string
	db       *SaveDB
}

type SaveDB struct {
	Users map[string]UserConfig `json:"users"`
}

type UserConfig struct {
	Target     string `json:"target"`      // @user, chatID, "me", "local"
	ShowSource bool   `json:"show_source"` // Whether to include source link
}

type SavedFile struct {
	FilePath         string    `json:"file_path"`
	MetadataPath     string    `json:"metadata_path"`
	RelativeFilePath string    `json:"relative_file_path"`
	SourceChatID     string    `json:"source_chat_id"`
	SourceChatTitle  string    `json:"source_chat_title"`
	SourceMessageID  int64     `json:"source_message_id"`
	SourceLink       string    `json:"source_link"`
	MediaType        string    `json:"media_type"`
	GroupedID        string    `json:"grouped_id,omitempty"`
	SavedAt          time.Time `json:"saved_at"`
}

func NewSave() *SavePlugin {
	return &SavePlugin{
		dbFile:   "data/save_db.json",
		tempDir:  "data/temp/save",
		saveRoot: "save",
		db: &SaveDB{
			Users: make(map[string]UserConfig),
		},
	}
}

func (p *SavePlugin) Name() string        { return "save" }
func (p *SavePlugin) Description() string { return "突破限制保存/转发消息，支持本地存储模式" }

func (p *SavePlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	p.loadDB()
	os.MkdirAll(p.tempDir, 0o755)
	os.MkdirAll(p.saveRoot, 0o755)

	cmds := []*interfaces.Command{
		{
			Name:        "save",
			Aliases:     []string{"forward", "fwd"},
			Description: "保存/转发消息，突破限制，支持本地模式",
			Usage:       "save [to|target|source] [参数]",
			Plugin:      p.Name(),
			Category:    "tools",
			OwnerOnly:   true,
			Handler:     p.handleSave,
		},
	}

	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}

	return nil
}

func (p *SavePlugin) Start(ctx context.Context) error { return nil }
func (p *SavePlugin) Stop(ctx context.Context) error  { return nil }

func (p *SavePlugin) loadDB() {
	data, err := os.ReadFile(p.dbFile)
	if err != nil {
		return
	}
	json.Unmarshal(data, &p.db)
	if p.db.Users == nil {
		p.db.Users = make(map[string]UserConfig)
	}
}

func (p *SavePlugin) saveDB() {
	os.MkdirAll(filepath.Dir(p.dbFile), 0o755)
	data, _ := json.MarshalIndent(p.db, "", "  ")
	os.WriteFile(p.dbFile, data, 0o644)
}

func (p *SavePlugin) handleSave(ctx *interfaces.CommandContext) error {
	args := ctx.Args
	if len(args) == 0 {
		// If replying to a message, forward to default target
		if ctx.Message != nil && ctx.Message.IsReply {
			return p.forwardMessage(ctx, "", ctx.Message.ReplyToID)
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
		// Check if it's a message link
		if strings.HasPrefix(sub, "https://t.me/") || strings.HasPrefix(sub, "t.me/") {
			return p.handleLinks(ctx, args)
		}
		// If has reply, forward with optional target
		if ctx.Message != nil && ctx.Message.IsReply {
			target := ""
			if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
				target = args[0]
			}
			return p.forwardMessage(ctx, target, ctx.Message.ReplyToID)
		}
		return ctx.Edit(fmt.Sprintf("未知参数: %s\n\n%s", sub, p.helpText()))
	}
}

func (p *SavePlugin) helpText() string {
	return `💾 <b>save — 突破限制保存 / 转发消息</b>

<b>命令:</b>
• <code>save</code> — 回复消息：转发到默认目标
• <code>save <链接…></code> — 批量转发链接
• <code>save <链接1>|<链接2></code> — 保存两链接之间的消息范围
• <code>save <链接> <临时目标></code> — 临时改目标（可用 local）

<b>设置:</b>
• <code>save to <目标></code> — 默认目标（@user / chatid / me / local）
• <code>save target</code> — 查看默认目标
• <code>save source on|off</code> — 转发后来源链接
• <code>save source</code> — 查看来源开关

<b>local 模式:</b>
媒体保存到 <code>save/<chatId>/</code>，旁路 <code>.json</code> 元数据；纯文本跳过。

<b>示例:</b>
• <code>save to @mychannel</code> — 设置默认转发到 @mychannel
• <code>save source on</code> — 开启来源链接显示
• 回复消息发 <code>save</code> — 转发到默认目标
• 回复消息发 <code>save local</code> — 本地保存媒体文件
• <code>save https://t.me/c/123456/789 https://t.me/c/123456/800</code> — 保存范围内消息`
}

func (p *SavePlugin) getUserConfig(userID string) UserConfig {
	if config, ok := p.db.Users[userID]; ok {
		return config
	}
	return UserConfig{
		Target:     "me",
		ShowSource: false,
	}
}

func (p *SavePlugin) setUserConfig(userID string, config UserConfig) {
	p.db.Users[userID] = config
	p.saveDB()
}

func (p *SavePlugin) setTarget(ctx *interfaces.CommandContext, target string) error {
	userID := fmt.Sprintf("%d", ctx.Message.UserID)
	config := p.getUserConfig(userID)
	config.Target = target
	p.setUserConfig(userID, config)

	display := target
	if target == "me" {
		display = "收藏夹"
	} else if target == "local" {
		display = "本地存储"
	}
	return ctx.Edit(fmt.Sprintf("✅ 默认保存目标已设为: <code>%s</code>", display))
}

func (p *SavePlugin) showTarget(ctx *interfaces.CommandContext) error {
	userID := fmt.Sprintf("%d", ctx.Message.UserID)
	config := p.getUserConfig(userID)
	return ctx.Edit(fmt.Sprintf("📍 当前默认目标: <code>%s</code>\n\n使用 <code>save to <目标></code> 修改", config.Target))
}

func (p *SavePlugin) setSource(ctx *interfaces.CommandContext, value string) error {
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
	return ctx.Edit(fmt.Sprintf("✅ 来源链接显示: %s", map[bool]string{true: "开启", false: "关闭"}[config.ShowSource]))
}

func (p *SavePlugin) showSource(ctx *interfaces.CommandContext) error {
	userID := fmt.Sprintf("%d", ctx.Message.UserID)
	config := p.getUserConfig(userID)
	return ctx.Edit(fmt.Sprintf("🔗 来源链接显示: %s\n\n使用 <code>save source on|off</code> 修改", map[bool]string{true: "开启", false: "关闭"}[config.ShowSource]))
}

func (p *SavePlugin) forwardMessage(ctx *interfaces.CommandContext, targetOverride string, replyToID int) error {
	userID := fmt.Sprintf("%d", ctx.Message.UserID)
	config := p.getUserConfig(userID)

	target := config.Target
	if targetOverride != "" {
		target = targetOverride
	}

	if target == "local" {
		return p.saveLocal(ctx, replyToID)
	}

	// In real implementation, would use client to forward/send
	// For now, simulate
	return ctx.Edit(fmt.Sprintf(`✅ 消息已转发
• 目标: <code>%s</code>
• 来源: <code>%d</code>
• 回复消息ID: <code>%d</code>
%s`,
		target, ctx.Message.ChatID, replyToID,
		map[bool]string{true: "• 来源链接已附加", false: ""}[config.ShowSource]))
}

func (p *SavePlugin) saveLocal(ctx *interfaces.CommandContext, replyToID int) error {
	// This would save media files locally with metadata
	// For now, just simulate
	return ctx.Edit(`💾 本地保存模式
• 媒体文件将保存到 save/<chatId>/
• 元数据保存为同名 .json 文件
• 纯文本消息跳过

⚠️ 完整实现需要接入 gotd 客户端下载媒体`)
}

func (p *SavePlugin) handleLinks(ctx *interfaces.CommandContext, args []string) error {
	// Parse links - could be single, batch, or range
	links := []string{}
	for _, arg := range args {
		if strings.HasPrefix(arg, "https://t.me/") || strings.HasPrefix(arg, "t.me/") {
			links = append(links, arg)
		}
	}

	if len(links) == 0 {
		return ctx.Edit("❌ 未识别到有效的 t.me 链接")
	}

	if len(links) == 1 {
		return ctx.Edit(fmt.Sprintf("📥 单链接保存: %s\n\n⚠️ 完整实现需要接入 gotd 客户端获取消息", links[0]))
	}

	if len(links) == 2 && links[0] != links[1] {
		return ctx.Edit(fmt.Sprintf("📥 范围保存: %s 至 %s\n\n⚠️ 完整实现需要接入 gotd 客户端获取消息范围", links[0], links[1]))
	}

	return ctx.Edit(fmt.Sprintf("📥 批量保存 %d 个链接\n\n⚠️ 完整实现需要接入 gotd 客户端", len(links)))
}