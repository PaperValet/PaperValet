package builtin

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// DebugPlugin provides debugging and profiling commands.
type DebugPlugin struct{}

func NewDebug() *DebugPlugin { return &DebugPlugin{} }

func (p *DebugPlugin) Name() string        { return "debug" }
func (p *DebugPlugin) Description() string { return "调试与性能分析工具" }

func (p *DebugPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	cmds := []*interfaces.Command{
		{Name: "goroutines", Aliases: []string{"gr"}, Description: "显示 Goroutine 栈", Plugin: p.Name(), Category: "debug", OwnerOnly: true, Handler: p.handleGoroutines},
		{Name: "heap", Description: "显示堆内存概要", Plugin: p.Name(), Category: "debug", OwnerOnly: true, Handler: p.handleHeap},
		{Name: "stack", Description: "打印所有栈追踪", Plugin: p.Name(), Category: "debug", OwnerOnly: true, Handler: p.handleStack},
		{Name: "profile", Aliases: []string{"pprof"}, Description: "性能分析", Usage: "profile <cpu|mem|block|mutex> [时长秒数]", Plugin: p.Name(), Category: "debug", OwnerOnly: true, Handler: p.handleProfile},
		{Name: "memstats", Aliases: []string{"ms"}, Description: "显示内存统计详情", Plugin: p.Name(), Category: "debug", OwnerOnly: true, Handler: p.handleMemStats},
	}
	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *DebugPlugin) Start(_ context.Context) error { return nil }
func (p *DebugPlugin) Stop(_ context.Context) error  { return nil }

func (p *DebugPlugin) handleGoroutines(ctx *interfaces.CommandContext) error {
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	return ctx.Edit(fmt.Sprintf("🔀 <b>Goroutine 栈追踪</b> (%d bytes)\n\n<pre>%s</pre>", n, truncate(string(buf[:n]), 3800)))
}

func (p *DebugPlugin) handleHeap(ctx *interfaces.CommandContext) error {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return ctx.Edit(fmt.Sprintf("📊 <b>堆内存概要</b>\n\nAlloc: %.1f MB\nTotalAlloc: %.1f MB\nSys: %.1f MB\nNumGC: %d\nGCCPUFraction: %.2f%%",
		float64(mem.Alloc)/1024/1024, float64(mem.TotalAlloc)/1024/1024, float64(mem.Sys)/1024/1024, mem.NumGC, mem.GCCPUFraction*100))
}

func (p *DebugPlugin) handleStack(ctx *interfaces.CommandContext) error {
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	return ctx.Edit(fmt.Sprintf("📚 <b>所有栈追踪</b> (%d bytes)\n\n<pre>%s</pre>", n, truncate(string(buf[:n]), 3800)))
}

func (p *DebugPlugin) handleProfile(ctx *interfaces.CommandContext) error {
	if ctx.ArgCount() == 0 {
		return ctx.Edit("用法: profile <cpu|mem|block|mutex> [时长秒数]\n示例: profile cpu 30")
	}
	profType := ctx.GetArg(0)
	duration := 30
	if ctx.ArgCount() > 1 {
		if d, err := parseInt(ctx.GetArg(1)); err == nil && d > 0 && d <= 300 {
			duration = d
		}
	}

	switch profType {
	case "cpu":
		return p.runCPUProfile(ctx, duration)
	case "mem", "heap":
		return p.runMemProfile(ctx)
	case "block":
		return p.runBlockProfile(ctx, duration)
	case "mutex":
		return p.runMutexProfile(ctx, duration)
	default:
		return ctx.Edit("未知类型: " + profType)
	}
}

func (p *DebugPlugin) runCPUProfile(ctx *interfaces.CommandContext, duration int) error {
	file := fmt.Sprintf("/tmp/cpu_profile_%d.prof", time.Now().Unix())
	f, err := os.Create(file)
	if err != nil {
		return ctx.Edit("创建文件失败: " + err.Error())
	}
	defer f.Close()

	_ = ctx.Edit(fmt.Sprintf("⏳ CPU 分析运行中 (%ds)...", duration))
	if err := pprof.StartCPUProfile(f); err != nil {
		return ctx.Edit("启动失败: " + err.Error())
	}
	time.Sleep(time.Duration(duration) * time.Second)
	pprof.StopCPUProfile()
	return ctx.Edit(fmt.Sprintf("✅ CPU 分析完成\n文件: <code>%s</code>", file))
}

func (p *DebugPlugin) runMemProfile(ctx *interfaces.CommandContext) error {
	file := fmt.Sprintf("/tmp/mem_profile_%d.prof", time.Now().Unix())
	f, err := os.Create(file)
	if err != nil {
		return ctx.Edit("创建文件失败: " + err.Error())
	}
	defer f.Close()
	runtime.GC()
	if err := pprof.WriteHeapProfile(f); err != nil {
		return ctx.Edit("写入失败: " + err.Error())
	}
	return ctx.Edit(fmt.Sprintf("✅ 堆分析完成\n文件: <code>%s</code>", file))
}

func (p *DebugPlugin) runBlockProfile(ctx *interfaces.CommandContext, duration int) error {
	file := fmt.Sprintf("/tmp/block_profile_%d.prof", time.Now().Unix())
	f, err := os.Create(file)
	if err != nil {
		return ctx.Edit("创建文件失败: " + err.Error())
	}
	defer f.Close()
	_ = ctx.Edit(fmt.Sprintf("⏳ Block 分析运行中 (%ds)...", duration))
	runtime.SetBlockProfileRate(1)
	time.Sleep(time.Duration(duration) * time.Second)
	runtime.SetBlockProfileRate(0)
	if err := pprof.Lookup("block").WriteTo(f, 0); err != nil {
		return ctx.Edit("写入失败: " + err.Error())
	}
	return ctx.Edit(fmt.Sprintf("✅ Block 分析完成\n文件: <code>%s</code>", file))
}

func (p *DebugPlugin) runMutexProfile(ctx *interfaces.CommandContext, duration int) error {
	file := fmt.Sprintf("/tmp/mutex_profile_%d.prof", time.Now().Unix())
	f, err := os.Create(file)
	if err != nil {
		return ctx.Edit("创建文件失败: " + err.Error())
	}
	defer f.Close()
	_ = ctx.Edit(fmt.Sprintf("⏳ Mutex 分析运行中 (%ds)...", duration))
	runtime.SetMutexProfileFraction(1)
	time.Sleep(time.Duration(duration) * time.Second)
	runtime.SetMutexProfileFraction(0)
	if err := pprof.Lookup("mutex").WriteTo(f, 0); err != nil {
		return ctx.Edit("写入失败: " + err.Error())
	}
	return ctx.Edit(fmt.Sprintf("✅ Mutex 分析完成\n文件: <code>%s</code>", file))
}

func (p *DebugPlugin) handleMemStats(ctx *interfaces.CommandContext) error {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return ctx.Edit(fmt.Sprintf("📊 <b>内存统计详情</b>\n\n"+
		"Alloc: %.2f MB\nTotalAlloc: %.2f MB\nSys: %.2f MB\nLookups: %d\nMallocs: %d\nFrees: %d\n"+
		"HeapAlloc: %.2f MB\nHeapSys: %.2f MB\nHeapIdle: %.2f MB\nHeapInuse: %.2f MB\nHeapReleased: %.2f MB\n"+
		"HeapObjects: %d\nStackInuse: %.2f MB\nStackSys: %.2f MB\nMSpanInuse: %.2f MB\nMSpanSys: %.2f MB\n"+
		"MCacheInuse: %.2f MB\nMCacheSys: %.2f MB\nBuckHashSys: %.2f MB\nGCSys: %.2f MB\nOtherSys: %.2f MB\n"+
		"NextGC: %.2f MB\nLastGC: %s\nPauseTotalNs: %.2f ms\nNumGC: %d\nNumForcedGC: %d\nGCCPUFraction: %.4f%%",
		float64(mem.Alloc)/1024/1024, float64(mem.TotalAlloc)/1024/1024, float64(mem.Sys)/1024/1024,
		mem.Lookups, mem.Mallocs, mem.Frees,
		float64(mem.HeapAlloc)/1024/1024, float64(mem.HeapSys)/1024/1024, float64(mem.HeapIdle)/1024/1024,
		float64(mem.HeapInuse)/1024/1024, float64(mem.HeapReleased)/1024/1024,
		mem.HeapObjects, float64(mem.StackInuse)/1024/1024, float64(mem.StackSys)/1024/1024,
		float64(mem.MSpanInuse)/1024/1024, float64(mem.MSpanSys)/1024/1024,
		float64(mem.MCacheInuse)/1024/1024, float64(mem.MCacheSys)/1024/1024,
		float64(mem.BuckHashSys)/1024/1024, float64(mem.GCSys)/1024/1024, float64(mem.OtherSys)/1024/1024,
		float64(mem.NextGC)/1024/1024, time.Unix(0, int64(mem.LastGC)).Format("15:04:05"),
		float64(mem.PauseTotalNs)/1e6, mem.NumGC, mem.NumForcedGC, mem.GCCPUFraction*100))
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n... (truncated)"
}

var _ = strings.TrimSpace