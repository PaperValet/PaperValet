package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type PrefixPlugin struct {
	manager plugin.Manager
}

func New() (plugin.Plugin, error) {
	return &PrefixPlugin{}, nil
}

var Metadata = &plugin.PluginMetadata{
	Name:        "prefix",
	Description: "命令前缀管理",
	Version:     "1.0.0",
	Author:      "PaperValet",
	MinVersion:  "0.1.0",
}

func (p *PrefixPlugin) Name() string        { return "prefix" }
func (p *PrefixPlugin) Description() string { return "命令前缀管理" }

func (p *PrefixPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	p.manager = mgr
	return mgr.RegisterCommand(&plugin.Command{
		Name:        "prefix",
		Aliases:     []string{"pfx"},
		Description: "命令前缀管理",
		Usage:       "prefix [get|set|add|del|list] [前缀]",
		Plugin:      p.Name(),
		Category:    "admin",
		OwnerOnly:   true,
		Handler:     p.handlePrefix,
	})
}

func (p *PrefixPlugin) handlePrefix(ctx *plugin.CommandContext) error {
	args := ctx.Args()
	if len(args) == 0 {
		return ctx.Edit(fmt.Sprintf("当前前缀: <code>%s</code>\n\n用法: prefix [get|set|add|del|list] [前缀]", p.manager.Commands().GetPrefix()))
	}

	sub := args[0]

	switch sub {
	case "get":
		return ctx.Edit(fmt.Sprintf("当前主前缀: <code>%s</code>", p.manager.Commands().GetPrefix()))

	case "set":
		if len(args) < 2 {
			return ctx.Edit("用法: prefix set <前缀>")
		}
		return ctx.Edit("⚠️ 前缀修改需要重启生成，建议修改配置文件")

	case "list", "ls":
		prefixes := p.manager.Commands().GetAllPrefixes()
		return ctx.Edit(fmt.Sprintf("支持的前缀: %s", strings.Join(prefixes, ", ")))

	case "help", "h":
		return ctx.Edit(`🔧 <b>前缀管理</b>

<b>用法:</b>
• <code>prefix</code> - 显示当前前缀
• <code>prefix list</code> - 列出所有支持的前缀
• <code>prefix set <前缀></code> - 设置主前缀 (需重启)

<b>示例:</b>
• <code>prefix set !</code> - 设置为 !

⚠️ <i>修改配置文件 bot.prefix 后重启生效</i>`)

	default:
		return ctx.Edit("未知子命令: " + sub)
	}
}

func (p *PrefixPlugin) Start(ctx context.Context) error { return nil }
func (p *PrefixPlugin) Stop(ctx context.Context) error  { return nil }