package builtin

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// MemoryPlugin provides memory monitoring with auto GC.
// Renamed from health to match TeleBox's memory plugin semantics.
type MemoryPlugin struct {
	thresholdMB float64
}

func NewMemory() *MemoryPlugin {
	return &MemoryPlugin{thresholdMB: 150}
}

func (p *MemoryPlugin) Name() string        { return "memory" }
func (p *MemoryPlugin) Description() string { return "内存守护 — 监控/自动清理" }

func (p *MemoryPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	cmds := []*interfaces.Command{
		{
			Name:        "memory",
			Aliases:     []string{"mem", "health"},
			Description: "内存守护 — 查看状态/控制",
			Usage:       "memory [on|off|status]",
			Plugin:      p.Name(),
			Category:    "admin",
			OwnerOnly:   true,
			Handler:     p.handleMemory,
		},
	}
	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *MemoryPlugin) Start(ctx context.Context) error {
	go p.monitorLoop(ctx)
	return nil
}

func (p *MemoryPlugin) Stop(_ context.Context) error { return nil }

func (p *MemoryPlugin) handleMemory(ctx *interfaces.CommandContext) error {
	if ctx.ArgCount() == 0 || ctx.GetArg(0) == "status" {
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)
		return ctx.Edit(fmt.Sprintf("🧠 <b>内存守护状态</b>\n\n"+
			"Heap 使用: %.1f MB\n"+
			"Heap 总量: %.1f MB\n"+
			"系统申请: %.1f MB\n"+
			"GC 次数: %d\n"+
			"阈值: %.0f MB",
			float64(mem.HeapAlloc)/1024/1024,
			float64(mem.HeapSys)/1024/1024,
			float64(mem.Sys)/1024/1024,
			mem.NumGC,
			p.thresholdMB,
		))
	}
	return ctx.Edit("用法: memory [status]")
}

func (p *MemoryPlugin) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var mem runtime.MemStats
			runtime.ReadMemStats(&mem)
			heapMB := float64(mem.HeapAlloc) / 1024 / 1024
			if heapMB > p.thresholdMB {
				runtime.GC()
			}
		}
	}
}