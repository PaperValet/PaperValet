package builtin

import (
	"context"
	"fmt"
	"strings"

	"github.com/TiaraBasori/PaperValet/internal/cron"
	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
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

func (p *CronPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "cron",
		Aliases:     []string{"schedule", "timer"},
		Description: "定时任务管理",
		Usage:       "cron list | cron add <name> <expr> <cmd> | cron del <name> | cron run <name>",
		Plugin:      p.Name(),
		Category:    "tools",
		OwnerOnly:   true,
		Handler:     p.handleCron,
	})
}

func (p *CronPlugin) Start(_ context.Context) error { return nil }
func (p *CronPlugin) Stop(_ context.Context) error  { return nil }

func (p *CronPlugin) handleCron(ctx *interfaces.CommandContext) error {
	if ctx.ArgCount() == 0 {
		return ctx.Edit(
			"⏰ <b>Cron 定时任务</b>\n\n" +
				"<b>用法:</b>\n" +
				"• <code>cron list</code> — 列出所有任务\n" +
				"• <code>cron add &lt;名称&gt; &lt;表达式&gt; &lt;命令&gt;</code> — 添加任务\n" +
				"• <code>cron del &lt;名称&gt;</code> — 删除任务\n" +
				"• <code>cron run &lt;名称&gt;</code> — 立即执行\n\n" +
				"<b>表达式示例:</b>\n" +
				"• <code>@every 30s</code> — 每30秒\n" +
				"• <code>0 */5 * * * *</code> — 每5分钟\n" +
				"• <code>0 0 9 * * *</code> — 每天9点",
		)
	}

	sub := ctx.GetArg(0)

	switch sub {
	case "add", "create":
		if ctx.ArgCount() < 4 {
			return ctx.Edit("用法: cron add <名称> <表达式> <命令...>")
		}
		name := ctx.GetArg(1)
		schedule := ctx.GetArg(2)
		cmdText := strings.Join(ctx.Args[3:], " ")

		handler := func(ctx context.Context) {
			// In real implementation, this would execute the command
			fmt.Printf("[CRON] Executing: %s (%s)\n", name, cmdText)
		}

		if err := p.mgr.AddJob(name, schedule, handler); err != nil {
			return ctx.Edit(fmt.Sprintf("❌ 添加失败: %v", err))
		}
		return ctx.Edit(fmt.Sprintf("✅ 定时任务已添加: <b>%s</b> (%s)", name, schedule))

	case "list", "ls":
		jobs := p.mgr.GetJobs()
		if len(jobs) == 0 {
			return ctx.Edit("⏰ 暂无定时任务")
		}
		var b strings.Builder
		b.WriteString("⏰ <b>定时任务列表</b>\n\n")
		for name, job := range jobs {
			next := "未知"
			if !job.NextRun.IsZero() {
				next = job.NextRun.Format("01-02 15:04:05")
			}
			last := "从未"
			if !job.LastRun.IsZero() {
				last = job.LastRun.Format("01-02 15:04:05")
			}
			b.WriteString(fmt.Sprintf(
				"• <b>%s</b>\n"+
					"  表达式: <code>%s</code>\n"+
					"  下次: %s\n"+
					"  上次: %s\n\n",
				name, job.Schedule, next, last,
			))
		}
		return ctx.Edit(b.String())

	case "del", "delete", "remove":
		if ctx.ArgCount() < 2 {
			return ctx.Edit("用法: cron del <名称>")
		}
		name := ctx.GetArg(1)
		if err := p.mgr.RemoveJob(name); err != nil {
			return ctx.Edit(fmt.Sprintf("❌ 删除失败: %v", err))
		}
		return ctx.Edit(fmt.Sprintf("🗑 已删除: %s", name))

	case "run", "execute":
		if ctx.ArgCount() < 2 {
			return ctx.Edit("用法: cron run <名称>")
		}
		name := ctx.GetArg(1)
		if err := p.mgr.RunJob(ctx.Context(), name); err != nil {
			return ctx.Edit(fmt.Sprintf("❌ 执行失败: %v", err))
		}
		return ctx.Edit(fmt.Sprintf("▶️ 已执行: %s", name))

	case "help", "h":
		return ctx.Edit(
			"⏰ <b>Cron 定时任务</b>\n\n" +
				"<b>用法:</b>\n" +
				"• <code>cron add &lt;名称&gt; &lt;表达式&gt; &lt;命令&gt;</code>\n" +
				"• <code>cron list</code>\n" +
				"• <code>cron del &lt;名称&gt;</code>\n" +
				"• <code>cron run &lt;名称&gt;</code>",
		)

	default:
		return ctx.Edit("未知子命令: " + sub + "\n\n使用 <code>cron help</code> 查看帮助")
	}
}