package builtin

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/internal/command"
	"github.com/TiaraBasori/PaperValet/internal/core"
	"github.com/TiaraBasori/PaperValet/internal/plugin"
)

// CorePlugin provides .help and .status.
type CorePlugin struct {
	mgr       *plugin.Manager
	startTime time.Time
	version   string
}

func NewCore(version string) *CorePlugin {
	return &CorePlugin{version: version, startTime: time.Now()}
}

func (p *CorePlugin) Name() string        { return "core" }
func (p *CorePlugin) Description() string { return "核心命令：help / status" }

func (p *CorePlugin) Init(_ context.Context, mgr *plugin.Manager) error {
	p.mgr = mgr
	_ = mgr.RegisterCommand(&command.Command{
		Name:        "help",
		Aliases:     []string{"h", "?"},
		Description: "显示帮助",
		Usage:       "help [命令|插件]",
		Plugin:      p.Name(),
		Category:    "core",
		Handler:     p.handleHelp,
	})
	_ = mgr.RegisterCommand(&command.Command{
		Name:        "status",
		Aliases:     []string{"stat"},
		Description: "显示运行状态",
		Plugin:      p.Name(),
		Category:    "core",
		Handler:     p.handleStatus,
	})
	return nil
}

func (p *CorePlugin) Start(_ context.Context) error { return nil }
func (p *CorePlugin) Stop(_ context.Context) error  { return nil }

func (p *CorePlugin) handleHelp(ctx *core.CommandContext) error {
	prefix := p.mgr.Commands().GetPrefix()
	arg := ctx.GetArg(0)
	if arg == "" {
		cmds := p.mgr.Commands().GetAll()
		names := make([]string, 0, len(cmds))
		for name, cmd := range cmds {
			if !cmd.Hidden {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		var b strings.Builder
		b.WriteString("PaperValet 命令列表\n")
		for _, name := range names {
			cmd := cmds[name]
			b.WriteString(fmt.Sprintf("%s%s — %s\n", prefix, name, cmd.Description))
		}
		b.WriteString(fmt.Sprintf("\n详情: %shelp <命令>", prefix))
		return ctx.Edit(b.String())
	}

	if cmd, ok := p.mgr.Commands().Get(arg); ok {
		text := fmt.Sprintf("%s%s\n%s", prefix, cmd.Name, cmd.Description)
		if cmd.Usage != "" {
			text += "\n用法: " + prefix + cmd.Usage
		}
		if len(cmd.Aliases) > 0 {
			text += "\n别名: " + strings.Join(cmd.Aliases, ", ")
		}
		return ctx.Edit(text)
	}

	if info, ok := p.mgr.GetInfo(arg); ok {
		cmds := p.mgr.Commands().GetByPlugin(arg)
		var b strings.Builder
		b.WriteString(fmt.Sprintf("插件 %s\n%s\n", info.Name, info.Description))
		for name, cmd := range cmds {
			b.WriteString(fmt.Sprintf("%s%s — %s\n", prefix, name, cmd.Description))
		}
		return ctx.Edit(b.String())
	}

	return ctx.Edit("未找到命令或插件: " + arg)
}

func (p *CorePlugin) handleStatus(ctx *core.CommandContext) error {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	infos := p.mgr.GetAllInfo()
	active := 0
	for _, i := range infos {
		if i.Status == plugin.StatusActive {
			active++
		}
	}
	uptime := time.Since(p.startTime).Truncate(time.Second)
	text := fmt.Sprintf(
		"PaperValet %s\n运行: %s\n插件: %d/%d\nGoroutine: %d\n内存: %.1f MB",
		p.version,
		uptime,
		active, len(infos),
		runtime.NumGoroutine(),
		float64(mem.Alloc)/1024/1024,
	)
	return ctx.Edit(text)
}
