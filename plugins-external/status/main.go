package main

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type StatusPlugin struct {
	startTime time.Time
}

func New() (plugin.Plugin, error) {
	return &StatusPlugin{startTime: time.Now()}, nil
}

var Metadata = &plugin.PluginMetadata{
	Name:        "status",
	Description: "系统状态监控",
	Version:     "1.0.0",
	Author:      "PaperValet",
	MinVersion:  "0.1.0",
}

func (p *StatusPlugin) Name() string        { return "status" }
func (p *StatusPlugin) Description() string { return "系统状态监控" }

func (p *StatusPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	cmds := []*plugin.Command{
		{
			Name:        "status",
			Aliases:     []string{"stat", "st"},
			Description: "显示运行状态",
			Plugin:      p.Name(),
			Category:    "core",
			Handler:     p.handleStatus,
		},
		{
			Name:        "sysinfo",
			Description: "显示系统信息",
			Plugin:      p.Name(),
			Category:    "tools",
			Handler:     p.handleSysInfo,
		},
		{
			Name:        "memory",
			Aliases:     []string{"mem"},
			Description: "显示内存使用情况",
			Plugin:      p.Name(),
			Category:    "tools",
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

func (p *StatusPlugin) handleStatus(ctx *plugin.CommandContext) error {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	uptime := time.Since(p.startTime).Truncate(time.Second)

	// Get plugin info
	infos := ctx.Manager().GetAllInfo()
	active := 0
	for _, i := range infos {
		if i.Status == plugin.StatusActive {
			active++
		}
	}

	return ctx.Edit(fmt.Sprintf(`📊 <b>PaperValet 状态</b>

🤖 <b>版本:</b> <code>0.1.0</code>
⏱ <b>运行时间:</b> <code>%s</code>
📦 <b>插件:</b> <code>%d/%d</code>
🔀 <b>Goroutines:</b> <code>%d</code>
🧠 <b>内存:</b> <code>%.1f MB</code>

⏰ <i>%s</i>`,
		uptime,
		active, len(infos),
		runtime.NumGoroutine(),
		float64(mem.Alloc)/1024/1024,
		time.Now().Format("2006-01-02 15:04:05"),
	))
}

func (p *StatusPlugin) handleSysInfo(ctx *plugin.CommandContext) error {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	uptime := time.Since(p.startTime).Truncate(time.Second)

	return ctx.Edit(fmt.Sprintf(`🖥 <b>系统信息</b>

🤖 <b>PaperValet:</b> <code>0.1.0</code>
⏱ <b>运行时间:</b> <code>%s</code>
🐹 <b>Go版本:</b> <code>%s</code>
🖥 <b>操作系统:</b> <code>%s/%s</code>
🔀 <b>CPU核心:</b> <code>%d</code>
🔀 <b>Goroutines:</b> <code>%d</code>

🧠 <b>内存统计:</b>
  分配: <code>%.1f MB</code>
  总分配: <code>%.1f MB</code>
  系统: <code>%.1f MB</code>
  堆对象: <code>%d</code>
  GC次数: <code>%d</code>
  GC暂停总计: <code>%d ms</code>

⏰ <i>%s</i>`,
		uptime,
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH,
		runtime.NumCPU(),
		runtime.NumGoroutine(),
		float64(mem.Alloc)/1024/1024,
		float64(mem.TotalAlloc)/1024/1024,
		float64(mem.Sys)/1024/1024,
		mem.Mallocs-mem.Frees,
		mem.NumGC,
		mem.PauseTotalNs/1000000,
		time.Now().Format("2006-01-02 15:04:05"),
	))
}

func (p *StatusPlugin) handleMemory(ctx *plugin.CommandContext) error {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	var b strings.Builder
	b.WriteString("🧠 <b>内存详情</b>\n\n")
	b.WriteString(fmt.Sprintf("堆分配: <code>%.1f MB</code>\n", float64(mem.HeapAlloc)/1024/1024))
	b.WriteString(fmt.Sprintf("堆系统: <code>%.1f MB</code>\n", float64(mem.HeapSys)/1024/1024))
	b.WriteString(fmt.Sprintf("堆空闲: <code>%.1f MB</code>\n", float64(mem.HeapIdle)/1024/1024))
	b.WriteString(fmt.Sprintf("堆使用: <code>%.1f MB</code>\n", float64(mem.HeapInuse)/1024/1024))
	b.WriteString(fmt.Sprintf("堆释放: <code>%.1f MB</code>\n", float64(mem.HeapReleased)/1024/1024))
	b.WriteString(fmt.Sprintf("栈使用: <code>%.1f MB</code>\n", float64(mem.StackInuse)/1024/1024))
	b.WriteString(fmt.Sprintf("栈系统: <code>%.1f MB</code>\n", float64(mem.StackSys)/1024/1024))
	b.WriteString(fmt.Sprintf("总分配: <code>%.1f MB</code>\n", float64(mem.TotalAlloc)/1024/1024))
	b.WriteString(fmt.Sprintf("系统内存: <code>%.1f MB</code>\n", float64(mem.Sys)/1024/1024))
	b.WriteString(fmt.Sprintf("查找表: <code>%.1f MB</code>\n", float64(mem.Lookups)/1024/1024))
	b.WriteString(fmt.Sprintf("Mallocs: <code>%d</code>\n", mem.Mallocs))
	b.WriteString(fmt.Sprintf("Frees: <code>%d</code>\n", mem.Frees))
	b.WriteString(fmt.Sprintf("活跃对象: <code>%d</code>\n", mem.Mallocs-mem.Frees))
	b.WriteString(fmt.Sprintf("\nGC次数: <code>%d</code>\n", mem.NumGC))
	b.WriteString(fmt.Sprintf("GC暂停总计: <code>%d ms</code>\n", mem.PauseTotalNs/1000000))
	b.WriteString(fmt.Sprintf("最后GC: <code>%s</code>\n", time.Unix(0, int64(mem.LastGC)).Format("15:04:05")))

	b.WriteString(fmt.Sprintf("\n⏰ <i>%s</i>", time.Now().Format("2006-01-02 15:04:05")))

	return ctx.Edit(b.String())
}