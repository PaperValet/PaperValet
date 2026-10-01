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
	var b strings.Builder
	b.WriteString("📚 <b>PaperValet</b>\n")
	builtin := map[string]bool{}
	for _, n := range builtinOrder {
		builtin[n] = true
	}
	shownExternal := false
	for _, name := range names {
		if !builtin[name] && !shownExternal {
			b.WriteString("\n━━ " + ctx.Tlocal("外部插件", "External plugins") + " ━━\n")
			shownExternal = true
		}
		info, _ := p.mgr.GetInfo(name)
		desc := pluginDesc(ctx, info)
		fmt.Fprintf(&b, "\n<b>%s</b>", htmlEscape(name))
		if desc != "" {
			b.WriteString(" · " + desc)
		}
		b.WriteString("\n")
		for _, cmd := range by[name] {
			b.WriteString("  " + p.commandLine(ctx, prefix, cmd) + "\n")
		}
	}
	b.WriteString("\n" + ctx.Tlocal(
		fmt.Sprintf("💡 <code>%shelp 命令</code> 看详细用法", prefix),
		fmt.Sprintf("💡 <code>%shelp command</code> for details", prefix)))
	return b.String()
}

func (p *HelpPlugin) commandPage(ctx *interfaces.CommandContext, prefix string, cmd *interfaces.Command) string {
	var b strings.Builder
	fmt.Fprintf(&b, "📖 <b>%s%s</b> · %s\n", prefix, cmd.Name, cmdDesc(ctx, cmd))
	info, _ := p.mgr.GetInfo(cmd.Plugin)
	fmt.Fprintf(&b, "<i>%s %s</i>\n", ctx.Tlocal("插件", "plugin"), htmlEscape(info.Name))

	usage := cmdUsage(ctx, cmd)
	if usage == "" {
		usage = cmd.Name
	}
	first, rest, _ := strings.Cut(usage, "\n")
	fmt.Fprintf(&b, "\n<b>%s</b>  <code>%s%s</code>\n", ctx.Tlocal("用法", "Usage"), prefix, first)
	if strings.TrimSpace(rest) != "" {
		b.WriteString(strings.TrimRight(rest, "\n") + "\n")
	}

	var tags []string
	aliases := append(append([]string(nil), cmd.Aliases...), p.userAliasesFor(cmd.Name)...)
	if len(aliases) > 0 {
		parts := make([]string, len(aliases))
		for i, a := range aliases {
			parts[i] = "<code>" + prefix + htmlEscape(a) + "</code>"
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
		b.WriteString("\n" + strings.Join(tags, " · "))
	}
	return b.String()
}

func (p *HelpPlugin) pluginPage(ctx *interfaces.CommandContext, prefix string, info plugin.PluginInfo) string {
	_, by := p.groups()
	var b strings.Builder
	fmt.Fprintf(&b, "📦 <b>%s</b> · %s\n\n", htmlEscape(info.Name), pluginDesc(ctx, info))
	cmds := by[info.Name]
	if len(cmds) == 0 {
		b.WriteString(ctx.Tlocal("这个插件没有命令", "This plugin has no commands"))
		return b.String()
	}
	for _, cmd := range cmds {
		b.WriteString(p.commandLine(ctx, prefix, cmd) + "\n")
	}
	b.WriteString("\n" + ctx.Tlocal(
		fmt.Sprintf("💡 <code>%shelp 命令</code> 看详细用法", prefix),
		fmt.Sprintf("💡 <code>%shelp command</code> for details", prefix)))
	return b.String()
}
