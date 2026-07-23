package builtin

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// CorePlugin provides core system commands: help, status, restart, shutdown, gc, version.
type CorePlugin struct {
	mgr       plugin.Manager
	startTime time.Time
	version   string
}

func NewCore(version string) *CorePlugin {
	return &CorePlugin{version: version, startTime: time.Now()}
}

func (p *CorePlugin) Name() string        { return "core" }
func (p *CorePlugin) Description() string { return "核心系统命令" }

func (p *CorePlugin) Init(_ context.Context, mgr plugin.Manager) error {
	p.mgr = mgr

	cmds := []*interfaces.Command{
		{
			Name:        "help",
			Aliases:     []string{"h", "?"},
			Description: "显示帮助",
			Usage:       "help [命令|插件]",
			Plugin:      p.Name(),
			Category:    "core",
			Handler:     p.handleHelp,
		},
		{
			Name:        "status",
			Aliases:     []string{"stat", "st"},
			Description: "显示运行状态",
			Plugin:      p.Name(),
			Category:    "core",
			Handler:     p.handleStatus,
		},
		{
			Name:        "restart",
			Description: "重启机器人 (仅拥有者)",
			Plugin:      p.Name(),
			Category:    "admin",
			OwnerOnly:   true,
			Handler:     p.handleRestart,
		},
		{
			Name:        "shutdown",
			Description: "关闭机器人 (仅拥有者)",
			Plugin:      p.Name(),
			Category:    "admin",
			OwnerOnly:   true,
			Handler:     p.handleShutdown,
		},
		{
			Name:        "gc",
			Description: "强制 GC (仅拥有者)",
			Plugin:      p.Name(),
			Category:    "admin",
			OwnerOnly:   true,
			Handler:     p.handleGC,
		},
		{
			Name:        "version",
			Description: "显示版本信息 (仅拥有者)",
			Plugin:      p.Name(),
			Category:    "admin",
			OwnerOnly:   true,
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

func (p *CorePlugin) Start(_ context.Context) error { return nil }
func (p *CorePlugin) Stop(_ context.Context) error  { return nil }

func (p *CorePlugin) handleHelp(ctx *interfaces.CommandContext) error {
	prefix := p.mgr.Commands().GetPrefix()
	arg := ctx.GetArg(0)
	if arg == "" {
		cmds := p.mgr.Commands().GetAll()

		names := make([]string, 0, len(cmds))
		for name, cmd := range cmds {
			if !cmd.Hidden {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		var b strings.Builder
		b.WriteString("PaperValet 命令列表\n")
		for _, name := range names {
			cmd := cmds[name]
			b.WriteString(fmt.Sprintf("%s%s — %s\n", prefix, name, cmd.Description))
		}
		b.WriteString(fmt.Sprintf("\n详情: %shelp <命令>", prefix))
		return ctx.Edit(b.String())
	}

	if cmd, ok := p.mgr.Commands().Get(arg); ok {
		text := fmt.Sprintf("%s%s\n%s", prefix, cmd.Name, cmd.Description)
		if cmd.Usage != "" {
			text += "\n用法: " + prefix + cmd.Usage
		}
		if len(cmd.Aliases) > 0 {
			text += "\n别名: " + strings.Join(cmd.Aliases, ", ")
		}
		return ctx.Edit(text)
	}

	if info, ok := p.mgr.GetInfo(arg); ok {
		cmds := p.mgr.Commands().GetByPlugin(arg)
		var b strings.Builder
		b.WriteString(fmt.Sprintf("插件 %s\n%s\n", info.Name, info.Description))
		for name, cmd := range cmds {
			b.WriteString(fmt.Sprintf("%s%s — %s\n", prefix, name, cmd.Description))
		}
		return ctx.Edit(b.String())
	}

	return ctx.Edit("未找到命令或插件: " + arg)
}

func (p *CorePlugin) handleStatus(ctx *interfaces.CommandContext) error {
	var mem runtime.MemStats

	runtime.ReadMemStats(&mem)
	infos := p.mgr.GetAllInfo()
	active := 0
	for _, i := range infos {
		if i.Status == plugin.StatusActive {
			active++
		}
	}
	uptime := time.Since(p.startTime).Truncate(time.Second)
	text := fmt.Sprintf(
		"PaperValet %s\n运行: %s\n插件: %d/%d\nGoroutine: %d\n内存: %.1f MB",
		p.version,
		uptime,
		active, len(infos),
		runtime.NumGoroutine(),
		float64(mem.Alloc)/1024/1024,
	)
	return ctx.Edit(text)
}

func (p *CorePlugin) handleRestart(ctx *interfaces.CommandContext) error {
	return ctx.Edit("🔄 重启功能需要外部进程管理器 (systemd/PM2) 支持")
}

func (p *CorePlugin) handleShutdown(ctx *interfaces.CommandContext) error {
	return ctx.Edit("🛑 关闭功能需要外部进程管理器支持")
}

func (p *CorePlugin) handleGC(ctx *interfaces.CommandContext) error {
	var mem runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&mem)
	return ctx.Edit(fmt.Sprintf("🗑 GC 完成\n内存: %.1f MB", float64(mem.Alloc)/1024/1024))
}

func (p *CorePlugin) handleVersion(ctx *interfaces.CommandContext) error {
	uptime := time.Since(p.startTime).Truncate(time.Second)
	return ctx.Edit(fmt.Sprintf("PaperValet %s\n运行时间: %s", p.version, uptime))
}