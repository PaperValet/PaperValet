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
func (p *PrefixPlugin) Description() string { return "命令前缀管理（多前缀支持）" }

func (p *PrefixPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	p.mgr = mgr
	p.load()
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "prefix",
		Aliases:     []string{"pfx", "cmdprefix"},
		Description: "命令前缀管理",
		Usage:       "prefix [list|add|del|set] [前缀]",
		Plugin:      p.Name(),
		Category:    "admin",
		OwnerOnly:   true,
		Handler:     p.handlePrefix,
	})
}

func (p *PrefixPlugin) Start(_ context.Context) error { return nil }
func (p *PrefixPlugin) Stop(_ context.Context) error  { return nil }

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
}

func (p *PrefixPlugin) handlePrefix(ctx *interfaces.CommandContext) error {
	args := ctx.Args
	if len(args) == 0 {
		mainPrefix := p.prefixes[0]
		return ctx.Edit(fmt.Sprintf(
			"🔧 <b>前缀管理</b>\n\n"+
				"当前主前缀: <code>%s</code>\n"+
				"所有前缀: <code>%s</code>\n\n"+
				"<b>用法:</b>\n"+
				"<code>prefix list</code> — 列出所有前缀\n"+
				"<code>prefix add &lt;前缀&gt;</code> — 添加前缀\n"+
				"<code>prefix del &lt;前缀&gt;</code> — 删除前缀\n"+
				"<code>prefix set &lt;前缀&gt;</code> — 设置为主前缀",
			mainPrefix, strings.Join(p.prefixes, "</code> <code>"),
		))
	}

	sub := args[0]
	switch sub {
	case "get":
		return ctx.Edit(fmt.Sprintf("当前主前缀: <code>%s</code>", p.prefixes[0]))

	case "list", "ls":
		return ctx.Edit(fmt.Sprintf(
			"🔧 <b>支持的前缀</b> (%d 个)\n\n<code>%s</code>",
			len(p.prefixes), strings.Join(p.prefixes, "</code> <code>"),
		))

	case "add":
		if len(args) < 2 {
			return ctx.Edit("用法: prefix add <前缀>")
		}
		newPrefix := args[1]
		for _, existing := range p.prefixes {
			if existing == newPrefix {
				return ctx.Edit(fmt.Sprintf("⚠️ 前缀 <code>%s</code> 已存在", newPrefix))
			}
		}
		p.prefixes = append(p.prefixes, newPrefix)
		p.save()
		return ctx.Edit(fmt.Sprintf("✅ 已添加前缀 <code>%s</code>", newPrefix))

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
				p.prefixes = append(p.prefixes[:i], p.prefixes[i+1:]...)
				p.save()
				return ctx.Edit(fmt.Sprintf("🗑 已删除前缀 <code>%s</code>", target))
			}
		}
		return ctx.Edit(fmt.Sprintf("❌ 未找到前缀 <code>%s</code>", target))

	case "set", "main":
		if len(args) < 2 {
			return ctx.Edit("用法: prefix set <前缀>")
		}
		target := args[1]
		for i, pref := range p.prefixes {
			if pref == target {
				// Move to front
				p.prefixes = append([]string{target}, append(p.prefixes[:i], p.prefixes[i+1:]...)...)
				p.save()
				return ctx.Edit(fmt.Sprintf("✅ 主前缀已设置为 <code>%s</code>", target))
			}
		}
		// Not found, add it
		p.prefixes = append([]string{target}, p.prefixes...)
		p.save()
		return ctx.Edit(fmt.Sprintf("✅ 已添加并设置为主前缀 <code>%s</code>", target))

	default:
		return ctx.Edit(fmt.Sprintf("❌ 未知子命令: %s\n\n用法: prefix [list|add|del|set]", sub))
	}
}