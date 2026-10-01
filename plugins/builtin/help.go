package builtin

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// HelpPlugin provides help and command discovery.
type HelpPlugin struct {
	mgr plugin.Manager
}

func NewHelp() *HelpPlugin { return &HelpPlugin{} }

func (p *HelpPlugin) Name() string        { return "help" }
func (p *HelpPlugin) Description() string { return "命令列表与用法" }
func (p *HelpPlugin) DescEN() string      { return "Command list and usage" }

// cmdDesc picks the description in the active language.
func cmdDesc(ctx *interfaces.CommandContext, cmd *interfaces.Command) string {
	if ctx.Lang == "en-US" && cmd.DescEN != "" {
		return cmd.DescEN
	}
	return cmd.Description
}

func cmdUsage(ctx *interfaces.CommandContext, cmd *interfaces.Command) string {
	if ctx.Lang == "en-US" && cmd.UsageEN != "" {
		return cmd.UsageEN
	}
	return cmd.Usage
}

func pluginDesc(ctx *interfaces.CommandContext, info plugin.PluginInfo) string {
	if ctx.Lang == "en-US" && info.DescEN != "" {
		return info.DescEN
	}
	return info.Description
}

func (p *HelpPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	p.mgr = mgr
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "help",
		Aliases:     []string{"h", "?"},
		Description: "列出所有命令；help 命令名 看详细用法",
		DescEN:      "List commands; help <command> for details",
		Usage:       "help [命令]",
		UsageEN:     "help [command]",
		Plugin:      p.Name(),
		Category:    "core",
		Handler:     p.handleHelp,
	})
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

	if cmd, ok := p.mgr.Commands().Get(target); ok {
		return p.showCommandHelp(ctx, prefix, cmd)
	}

	if info, ok := p.mgr.GetInfo(target); ok {
		return p.showPluginHelp(ctx, prefix, info)
	}

	for _, cmd := range p.mgr.Commands().GetAll() {
		for _, alias := range cmd.Aliases {
			if alias == target {
				return p.showCommandHelp(ctx, prefix, cmd)
			}
		}
	}

	return ctx.Edit(ctx.T("help.not_found", target))
}

func (p *HelpPlugin) showAllHelp(ctx *interfaces.CommandContext, prefix string) error {
	cmds := p.mgr.Commands().GetAll()

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

	var catNames []string
	for cat := range categories {
		catNames = append(catNames, cat)
	}
	sort.Strings(catNames)

	var b strings.Builder
	b.WriteString(ctx.T("help.title") + "\n\n")

	for _, cat := range catNames {
		cmds := categories[cat]
		sort.Slice(cmds, func(i, j int) bool {
			return cmds[i].Name < cmds[j].Name
		})

		catDisplay := ctx.T("help.cat_" + cat)
		b.WriteString(fmt.Sprintf("<b>%s</b>\n", catDisplay))
		for _, cmd := range cmds {
			b.WriteString(fmt.Sprintf("  <code>%s%s</code> — %s\n", prefix, cmd.Name, cmdDesc(ctx, cmd)))
		}
		b.WriteString("\n")
	}

	b.WriteString(ctx.T("help.detail_hint", prefix) + "\n")
	b.WriteString(ctx.T("help.plugins_hint", prefix))
	return ctx.Edit(b.String())
}

func (p *HelpPlugin) showCommandHelp(ctx *interfaces.CommandContext, prefix string, cmd *interfaces.Command) error {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("<b>%s%s</b>\n", prefix, cmd.Name))
	b.WriteString(fmt.Sprintf("%s\n", cmdDesc(ctx, cmd)))

	if u := cmdUsage(ctx, cmd); u != "" {
		b.WriteString("\n" + ctx.T("help.usage", prefix+u) + "\n")
	}

	if len(cmd.Aliases) > 0 {
		b.WriteString("\n" + ctx.T("help.aliases", strings.Join(cmd.Aliases, "</code>, <code>")) + "\n")
	}

	if cmd.OwnerOnly {
		b.WriteString("\n" + ctx.T("help.owner_only"))
	}

	if cmd.RateLimit > 0 {
		b.WriteString("\n" + ctx.T("help.rate_limit", 1, cmd.RateLimit))
	}

	// Commands with rich built-in guides show them on bare invocation.
	b.WriteString("\n" + ctx.Tlocal(
		fmt.Sprintf("💡 直接发 <code>%s%s</code> 看示例", prefix, cmd.Name),
		fmt.Sprintf("💡 Send <code>%s%s</code> alone for examples", prefix, cmd.Name)))

	return ctx.Edit(b.String())
}

func (p *HelpPlugin) showPluginHelp(ctx *interfaces.CommandContext, prefix string, info plugin.PluginInfo) error {
	cmds := p.mgr.Commands().GetByPlugin(info.Name)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("📦 <b>%s</b>\n", info.Name))
	b.WriteString(fmt.Sprintf("%s\n", pluginDesc(ctx, info)))

	statusStr := ctx.T("help.status_idle")
	switch info.Status {
	case plugin.StatusActive:
		statusStr = ctx.T("help.status_active")
	case plugin.StatusError:
		statusStr = ctx.T("help.status_error")
	}
	b.WriteString(ctx.T("help.status", statusStr) + "\n\n")

	if len(cmds) == 0 {
		b.WriteString(ctx.T("help.no_commands"))
	} else {
		b.WriteString(ctx.T("help.commands") + "\n")
		var names []string
		for name := range cmds {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			cmd := cmds[name]
			b.WriteString(fmt.Sprintf("  <code>%s%s</code> — %s\n", prefix, name, cmdDesc(ctx, cmd)))
		}
	}
	return ctx.Edit(b.String())
}
