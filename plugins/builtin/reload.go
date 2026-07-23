package builtin

import (
	"context"
	"fmt"
	"strings"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/internal/plugin/loader"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// ReloadPlugin provides hot-reload functionality for external plugins.
type ReloadPlugin struct {
	loader *loader.Loader
	mgr    plugin.Manager
}

func NewReload(pluginLoader *loader.Loader) *ReloadPlugin {
	return &ReloadPlugin{loader: pluginLoader}
}

func (p *ReloadPlugin) Name() string        { return "reload" }
func (p *ReloadPlugin) Description() string { return "外部插件热重载" }

func (p *ReloadPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	p.mgr = mgr
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "reload",
		Aliases:     []string{"rl"},
		Description: "重载外部插件",
		Usage:       "reload <插件名|all|list>",
		Plugin:      p.Name(),
		Category:    "admin",
		OwnerOnly:   true,
		Handler:     p.handleReload,
	})
}

func (p *ReloadPlugin) Start(_ context.Context) error { return nil }
func (p *ReloadPlugin) Stop(_ context.Context) error  { return nil }

func (p *ReloadPlugin) handleReload(ctx *interfaces.CommandContext) error {
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
				b.WriteString(fmt.Sprintf("  版本: %s | 作者: %s\n", info.Metadata.Version, info.Metadata.Author))
			}
		}
		return ctx.Edit(b.String())

	case "all":
		loaded := p.loader.GetLoaded()
		var results []string
		for name := range loaded {
			if err := p.loader.Unload(ctx.Context(), name); err != nil {
				results = append(results, fmt.Sprintf("❌ %s 卸载失败: %v", name, err))
				continue
			}
			if err := p.loader.Load(ctx.Context(), name); err != nil {
				results = append(results, fmt.Sprintf("❌ %s 重载失败: %v", name, err))
			} else {
				results = append(results, fmt.Sprintf("✅ %s", name))
			}
		}
		return ctx.Edit("🔄 <b>全量重载结果:</b>\n\n" + strings.Join(results, "\n"))

	default:
		// Reload single plugin
		if err := p.loader.Unload(ctx.Context(), target); err != nil {
			// Not loaded, try to load directly
			if err := p.loader.Load(ctx.Context(), target); err != nil {
				return ctx.Edit(fmt.Sprintf("❌ 加载失败: %v", err))
			}
			return ctx.Edit(fmt.Sprintf("✅ 已加载: %s", target))
		}

		if err := p.loader.Load(ctx.Context(), target); err != nil {
			return ctx.Edit(fmt.Sprintf("❌ 重载失败: %v", err))
		}
		return ctx.Edit(fmt.Sprintf("🔄 已重载: %s", target))
	}
}