package builtin

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/TiaraBasori/PaperValet/internal/command"
	"github.com/TiaraBasori/PaperValet/internal/core"
	"github.com/TiaraBasori/PaperValet/internal/plugin"
)

// AdminPlugin provides admin/owner-only commands.
type AdminPlugin struct{}

func NewAdmin() *AdminPlugin { return &AdminPlugin{} }

func (p *AdminPlugin) Name() string        { return "admin" }
func (p *AdminPlugin) Description() string { return "管理员命令" }

func (p *AdminPlugin) Init(_ context.Context, mgr *plugin.Manager) error {
	cmds := []*command.Command{
		{
			Name:        "restart",
			Description: "重启 bot (仅所有者)",
			Plugin:      p.Name(),
			Category:    "admin",
			OwnerOnly:   true,
			Handler:     p.handleRestart,
		},
		{
			Name:        "shutdown",
			Description: "关闭 bot (仅所有者)",
			Plugin:      p.Name(),
			Category:    "admin",
			OwnerOnly:   true,
			Handler:     p.handleShutdown,
		},
		{
			Name:        "eval",
			Description: "执行 Go 表达式 (仅所有者，危险)",
			Usage:       "eval <表达式>",
			Plugin:      p.Name(),
			Category:    "admin",
			OwnerOnly:   true,
			Hidden:      true,
			Handler:     p.handleEval,
		},
		{
			Name:        "exec",
			Description: "执行 shell 命令 (仅所有者，危险)",
			Usage:       "exec <命令>",
			Plugin:      p.Name(),
			Category:    "admin",
			OwnerOnly:   true,
			Hidden:      true,
			Handler:     p.handleExec,
		},
		{
			Name:        "gc",
			Description: "手动触发 GC",
			Plugin:      p.Name(),
			Category:    "admin",
			OwnerOnly:   true,
			Handler:     p.handleGC,
		},
		{
			Name:        "version",
			Aliases:     []string{"ver"},
			Description: "显示版本信息",
			Plugin:      p.Name(),
			Category:    "admin",
			Handler:     p.handleVersion,
		},
	}
	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *AdminPlugin) Start(_ context.Context) error { return nil }
func (p *AdminPlugin) Stop(_ context.Context) error  { return nil }

func (p *AdminPlugin) handleRestart(ctx *core.CommandContext) error {
	return ctx.Edit("🔄 重启功能需配合进程管理器使用")
}

func (p *AdminPlugin) handleShutdown(ctx *core.CommandContext) error {
	return ctx.Edit("👋 关闭功能需配合进程管理器使用")
}

func (p *AdminPlugin) handleEval(ctx *core.CommandContext) error {
	return ctx.Edit("⚠️ eval 需要嵌入式 Go 解释器 (如 yaegi)，暂未实现")
}

func (p *AdminPlugin) handleExec(ctx *core.CommandContext) error {
	if ctx.ArgCount() == 0 {
		return ctx.Edit("用法: exec <命令>")
	}
	return ctx.Edit("⚠️ exec 需要 shell 执行权限，暂未实现")
}

func (p *AdminPlugin) handleGC(ctx *core.CommandContext) error {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return ctx.Edit(fmt.Sprintf("🗑 GC 完成\n内存: %.1f MB\nGoroutine: %d", float64(m.Alloc)/1024/1024, runtime.NumGoroutine()))
}

func (p *AdminPlugin) handleVersion(ctx *core.CommandContext) error {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return ctx.Edit(fmt.Sprintf(
		"PaperValet\nGo: %s\nGoroutine: %d\n内存: %.1f MB\n启动: %s",
		runtime.Version(), runtime.NumGoroutine(), float64(m.Alloc)/1024/1024,
		time.Now().Format("2006-01-02 15:04:05"),
	))
}