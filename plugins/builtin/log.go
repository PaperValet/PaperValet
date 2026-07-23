package builtin

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// LogPlugin provides log viewing and management commands.
type LogPlugin struct {
	buffer   []logEntry
	maxLines int
	enabled  bool
	target   int64
}

type logEntry struct {
	level string
	msg   string
	ts    time.Time
}

func NewLog() *LogPlugin {
	return &LogPlugin{
		buffer:  make([]logEntry, 0, 500),
		maxLines: 500,
	}
}

func (p *LogPlugin) Name() string        { return "log" }
func (p *LogPlugin) Description() string { return "日志查看与管理" }

func (p *LogPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "log",
		Aliases:     []string{"sendlog", "logs"},
		Description: "日志管理与查看",
		Usage:       "log [show|clear|level <debug|info|warn|error>|target <chat_id>|on|off]",
		Plugin:      p.Name(),
		Category:    "admin",
		OwnerOnly:   true,
		Handler:     p.handleLog,
	})
}

func (p *LogPlugin) Start(_ context.Context) error { return nil }
func (p *LogPlugin) Stop(_ context.Context) error  { return nil }

// AddEntry adds a log entry (for integration with logger)
func (p *LogPlugin) AddEntry(level, msg string) {
	p.buffer = append(p.buffer, logEntry{level: level, msg: msg, ts: time.Now()})
	if len(p.buffer) > p.maxLines {
		p.buffer = p.buffer[len(p.buffer)-p.maxLines:]
	}
}

func (p *LogPlugin) handleLog(ctx *interfaces.CommandContext) error {
	args := ctx.Args
	if len(args) == 0 {
		return ctx.Edit(p.status())
	}

	sub := args[0]

	switch sub {
	case "show", "view":
		return ctx.Edit(p.formatBuffer())

	case "clear":
		p.buffer = p.buffer[:0]
		return ctx.Edit("🗑 缓冲区已清空")

	case "on", "enable":
		p.enabled = true
		return ctx.Edit("✅ 日志记录已启用")

	case "off", "disable":
		p.enabled = false
		return ctx.Edit("⏸️ 日志记录已禁用")

	case "target":
		if len(args) < 2 {
			return ctx.Edit("用法: log target <chat_id>")
		}
		var chatID int64
		fmt.Sscanf(args[1], "%d", &chatID)
		if chatID == 0 {
			return ctx.Edit("无效的 chat_id")
		}
		p.target = chatID
		return ctx.Edit(fmt.Sprintf("✅ 日志投递目标设置为: %d", chatID))

	case "level":
		if len(args) < 2 {
			return ctx.Edit("用法: log level <debug|info|warn|error>")
		}
		return ctx.Edit("日志级别调整需修改配置文件后重启")

	case "max":
		if len(args) < 2 {
			return ctx.Edit("用法: log max <行数>")
		}
		var n int
		fmt.Sscanf(args[1], "%d", &n)
		if n > 0 && n <= 5000 {
			p.maxLines = n
			if len(p.buffer) > n {
				p.buffer = p.buffer[len(p.buffer)-n:]
			}
			return ctx.Edit(fmt.Sprintf("✅ 最大行数设置为: %d", n))
		}
		return ctx.Edit("行数范围: 1-5000")

	default:
		return ctx.Edit("未知子命令: " + sub + "\n\n" + p.status())
	}
}

func (p *LogPlugin) status() string {
	state := "⏸️ 禁用"
	if p.enabled {
		state = "✅ 启用"
	}
	target := "未设置"
	if p.target != 0 {
		target = fmt.Sprintf("%d", p.target)
	}
	return fmt.Sprintf("📝 <b>日志管理状态</b>\n\n状态: %s\n目标: %s\n缓冲: %d/%d 行\n最大: %d 行", state, target, len(p.buffer), p.maxLines, p.maxLines)
}

func (p *LogPlugin) formatBuffer() string {
	if len(p.buffer) == 0 {
		return "缓冲区为空"
	}

	var b strings.Builder
	b.WriteString("📋 <b>最近日志</b>\n\n")

	levelIcon := map[string]string{"debug": "🔍", "info": "ℹ️", "warn": "⚠️", "error": "❌"}

	start := 0
	if len(p.buffer) > 50 {
		start = len(p.buffer) - 50
	}
	for _, e := range p.buffer[start:] {
		icon := levelIcon[e.level]
		if icon == "" {
			icon = "📝"
		}
		b.WriteString(fmt.Sprintf("%s [%s] %s\n", icon, e.ts.Format("15:04:05"), e.msg))
	}
	return b.String()
}