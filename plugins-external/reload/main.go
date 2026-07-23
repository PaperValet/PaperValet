package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type ReloadPlugin struct {
	loader plugin.PluginLoader
}

func New(loader plugin.PluginLoader) (plugin.Plugin, error) {
	return &ReloadPlugin{loader: loader}, nil
}

var Metadata = &plugin.PluginMetadata{
	Name:        "reload",
	Description: "插件热重载",
	Version:     "1.0.0",
	Author:      "PaperValet",
	MinVersion:  "0.1.0",
}

func (p *ReloadPlugin) Name() string        { return "reload" }
func (p *ReloadPlugin) Description() string { return "插件热重载" }

func (p *ReloadPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&plugin.Command{
		Name:        "reload",
		Aliases:     []string{"rl"},
		Description: "重载插件",
		Usage:       "reload <插件名|all|list>",
		Plugin:      p.Name(),
		Category:    "admin",
		OwnerOnly:   true,
		Handler:     p.handleReload,
	})
}

func (p *ReloadPlugin) handleReload(ctx *plugin.CommandContext) error {
	args := ctx.Args
	if len(args) == 0 {
		return ctx.Edit("用法: reload <插件名|all|list>")
	}

	target := args[0]

	switch target {
	case "list", "ls":
		loaded := p.loader.GetLoaded()
		if len(loaded) == 0 {
			return ctx.Edit("暂无已加载的外部插件")
		}
		var b strings.Builder
		b.WriteString("🔌 <b>已加载外部插件:</b>\n\n")
		for name, info := range loaded {
			b.WriteString(fmt.Sprintf("• <code>%s</code> (%s)\n", name, info.Path))
			if info.Metadata != nil {
				b.WriteString(fmt.Sprintf("  版本: %s, 作者: %s\n", info.Metadata.Version, info.Metadata.Author))
			}
		}
		return ctx.Edit(b.String())

	case "all":
		loaded := p.loader.GetLoaded()
		var results []string
		for name := range loaded {
			if err := p.loader.Unload(ctx.Context(), name); err != nil {
				results = append(results, fmt.Sprintf("❌ %s: %v", name, err))
			} else {
				if err := p.loader.Load(ctx.Context(), name); err != nil {
					results = append(results, fmt.Sprintf("❌ %s (重载失败): %v", name, err))
				} else {
					results = append(results, fmt.Sprintf("✅ %s", name))
				}
			}
		}
		return ctx.Edit("🔄 全部重载完成:\n" + strings.Join(results, "\n"))

	default:
		// Reload specific plugin
		if err := p.loader.Unload(ctx.Context(), target); err != nil {
			return ctx.Edit(fmt.Sprintf("❌ 卸载失败: %v", err))
		}
		if err := p.loader.Load(ctx.Context(), target); err != nil {
			return ctx.Edit(fmt.Sprintf("❌ 重载失败: %v", err))
		}
		return ctx.Edit(fmt.Sprintf("✅ 插件已重载: %s", target))
	}
}

func (p *ReloadPlugin) Start(ctx context.Context) error { return nil }
func (p *ReloadPlugin) Stop(ctx context.Context) error  { return nil }