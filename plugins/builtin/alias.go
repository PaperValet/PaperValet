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
		Usage: "alias 名字=命令 · alias del 名字 · alias list\n" +
			"\n" +
			"**示例**\n" +
			"• `alias p=ping`  以后发 .p 等于 .ping\n" +
			"• `alias set d5 dme 5`  带固定参数（之后 .d5）\n" +
			"• `alias list`  查看全部\n" +
			"• `alias del p`  删除\n" +
			"\n" +
			"**机制**\n" +
			"• 只展开一层，别名里再写别名不会继续展开\n" +
			"• 发别名时后面跟的参数会接在固定参数后面\n" +
			"• 命令部分不用写前缀\n" +
			"• 保存在 data/aliases.json，重启不丢",
		UsageEN: "alias name=command · alias del name · alias list\n" +
			"\n" +
			"**Examples**\n" +
			"• `alias p=ping`  .p now means .ping\n" +
			"• `alias set d5 dme 5`  fixed arguments (then .d5)\n" +
			"• `alias list`  list all\n" +
			"• `alias del p`  remove\n" +
			"\n" +
			"**How it works**\n" +
			"• Expands once; aliases inside aliases are not expanded\n" +
			"• Extra arguments are appended after the fixed ones\n" +
			"• No prefix needed in the command part\n" +
			"• Stored in data/aliases.json, survives restarts",
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
	return plugin.Code(prefix+name) + " → " + plugin.Code(prefix+repl)
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
