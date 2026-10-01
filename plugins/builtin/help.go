package builtin

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// HelpPlugin lists commands grouped by plugin and shows detailed usage.
type HelpPlugin struct {
	mgr plugin.Manager
}

func NewHelp() *HelpPlugin { return &HelpPlugin{} }

func (p *HelpPlugin) Name() string        { return "help" }
func (p *HelpPlugin) Description() string { return "命令帮助" }
func (p *HelpPlugin) DescEN() string      { return "Command help" }

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
		Aliases:     []string{"h"},
		Description: "命令帮助",
		DescEN:      "Command help",
		Usage: `help [命令|插件]

<b>示例</b>
• <code>help</code>  按插件列出全部命令和别名
• <code>help dme</code>  看 dme 的详细用法和机制
• <code>help core</code>  看一个插件下的所有命令

别名 <code>h</code>`,
		UsageEN: `help [command|plugin]

<b>Examples</b>
• <code>help</code>  every command and alias, grouped by plugin
• <code>help dme</code>  detailed usage and behavior of dme
• <code>help core</code>  all commands of one plugin

Alias <code>h</code>`,
		Plugin:   p.Name(),
		Category: "core",
		Handler:  p.handleHelp,
	})
}

func (p *HelpPlugin) Start(_ context.Context) error { return nil }
func (p *HelpPlugin) Stop(_ context.Context) error  { return nil }

func (p *HelpPlugin) handleHelp(ctx *interfaces.CommandContext) error {
	prefix := p.mgr.Commands().GetPrefix()
	if ctx.ArgCount() == 0 {
		return ctx.Edit(p.overview(ctx, prefix))
	}
	target := strings.TrimPrefix(ctx.GetArg(0), prefix)
	if cmd, ok := p.mgr.Commands().Get(target); ok {
		return ctx.Edit(p.commandPage(ctx, prefix, cmd))
	}
	if repl, ok := p.mgr.Commands().UserAliases()[target]; ok {
		name := strings.Fields(repl)
		if len(name) > 0 {
			if cmd, ok := p.mgr.Commands().Get(name[0]); ok {
				return ctx.Edit(fmt.Sprintf("🔗 <code>%s%s</code> → <code>%s%s</code>\n\n%s",
					prefix, htmlEscape(target), prefix, htmlEscape(repl), p.commandPage(ctx, prefix, cmd)))
			}
		}
	}
	if info, ok := p.mgr.GetInfo(target); ok {
		return ctx.Edit(p.pluginPage(ctx, prefix, info))
	}
	return ctx.Edit(ctx.Tlocal(
		fmt.Sprintf("没有 <code>%s</code> 这个命令或插件，发 <code>%shelp</code> 看全部", htmlEscape(target), prefix),
		fmt.Sprintf("No command or plugin <code>%s</code>; send <code>%shelp</code> for the list", htmlEscape(target), prefix)))
}

// userAliasesFor returns runtime aliases pointing at cmd.
func (p *HelpPlugin) userAliasesFor(cmd string) []string {
	var out []string
	for name, repl := range p.mgr.Commands().UserAliases() {
		if f := strings.Fields(repl); len(f) > 0 && f[0] == cmd {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// commandLine renders ".name (.a .b) — description".
func (p *HelpPlugin) commandLine(ctx *interfaces.CommandContext, prefix string, cmd *interfaces.Command) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<code>%s%s</code>", prefix, cmd.Name)
	aliases := append(append([]string(nil), cmd.Aliases...), p.userAliasesFor(cmd.Name)...)
	if len(aliases) > 0 {
		parts := make([]string, len(aliases))
		for i, a := range aliases {
			parts[i] = "<code>" + prefix + htmlEscape(a) + "</code>"
		}
		b.WriteString(" · " + strings.Join(parts, " "))
	}
	b.WriteString("  " + cmdDesc(ctx, cmd))
	return b.String()
}

// groups returns visible commands keyed by plugin, plugins sorted with
// built-ins first.
func (p *HelpPlugin) groups() ([]string, map[string][]*interfaces.Command) {
	by := map[string][]*interfaces.Command{}
	for _, cmd := range p.mgr.Commands().GetAll() {
		if cmd.Hidden {
			continue
		}
		by[cmd.Plugin] = append(by[cmd.Plugin], cmd)
	}
	names := make([]string, 0, len(by))
	for n, cmds := range by {
		names = append(names, n)
		sort.Slice(cmds, func(i, j int) bool { return cmds[i].Name < cmds[j].Name })
	}
	order := map[string]int{}
	for i, n := range builtinOrder {
		order[n] = i + 1
	}
	sort.Slice(names, func(i, j int) bool {
		oi, oj := order[names[i]], order[names[j]]
		switch {
		case oi != 0 && oj != 0:
			return oi < oj
		case oi != 0:
			return true
		case oj != 0:
			return false
		}
		return names[i] < names[j]
	})
	return names, by
}

// builtinOrder groups related built-ins: everyday tools first, admin last.
var builtinOrder = []string{
	"help", "core", "status", "info", "re", "dme", "exec",
	"apt", "reload", "update", "backup", "sudo", "alias", "prefix", "lang", "log",
}

func (p *HelpPlugin) overview(ctx *interfaces.CommandContext, prefix string) string {
	names, by := p.groups()
	c := newCard("📚", "PaperValet")
	builtin := map[string]bool{}
	for _, n := range builtinOrder {
		builtin[n] = true
	}
	shownExternal := false
	for _, name := range names {
		if !builtin[name] && !shownExternal {
			c.blank().line("━━ " + ctx.Tlocal("外部插件", "External plugins") + " ━━")
			shownExternal = true
		}
		info, _ := p.mgr.GetInfo(name)
		c.blank()
		// A plugin whose only command shares its name needs no second line.
		if cmds := by[name]; len(cmds) == 1 && cmds[0].Name == name && len(cmds[0].Aliases) == 0 && len(p.userAliasesFor(name)) == 0 {
			c.line("<b>" + htmlEscape(name) + "</b> · " + pluginDesc(ctx, info))
			continue
		}
		c.line("<b>" + htmlEscape(name) + "</b> · " + pluginDesc(ctx, info))
		for _, cmd := range by[name] {
			c.line("  " + p.commandLine(ctx, prefix, cmd))
		}
	}
	c.hint(ctx.Tlocal(
		cmdRef(prefix+"help 命令")+" 看详细用法",
		cmdRef(prefix+"help command")+" for details"))
	return c.String()
}

func (p *HelpPlugin) commandPage(ctx *interfaces.CommandContext, prefix string, cmd *interfaces.Command) string {
	c := newCard("📖", prefix+cmd.Name+" · "+cmdDesc(ctx, cmd))
	info, _ := p.mgr.GetInfo(cmd.Plugin)
	c.line("<i>" + ctx.Tlocal("插件", "plugin") + " " + htmlEscape(info.Name) + "</i>")

	usage := cmdUsage(ctx, cmd)
	if usage == "" {
		usage = cmd.Name
	}
	first, rest, _ := strings.Cut(usage, "\n")
	c.blank().rawField(ctx.Tlocal("用法", "Usage"), cmdRef(prefix+first))
	if strings.TrimSpace(rest) != "" {
		c.line(strings.TrimRight(rest, "\n"))
	}

	var tags []string
	aliases := append(append([]string(nil), cmd.Aliases...), p.userAliasesFor(cmd.Name)...)
	if len(aliases) > 0 {
		parts := make([]string, len(aliases))
		for i, a := range aliases {
			parts[i] = cmdRef(prefix + a)
		}
		tags = append(tags, ctx.Tlocal("别名 ", "aliases ")+strings.Join(parts, " "))
	}
	if cmd.OwnerOnly {
		tags = append(tags, ctx.Tlocal("仅主人和 sudo 用户", "owner and sudo users only"))
	}
	if cmd.RateLimit > 0 {
		tags = append(tags, ctx.Tlocal(fmt.Sprintf("%d 秒冷却", cmd.RateLimit), fmt.Sprintf("%ds cooldown", cmd.RateLimit)))
	}
	if len(tags) > 0 {
		c.blank().line(strings.Join(tags, " · "))
	}
	return c.String()
}

func (p *HelpPlugin) pluginPage(ctx *interfaces.CommandContext, prefix string, info plugin.PluginInfo) string {
	c := newCard("📦", htmlEscape(info.Name)+" · "+pluginDesc(ctx, info)).blank()
	_, by := p.groups()
	cmds := by[info.Name]
	if len(cmds) == 0 {
		c.line(ctx.Tlocal("这个插件没有命令", "This plugin has no commands"))
		return c.String()
	}
	for _, cmd := range cmds {
		c.line(p.commandLine(ctx, prefix, cmd))
	}
	c.hint(ctx.Tlocal(
		cmdRef(prefix+"help 命令")+" 看详细用法",
		cmdRef(prefix+"help command")+" for details"))
	return c.String()
}
