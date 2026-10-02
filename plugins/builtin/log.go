package builtin

import (
	"context"
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

// LogPlugin delivers log files. The level and the delivery target are
// settings in the bot panel; the command only performs actions.
type LogPlugin struct {
	set plugin.Settings
}

func NewLog() *LogPlugin { return &LogPlugin{} }

func (p *LogPlugin) Name() string        { return "log" }
func (p *LogPlugin) Description() string { return "运行日志" }
func (p *LogPlugin) DescEN() string      { return "Logs: send, tail, clean" }

func validLogTarget(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "me" {
		return s, nil
	}
	if id, err := strconv.ParseInt(s, 10, 64); err == nil && id != 0 {
		return s, nil
	}
	return "", plugin.Invalid("填 me 或数字 chatID", "me or a numeric chat ID")
}

func (p *LogPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	set, err := mgr.Host().Settings(&plugin.SettingsSpec{
		Plugin:  p.Name(),
		Title:   "📋 日志",
		TitleEN: "📋 Logs",
		Settings: []plugin.Setting{
			{
				Key: "level", Label: "日志级别", LabelEN: "Log level",
				Hint:   "排查问题时切 debug，用完切回 info",
				HintEN: "Switch to debug while debugging, back to info afterwards",
				Kind:   plugin.SettingChoice, Default: strings.ToLower(logger.GetLevel()),
				Choices: []plugin.Choice{
					{Value: "debug", Label: "debug"}, {Value: "info", Label: "info"},
					{Value: "warn", Label: "warn"}, {Value: "error", Label: "error"},
				},
			},
			{
				Key: "target", Label: "日志发到", LabelEN: "Send logs to",
				Hint:   "me 是收藏夹，也可以填数字 chatID",
				HintEN: "me is Saved Messages, or a numeric chat ID",
				Kind:   plugin.SettingText, Default: "me", Validate: validLogTarget,
			},
		},
		OnChange: func(key string) {
			if key == "level" {
				p.applyLevel()
			}
		},
	})
	if err != nil {
		return err
	}
	p.set = set
	p.applyLevel()
	if err := mgr.RegisterCommand(&interfaces.Command{
		Name:        "loglevel",
		Description: "日志级别（在机器人面板设置）",
		DescEN:      "Log level (set in the bot panel)",
		Usage:       "loglevel",
		UsageEN:     "loglevel",
		Plugin:      p.Name(),
		Category:    "admin",
		OwnerOnly:   true,
		Hidden:      true,
		Handler: func(ctx *interfaces.CommandContext) error {
			return ctx.Edit(ctx.Tlocal(
				"📝 日志级别挪到机器人的 /menu 按钮面板了",
				"📝 The log level now lives in the bot's /menu panel"))
		},
	}); err != nil {
		return err
	}
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "sendlog",
		Description: "发送日志",
		DescEN:      "Send logs",
		Usage: "sendlog · sendlog tail [行数] · sendlog clean\n" +
			"\n" +
			"**示例**\n" +
			"• `sendlog`  把最新日志文件发到设置的目标（默认收藏夹）\n" +
			"• `sendlog tail 50`  直接在聊天里看最后 50 行\n" +
			"• `sendlog clean`  删除日志文件\n" +
			"\n" +
			"**机制**\n" +
			"• 日志级别和发送目标在机器人面板里调\n" +
			"• 在 ./logs、~/.pm2/logs、/var/log/papervalet 里找文件名含 paper 的 .log\n" +
			"• 超过 50MB 不发送，请用 tail\n" +
			"• systemd 运行时日志在 journald，用 `exec journalctl -u 服务名 -n 50` 查看",
		UsageEN: "sendlog · sendlog tail [lines] · sendlog clean\n" +
			"\n" +
			"**Examples**\n" +
			"• `sendlog`  send the newest log file to the configured target (Saved Messages by default)\n" +
			"• `sendlog tail 50`  print the last 50 lines here\n" +
			"• `sendlog clean`  delete log files\n" +
			"\n" +
			"**How it works**\n" +
			"• Level and target are set in the bot panel\n" +
			"• Looks for *.log files containing \"paper\" in ./logs, ~/.pm2/logs and /var/log/papervalet\n" +
			"• Files over 50MB are not sent; use tail\n" +
			"• Under systemd logs go to journald: `exec journalctl -u service -n 50`",
		Plugin:    p.Name(),
		Category:  "admin",
		OwnerOnly: true,
		Handler:   p.handleSendLog,
	})
}

func (p *LogPlugin) Start(_ context.Context) error { return nil }
func (p *LogPlugin) Stop(_ context.Context) error  { return nil }

func (p *LogPlugin) applyLevel() {
	if l := p.set.String("level"); parseZapLevel(l) != zapcore.InvalidLevel {
		_ = logger.SetLevel(l)
	}
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
		return ctx.Edit(ctx.Tlocal(
			"📌 日志发送目标挪到机器人的 /menu 按钮面板了",
			"📌 The log target now lives in the bot's /menu panel"))
	case "clean":
		return p.cleanLogs(ctx)
	case "":
		return p.sendFile(ctx)
	default:
		return ctx.Edit(ctx.Tlocal("用法: `sendlog [tail [行数]|clean]`", "Usage: `sendlog [tail [lines]|clean]`"))
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
		return ctx.Edit("❌ 读取失败: " + esc(err.Error()))
	}
	if info.Size() > 50*1024*1024 {
		return ctx.Edit(fmt.Sprintf("⚠️ 日志过大 (%dKB)，请用 `sendlog tail` 查看尾部", info.Size()/1024))
	}

	target := p.set.String("target")
	chatID := ctx.Message.UserID
	if target != "me" {
		id, err := strconv.ParseInt(target, 10, 64)
		if err != nil || id == 0 {
			return ctx.Edit(ctx.Tlocal("❌ 发送目标无效，去机器人面板改一下", "❌ Invalid target, fix it in the bot panel"))
		}
		chatID = id
	}

	if err := ctx.Media.SendFile(ctx.Context(), chatID, logFile,
		fmt.Sprintf("📋 %s (%dKB)", filepath.Base(logFile), info.Size()/1024), 0); err != nil {
		return ctx.Edit("❌ 发送失败: " + esc(err.Error()))
	}
	return ctx.Edit(ctx.Tlocal("✅ 日志已发送到 ", "✅ Log sent to ") + plugin.Code(target))
}

// sendTail prints the last N lines of the newest log file into the chat.
func (p *LogPlugin) sendTail(ctx *interfaces.CommandContext, lines int) error {
	logFile := findLatestLog()
	if logFile == "" {
		return ctx.Edit("❌ 未找到日志文件\n\n已检查: ./logs、~/.pm2/logs、/var/log/papervalet")
	}
	data, err := readTail(logFile, lines, 3500)
	if err != nil {
		return ctx.Edit("❌ 读取日志失败: " + esc(err.Error()))
	}
	if strings.TrimSpace(data) == "" {
		return ctx.Edit("📋 日志文件为空: " + plugin.Code(logFile))
	}
	return ctx.Edit("📋 **日志尾部** (" + plugin.Code(filepath.Base(logFile)) + ")\n\n" + plugin.Pre(data))
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
	return ctx.Edit(fmt.Sprintf("🗑️ **日志清理完成**\n\n%s\n\n📊 已清理 %d 个文件", strings.Join(results, "\n"), cleaned))
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

func readTail(path string, lines int, maxBytes int64) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	start := info.Size() - maxBytes
	if start < 0 {
		start = 0
	}
	if _, err := f.Seek(start, 0); err != nil {
		return "", err
	}
	buf := make([]byte, info.Size()-start)
	if _, err := f.Read(buf); err != nil && len(buf) > 0 {
		return "", err
	}
	text := strings.ToValidUTF8(string(buf), "�")
	if start > 0 {
		if idx := strings.IndexByte(text, '\n'); idx >= 0 {
			text = text[idx+1:]
		}
	}
	parts := strings.Split(text, "\n")
	if len(parts) > int(lines) {
		parts = parts[len(parts)-int(lines):]
	}
	return strings.Join(parts, "\n"), nil
}
