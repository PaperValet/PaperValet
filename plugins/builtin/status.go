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

// StatusPlugin provides detailed system status and metrics.
type StatusPlugin struct {
	startTime time.Time
	version   string
	mgr       plugin.Manager
}

func NewStatus(version string) *StatusPlugin {
	return &StatusPlugin{startTime: time.Now(), version: version}
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
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	uptime := time.Since(p.startTime).Truncate(time.Second)

	// Memory breakdown
	allocMB := float64(mem.Alloc) / 1024 / 1024
	sysMB := float64(mem.Sys) / 1024 / 1024
	heapMB := float64(mem.HeapAlloc) / 1024 / 1024
	heapSysMB := float64(mem.HeapSys) / 1024 / 1024

	// GC stats
	nextGCMB := float64(mem.NextGC) / 1024 / 1024
	gcCount := mem.NumGC
	lastGC := time.Unix(0, int64(mem.LastGC)).Format("15:04:05")

	var b strings.Builder
	b.WriteString("📊 <b>PaperValet 系统状态</b>\n\n")
	b.WriteString(fmt.Sprintf("🏷 <b>版本:</b> %s\n", p.version))
	b.WriteString(fmt.Sprintf("⏱ <b>运行时间:</b> %s\n", uptime))
	b.WriteString(fmt.Sprintf("🔀 <b>Goroutines:</b> %d\n", runtime.NumGoroutine()))
	b.WriteString(fmt.Sprintf("🧵 <b>线程数:</b> %d\n", runtime.GOMAXPROCS(0)))
	b.WriteString("\n")

	b.WriteString("<b>💾 内存分配:</b>\n")
	b.WriteString(fmt.Sprintf("  分配: %.2f MB\n", allocMB))
	b.WriteString(fmt.Sprintf("  堆内存: %.2f MB / %.2f MB\n", heapMB, heapSysMB))
	b.WriteString(fmt.Sprintf("  系统申请: %.2f MB\n", sysMB))
	b.WriteString(fmt.Sprintf("  下次 GC: %.2f MB\n", nextGCMB))
	b.WriteString("\n")

	b.WriteString("<b>🗑 垃圾回收:</b>\n")
	b.WriteString(fmt.Sprintf("  次数: %d\n", gcCount))
	b.WriteString(fmt.Sprintf("  最近: %s\n", lastGC))
	b.WriteString(fmt.Sprintf("  CPU 分摊: %.2f%%\n", mem.GCCPUFraction*100))
	b.WriteString("\n")

	b.WriteString(fmt.Sprintf("📦 <b>插件:</b> %d 已加载\n", len(p.mgr.GetAllInfo())))
	b.WriteString(fmt.Sprintf("⚙️ <b>Go 版本:</b> %s\n", runtime.Version()))

	return ctx.Edit(b.String())
}