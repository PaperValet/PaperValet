package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/logger"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
	"go.uber.org/zap/zapcore"
)

// LogPlugin manages runtime logging: level control + log file delivery.
// Absorbs the old sendlog plugin.
type LogPlugin struct {
	configFile string
	target     string // "me" (saved messages) or a numeric chat ID
}

func NewLog() *LogPlugin {
	return &LogPlugin{
		configFile: "data/sendlog_config.json",
		target:     "me",
	}
}

func (p *LogPlugin) Name() string        { return "log" }
func (p *LogPlugin) Description() string { return "日志管理（级别/发送/清理）" }

func (p *LogPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	p.loadConfig()
	cmds := []*interfaces.Command{
		{
			Name:        "loglevel",
			Aliases:     []string{"loglvl", "ll"},
			Description: "查看/设置运行时日志级别",
			Usage:       "loglevel [debug|info|warn|error]",
			Plugin:      p.Name(),
			Category:    "admin",
			OwnerOnly:   true,
			Handler:     p.handleLogLevel,
		},
		{
			Name:        "sendlog",
			Aliases:     []string{"logs"},
			Description: "发送最新日志文件到收藏夹或指定目标",
			Usage:       "sendlog [tail [行数]|set <me|chatID>|clean]",
			Plugin:      p.Name(),
			Category:    "admin",
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

func (p *LogPlugin) Start(_ context.Context) error { return nil }
func (p *LogPlugin) Stop(_ context.Context) error  { return nil }

func (p *LogPlugin) loadConfig() {
	data, err := os.ReadFile(p.configFile)
	if err != nil {
		return
	}
	var st struct {
		Target string `json:"target"`
	}
	if json.Unmarshal(data, &st) == nil && st.Target != "" {
		p.target = st.Target
	}
}

func (p *LogPlugin) saveConfig() {
	_ = os.MkdirAll(filepath.Dir(p.configFile), 0o755)
	data, _ := json.MarshalIndent(map[string]string{"target": p.target}, "", "  ")
	_ = os.WriteFile(p.configFile, data, 0o600)
}

func (p *LogPlugin) handleLogLevel(ctx *interfaces.CommandContext) error {
	if ctx.ArgCount() == 0 {
		return ctx.Edit(fmt.Sprintf("📝 <b>日志级别</b>\n\n当前: <code>%s</code>\n\n用法: <code>loglevel debug|info|warn|error</code>", logger.GetLevel()))
	}

	levelStr := strings.ToLower(ctx.GetArg(0))
	if parseZapLevel(levelStr) == zapcore.InvalidLevel {
		return ctx.Edit(fmt.Sprintf("❌ 无效级别: %s\n支持: debug, info, warn, error", levelStr))
	}

	if err := logger.SetLevel(levelStr); err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 设置失败: %v", err))
	}
	return ctx.Edit(fmt.Sprintf("✅ 日志级别已切换为: <b>%s</b>", levelStr))
}

func (p *LogPlugin) handleSendLog(ctx *interfaces.CommandContext) error {
	switch ctx.GetArg(0) {
	case "tail":
		lines := 100
		if ctx.ArgCount() > 1 {
			if n, err := strconv.Atoi(ctx.GetArg(1)); err == nil && n > 0 && n <= 1000 {
				lines = n
			}
		}
		return p.sendTail(ctx, lines)
	case "set":
		if ctx.ArgCount() < 2 {
			return ctx.Edit("用法: sendlog set <me|chatID>")
		}
		target := ctx.GetArg(1)
		if target != "me" {
			var id int64
			if _, err := fmt.Sscanf(target, "%d", &id); err != nil || id == 0 {
				return ctx.Edit("❌ 目标无效: 只支持 <code>me</code> 或数字 chatID")
			}
		}
		p.target = target
		p.saveConfig()
		return ctx.Edit(fmt.Sprintf("✅ 日志发送目标已设为: <code>%s</code>", target))
	case "clean":
		return p.cleanLogs(ctx)
	case "":
		return p.sendFile(ctx)
	default:
		return ctx.Edit("用法: sendlog [tail [行数]|set <me|chatID>|clean]")
	}
}

// sendFile uploads the newest log file to the configured target.
func (p *LogPlugin) sendFile(ctx *interfaces.CommandContext) error {
	if ctx.Media == nil {
		return ctx.Edit("❌ 媒体发送不可用")
	}
	logFile := findLatestLog()
	if logFile == "" {
		return ctx.Edit("❌ 未找到日志文件\n\n已检查: ./logs、~/.pm2/logs、/var/log/papervalet")
	}
	info, err := os.Stat(logFile)
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 读取失败: %v", err))
	}
	if info.Size() > 50*1024*1024 {
		return ctx.Edit(fmt.Sprintf("⚠️ 日志过大 (%dKB)，请用 <code>sendlog tail</code> 查看尾部", info.Size()/1024))
	}

	chatID := ctx.Message.UserID
	if p.target != "me" {
		var id int64
		if _, err := fmt.Sscanf(p.target, "%d", &id); err != nil || id == 0 {
			return ctx.Edit("❌ 目标配置无效，请用 <code>sendlog set</code> 重新设置")
		}
		chatID = id
	}

	if err := ctx.Media.SendFile(ctx.Context(), chatID, logFile,
		fmt.Sprintf("📋 %s (%dKB)", filepath.Base(logFile), info.Size()/1024), 0); err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 发送失败: %v", err))
	}
	return ctx.Edit(fmt.Sprintf("✅ 日志已发送到 <code>%s</code>", p.target))
}

// sendTail prints the last N lines of the newest log file into the chat.
func (p *LogPlugin) sendTail(ctx *interfaces.CommandContext, lines int) error {
	logFile := findLatestLog()
	if logFile == "" {
		return ctx.Edit("❌ 未找到日志文件\n\n已检查: ./logs、~/.pm2/logs、/var/log/papervalet")
	}
	data, err := os.ReadFile(logFile)
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 读取日志失败: %v", err))
	}
	text := string(data)
	if strings.TrimSpace(text) == "" {
		return ctx.Edit(fmt.Sprintf("📋 日志文件为空: <code>%s</code>", logFile))
	}
	all := strings.Split(text, "\n")
	if len(all) > lines {
		all = all[len(all)-lines:]
	}
	tail := strings.Join(all, "\n")
	// Telegram messages cap at 4096 chars; truncate from the head.
	const maxLen = 3500
	if len(tail) > maxLen {
		tail = "…(截断)\n" + tail[len(tail)-maxLen:]
	}
	return ctx.Edit(fmt.Sprintf("📋 <b>日志尾部</b> (<code>%s</code>)\n\n<pre>%s</pre>", filepath.Base(logFile), tail))
}

func (p *LogPlugin) cleanLogs(ctx *interfaces.CommandContext) error {
	files := findLogFiles()
	if len(files) == 0 {
		return ctx.Edit("❌ 未找到日志文件")
	}
	cleaned := 0
	var results []string
	for _, f := range files {
		info, err := os.Stat(f)
		sizeKB := int64(0)
		if err == nil {
			sizeKB = info.Size() / 1024
		}
		if err := os.Remove(f); err != nil {
			results = append(results, fmt.Sprintf("❌ 删除 %s 失败: %v", filepath.Base(f), err))
		} else {
			results = append(results, fmt.Sprintf("✅ 已删除 %s (%dKB)", filepath.Base(f), sizeKB))
			cleaned++
		}
	}
	return ctx.Edit(fmt.Sprintf("🗑️ <b>日志清理完成</b>\n\n%s\n\n📊 已清理 %d 个文件", strings.Join(results, "\n"), cleaned))
}

// findLatestLog returns the most recently modified matching log file.
func findLatestLog() string {
	files := findLogFiles()
	if len(files) == 0 {
		return ""
	}
	sort.Slice(files, func(i, j int) bool {
		iInfo, iErr := os.Stat(files[i])
		jInfo, jErr := os.Stat(files[j])
		if iErr != nil || jErr != nil {
			return false
		}
		return iInfo.ModTime().After(jInfo.ModTime())
	})
	return files[0]
}

func findLogFiles() []string {
	var files []string
	for _, dir := range []string{
		"logs",
		filepath.Join(os.Getenv("HOME"), ".pm2", "logs"),
		"/var/log/papervalet",
	} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := strings.ToLower(e.Name())
			if strings.Contains(name, "paper") && strings.HasSuffix(name, ".log") {
				files = append(files, filepath.Join(dir, e.Name()))
			}
		}
	}
	return files
}

func parseZapLevel(s string) zapcore.Level {
	switch strings.ToLower(s) {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InvalidLevel
	}
}
