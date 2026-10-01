package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// PrefixPlugin manages command prefixes with persistent storage.
type PrefixPlugin struct {
	mgr      plugin.Manager
	prefixes []string
	file     string
}

func NewPrefix() *PrefixPlugin {
	return &PrefixPlugin{
		prefixes: []string{"."},
		file:     "data/prefixes.json",
	}
}

func (p *PrefixPlugin) Name() string        { return "prefix" }
func (p *PrefixPlugin) Description() string { return "管理命令前缀" }
func (p *PrefixPlugin) DescEN() string      { return "Manage command prefixes" }

func (p *PrefixPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	p.mgr = mgr
	p.load()
	// Apply persisted prefixes to the command registry immediately.
	p.mgr.Commands().SetPrefixes(p.prefixes)
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "prefix",
		Aliases:     []string{"pfx", "cmdprefix"},
		Description: "改命令触发符号，比如把 . 换成 !",
		DescEN:      "Change the command trigger symbol, e.g. . to !",
		Usage:       "prefix add|del|set <符号>",
		UsageEN:     "prefix add|del|set <symbol>",
		Plugin:      p.Name(),
		Category:    "admin",
		OwnerOnly:   true,
		Handler:     p.handlePrefix,
	})
}

func (p *PrefixPlugin) Start(_ context.Context) error { return nil }
func (p *PrefixPlugin) Stop(_ context.Context) error  { return nil }

func escapedPrefixes(prefixes []string) string {
	out := make([]string, 0, len(prefixes))
	for _, p := range prefixes {
		out = append(out, html.EscapeString(p))
	}
	return strings.Join(out, "</code> <code>")
}

func (p *PrefixPlugin) load() {
	data, err := os.ReadFile(p.file)
	if err != nil {
		return
	}
	var prefixes []string
	if err := json.Unmarshal(data, &prefixes); err == nil && len(prefixes) > 0 {
		p.prefixes = prefixes
	}
}

func (p *PrefixPlugin) save() {
	os.MkdirAll(filepath.Dir(p.file), 0o755)
	data, _ := json.MarshalIndent(p.prefixes, "", "  ")
	os.WriteFile(p.file, data, 0o644)
	p.mgr.Commands().SetPrefixes(p.prefixes)
}

func (p *PrefixPlugin) handlePrefix(ctx *interfaces.CommandContext) error {
	args := ctx.Args
	if len(args) == 0 {
		mainPrefix := p.prefixes[0]
		return ctx.Edit(ctx.Tlocal(
			fmt.Sprintf("🔧 <b>前缀</b>\n\n主前缀: <code>%s</code>\n全部: <code>%s</code>\n\n<code>prefix add !</code> 添加\n<code>prefix del !</code> 删除\n<code>prefix set !</code> 设为主前缀",
				html.EscapeString(mainPrefix), escapedPrefixes(p.prefixes)),
			fmt.Sprintf("🔧 <b>Prefixes</b>\n\nMain: <code>%s</code>\nAll: <code>%s</code>\n\n<code>prefix add !</code> add\n<code>prefix del !</code> remove\n<code>prefix set !</code> make main",
				html.EscapeString(mainPrefix), escapedPrefixes(p.prefixes))))
	}

	sub := strings.ToLower(ctx.GetArg(0))
	switch sub {
	case "get":
		return ctx.Edit(fmt.Sprintf("当前主前缀: <code>%s</code>", html.EscapeString(p.prefixes[0])))

	case "list", "ls":
		return ctx.Edit(fmt.Sprintf(
			"🔧 <b>支持的前缀</b> (%d 个)\n\n<code>%s</code>",
			len(p.prefixes), escapedPrefixes(p.prefixes),
		))

	case "add":
		if len(args) < 2 {
			return ctx.Edit("用法: prefix add <前缀>")
		}
		newPrefix := args[1]
		for _, existing := range p.prefixes {
			if existing == newPrefix {
				return ctx.Edit(fmt.Sprintf("⚠️ 前缀 <code>%s</code> 已存在", html.EscapeString(newPrefix)))
			}
		}
		prefixes := p.mgr.Commands().GetPrefixes()
		prefixes = append(prefixes, newPrefix)
		p.mgr.Commands().SetPrefixes(prefixes)
		p.prefixes = prefixes
		p.save()
		return ctx.Edit(fmt.Sprintf("✅ 已添加前缀 <code>%s</code>", html.EscapeString(newPrefix)))

	case "del", "delete", "remove":
		if len(args) < 2 {
			return ctx.Edit("用法: prefix del <前缀>")
		}
		target := args[1]
		if len(p.prefixes) <= 1 {
			return ctx.Edit("⚠️ 至少保留一个前缀")
		}
		for i, pref := range p.prefixes {
			if pref == target {
				prefixes := p.mgr.Commands().GetPrefixes()
				prefixes = append(prefixes[:i], prefixes[i+1:]...)
				p.mgr.Commands().SetPrefixes(prefixes)
				p.prefixes = prefixes
				p.save()
				return ctx.Edit(fmt.Sprintf("🗑 已删除前缀 <code>%s</code>", html.EscapeString(target)))
			}
		}
		return ctx.Edit(fmt.Sprintf("❌ 未找到前缀 <code>%s</code>", html.EscapeString(target)))

	case "set", "main":
		if len(args) < 2 {
			return ctx.Edit("用法: prefix set <前缀>")
		}
		target := args[1]
		for i, pref := range p.prefixes {
			if pref == target {
				// Move to front
				prefixes := p.mgr.Commands().GetPrefixes()
				prefixes = append([]string{target}, append(prefixes[:i], prefixes[i+1:]...)...)
				p.mgr.Commands().SetPrefixes(prefixes)
				p.prefixes = prefixes
				p.save()
				return ctx.Edit(fmt.Sprintf("✅ 主前缀已设置为 <code>%s</code>", html.EscapeString(target)))
			}
		}
		// Not found, add it and promote it to main
		prefixes := p.mgr.Commands().GetPrefixes()
		prefixes = append([]string{target}, prefixes...)
		p.mgr.Commands().SetPrefixes(prefixes)
		p.prefixes = prefixes
		p.save()
		return ctx.Edit(fmt.Sprintf("✅ 已添加并设置为主前缀 <code>%s</code>", target))

	default:
		return ctx.Edit(fmt.Sprintf("❌ 未知子命令: %s\n\n用法: prefix [list|add|del|set]", sub))
	}
}
