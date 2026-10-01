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
func (p *StatusPlugin) Description() string { return "运行状态" }
func (p *StatusPlugin) DescEN() string      { return "Version, uptime and runtime status" }

func (p *StatusPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	p.mgr = mgr
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "status",
		Description: "查看运行状态",
		DescEN:      "Show runtime status",
		Usage: `status

<b>显示</b>
• 版本号
• 本次运行时长
• Go 堆内存占用和协程数
• 已注册插件数（内建 + 外部）
• Go 版本`,
		UsageEN: `status

<b>Shows</b>
• version
• uptime of this run
• Go heap usage and goroutine count
• registered plugins (built-in + external)
• Go version`,
		Plugin:   p.Name(),
		Category: "core",
		Handler:  p.handleStatus,
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
