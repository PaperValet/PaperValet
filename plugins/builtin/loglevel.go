package builtin

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogLevelPlugin provides runtime log level changes.
// Inspired by TeleBox's loglevel plugin.
type LogLevelPlugin struct {
	logger *zap.Logger
	level  zap.AtomicLevel
}

func NewLogLevel() *LogLevelPlugin {
	return &LogLevelPlugin{}
}

func (p *LogLevelPlugin) Name() string        { return "loglevel" }
func (p *LogLevelPlugin) Description() string { return "运行时日志级别调整" }

func (p *LogLevelPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	// We can't easily swap the global logger in zap without access to it
	// This plugin provides the commands; actual level change would need logger reference
	cmds := []*interfaces.Command{
		{
			Name:        "loglevel",
			Aliases:     []string{"loglvl", "ll"},
			Description: "查看/设置日志级别",
			Usage:       "loglevel [debug|info|warn|error|dpanic|panic|fatal]",
			Plugin:      p.Name(),
			Category:    "admin",
			OwnerOnly:   true,
			Handler:     p.handleLogLevel,
		},
	}

	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}

	return nil
}

func (p *LogLevelPlugin) Start(ctx context.Context) error { return nil }
func (p *LogLevelPlugin) Stop(ctx context.Context) error  { return nil }

func (p *LogLevelPlugin) handleLogLevel(ctx *interfaces.CommandContext) error {
	args := ctx.Args

	if len(args) == 0 {
		// Show current level
		return ctx.Edit(p.formatStatus())
	}

	levelStr := strings.ToLower(args[0])
	level := parseLevel(levelStr)
	if level == zapcore.InvalidLevel {
		return ctx.Edit(fmt.Sprintf("❌ 无效级别: %s\n支持: debug, info, warn, error, dpanic, panic, fatal", levelStr))
	}

	// Note: In practice, this would need access to the atomic level used by the app
	// For now, we show what would be done
	return ctx.Edit(fmt.Sprintf("📝 日志级别将设为: <b>%s</b>\n\n⚠️ 注意: 实际修改需要重启或在 app.go 中暴露 AtomicLevel", levelStr))
}

func (p *LogLevelPlugin) formatStatus() string {
	return `📝 <b>日志级别控制</b>

<b>当前级别:</b> 由 zap 全局配置决定（需重启生效）

<b>支持级别:</b>
• <code>debug</code> - 详细调试信息
• <code>info</code> - 常规信息（默认）
• <code>warn</code> - 警告
• <code>error</code> - 错误
• <code>dpanic</code> - 开发环境 panic
• <code>panic</code> - panic
• <code>fatal</code> - 致命错误并退出

<b>用法:</b>
<code>loglevel debug</code> — 设为调试级别
<code>loglevel info</code> — 设为信息级别
<code>loglevel warn</code> — 设为警告级别

<b>提示:</b> 运行时修改需要应用层暴露 zap.AtomicLevel，当前仅显示说明。`
}

func parseLevel(s string) zapcore.Level {
	switch s {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "dpanic":
		return zapcore.DPanicLevel
	case "panic":
		return zapcore.PanicLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InvalidLevel
	}
}