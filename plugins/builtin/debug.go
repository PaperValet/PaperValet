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
		{
			Name:        "goroutines",
			Aliases:     []string{"gr", "goroutine"},
			Description: "显示 Goroutine 信息",
			Plugin:      p.Name(),
			Category:    "debug",
			OwnerOnly:   true,
			Handler:     p.handleGoroutines,
		},
		{
			Name:        "heap",
			Description: "显示堆内存分析",
			Plugin:      p.Name(),
			Category:    "debug",
			OwnerOnly:   true,
			Handler:     p.handleHeap,
		},
		{
			Name:        "stack",
			Description: "打印所有栈追踪",
			Plugin:      p.Name(),
			Category:    "debug",
			OwnerOnly:   true,
			Handler:     p.handleStack,
		},
		{
			Name:        "profile",
			Aliases:     []string{"pprof"},
			Description: "性能分析",
			Usage:       "profile <cpu|mem|block|mutex> [时长秒数]",
			Plugin:      p.Name(),
			Category:    "debug",
			OwnerOnly:   true,
			Handler:     p.handleProfile,
		},
		{
			Name:        "memstats",
			Aliases:     []string{"ms"},
			Description: "显示内存统计详情",
			Plugin:      p.Name(),
			Category:    "debug",
			OwnerOnly:   true,
			Handler:     p.handleMemStats,
		},
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
	return ctx.Edit(fmt.Sprintf("🔀 <b>Goroutine 栈追踪</b> (%d bytes)\n\n<pre>%s</pre>", n, string(buf[:n])))
}

func (p *DebugPlugin) handleHeap(ctx *interfaces.CommandContext) error {
	var b strings.Builder
	b.WriteString("📊 <b>堆内存概要</b>\n\n")

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	b.WriteString(fmt.Sprintf("Alloc: %.1f MB\n", float64(mem.Alloc)/1024/1024))
	b.WriteString(fmt.Sprintf("TotalAlloc: %.1f MB\n", float64(mem.TotalAlloc)/1024/1024))
	b.WriteString(fmt.Sprintf("Sys: %.1f MB\n", float64(mem.Sys)/1024/1024))
	b.WriteString(fmt.Sprintf("NumGC: %d\n", mem.NumGC))
	b.WriteString(fmt.Sprintf("GCCPUFraction: %.2f%%\n", mem.GCCPUFraction*100))

	return ctx.Edit(b.String())
}

func (p *DebugPlugin) handleStack(ctx *interfaces.CommandContext) error {
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	return ctx.Edit(fmt.Sprintf("📚 <b>所有栈追踪</b> (%d bytes)\n\n<pre>%s</pre>", n, string(buf[:n])))
}

func (p *DebugPlugin) handleProfile(ctx *interfaces.CommandContext) error {
	args := ctx.Args
	if len(args) == 0 {
		return ctx.Edit("用法: profile <cpu|mem|block|mutex> [时长秒数]\n示例: profile cpu 30")
	}

	profType := args[0]
	duration := 30
	if len(args) > 1 {
		fmt.Sscanf(args[1], "%d", &duration)
		if duration <= 0 || duration > 300 {
			duration = 30
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
		return ctx.Edit("未知类型: " + profType + " (支持: cpu, mem, block, mutex)")
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

	return ctx.Edit(fmt.Sprintf("✅ CPU 分析完成\n文件: <code>%s</code>\n用 go tool pprof 分析", file))
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

	runtime.SetBlockProfileRate(1)
	_ = ctx.Edit(fmt.Sprintf("⏳ 阻塞分析运行中 (%ds)...", duration))
	time.Sleep(time.Duration(duration) * time.Second)
	runtime.SetBlockProfileRate(0)

	if err := pprof.Lookup("block").WriteTo(f, 0); err != nil {
		return ctx.Edit("写入失败: " + err.Error())
	}

	return ctx.Edit(fmt.Sprintf("✅ 阻塞分析完成\n文件: <code>%s</code>", file))
}

func (p *DebugPlugin) runMutexProfile(ctx *interfaces.CommandContext, duration int) error {
	file := fmt.Sprintf("/tmp/mutex_profile_%d.prof", time.Now().Unix())
	f, err := os.Create(file)
	if err != nil {
		return ctx.Edit("创建文件失败: " + err.Error())
	}
	defer f.Close()

	runtime.SetMutexProfileFraction(1)
	_ = ctx.Edit(fmt.Sprintf("⏳ 互斥锁分析运行中 (%ds)...", duration))
	time.Sleep(time.Duration(duration) * time.Second)
	runtime.SetMutexProfileFraction(0)

	if err := pprof.Lookup("mutex").WriteTo(f, 0); err != nil {
		return ctx.Edit("写入失败: " + err.Error())
	}

	return ctx.Edit(fmt.Sprintf("✅ 互斥锁分析完成\n文件: <code>%s</code>", file))
}

func (p *DebugPlugin) handleMemStats(ctx *interfaces.CommandContext) error {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	var b strings.Builder
	b.WriteString("📊 <b>内存统计详情</b>\n\n")

	fields := []struct {
		name string
		val  uint64
	}{
		{"Alloc", mem.Alloc}, {"TotalAlloc", mem.TotalAlloc}, {"Sys", mem.Sys},
		{"Lookups", mem.Lookups}, {"Mallocs", mem.Mallocs}, {"Frees", mem.Frees},
		{"HeapAlloc", mem.HeapAlloc}, {"HeapSys", mem.HeapSys}, {"HeapIdle", mem.HeapIdle},
		{"HeapInuse", mem.HeapInuse}, {"HeapReleased", mem.HeapReleased},
		{"HeapObjects", mem.HeapObjects}, {"StackInuse", mem.StackInuse},
		{"StackSys", mem.StackSys}, {"MSpanInuse", mem.MSpanInuse},
		{"MSpanSys", mem.MSpanSys}, {"MCacheInuse", mem.MCacheInuse},
		{"MCacheSys", mem.MCacheSys}, {"BuckHashSys", mem.BuckHashSys},
		{"GCSys", mem.GCSys}, {"OtherSys", mem.OtherSys}, {"NextGC", mem.NextGC},
	}

	for _, f := range fields {
		b.WriteString(fmt.Sprintf("%s: %s\n", f.name, formatBytesUint64(f.val)))
	}

	b.WriteString(fmt.Sprintf("LastGC: %s\n", time.Unix(0, int64(mem.LastGC)).Format("15:04:05")))
	b.WriteString(fmt.Sprintf("PauseTotalNs: %s\n", formatDurationNs(mem.PauseTotalNs)))
	b.WriteString(fmt.Sprintf("NumGC: %d\n", mem.NumGC))
	b.WriteString(fmt.Sprintf("NumForcedGC: %d\n", mem.NumForcedGC))
	b.WriteString(fmt.Sprintf("GCCPUFraction: %.4f%%\n", mem.GCCPUFraction*100))

	return ctx.Edit(b.String())
}

func formatBytesUint64(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func formatDurationNs(ns uint64) string {
	ms := float64(ns) / 1e6
	if ms < 1000 {
		return fmt.Sprintf("%.2f ms", ms)
	}
	s := ms / 1000
	if s < 60 {
		return fmt.Sprintf("%.2f s", s)
	}
	m := s / 60
	return fmt.Sprintf("%.2f m", m)
}