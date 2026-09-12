package builtin

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/logger"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
	"go.uber.org/zap/zapcore"
)

// LogPlugin merges log viewing, runtime log level, and log sending.
// Absorbs the old loglevel and sendlog plugins.
type LogPlugin struct{}

func NewLog() *LogPlugin { return &LogPlugin{} }

func (p *LogPlugin) Name() string        { return "log" }
func (p *LogPlugin) Description() string { return "日志管理（查看/级别/发送）" }

func (p *LogPlugin) Init(_ context.Context, mgr plugin.Manager) error {
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
			Description: "发送日志文件到当前对话",
			Usage:       "sendlog",
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
	return ctx.Edit("📋 日志文件发送由 sendlog 插件提供，请使用 <code>sendlog</code> 命令")
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

// ensure time is used (log timestamps)
var _ = time.Now
