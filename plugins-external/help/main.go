package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type HelpPlugin struct{}

func New() (plugin.Plugin, error) {
	return &HelpPlugin{}, nil
}

var Metadata = &plugin.PluginMetadata{
	Name:        "help",
	Description: "帮助命令",
	Version:     "1.0.0",
	Author:      "PaperValet",
	MinVersion:  "0.1.0",
}

func (p *HelpPlugin) Name() string        { return "help" }
func (p *HelpPlugin) Description() string { return "帮助命令" }

func (p *HelpPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	cmds := []*plugin.Command{
		{
			Name:        "help",
			Aliases:     []string{"h", "?"},
			Description: "显示帮助信息",
			Usage:       "help [命令|插件]",
			Plugin:      p.Name(),
			Category:    "core",
			Handler:     p.handleHelp,
		},
		{
			Name:        "plugins",
			Aliases:     []string{"plugin", "apt"},
			Description: "列出所有插件",
			Plugin:      p.Name(),
			Category:    "core",
			Handler:     p.handlePlugins,
		},
	}

	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *HelpPlugin) handleHelp(ctx *plugin.CommandContext) error {
	args := ctx.Args()
	prefix := ctx.Prefix()

	if len(args) == 0 {
		return p.showAllHelp(ctx, prefix)
	}

	target := args[0]

	// Check if it's a command
	if cmd, ok := ctx.Manager().Commands().Get(target); ok {
		return p.showCommandHelp(ctx, prefix, cmd)
	}

	// Check if it's a plugin
	if info, ok := ctx.Manager().GetInfo(target); ok {
		return p.showPluginHelp(ctx, prefix, info)
	}

	// Check aliases
	for name, cmd := range ctx.Manager().Commands().GetAll() {
		for _, alias := range cmd.Aliases {
			if alias == target {
				return p.showCommandHelp(ctx, prefix, cmd)
			}
		}
	}

	return ctx.Edit(fmt.Sprintf("❌ 未找到命令或插件: %s", target))
}

func (p *HelpPlugin) showAllHelp(ctx *plugin.CommandContext, prefix string) error {
	cmds := ctx.Manager().Commands().GetAll()
	
	// Group by plugin
	byPlugin := make(map[string][]*plugin.Command)
	for _, cmd := range cmds {
		if cmd.Hidden {
			continue
		}
		byPlugin[cmd.Plugin] = append(byPlugin[cmd.Plugin], cmd)
	}

	// Sort plugins
	plugins := make([]string, 0, len(byPlugin))
	for p := range byPlugin {
		plugins = append(plugins, p)
	}
	sort.Strings(plugins)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("📖 <b>PaperValet 帮助</b>\n前缀: <code>%s</code>\n\n", prefix))

	for _, pName := range plugins {
		cmds := byPlugin[pName]
		if len(cmds) == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("📦 <b>%s</b>\n", pName))
		sort.Slice(cmds, func(i, j int) bool { return cmds[i].Name < cmds[j].Name })
		for _, cmd := range cmds {
			aliases := ""
			if len(cmd.Aliases) > 0 {
				aliases = " (" + strings.Join(cmd.Aliases, ", ") + ")"
			}
			b.WriteString(fmt.Sprintf("  <code>%s%s</code> — %s\n", prefix, cmd.Name, cmd.Description))
			_ = aliases
		}
		b.WriteString("\n")
	}

	b.WriteString(fmt.Sprintf("💡 使用 <code>%shelp <命令></code> 查看详情\n", prefix))
	b.WriteString(fmt.Sprintf("📦 使用 <code>%splugins</code> 查看插件列表", prefix))

	return ctx.Edit(b.String())
}

func (p *HelpPlugin) showCommandHelp(ctx *plugin.CommandContext, prefix string, cmd *plugin.Command) error {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("📖 <b>命令: %s</b>\n\n", cmd.Name))
	b.WriteString(fmt.Sprintf("📝 <b>描述:</b> %s\n", cmd.Description))
	
	if cmd.Usage != "" {
		b.WriteString(fmt.Sprintf("💡 <b>用法:</b> <code>%s%s</code>\n", prefix, cmd.Usage))
	} else {
		b.WriteString(fmt.Sprintf("💡 <b>用法:</b> <code>%s%s</code>\n", prefix, cmd.Name))
	}

	if len(cmd.Aliases) > 0 {
		b.WriteString(fmt.Sprintf("🔗 <b>别名:</b> <code>%s</code>\n", strings.Join(cmd.Aliases, ", ")))
	}
	
	b.WriteString(fmt.Sprintf("📦 <b>插件:</b> %s\n", cmd.Plugin))
	b.WriteString(fmt.Sprintf("🏷 <b>分类:</b> %s\n", cmd.Category))

	if cmd.OwnerOnly {
		b.WriteString("👑 <b>仅所有者可用</b>\n")
	}

	if cmd.Args != nil && len(cmd.Args) > 0 {
		b.WriteString("\n<b>参数:</b>\n")
		for _, arg := range cmd.Args {
			required := ""
			if arg.Required {
				required = " (必填)"
			}
			b.WriteString(fmt.Sprintf("  <code>%s</code>%s — %s\n", arg.Name, required, arg.Description))
		}
	}

	return ctx.Edit(b.String())
}

func (p *HelpPlugin) showPluginHelp(ctx *plugin.CommandContext, prefix string, info *plugin.PluginInfo) error {
	cmds := ctx.Manager().Commands().GetByPlugin(info.Name)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("📦 <b>插件: %s</b>\n\n", info.Name))
	b.WriteString(fmt.Sprintf("📝 <b>描述:</b> %s\n", info.Description))
	b.WriteString(fmt.Sprintf("📊 <b>状态:</b> %s\n", info.Status))
	b.WriteString(fmt.Sprintf("🔢 <b>版本:</b> %s\n", info.Version))
	b.WriteString(fmt.Sprintf("👤 <b>作者:</b> %s\n", info.Author))

	if len(cmds) > 0 {
		b.WriteString(fmt.Sprintf("\n📋 <b>命令 (%d):</b>\n", len(cmds)))
		sort.Slice(cmds, func(i, j int) bool { return cmds[i].Name < cmds[j].Name })
		for _, cmd := range cmds {
			aliases := ""
			if len(cmd.Aliases) > 0 {
				aliases = " (" + strings.Join(cmd.Aliases, ", ") + ")"
			}
			b.WriteString(fmt.Sprintf("  <code>%s%s</code>%s — %s\n", prefix, cmd.Name, aliases, cmd.Description))
		}
	}

	return ctx.Edit(b.String())
}

func (p *HelpPlugin) handlePlugins(ctx *plugin.CommandContext) error {
	infos := ctx.Manager().GetAllInfo()

	var b strings.Builder
	b.WriteString(fmt.Sprintf("📦 <b>插件列表 (%d)</b>\n\n", len(infos)))

	for _, info := range infos {
		status := "⏸️"
		if info.Status == plugin.StatusActive {
			status = "✅"
		}
		b.WriteString(fmt.Sprintf("%s <b>%s</b> v%s — %s\n", status, info.Name, info.Version, info.Description))
	}

	return ctx.Edit(b.String())
}