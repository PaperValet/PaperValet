package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type AliasPlugin struct {
	aliases map[string]string
	file    string
}

func New() (plugin.Plugin, error) {
	p := &AliasPlugin{
		aliases: make(map[string]string),
		file:    "data/aliases.json",
	}
	p.load()
	return p, nil
}

var Metadata = &plugin.PluginMetadata{
	Name:        "alias",
	Description: "命令别名管理",
	Version:     "1.0.0",
	Author:      "PaperValet",
	MinVersion:  "0.1.0",
}

func (p *AliasPlugin) Name() string        { return "alias" }
func (p *AliasPlugin) Description() string { return "命令别名管理" }

func (p *AliasPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	cmds := []*plugin.Command{
		{
			Name:        "alias",
			Aliases:     []string{"al"},
			Description: "管理命令别名",
			Usage:       "alias [set|del|list] [名称] [命令]",
			Plugin:      p.Name(),
			Category:    "tools",
			Handler:     p.handleAlias,
		},
	}

	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *AliasPlugin) load() {
	data, err := os.ReadFile(p.file)
	if err != nil {
		return
	}
	json.Unmarshal(data, &p.aliases)
}

func (p *AliasPlugin) save() {
	os.MkdirAll("data", 0755)
	data, _ := json.MarshalIndent(p.aliases, "", "  ")
	os.WriteFile(p.file, data, 0644)
}

func (p *AliasPlugin) handleAlias(ctx *plugin.CommandContext) error {
	args := ctx.Args
	if len(args) == 0 {
		return p.listAliases(ctx)
	}

	sub := args[0]

	switch sub {
	case "set", "add":
		if len(args) < 3 {
			return ctx.Edit("用法: alias set <别名> <命令>")
		}
		name := args[1]
		cmd := strings.Join(args[2:], " ")
		p.aliases[name] = cmd
		p.save()
		return ctx.Edit(fmt.Sprintf("✅ 别名已设置: <code>%s</code> → <code>%s</code>", name, cmd))

	case "del", "delete", "remove":
		if len(args) < 2 {
			return ctx.Edit("用法: alias del <别名>")
		}
		name := args[1]
		if _, ok := p.aliases[name]; !ok {
			return ctx.Edit(fmt.Sprintf("❌ 别名不存在: %s", name))
		}
		delete(p.aliases, name)
		p.save()
		return ctx.Edit(fmt.Sprintf("🗑 别名已删除: <code>%s</code>", name))

	case "list", "ls":
		return p.listAliases(ctx)

	default:
		return ctx.Edit("用法: alias [set|del|list] [名称] [命令]")
	}
}

func (p *AliasPlugin) listAliases(ctx *plugin.CommandContext) error {
	if len(p.aliases) == 0 {
		return ctx.Edit("暂无别名")
	}

	var b strings.Builder
	b.WriteString("📝 <b>命令别名列表</b>\n\n")
	for name, cmd := range p.aliases {
		b.WriteString(fmt.Sprintf("• <code>%s</code> → <code>%s</code>\n", name, cmd))
	}
	return ctx.Edit(b.String())
}

func (p *AliasPlugin) Start(ctx context.Context) error { return nil }
func (p *AliasPlugin) Stop(ctx context.Context) error  { return nil }

// GetAlias returns the command for an alias
func (p *AliasPlugin) GetAlias(name string) (string, bool) {
	cmd, ok := p.aliases[name]
	return cmd, ok
}