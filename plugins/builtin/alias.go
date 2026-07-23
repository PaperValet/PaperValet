package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// AliasPlugin manages command aliases with persistent storage.
type AliasPlugin struct {
	aliases map[string]string
	file    string
}

func NewAlias() *AliasPlugin {
	return &AliasPlugin{
		aliases: make(map[string]string),
		file:    "data/aliases.json",
	}
}

func (p *AliasPlugin) Name() string        { return "alias" }
func (p *AliasPlugin) Description() string { return "命令别名管理" }

func (p *AliasPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	p.load()
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "alias",
		Aliases:     []string{"al"},
		Description: "管理命令别名",
		Usage:       "alias [set|del|list] [名称] [命令]",
		Plugin:      p.Name(),
		Category:    "tools",
		Handler:     p.handleAlias,
	})
}

func (p *AliasPlugin) Start(_ context.Context) error { return nil }
func (p *AliasPlugin) Stop(_ context.Context) error  { return nil }

func (p *AliasPlugin) load() {
	data, err := os.ReadFile(p.file)
	if err != nil {
		return
	}
	json.Unmarshal(data, &p.aliases)
}

func (p *AliasPlugin) save() {
	os.MkdirAll(filepath.Dir(p.file), 0o755)
	data, _ := json.MarshalIndent(p.aliases, "", "  ")
	os.WriteFile(p.file, data, 0o644)
}

func (p *AliasPlugin) handleAlias(ctx *interfaces.CommandContext) error {
	args := ctx.Args
	if len(args) == 0 {
		return p.listAliases(ctx)
	}

	sub := args[0]
	switch sub {
	case "set":
		if len(args) < 3 {
			return ctx.Edit("用法: alias set <名称> <命令>")
		}
		name := args[1]
		cmd := strings.Join(args[2:], " ")
		p.aliases[name] = cmd
		p.save()
		return ctx.Edit(fmt.Sprintf("✅ 别名已设置: %s → %s", name, cmd))

	case "del", "delete", "remove":
		if len(args) < 2 {
			return ctx.Edit("用法: alias del <名称>")
		}
		name := args[1]
		if _, ok := p.aliases[name]; ok {
			delete(p.aliases, name)
			p.save()
			return ctx.Edit(fmt.Sprintf("🗑 别名已删除: %s", name))
		}
		return ctx.Edit("别名不存在: " + name)

	case "list", "ls":
		return p.listAliases(ctx)

	case "help", "h":
		return ctx.Edit(`🔧 <b>别名管理</b>

<b>用法:</b>
• <code>alias set <名称> <命令></code> - 设置别名
• <code>alias del <名称></code> - 删除别名
• <code>alias list</code> - 列出所有别名

<b>示例:</b>
• <code>alias set ping .ping</code>
• <code>alias set deploy .exec ./deploy.sh</code>

<b>注意:</b> 别名不支持嵌套，仅展开一次。`)

	default:
		return ctx.Edit("未知子命令: " + sub)
	}
}

func (p *AliasPlugin) listAliases(ctx *interfaces.CommandContext) error {
	if len(p.aliases) == 0 {
		return ctx.Edit("暂无别名")
	}
	var b strings.Builder
	b.WriteString("🔧 <b>命令别名列表:</b>\n\n")
	for name, cmd := range p.aliases {
		b.WriteString(fmt.Sprintf("• <code>%s</code> → <code>%s</code>\n", name, cmd))
	}
	return ctx.Edit(b.String())
}