package main

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type AdminPlugin struct {
	startTime time.Time
}

func New() (plugin.Plugin, error) {
	return &AdminPlugin{startTime: time.Now()}, nil
}

var Metadata = &plugin.PluginMetadata{
	Name:        "admin",
	Description: "管理员命令",
	Version:     "1.0.0",
	Author:      "PaperValet",
	MinVersion:  "0.1.0",
}

func (p *AdminPlugin) Name() string        { return "admin" }
func (p *AdminPlugin) Description() string { return "管理员命令" }

func (p *AdminPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	cmds := []*plugin.Command{
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
			Description: "强制垃圾回收 (仅拥有者)",
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
		{
			Name:        "stats",
			Description: "显示详细统计信息 (仅拥有者)",
			Plugin:      p.Name(),
			Category:    "admin",
			OwnerOnly:   true,
			Handler:     p.handleStats,
		},
	}

	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *AdminPlugin) handleRestart(ctx *plugin.CommandContext) error {
	return ctx.Edit("🔄 重启功能需要外部进程管理器 (systemd/PM2) 支持")
}

func (p *AdminPlugin) handleShutdown(ctx *plugin.CommandContext) error {
	return ctx.Edit("🛑 关闭功能需要外部进程管理器支持")
}

func (p *AdminPlugin) handleGC(ctx *plugin.CommandContext) error {
	var mem runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&mem)
	return ctx.Edit(fmt.Sprintf("🗑 GC 完成\n内存: %.1f MB\n对象: %d\nGC次数: %d",
		float64(mem.Alloc)/1024/1024,
		mem.Mallocs-mem.Frees,
		mem.NumGC))
}

func (p *AdminPlugin) handleVersion(ctx *plugin.CommandContext) error {
	uptime := time.Since(p.startTime).Truncate(time.Second)
	return ctx.Edit(fmt.Sprintf("PaperValet %s\n运行时间: %s\nGo: %s", "0.1.0", uptime, runtime.Version()))
}

func (p *AdminPlugin) handleStats(ctx *plugin.CommandContext) error {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	uptime := time.Since(p.startTime).Truncate(time.Second)

	return ctx.Edit(fmt.Sprintf(`📊 <b>详细统计</b>

⏱ <b>运行时间:</b> <code>%s</code>
🧠 <b>内存:</b>
  分配: <code>%.1f MB</code>
  总分配: <code>%.1f MB</code>
  系统: <code>%.1f MB</code>
  堆对象: <code>%d</code>
  GC次数: <code>%d</code>
  GC暂停总计: <code>%d ms</code>

🔀 <b>Goroutines:</b> <code>%d</code>
🖥 <b>CPU核心:</b> <code>%d</code>
📦 <b>Go版本:</b> <code>%s</code>
`,
		uptime,
		float64(mem.Alloc)/1024/1024,
		float64(mem.TotalAlloc)/1024/1024,
		float64(mem.Sys)/1024/1024,
		mem.Mallocs-mem.Frees,
		mem.NumGC,
		mem.PauseTotalNs/1000000,
		runtime.NumGoroutine(),
		runtime.NumCPU(),
		runtime.Version(),
	))
}