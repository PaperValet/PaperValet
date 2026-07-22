package builtin

import (
	"context"
	"fmt"
	"strings"

	"github.com/TiaraBasori/PaperValet/internal/command"
	"github.com/TiaraBasori/PaperValet/internal/core"
	"github.com/TiaraBasori/PaperValet/internal/plugin"
)

// AptPlugin manages plugin enable/disable/list.
type AptPlugin struct {
	mgr *plugin.Manager
}

func NewApt() *AptPlugin { return &AptPlugin{} }

func (p *AptPlugin) Name() string        { return "apt" }
func (p *AptPlugin) Description() string { return "插件管理：list / enable / disable" }

func (p *AptPlugin) Init(_ context.Context, mgr *plugin.Manager) error {
	p.mgr = mgr
	return mgr.RegisterCommand(&command.Command{
		Name:        "apt",
		Description: "插件管理",
		Usage:       "apt <list|enable|disable> [插件名]",
		Plugin:      p.Name(),
		Category:    "core",
		Handler:     p.handle,
	})
}

func (p *AptPlugin) Start(_ context.Context) error { return nil }
func (p *AptPlugin) Stop(_ context.Context) error  { return nil }

func (p *AptPlugin) handle(ctx *core.CommandContext) error {
	sub := strings.ToLower(ctx.GetArg(0))
	switch sub {
	case "", "list", "ls":
		return p.list(ctx)
	case "enable", "on":
		return ctx.Edit("运行时启用需重启；当前插件均为内置编译进二进制。")
	case "disable", "off":
		return ctx.Edit("运行时禁用需重启；当前插件均为内置编译进二进制。")
	default:
		return ctx.Edit("用法: apt list | apt enable <名> | apt disable <名>")
	}
}

func (p *AptPlugin) list(ctx *core.CommandContext) error {
	infos := p.mgr.GetAllInfo()
	var b strings.Builder
	b.WriteString(fmt.Sprintf("已注册插件 (%d)\n", len(infos)))
	for _, info := range infos {
		status := "停用"
		switch info.Status {
		case plugin.StatusActive:
			status = "运行中"
		case plugin.StatusError:
			status = "错误"
		}
		b.WriteString(fmt.Sprintf("• %s [%s] — %s\n", info.Name, status, info.Description))
	}
	return ctx.Edit(b.String())
}
