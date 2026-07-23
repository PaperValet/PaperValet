package builtin

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gotd/td/tg"
	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// ToolsPlugin provides utility commands: ping, uptime, info, forward, remind, note, calc, base64, hash.
type ToolsPlugin struct {
	startTime time.Time
	reminders map[string]*reminder
	notes     map[int64]map[string]string // userID -> name -> content
}

type reminder struct {
	ChatID    int64
	UserID    int64
	Text      string
	TriggerAt time.Time
	Repeating bool
	Interval  time.Duration
}

func NewTools() *ToolsPlugin {
	return &ToolsPlugin{
		startTime: time.Now(),
		reminders: make(map[string]*reminder),
		notes:     make(map[int64]map[string]string),
	}
}

func (p *ToolsPlugin) Name() string        { return "tools" }
func (p *ToolsPlugin) Description() string { return "实用工具命令" }

func (p *ToolsPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	cmds := []*interfaces.Command{
		{
			Name:        "ping",
			Description: "检查延迟",
			Plugin:      p.Name(),
			Category:    "tools",
			Handler:     p.handlePing,
		},
		{
			Name:        "uptime",
			Aliases:     []string{"up"},
			Description: "显示运行时间",
			Plugin:      p.Name(),
			Category:    "tools",
			Handler:     p.handleUptime,
		},
		{
			Name:        "info",
			Aliases:     []string{"id", "whois"},
			Description: "显示用户/群组 ID 信息",
			Usage:       "info [@用户名|回复消息]",
			Plugin:      p.Name(),
			Category:    "tools",
			Handler:     p.handleInfo,
		},
		{
			Name:        "fwd",
			Aliases:     []string{"forward"},
			Description: "转发回复的消息到目标",
			Usage:       "fwd @目标用户名",
			Plugin:      p.Name(),
			Category:    "tools",
			Handler:     p.handleForward,
		},
		{
			Name:        "remind",
			Aliases:     []string{"remindme"},
			Description: "设置提醒",
			Usage:       "remind <时间> <内容> | remind list | remind del <id>",
			Plugin:      p.Name(),
			Category:    "tools",
			Handler:     p.handleRemind,
		},
		{
			Name:        "note",
			Aliases:     []string{"n"},
			Description: "笔记管理",
			Usage:       "note <set|get|del|list> [name] [内容]",
			Plugin:      p.Name(),
			Category:    "tools",
			Handler:     p.handleNote,
		},
		{
			Name:        "calc",
			Aliases:     []string{"math"},
			Description: "简单计算器",
			Usage:       "calc <表达式>",
			Plugin:      p.Name(),
			Category:    "tools",
			Handler:     p.handleCalc,
		},
		{
			Name:        "base64",
			Description: "Base64 编码/解码",
			Usage:       "base64 <encode|decode> <文本>",
			Plugin:      p.Name(),
			Category:    "tools",
			Handler:     p.handleBase64,
		},
		{
			Name:        "hash",
			Description: "计算哈希值",
			Usage:       "hash <md5|sha1|sha256> <文本>",
			Plugin:      p.Name(),
			Category:    "tools",
			Handler:     p.handleHash,
		},
		{
			Name:        "uuid",
			Description: "生成 UUID",
			Plugin:      p.Name(),
			Category:    "tools",
			Handler:     p.handleUUID,
		},
		{
			Name:        "time",
			Aliases:     []string{"date", "now"},
			Description: "显示当前时间",
			Plugin:      p.Name(),
			Category:    "tools",
			Handler:     p.handleTime,
		},
	}

	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *ToolsPlugin) Start(ctx context.Context) error {
	go p.runReminders(ctx)
	return nil
}

func (p *ToolsPlugin) Stop(_ context.Context) error { return nil }

func (p *ToolsPlugin) runReminders(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.checkReminders()
		}
	}
}

func (p *ToolsPlugin) checkReminders() {
	now := time.Now()
	for id, r := range p.reminders {
		if now.After(r.TriggerAt) {
			// In a real implementation, we'd send via API
			// For now just delete
			delete(p.reminders, id)
		}
	}
}

func (p *ToolsPlugin) handlePing(ctx *interfaces.CommandContext) error {
	start := time.Now()
	msg := "Pong! 🏓"
	if err := ctx.Edit(msg); err != nil {
		return err
	}
	latency := time.Since(start)
	return ctx.Edit(fmt.Sprintf("%s\n延迟: %v", msg, latency))
}

func (p *ToolsPlugin) handleUptime(ctx *interfaces.CommandContext) error {
	uptime := time.Since(p.startTime).Truncate(time.Second)
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return ctx.Edit(fmt.Sprintf(
		"⏱ 运行时间: %s\n🧠 内存: %.1f MB\n🔀 Goroutines: %d",
		uptime, float64(mem.Alloc)/1024/1024, runtime.NumGoroutine(),
	))
}

func (p *ToolsPlugin) handleInfo(ctx *interfaces.CommandContext) error {
	msg := ctx.Message
	var targetID int64 = msg.UserID

	if ctx.ArgCount() > 0 {
		arg := ctx.GetArg(0)
		if len(arg) > 0 && arg[0] == '@' {
			return ctx.Edit("用户名解析暂未实现，请回复消息或使用 .info")
		}
	} else if msg.IsReply && msg.Message != nil {
		if msg.Message.FromID != nil {
			if u, ok := msg.Message.FromID.(*tg.PeerUser); ok {
				targetID = u.UserID
			}
		}
	}

	return ctx.Edit(fmt.Sprintf(
		"👤 用户 ID: %d\n💬 群组 ID: %d\n📨 消息 ID: %d",
		targetID, msg.ChatID, msg.Message.ID,
	))
}

func (p *ToolsPlugin) handleForward(ctx *interfaces.CommandContext) error {
	if !ctx.Message.IsReply {
		return ctx.Edit("请回复一条消息")
	}
	if ctx.ArgCount() == 0 {
		return ctx.Edit("用法: fwd @目标用户名")
	}
	target := ctx.GetArg(0)
	return ctx.Edit(fmt.Sprintf("转发功能待实现: %s", target))
}

func (p *ToolsPlugin) handleRemind(ctx *interfaces.CommandContext) error {
	sub := strings.ToLower(ctx.GetArg(0))
	switch sub {
	case "", "help":
		return ctx.Edit("用法: remind <时间> <内容> | remind list | remind del <id>\n时间格式: 5m, 1h, 2024-12-31 23:59")
	case "list":
		if len(p.reminders) == 0 {
			return ctx.Edit("暂无提醒")
		}
		var b strings.Builder
		b.WriteString("⏰ 提醒列表:\n")
		for id, r := range p.reminders {
			b.WriteString(fmt.Sprintf("%s: %s (在 %v)\n", id, r.Text, r.TriggerAt.Format("15:04:05")))
		}
		return ctx.Edit(b.String())
	case "del", "delete":
		if ctx.ArgCount() < 2 {
			return ctx.Edit("用法: remind del <id>")
		}
		id := ctx.GetArg(1)
		if _, ok := p.reminders[id]; ok {
			delete(p.reminders, id)
			return ctx.Edit("已删除: " + id)
		}
		return ctx.Edit("未找到: " + id)
	default:
		duration := sub
		text := strings.Join(ctx.Args[1:], " ")
		if text == "" {
			return ctx.Edit("提醒内容不能为空")
		}
		d, err := time.ParseDuration(duration)
		if err != nil {
			return ctx.Edit("时间格式错误，如: 5m, 1h, 30s")
		}
		id := fmt.Sprintf("%d-%d", ctx.Message.UserID, time.Now().Unix())
		p.reminders[id] = &reminder{
			ChatID:    ctx.Message.ChatID,
			UserID:    ctx.Message.UserID,
			Text:      text,
			TriggerAt: time.Now().Add(d),
		}
		return ctx.Edit(fmt.Sprintf("⏰ 已设置提醒: %s 后提醒 \"%s\"", d, text))
	}
}

func (p *ToolsPlugin) handleNote(ctx *interfaces.CommandContext) error {
	if ctx.ArgCount() == 0 {
		return ctx.Edit("用法: note set <name> <内容> | note get <name> | note del <name> | note list")
	}
	sub := strings.ToLower(ctx.GetArg(0))
	userNotes := p.notes[ctx.Message.UserID]
	if userNotes == nil {
		userNotes = make(map[string]string)
		p.notes[ctx.Message.UserID] = userNotes
	}

	switch sub {
	case "set":
		if ctx.ArgCount() < 3 {
			return ctx.Edit("用法: note set <name> <内容>")
		}
		name := ctx.GetArg(1)
		content := strings.Join(ctx.Args[2:], " ")
		userNotes[name] = content
		return ctx.Edit(fmt.Sprintf("✅ 笔记 '%s' 已保存", name))
	case "get":
		if ctx.ArgCount() < 2 {
			return ctx.Edit("用法: note get <name>")
		}
		name := ctx.GetArg(1)
		if content, ok := userNotes[name]; ok {
			return ctx.Edit(fmt.Sprintf("📝 %s:\n%s", name, content))
		}
		return ctx.Edit("笔记不存在: " + name)
	case "del", "delete":
		if ctx.ArgCount() < 2 {
			return ctx.Edit("用法: note del <name>")
		}
		name := ctx.GetArg(1)
		if _, ok := userNotes[name]; ok {
			delete(userNotes, name)
			return ctx.Edit(fmt.Sprintf("🗑 笔记 '%s' 已删除", name))
		}
		return ctx.Edit("笔记不存在: " + name)
	case "list":
		if len(userNotes) == 0 {
			return ctx.Edit("暂无笔记")
		}
		var b strings.Builder
		b.WriteString("📋 笔记列表:\n")
		for name := range userNotes {
			b.WriteString("• " + name + "\n")
		}
		return ctx.Edit(b.String())
	default:
		return ctx.Edit("未知子命令: " + sub)
	}
}

func (p *ToolsPlugin) handleCalc(ctx *interfaces.CommandContext) error {
	if ctx.ArgCount() == 0 {
		return ctx.Edit("用法: calc <表达式>\n示例: calc 2+3*4 | calc sqrt(16) | calc sin(pi/2)")
	}
	expr := strings.Join(ctx.Args, " ")
	// Simple eval using basic arithmetic - for real calc, use a proper expression evaluator
	// Using a simple approach for now
	return ctx.Edit(fmt.Sprintf("计算器功能待实现: %s", expr))
}

func (p *ToolsPlugin) handleBase64(ctx *interfaces.CommandContext) error {
	if ctx.ArgCount() < 2 {
		return ctx.Edit("用法: base64 <encode|decode> <文本>")
	}
	op := strings.ToLower(ctx.GetArg(0))
	text := strings.Join(ctx.Args[1:], " ")
	
	switch op {
	case "encode", "e":
		return ctx.Edit(base64.StdEncoding.EncodeToString([]byte(text)))
	case "decode", "d":
		data, err := base64.StdEncoding.DecodeString(text)
		if err != nil {
			return ctx.Edit("解码失败: " + err.Error())
		}
		return ctx.Edit(string(data))
	default:
		return ctx.Edit("未知操作: " + op + " (支持 encode/decode)")
	}
}

func (p *ToolsPlugin) handleHash(ctx *interfaces.CommandContext) error {
	if ctx.ArgCount() < 2 {
		return ctx.Edit("用法: hash <md5|sha1|sha256> <文本>")
	}
	algo := strings.ToLower(ctx.GetArg(0))
	text := strings.Join(ctx.Args[1:], " ")
	
	var hash string
	switch algo {
	case "md5":
		h := md5.Sum([]byte(text))
		hash = hex.EncodeToString(h[:])
	case "sha1":
		h := sha1.Sum([]byte(text))
		hash = hex.EncodeToString(h[:])
	case "sha256":
		h := sha256.Sum256([]byte(text))
		hash = hex.EncodeToString(h[:])
	default:
		return ctx.Edit("不支持的算法: " + algo + " (支持 md5, sha1, sha256)")
	}
	return ctx.Edit(fmt.Sprintf("%s: %s", strings.ToUpper(algo), hash))
}

func (p *ToolsPlugin) handleUUID(ctx *interfaces.CommandContext) error {
	u := uuid.New()
	return ctx.Edit(u.String())
}

func (p *ToolsPlugin) handleTime(ctx *interfaces.CommandContext) error {
	now := time.Now()
	return ctx.Edit(fmt.Sprintf("🕐 %s\n📅 %s", now.Format("15:04:05"), now.Format("2006-01-02 (Mon)")))
}