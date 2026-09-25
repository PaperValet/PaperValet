package builtin

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// StatusPlugin provides detailed system status and metrics.
type StatusPlugin struct {
	version string
	mgr     plugin.Manager
}

func NewStatus(version string) *StatusPlugin {
	return &StatusPlugin{version: version}
}

func (p *StatusPlugin) Name() string        { return "status" }
func (p *StatusPlugin) Description() string { return "系统状态与性能监控" }

func (p *StatusPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	p.mgr = mgr
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "status",
		Aliases:     []string{"stat", "st"},
		Description: "显示运行状态与性能指标",
		Plugin:      p.Name(),
		Category:    "core",
		Handler:     p.handleStatus,
	})
}

func (p *StatusPlugin) Start(_ context.Context) error { return nil }
func (p *StatusPlugin) Stop(_ context.Context) error  { return nil }

func (p *StatusPlugin) handleStatus(ctx *interfaces.CommandContext) error {
	var b strings.Builder
	b.WriteString("📊 <b>PaperValet 系统状态</b>\n\n")
	b.WriteString(fmt.Sprintf("🏷 <b>版本:</b> %s\n", p.version))
	b.WriteString(fmt.Sprintf("🔀 <b>Goroutines:</b> %d\n", runtime.NumGoroutine()))
	b.WriteString(fmt.Sprintf("🧵 <b>线程数:</b> %d\n", runtime.GOMAXPROCS(0)))
	b.WriteString(fmt.Sprintf("📦 <b>插件:</b> %d 已加载\n", len(p.mgr.GetAllInfo())))
	b.WriteString(fmt.Sprintf("⚙️ <b>Go 版本:</b> %s\n", runtime.Version()))
	return ctx.Edit(b.String())
}
