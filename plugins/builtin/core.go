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

// CorePlugin provides fundamental commands + process lifecycle.
// Absorbs the old admin plugin (restart/shutdown/gc).
type CorePlugin struct {
	startTime time.Time
	version   string
}

func NewCore(version string) *CorePlugin {
	return &CorePlugin{version: version, startTime: time.Now()}
}

func (p *CorePlugin) Name() string        { return "core" }
func (p *CorePlugin) Description() string { return "核心命令与进程生命周期" }

func (p *CorePlugin) Init(_ context.Context, mgr plugin.Manager) error {
	cmds := []*interfaces.Command{
		{Name: "version", Aliases: []string{"v", "ver"}, Description: "显示版本信息", Plugin: p.Name(), Category: "core", Handler: p.handleVersion},
		{Name: "ping", Description: "检查延迟", Plugin: p.Name(), Category: "core", Handler: p.handlePing},
		{Name: "restart", Description: "重启机器人进程", Plugin: p.Name(), Category: "admin", OwnerOnly: true, Handler: p.handleRestart},
	}
	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *CorePlugin) Start(_ context.Context) error { return nil }
func (p *CorePlugin) Stop(_ context.Context) error  { return nil }

func (p *CorePlugin) handleVersion(ctx *interfaces.CommandContext) error {
	return ctx.Edit(fmt.Sprintf(
		"PaperValet <b>%s</b>\nGo: %s\nBuild: %s",
		p.version, runtime.Version(), p.startTime.Format("2006-01-02"),
	))
}

func (p *CorePlugin) handlePing(ctx *interfaces.CommandContext) error {
	start := time.Now()
	msg := "🏓 Pong!"
	if err := ctx.Edit(msg); err != nil {
		return err
	}
	latency := time.Since(start)
	return ctx.Edit(fmt.Sprintf("%s\n📡 <b>延迟:</b> %v", msg, latency))
}

func (p *CorePlugin) handleRestart(ctx *interfaces.CommandContext) error {
	_ = ctx.Edit("🔄 正在重启...")
	go func() {
		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()
	return nil
}
