package builtin

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// CorePlugin provides fundamental commands.
type CorePlugin struct {
	startTime time.Time
	version   string
}

func NewCore(version string) *CorePlugin {
	return &CorePlugin{version: version, startTime: time.Now()}
}

func (p *CorePlugin) Name() string        { return "core" }
func (p *CorePlugin) Description() string { return "核心命令" }

func (p *CorePlugin) Init(_ context.Context, mgr plugin.Manager) error {
	cmds := []*interfaces.Command{
		{
			Name:        "version",
			Aliases:     []string{"v", "ver"},
			Description: "显示版本信息",
			Plugin:      p.Name(),
			Category:    "core",
			Handler:     p.handleVersion,
		},
		{
			Name:        "uptime",
			Aliases:     []string{"up"},
			Description: "显示运行时间",
			Plugin:      p.Name(),
			Category:    "core",
			Handler:     p.handleUptime,
		},
		{
			Name:        "ping",
			Description: "检查延迟",
			Plugin:      p.Name(),
			Category:    "core",
			Handler:     p.handlePing,
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

func (p *CorePlugin) handleVersion(ctx *interfaces.CommandContext) error {
	return ctx.Edit(ctx.T("core.version", p.version, runtime.Version(), p.startTime.Format("2006-01-02")))
}

func (p *CorePlugin) handleUptime(ctx *interfaces.CommandContext) error {
	uptime := time.Since(p.startTime).Truncate(time.Second)
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return ctx.Edit(ctx.T("core.uptime", uptime, fmt.Sprintf("%.1f", float64(mem.Alloc)/1024/1024), runtime.NumGoroutine()))
}

func (p *CorePlugin) handlePing(ctx *interfaces.CommandContext) error {
	start := time.Now()
	if err := ctx.Edit(ctx.T("core.ping_start")); err != nil {
		return err
	}
	latency := time.Since(start)
	return ctx.Edit(ctx.T("core.ping", latency))
}