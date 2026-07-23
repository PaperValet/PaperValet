package builtin

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/internal/cron"
	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// AdminPlugin provides owner-only commands.
type AdminPlugin struct {
	startTime time.Time
}

func NewAdmin() *AdminPlugin { return &AdminPlugin{startTime: time.Now()} }

func (p *AdminPlugin) Name() string        { return "admin" }
func (p *AdminPlugin) Description() string { return "管理员命令" }

func (p *AdminPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	cmds := []*interfaces.Command{
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

func (p *AdminPlugin) Start(_ context.Context) error { return nil }
func (p *AdminPlugin) Stop(_ context.Context) error  { return nil }

func (p *AdminPlugin) handleRestart(ctx *interfaces.CommandContext) error {
	return ctx.Edit("🔄 重启功能需要外部进程管理器 (systemd/PM2) 支持")
}

func (p *AdminPlugin) handleShutdown(ctx *interfaces.CommandContext) error {
	return ctx.Edit("🛑 关闭功能需要外部进程管理器支持")
}

func (p *AdminPlugin) handleGC(ctx *interfaces.CommandContext) error {
	var mem runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&mem)
	return ctx.Edit(fmt.Sprintf("🗑 GC 完成\n内存: %.1f MB", float64(mem.Alloc)/1024/1024))
}

func (p *AdminPlugin) handleVersion(ctx *interfaces.CommandContext) error {
	uptime := time.Since(p.startTime).Truncate(time.Second)
	return ctx.Edit(fmt.Sprintf("PaperValet %s\n运行时间: %s", "0.1.0", uptime))
}

// CronPlugin provides cron management commands.
type CronPlugin struct {
	mgr *cron.Manager
}

func NewCron(cronMgr *cron.Manager) *CronPlugin {
	return &CronPlugin{mgr: cronMgr}
}

func (p *CronPlugin) Name() string        { return "cron" }
func (p *CronPlugin) Description() string { return "定时任务管理" }

func (p *CronPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "cron",
		Aliases:     []string{"schedule"},
		Description: "定时任务管理",
		Usage:       "cron add <名称> <表达式> <命令> | cron list | cron del <名称> | cron run <名称>",
		Plugin:      p.Name(),
		Category:    "tools",
		Handler:     p.handleCron,
	})
}

func (p *CronPlugin) Start(_ context.Context) error { return nil }
func (p *CronPlugin) Stop(_ context.Context) error  { return nil }

func (p *CronPlugin) handleCron(ctx *interfaces.CommandContext) error {
	if ctx.ArgCount() == 0 {
		return ctx.Edit("用法: cron add <名称> <表达式> <命令> | cron list | cron del <名称> | cron run <名称>")
	}
	sub := ctx.GetArg(0)

	switch sub {
	case "add":
		if ctx.ArgCount() < 4 {
			return ctx.Edit("用法: cron add <名称> <表达式> <命令...>\n表达式: @every 30s | 0 * * * * * | 0 0 9 * * *")
		}
		name := ctx.GetArg(1)
		schedule := ctx.GetArg(2)
		cmdText := ctx.GetArgs()[3:]

		handler := func(ctx context.Context) {
			fmt.Printf("[CRON] Executing: %v\n", cmdText)
		}

		if err := p.mgr.AddJob(name, schedule, handler); err != nil {
			return ctx.Edit(fmt.Sprintf("添加失败: %v", err))
		}
		return ctx.Edit(fmt.Sprintf("✅ 定时任务已添加: %s (%s)", name, schedule))

	case "list":
		jobs := p.mgr.GetJobs()
		if len(jobs) == 0 {
			return ctx.Edit("暂无定时任务")
		}
		var b strings.Builder
		b.WriteString("⏰ 定时任务列表:\n")
		for name, job := range jobs {
			next := "未知"
			if !job.NextRun.IsZero() {
				next = job.NextRun.Format("2006-01-02 15:04:05")
			}
			last := "从未"
			if !job.LastRun.IsZero() {
				last = job.LastRun.Format("2006-01-02 15:04:05")
			}
			b.WriteString(fmt.Sprintf("• %s\n  表达式: %s\n  下次: %s\n  上次: %s\n\n",
				name, job.Schedule, next, last))
		}
		return ctx.Edit(b.String())

	case "del", "delete", "remove":
		if ctx.ArgCount() < 2 {
			return ctx.Edit("用法: cron del <名称>")
		}
		name := ctx.GetArg(1)
		if err := p.mgr.RemoveJob(name); err != nil {
			return ctx.Edit(fmt.Sprintf("删除失败: %v", err))
		}
		return ctx.Edit(fmt.Sprintf("🗑 已删除: %s", name))

	case "run":
		if ctx.ArgCount() < 2 {
			return ctx.Edit("用法: cron run <名称>")
		}
		name := ctx.GetArg(1)
		if err := p.mgr.RunJob(ctx.Context(), name); err != nil {
			return ctx.Edit(fmt.Sprintf("执行失败: %v", err))
		}
		return ctx.Edit(fmt.Sprintf("▶️ 已执行: %s", name))

	default:
		return ctx.Edit("未知子命令: " + sub)
	}
}