package builtin

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// HelpPlugin provides help and plugin discovery commands.
// This is the primary user-facing help system and MUST be built-in.
type HelpPlugin struct {
	mgr plugin.Manager
}

func NewHelp() *HelpPlugin { return &HelpPlugin{} }

func (p *HelpPlugin) Name() string        { return "help" }
func (p *HelpPlugin) Description() string { return "帮助与命令发现" }

func (p *HelpPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	p.mgr = mgr
	cmds := []*interfaces.Command{
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
			Aliases:     []string{"plugin", "ppm"},
			Description: "列出所有已加载插件",
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

func (p *HelpPlugin) Start(_ context.Context) error { return nil }
func (p *HelpPlugin) Stop(_ context.Context) error  { return nil }

func (p *HelpPlugin) handleHelp(ctx *interfaces.CommandContext) error {
	prefix := p.mgr.Commands().GetPrefix()
	args := ctx.Args

	if len(args) == 0 {
		return p.showAllHelp(ctx, prefix)
	}

	target := args[0]

	// Check command
	if cmd, ok := p.mgr.Commands().Get(target); ok {
		return p.showCommandHelp(ctx, prefix, cmd)
	}

	// Check plugin
	if info, ok := p.mgr.GetInfo(target); ok {
		return p.showPluginHelp(ctx, prefix, info)
	}

	// Check aliases
	for _, cmd := range p.mgr.Commands().GetAll() {
		for _, alias := range cmd.Aliases {
			if alias == target {
				return p.showCommandHelp(ctx, prefix, cmd)
			}
		}
	}

	return ctx.Edit("未找到命令或插件: " + target)
}

func (p *HelpPlugin) showAllHelp(ctx *interfaces.CommandContext, prefix string) error {
	cmds := p.mgr.Commands().GetAll()

	// Group by category
	categories := make(map[string][]*interfaces.Command)
	for _, cmd := range cmds {
		if cmd.Hidden {
			continue
		}
		cat := cmd.Category
		if cat == "" {
			cat = "other"
		}
		categories[cat] = append(categories[cat], cmd)
	}

	// Sort category names
	var catNames []string
	for cat := range categories {
		catNames = append(catNames, cat)
	}
	sort.Strings(catNames)

	var b strings.Builder
	b.WriteString("📚 <b>PaperValet 帮助</b>\n\n")

	for _, cat := range catNames {
		cmds := categories[cat]
		sort.Slice(cmds, func(i, j int) bool {
			return cmds[i].Name < cmds[j].Name
		})

		catDisplay := cat
		switch cat {
		case "core":
			catDisplay = "🔧 核心"
		case "admin":
			catDisplay = "👑 管理员"
		case "tools":
			catDisplay = "🛠 工具"
		case "fun":
			catDisplay = "🎮 娱乐"
		case "debug":
			catDisplay = "🐛 调试"
		}
		b.WriteString(fmt.Sprintf("<b>%s</b>\n", catDisplay))
		for _, cmd := range cmds {
			b.WriteString(fmt.Sprintf("  <code>%s%s</code> — %s\n", prefix, cmd.Name, cmd.Description))
		}
		b.WriteString("\n")
	}

	b.WriteString(fmt.Sprintf("使用 <code>%shelp <命令></code> 查看详情\n", prefix))
	b.WriteString(fmt.Sprintf("使用 <code>%splugins</code> 查看插件列表", prefix))
	return ctx.Edit(b.String())
}

func (p *HelpPlugin) showCommandHelp(ctx *interfaces.CommandContext, prefix string, cmd *interfaces.Command) error {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("<b>%s%s</b>\n", prefix, cmd.Name))
	b.WriteString(fmt.Sprintf("%s\n", cmd.Description))

	if cmd.Usage != "" {
		b.WriteString(fmt.Sprintf("\n<b>用法:</b> <code>%s%s</code>\n", prefix, cmd.Usage))
	}

	if len(cmd.Aliases) > 0 {
		b.WriteString(fmt.Sprintf("\n<b>别名:</b> <code>%s</code>", strings.Join(cmd.Aliases, "</code>, <code>")))
	}

	if cmd.OwnerOnly {
		b.WriteString("\n\n⚠️ <b>仅拥有者可用</b>")
	}

	if cmd.RateLimit > 0 {
		b.WriteString(fmt.Sprintf("\n⏱ <b>频率限制:</b> %d次/%ds", cmd.RateLimit, cmd.RateLimit))
	}

	return ctx.Edit(b.String())
}

func (p *HelpPlugin) showPluginHelp(ctx *interfaces.CommandContext, prefix string, info plugin.PluginInfo) error {
	cmds := p.mgr.Commands().GetByPlugin(info.Name)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("📦 <b>%s</b>\n", info.Name))
	b.WriteString(fmt.Sprintf("%s\n", info.Description))
	b.WriteString(fmt.Sprintf("状态: %s\n\n", info.Status))

	if len(cmds) == 0 {
		b.WriteString("无命令")
	} else {
		b.WriteString("<b>命令:</b>\n")
		var names []string
		for name := range cmds {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			cmd := cmds[name]
			b.WriteString(fmt.Sprintf("  <code>%s%s</code> — %s\n", prefix, name, cmd.Description))
		}
	}
	return ctx.Edit(b.String())
}

func (p *HelpPlugin) handlePlugins(ctx *interfaces.CommandContext) error {
	infos := p.mgr.GetAllInfo()
	if len(infos) == 0 {
		return ctx.Edit("无已加载插件")
	}

	var b strings.Builder
	b.WriteString("📦 <b>已加载插件</b>\n\n")

	for _, info := range infos {
		status := "⏸️"
		if info.Status == plugin.StatusActive {
			status = "✅"
		}
		b.WriteString(fmt.Sprintf("%s <b>%s</b>\n", status, info.Name))
		b.WriteString(fmt.Sprintf("   %s\n", info.Description))
		b.WriteString(fmt.Sprintf("   命令: %d 个\n\n", len(p.mgr.Commands().GetByPlugin(info.Name))))
	}

	return ctx.Edit(b.String())
}