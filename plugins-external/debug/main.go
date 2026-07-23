package main

import (
	"context"
	"fmt"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type DebugPlugin struct{}

func New() (plugin.Plugin, error) {
	return &DebugPlugin{}, nil
}

var Metadata = &plugin.PluginMetadata{
	Name:        "debug",
	Description: "调试工具",
	Version:     "1.0.0",
	Author:      "PaperValet",
	MinVersion:  "0.1.0",
}

func (p *DebugPlugin) Name() string        { return "debug" }
func (p *DebugPlugin) Description() string { return "调试工具" }

func (p *DebugPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	cmds := []*plugin.Command{
		{
			Name:        "goroutines",
			Aliases:     []string{"gr", "goroutine"},
			Description: "显示Goroutine信息",
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
			Usage:       "profile <cpu|mem|block|mutex> [时长]",
			Plugin:      p.Name(),
			Category:    "debug",
			OwnerOnly:   true,
			Handler:     p.handleProfile,
		},
	}

	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *DebugPlugin) handleGoroutines(ctx *plugin.CommandContext) error {
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	return ctx.Edit(fmt.Sprintf("🔀 <b>Goroutines (%d)</b>\n\n<pre>%s</pre>", runtime.NumGoroutine(), string(buf[:n])))
}

func (p *DebugPlugin) handleHeap(ctx *plugin.CommandContext) error {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	return ctx.Edit(fmt.Sprintf(`📦 <b>堆内存</b>

分配: <code>%.1f MB</code>
系统: <code>%.1f MB</code>
空闲: <code>%.1f MB</code>
使用: <code>%.1f MB</code>
释放: <code>%.1f MB</code>
对象: <code>%d</code>
GC次数: <code>%d</code>`,
		float64(mem.HeapAlloc)/1024/1024,
		float64(mem.HeapSys)/1024/1024,
		float64(mem.HeapIdle)/1024/1024,
		float64(mem.HeapInuse)/1024/1024,
		float64(mem.HeapReleased)/1024/1024,
		mem.HeapObjects,
		mem.NumGC,
	))
}

func (p *DebugPlugin) handleStack(ctx *plugin.CommandContext) error {
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	if n > 3500 {
		return ctx.Edit(fmt.Sprintf("<pre>%s</pre>\n... (输出过长，已截断)", string(buf[:3500])))
	}
	return ctx.Edit(fmt.Sprintf("<pre>%s</pre>", string(buf[:n])))
}

func (p *DebugPlugin) handleProfile(ctx *plugin.CommandContext) error {
	args := ctx.Args()
	if len(args) == 0 {
		return ctx.Edit("用法: profile <cpu|mem|block|mutex> [秒]")
	}

	profileType := args[0]
	duration := 10 * time.Second
	if len(args) > 1 {
		if d, err := time.ParseDuration(args[1] + "s"); err == nil {
			duration = d
		}
	}

	ctx.Edit(fmt.Sprintf("⏳ 开始 %s 分析 (%v)...", profileType, duration))

	var pprofErr error
	switch profileType {
	case "cpu":
		pprofErr = pprof.StartCPUProfile(nil)
		if pprofErr == nil {
			time.Sleep(duration)
			pprof.StopCPUProfile()
		}
	case "mem":
		runtime.GC()
		pprofErr = pprof.WriteHeapProfile(nil)
	case "block":
		runtime.SetBlockProfileRate(1)
		time.Sleep(duration)
		runtime.SetBlockProfileRate(0)
		pprofErr = pprof.Lookup("block").WriteTo(nil, 0)
	case "mutex":
		runtime.SetMutexProfileFraction(1)
		time.Sleep(duration)
		runtime.SetMutexProfileFraction(0)
		pprofErr = pprof.Lookup("mutex").WriteTo(nil, 0)
	default:
		return ctx.Edit("未知类型: cpu, mem, block, mutex")
	}

	if pprofErr != nil {
		return ctx.Edit(fmt.Sprintf("❌ 分析失败: %v", pprofErr))
	}

	return ctx.Edit(fmt.Sprintf("✅ %s 分析完成，耗时 %v", profileType, duration))
}

func (p *DebugPlugin) Start(ctx context.Context) error { return nil }
func (p *DebugPlugin) Stop(ctx context.Context) error  { return nil }