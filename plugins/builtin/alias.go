package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// AliasPlugin manages runtime command aliases with persistent storage.
// Aliases are expanded once during command parsing; they do not nest.
type AliasPlugin struct {
	mgr  plugin.Manager
	file string
}

func NewAlias() *AliasPlugin {
	return &AliasPlugin{file: "data/aliases.json"}
}

func (p *AliasPlugin) Name() string        { return "alias" }
func (p *AliasPlugin) Description() string { return "命令别名" }
func (p *AliasPlugin) DescEN() string      { return "Short names for commands" }

func (p *AliasPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	p.mgr = mgr
	p.load()
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "alias",
		Description: "命令别名",
		DescEN:      "Command aliases",
		Usage: `alias 名字=命令 · alias del 名字 · alias list

<b>示例</b>
• <code>alias p=ping</code>  以后发 .p 等于 .ping
• <code>alias set d5 dme 5</code>  带固定参数（之后 .d5）
• <code>alias list</code>  查看全部
• <code>alias del p</code>  删除

<b>机制</b>
• 只展开一层，别名里再写别名不会继续展开
• 发别名时后面跟的参数会接在固定参数后面
• 命令部分不用写前缀
• 保存在 data/aliases.json，重启不丢`,
		UsageEN: `alias name=command · alias del name · alias list

<b>Examples</b>
• <code>alias p=ping</code>  .p now means .ping
• <code>alias set d5 dme 5</code>  fixed arguments (then .d5)
• <code>alias list</code>  list all
• <code>alias del p</code>  remove

<b>How it works</b>
• Expands once; aliases inside aliases are not expanded
• Extra arguments are appended after the fixed ones
• No prefix needed in the command part
• Stored in data/aliases.json, survives restarts`,
		Plugin:    p.Name(),
		Category:  "tools",
		OwnerOnly: true,
		Handler:   p.handleAlias,
	})
}

func (p *AliasPlugin) Start(_ context.Context) error { return nil }
func (p *AliasPlugin) Stop(_ context.Context) error  { return nil }

// load pushes persisted aliases into the command registry.
func (p *AliasPlugin) load() {
	data, err := os.ReadFile(p.file)
	if err != nil {
		return
	}
	var aliases map[string]string
	if err := json.Unmarshal(data, &aliases); err != nil {
		return
	}
	for name, cmd := range aliases {
		p.mgr.Commands().AddUserAlias(name, cmd)
	}
}

func (p *AliasPlugin) save() {
	_ = os.MkdirAll(filepath.Dir(p.file), 0o700)
	data, _ := json.MarshalIndent(p.mgr.Commands().UserAliases(), "", "  ")
	_ = os.WriteFile(p.file, data, 0o600)
}

func (p *AliasPlugin) help(ctx *interfaces.CommandContext) error {
	return ctx.Edit(ctx.Tlocal(
		`🔗 <b>alias 别名</b>

<code>alias p=ping</code>  以后 <code>.p</code> 就是 <code>.ping</code>
<code>alias d5="dme 5"</code>  带参数也行（设完发 .d5）
<code>alias list</code>  看全部
<code>alias del p</code>  删掉

别名只展开一层，命令部分不用写前缀。`,
		`🔗 <b>alias</b>

<code>alias p=ping</code>  now <code>.p</code> means <code>.ping</code>
<code>alias d5="dme 5"</code>  fixed args work too (then send .d5)
<code>alias list</code>  list all
<code>alias del p</code>  remove one

Aliases expand once; no prefix needed in the command part.`))
}

func (p *AliasPlugin) handleAlias(ctx *interfaces.CommandContext) error {
	args := ctx.Args
	if len(args) == 0 {
		return p.listAliases(ctx)
	}
	if args[0] == "help" || args[0] == "h" {
		return p.help(ctx)
	}

	switch args[0] {
	case "set":
		if len(args) < 3 {
			return p.help(ctx)
		}
		name, cmd := args[1], strings.Join(args[2:], " ")
		p.mgr.Commands().AddUserAlias(name, cmd)
		p.save()
		return ctx.Edit(fmt.Sprintf("✅ <code>.%s</code> → <code>.%s</code>", name, cmd))

	case "del", "delete", "remove":
		if len(args) < 2 {
			return p.help(ctx)
		}
		name := args[1]
		if _, ok := p.mgr.Commands().UserAliases()[name]; !ok {
			return ctx.Edit(ctx.Tlocal("没有这个别名: "+name, "No such alias: "+name))
		}
		p.mgr.Commands().RemoveUserAlias(name)
		p.save()
		return ctx.Edit(fmt.Sprintf("🗑 <code>.%s</code>", name))

	case "list", "ls":
		return p.listAliases(ctx)

	default:
		// Sugar: alias p=ping
		if name, cmd, ok := strings.Cut(args[0], "="); ok && len(args) == 1 {
			p.mgr.Commands().AddUserAlias(name, cmd)
			p.save()
			return ctx.Edit(fmt.Sprintf("✅ <code>.%s</code> → <code>.%s</code>", name, cmd))
		}
		return p.help(ctx)
	}
}

func (p *AliasPlugin) listAliases(ctx *interfaces.CommandContext) error {
	aliases := p.mgr.Commands().UserAliases()
	if len(aliases) == 0 {
		return ctx.Edit(ctx.Tlocal(
			"还没有别名。<code>alias p=ping</code> 试一个",
			"No aliases yet. Try <code>alias p=ping</code>"))
	}
	names := make([]string, 0, len(aliases))
	for name := range aliases {
		names = append(names, name)
	}
	sort.Strings(names)
	var b strings.Builder
	b.WriteString("🔗 " + ctx.Tlocal("别名", "Aliases") + "\n\n")
	for _, name := range names {
		fmt.Fprintf(&b, "<code>.%s</code> → <code>.%s</code>\n", name, aliases[name])
	}
	return ctx.Edit(b.String())
}
