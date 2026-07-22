package builtin

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/internal/command"
	"github.com/TiaraBasori/PaperValet/internal/core"
	"github.com/TiaraBasori/PaperValet/internal/cron"
	"github.com/TiaraBasori/PaperValet/internal/plugin"
)

// CronPlugin provides cron management commands.
type CronPlugin struct {
	mgr *cron.Manager
}

func NewCron(cronMgr *cron.Manager) *CronPlugin {
	return &CronPlugin{mgr: cronMgr}
}

func (p *CronPlugin) Name() string        { return "cron" }
func (p *CronPlugin) Description() string { return "定时任务管理" }

func (p *CronPlugin) Init(_ context.Context, mgr *plugin.Manager) error {
	cmds := []*command.Command{
		{
			Name:        "cron",
			Aliases:     []string{"schedule"},
			Description: "定时任务管理",
			Usage:       "cron add <名称> <表达式> <命令> | cron list | cron del <名称> | cron run <名称>",
			Plugin:      p.Name(),
			Category:    "tools",
			Handler:     p.handleCron,
		},
	}
	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *CronPlugin) Start(_ context.Context) error { return nil }
func (p *CronPlugin) Stop(_ context.Context) error  { return nil }

func (p *CronPlugin) handleCron(ctx *core.CommandContext) error {
	if ctx.ArgCount() == 0 {
		return ctx.Edit("用法: cron add <名称> <表达式> <命令> | cron list | cron del <名称> | cron run <名称>")
	}
	sub := strings.ToLower(ctx.GetArg(0))

	switch sub {
	case "add":
		if ctx.ArgCount() < 4 {
			return ctx.Edit("用法: cron add <名称> <表达式> <命令...>\n表达式: @every 30s | 0 * * * * * | 0 0 9 * * *")
		}
		name := ctx.GetArg(1)
		schedule := ctx.GetArg(2)
		cmdText := strings.Join(ctx.Args[3:], " ")

		// Wrap command execution
		handler := func(ctx context.Context) error {
			// This would need access to command registry to execute
			// For now, just log
			fmt.Printf("[CRON] Executing: %s\n", cmdText)
			return nil
		}

		if err := p.mgr.AddJob(name, schedule, handler); err != nil {
			return ctx.Edit(fmt.Sprintf("添加失败: %v", err))
		}
		return ctx.Edit(fmt.Sprintf("✅ 定时任务已添加: %s (%s)", name, schedule))

	case "list":
		jobs := p.mgr.GetAllJobs()
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
			errStr := ""
			if job.LastError != nil {
				errStr = fmt.Sprintf(" ❌ %v", job.LastError)
			}
			b.WriteString(fmt.Sprintf("• %s\n  表达式: %s\n  下次: %s\n  上次: %s (运行 %d 次)%s\n",
				name, job.Schedule, next, last, job.RunCount, errStr))
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