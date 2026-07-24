package builtin

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// AdminPlugin provides owner-only system commands.
type AdminPlugin struct {
	startTime time.Time
}

func NewAdmin() *AdminPlugin { return &AdminPlugin{startTime: time.Now()} }

func (p *AdminPlugin) Name() string        { return "admin" }
func (p *AdminPlugin) Description() string { return "管理员命令（重启/关闭/GC）" }

func (p *AdminPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	cmds := []*interfaces.Command{
		{
			Name:        "restart",
			Description: "重启机器人进程",
			Usage:       "restart",
			Plugin:      p.Name(),
			Category:    "admin",
			OwnerOnly:   true,
			Handler:     p.handleRestart,
		},
		{
			Name:        "shutdown",
			Aliases:     []string{"halt", "stop"},
			Description: "关闭机器人进程",
			Usage:       "shutdown",
			Plugin:      p.Name(),
			Category:    "admin",
			OwnerOnly:   true,
			Handler:     p.handleShutdown,
		},
		{
			Name:        "gc",
			Description: "强制触发垃圾回收",
			Usage:       "gc",
			Plugin:      p.Name(),
			Category:    "admin",
			OwnerOnly:   true,
			Handler:     p.handleGC,
		},
	}
	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *AdminPlugin) Start(_ context.Context) error { return nil }
func (p *AdminPlugin) Stop(_ context.Context) error  { return nil }

func (p *AdminPlugin) handleRestart(ctx *interfaces.CommandContext) error {
	_ = ctx.Edit("🔄 正在重启...")
	go func() {
		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()
	return nil
}

func (p *AdminPlugin) handleShutdown(ctx *interfaces.CommandContext) error {
	_ = ctx.Edit("🛑 正在关闭...")
	go func() {
		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()
	return nil
}

func (p *AdminPlugin) handleGC(ctx *interfaces.CommandContext) error {
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	beforeAlloc := before.Alloc

	runtime.GC()

	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	freed := beforeAlloc - after.Alloc
	elapsed := time.Since(p.startTime).Truncate(time.Second)
	return ctx.Edit(fmt.Sprintf(
		"🗑 <b>GC 完成</b>\n\n"+
			"之前: %.1f MB → 之后: %.1f MB\n"+
			"释放: %.1f MB\n"+
			"GC 次数: %d\n"+
			"运行时间: %s",
		float64(beforeAlloc)/1024/1024,
		float64(after.Alloc)/1024/1024,
		float64(freed)/1024/1024,
		after.NumGC-before.NumGC,
		elapsed,
	))
}