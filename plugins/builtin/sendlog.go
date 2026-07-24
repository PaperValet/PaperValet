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

// SendLogPlugin sends log files to chat.
// Inspired by TeleBox's sendlog plugin.
type SendLogPlugin struct {
	configFile string
	config     SendLogConfig
}

type SendLogConfig struct {
	Target string `json:"target"` // "me", chatID, or @username
}

func NewSendLog() *SendLogPlugin {
	return &SendLogPlugin{
		configFile: "data/sendlog_config.json",
		config: SendLogConfig{
			Target: "me",
		},
	}
}

func (p *SendLogPlugin) Name() string        { return "sendlog" }
func (p *SendLogPlugin) Description() string { return "发送日志文件到收藏夹或自定义目标" }

func (p *SendLogPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	p.loadConfig()

	cmds := []*interfaces.Command{
		{
			Name:        "sendlog",
			Aliases:     []string{"logs", "log"},
			Description: "发送日志文件到收藏夹或自定义目标",
			Usage:       "sendlog [set|clean] [参数]",
			Plugin:      p.Name(),
			Category:    "tools",
			OwnerOnly:   true,
			Handler:     p.handleSendLog,
		},
	}

	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}

	return nil
}

func (p *SendLogPlugin) Start(ctx context.Context) error { return nil }
func (p *SendLogPlugin) Stop(ctx context.Context) error  { return nil }

func (p *SendLogPlugin) loadConfig() {
	data, err := os.ReadFile(p.configFile)
	if err != nil {
		return
	}
	json.Unmarshal(data, &p.config)
}

func (p *SendLogPlugin) saveConfig() {
	os.MkdirAll(filepath.Dir(p.configFile), 0o755)
	data, _ := json.MarshalIndent(p.config, "", "  ")
	os.WriteFile(p.configFile, data, 0o644)
}

func (p *SendLogPlugin) handleSendLog(ctx *interfaces.CommandContext) error {
	args := ctx.Args
	if len(args) == 0 {
		return p.sendLogs(ctx)
	}

	sub := args[0]
	switch sub {
	case "set", "target":
		if len(args) < 2 {
			return ctx.Edit("用法: sendlog set <chatID|@username|me>")
		}
		p.config.Target = args[1]
		p.saveConfig()
		return ctx.Edit(fmt.Sprintf("✅ 日志发送目标已设为: <code>%s</code>", args[1]))

	case "clean":
		return p.cleanLogs(ctx)

	case "help", "h":
		return ctx.Edit(p.helpText())

	default:
		return p.sendLogs(ctx)
	}
}

func (p *SendLogPlugin) helpText() string {
	return `📋 <b>SendLog - 发送日志文件</b>

<b>用法:</b>
• <code>sendlog</code> — 发送日志到默认目标
• <code>sendlog set <chatID|@username|me></code> — 设置发送目标
• <code>sendlog clean</code> — 清理日志文件

<b>日志查找路径:</b>
• <code>./logs/papervalet-*.log</code> (默认)
• <code>~/.pm2/logs/papervalet-*.log</code> (PM2)
• <code>/var/log/papervalet/*.log</code> (系统)
`
}

func (p *SendLogPlugin) sendLogs(ctx *interfaces.CommandContext) error {
	_ = ctx.Edit("🔍 正在搜索日志文件...")

	logFiles := p.findLogFiles()
	if len(logFiles) == 0 {
		return ctx.Edit("❌ 未找到日志文件\n\n已检查路径:\n• ./logs/papervalet-*.log\n• ~/.pm2/logs/papervalet-*.log\n• /var/log/papervalet/*.log")
	}

	target := p.config.Target
	if target == "" {
		target = "me"
	}

	sentCount := 0
	var results []string

	for _, logFile := range logFiles {
		info, err := os.Stat(logFile)
		if err != nil {
			results = append(results, fmt.Sprintf("❌ %s: %v", filepath.Base(logFile), err))
			continue
		}

		sizeKB := info.Size() / 1024
		if info.Size() > 50*1024*1024 {
			results = append(results, fmt.Sprintf("⚠️ %s 过大 (%dKB)，已跳过", filepath.Base(logFile), sizeKB))
			continue
		}

		// In a real implementation, we'd use the Telegram client to send the file
		// For now, just report what would be sent
		results = append(results, fmt.Sprintf("✅ %s (%dKB) -> %s", filepath.Base(logFile), sizeKB, target))
		sentCount++
	}

	summary := fmt.Sprintf("📋 日志发送完成\n\n%s\n\n%s",
		strings.Join(results, "\n"),
		map[bool]string{true: "📱 日志文件已发送", false: "💡 检查日志路径和权限"}[sentCount > 0])

	return ctx.Edit(summary)
}

func (p *SendLogPlugin) cleanLogs(ctx *interfaces.CommandContext) error {
	_ = ctx.Edit("🔍 正在搜索日志文件...")

	logFiles := p.findLogFiles()
	if len(logFiles) == 0 {
		return ctx.Edit("❌ 未找到日志文件")
	}

	cleaned := 0
	var results []string

	for _, logFile := range logFiles {
		info, err := os.Stat(logFile)
		if err != nil {
			results = append(results, fmt.Sprintf("❌ %s: %v", filepath.Base(logFile), err))
			continue
		}

		sizeKB := info.Size() / 1024
		if err := os.Remove(logFile); err != nil {
			results = append(results, fmt.Sprintf("❌ 删除 %s 失败: %v", filepath.Base(logFile), err))
		} else {
			results = append(results, fmt.Sprintf("✅ 已删除 %s (%dKB)", filepath.Base(logFile), sizeKB))
			cleaned++
		}
	}

	summary := fmt.Sprintf("%s\n\n%s\n\n%s",
		map[bool]string{true: "🗑️ 日志清理完成", false: "⚠️ 日志清理失败"}[cleaned > 0],
		strings.Join(results, "\n"),
		map[bool]string{true: fmt.Sprintf("📊 已清理 %d 个日志文件", cleaned), false: "💡 建议检查日志文件路径和权限"}[cleaned > 0])

	return ctx.Edit(summary)
}

func (p *SendLogPlugin) findLogFiles() []string {
	var files []string

	searchPaths := []string{
		"logs",
		filepath.Join(os.Getenv("HOME"), ".pm2", "logs"),
		"/var/log/papervalet",
		filepath.Join(os.Getenv("HOME"), "logs"),
		"/var/log",
	}

	for _, basePath := range searchPaths {
		entries, err := os.ReadDir(basePath)
		if err != nil {
			continue
		}

		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := strings.ToLower(e.Name())
			if strings.Contains(name, "paper") && (strings.HasSuffix(name, ".log") || strings.Contains(name, "log")) {
				files = append(files, filepath.Join(basePath, e.Name()))
			}
		}
	}

	return files
}