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

func NewAlias() *AliasPlugin { return &AliasPlugin{file: "data/aliases.json"} }

func (p *AliasPlugin) Name() string        { return "alias" }
func (p *AliasPlugin) Description() string { return "命令别名" }
func (p *AliasPlugin) DescEN() string      { return "Command aliases" }

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
	if json.Unmarshal(data, &aliases) == nil {
		for name, cmd := range aliases {
			p.mgr.Commands().AddUserAlias(name, cmd)
		}
	}
}

func (p *AliasPlugin) save() {
	_ = os.MkdirAll(filepath.Dir(p.file), 0o700)
	data, _ := json.MarshalIndent(p.mgr.Commands().UserAliases(), "", "  ")
	_ = os.WriteFile(p.file, data, 0o600)
}

func aliasRow(prefix, name, repl string) string {
	return fmt.Sprintf("<code>%s%s</code> → <code>%s%s</code>", prefix, htmlEscape(name), prefix, htmlEscape(repl))
}

func (p *AliasPlugin) handleAlias(ctx *interfaces.CommandContext) error {
	prefix := p.mgr.Commands().GetPrefix()
	args := ctx.Args
	if len(args) == 0 || args[0] == "list" || args[0] == "ls" {
		return p.listAliases(ctx, prefix)
	}
	switch args[0] {
	case "set":
		if len(args) < 3 {
			return ctx.Edit(errText(ctx.Tlocal("用法 ", "Usage ") + cmdRef(prefix+"alias 名字=命令")))
		}
		name, repl := args[1], strings.Join(args[2:], " ")
		p.mgr.Commands().AddUserAlias(name, repl)
		p.save()
		return ctx.Edit("✅ " + aliasRow(prefix, name, repl))
	case "del", "delete", "remove", "rm":
		if len(args) < 2 {
			return ctx.Edit(errText(ctx.Tlocal("用法 ", "Usage ") + cmdRef(prefix+"alias del 名字")))
		}
		if _, ok := p.mgr.Commands().UserAliases()[args[1]]; !ok {
			return ctx.Edit(errText(ctx.Tlocal("没有这个别名 ", "No such alias: ") + cmdRef(prefix+args[1])))
		}
		p.mgr.Commands().RemoveUserAlias(args[1])
		p.save()
		return ctx.Edit("🗑 " + cmdRef(prefix+args[1]) + "  " + ctx.Tlocal("已删除", "removed"))
	default:
		// Sugar: alias p=ping
		if name, repl, ok := strings.Cut(args[0], "="); ok && len(args) == 1 {
			p.mgr.Commands().AddUserAlias(name, repl)
			p.save()
			return ctx.Edit("✅ " + aliasRow(prefix, name, repl))
		}
		return ctx.Edit(errText(ctx.Tlocal("用法 ", "Usage ") + cmdRef(prefix+"alias 名字=命令")))
	}
}

func (p *AliasPlugin) listAliases(ctx *interfaces.CommandContext, prefix string) error {
	aliases := p.mgr.Commands().UserAliases()
	if len(aliases) == 0 {
		c := newCard("🔗", ctx.Tlocal("别名", "Aliases"))
		c.blank().line(ctx.Tlocal("还没有别名", "No aliases yet"))
		c.hint(ctx.Tlocal(cmdRef(prefix+"alias p=ping")+" 试一个", cmdRef(prefix+"alias p=ping")+" to create one"))
		return ctx.Edit(c.String())
	}
	names := make([]string, 0, len(aliases))
	for name := range aliases {
		names = append(names, name)
	}
	sort.Strings(names)
	c := newCard("🔗", ctx.Tlocal(fmt.Sprintf("别名 · %d 个", len(names)), fmt.Sprintf("Aliases · %d", len(names)))).blank()
	for _, name := range names {
		c.line(aliasRow(prefix, name, aliases[name]))
	}
	c.hint(ctx.Tlocal("删除用 ", "Remove with ") + cmdRef(prefix+"alias del 名字"))
	return ctx.Edit(c.String())
}
