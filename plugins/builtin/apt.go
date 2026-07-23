package builtin

import (
	"context"
	"fmt"
	"strings"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// AptPlugin manages other plugins.
type AptPlugin struct {
	mgr plugin.Manager
}

func NewApt() *AptPlugin { return &AptPlugin{} }

func (p *AptPlugin) Name() string        { return "apt" }
func (p *AptPlugin) Description() string { return "插件管理器" }

func (p *AptPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	p.mgr = mgr
	_ = mgr.RegisterCommand(&interfaces.Command{
		Name:        "apt",
		Aliases:     []string{"plugin", "plugins"},
		Description: "插件管理",
		Usage:       "apt list | apt enable <name> | apt disable <name> | apt reload <name>",
		Plugin:      p.Name(),
		Category:    "core",
		Handler:     p.handleApt,
	})
	return nil
}

func (p *AptPlugin) Start(_ context.Context) error { return nil }
func (p *AptPlugin) Stop(_ context.Context) error  { return nil }

func (p *AptPlugin) handleApt(ctx *interfaces.CommandContext) error {
	if ctx.ArgCount() == 0 {
		return ctx.Edit("用法: apt list | apt enable <name> | apt disable <name> | apt reload <name>")
	}
	sub := ctx.GetArg(0)

	switch sub {
	case "list":
		infos := p.mgr.GetAllInfo()
		if len(infos) == 0 {
			return ctx.Edit("无已加载插件")
		}
		var b strings.Builder
		b.WriteString("📦 插件列表:\n")
		for _, info := range infos {
			status := "⏸️"
			if info.Status == plugin.StatusActive {
				status = "✅"
			}
			b.WriteString(fmt.Sprintf("%s %s — %s\n", status, info.Name, info.Description))
		}
		return ctx.Edit(b.String())

	case "enable":
		if ctx.ArgCount() < 2 {
			return ctx.Edit("用法: apt enable <name>")
		}
		name := ctx.GetArg(1)
		return ctx.Edit(fmt.Sprintf("启用 %s: 需外部插件加载器支持", name))

	case "disable":
		if ctx.ArgCount() < 2 {
			return ctx.Edit("用法: apt disable <name>")
		}
		name := ctx.GetArg(1)
		return ctx.Edit(fmt.Sprintf("禁用 %s: 需外部插件加载器支持", name))

	case "reload":
		if ctx.ArgCount() < 2 {
			return ctx.Edit("用法: apt reload <name>")
		}
		name := ctx.GetArg(1)
		return ctx.Edit(fmt.Sprintf("重载 %s: 需外部插件加载器支持", name))

	default:
		return ctx.Edit("未知子命令: " + sub)
	}
}