package builtin

import (
	"context"
	"fmt"
	"strings"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// PrefixPlugin manages command prefixes.
type PrefixPlugin struct {
	mgr plugin.Manager
}

func NewPrefix() *PrefixPlugin { return &PrefixPlugin{} }

func (p *PrefixPlugin) Name() string        { return "prefix" }
func (p *PrefixPlugin) Description() string { return "命令前缀管理" }

func (p *PrefixPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	p.mgr = mgr
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "prefix",
		Aliases:     []string{"pfx"},
		Description: "命令前缀管理",
		Usage:       "prefix [get|list|add|del] [前缀]",
		Plugin:      p.Name(),
		Category:    "admin",
		OwnerOnly:   true,
		Handler:     p.handlePrefix,
	})
}

func (p *PrefixPlugin) Start(_ context.Context) error { return nil }
func (p *PrefixPlugin) Stop(_ context.Context) error  { return nil }

func (p *PrefixPlugin) handlePrefix(ctx *interfaces.CommandContext) error {
	args := ctx.Args
	if len(args) == 0 {
		return ctx.Edit(fmt.Sprintf("当前主前缀: <code>%s</code>\n\n用法: prefix [get|list|add|del] [前缀]", p.mgr.Commands().GetPrefix()))
	}

	sub := args[0]
	switch sub {
	case "get":
		return ctx.Edit(fmt.Sprintf("当前主前缀: <code>%s</code>", p.mgr.Commands().GetPrefix()))

	case "list", "ls":
		prefixes := p.mgr.Commands().GetPrefixes()
		return ctx.Edit(fmt.Sprintf("支持的前缀: %s", strings.Join(prefixes, ", ")))

	case "add":
		if len(args) < 2 {
			return ctx.Edit("用法: prefix add <前缀>")
		}
		return ctx.Edit("⚠️ 运行时添加前缀需命令注册表支持，建议修改配置文件后重启")

	case "del", "delete", "remove":
		if len(args) < 2 {
			return ctx.Edit("用法: prefix del <前缀>")
		}
		return ctx.Edit("⚠️ 运行时删除前缀需命令注册表支持，建议修改配置文件后重启")

	case "help", "h":
		return ctx.Edit(`🔧 <b>前缀管理</b>

<b>用法:</b>
• <code>prefix</code> - 显示当前前缀
• <code>prefix list</code> - 列出所有支持的前缀
• <code>prefix add <前缀></code> - 添加前缀 (需重启)
• <code>prefix del <前缀></code> - 删除前缀 (需重启)

<b>示例:</b>
• <code>prefix add !</code> - 添加 ! 前缀
• <code>prefix del /</code> - 删除 / 前缀

⚠️ <i>修改配置文件 bot.prefix 后重启生效</i>`)

	default:
		return ctx.Edit("未知子命令: " + sub)
	}
}