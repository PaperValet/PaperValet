package builtin

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// StatusPlugin reports version, uptime and runtime metrics.
// It absorbed the old standalone version command.
type StatusPlugin struct {
	version   string
	mgr       plugin.Manager
	startTime time.Time
}

func NewStatus(version string) *StatusPlugin {
	return &StatusPlugin{version: version, startTime: time.Now()}
}

func (p *StatusPlugin) Name() string        { return "status" }
func (p *StatusPlugin) Description() string { return "版本、运行时间与资源状态" }
func (p *StatusPlugin) DescEN() string      { return "Version, uptime and runtime status" }

func (p *StatusPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	p.mgr = mgr
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "status",
		Aliases:     []string{"stat", "st", "version", "v", "ver"},
		Description: "查看版本、运行时长、插件数与资源占用",
		DescEN:      "Show version, uptime, plugin count and resource usage",
		Usage:       "status",
		UsageEN:     "status",
		Plugin:      p.Name(),
		Category:    "core",
		Handler:     p.handleStatus,
	})
}

func (p *StatusPlugin) Start(_ context.Context) error { return nil }
func (p *StatusPlugin) Stop(_ context.Context) error  { return nil }

func (p *StatusPlugin) handleStatus(ctx *interfaces.CommandContext) error {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	up := time.Since(p.startTime).Round(time.Second)
	plugins := 0
	if p.mgr != nil {
		plugins = len(p.mgr.GetAllInfo())
	}

	var b strings.Builder
	if ctx.Lang == "en-US" {
		fmt.Fprintf(&b, "📊 <b>PaperValet</b>\n\n")
		fmt.Fprintf(&b, "🏷 Version: <b>%s</b>\n", p.version)
		fmt.Fprintf(&b, "⏱ Uptime: %s\n", up)
		fmt.Fprintf(&b, "🧠 Memory: %s · goroutines %d\n", formatBytes(int64(m.HeapAlloc)), runtime.NumGoroutine())
		fmt.Fprintf(&b, "📦 Plugins: %d\n", plugins)
		fmt.Fprintf(&b, "⚙️ Go %s\n", runtime.Version())
	} else {
		fmt.Fprintf(&b, "📊 <b>PaperValet 状态</b>\n\n")
		fmt.Fprintf(&b, "🏷 版本: <b>%s</b>\n", p.version)
		fmt.Fprintf(&b, "⏱ 已运行: %s\n", up)
		fmt.Fprintf(&b, "🧠 内存: %s · 协程 %d\n", formatBytes(int64(m.HeapAlloc)), runtime.NumGoroutine())
		fmt.Fprintf(&b, "📦 插件: %d 个\n", plugins)
		fmt.Fprintf(&b, "⚙️ Go %s\n", runtime.Version())
	}
	return ctx.Edit(b.String())
}
