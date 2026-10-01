package builtin

import (
	"context"
	"fmt"
	"sort"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/internal/plugin/loader"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// ReloadPlugin hot-reloads every external plugin.
type ReloadPlugin struct {
	loader *loader.Loader
	mgr    plugin.Manager
}

func NewReload(pluginLoader *loader.Loader) *ReloadPlugin {
	return &ReloadPlugin{loader: pluginLoader}
}

func (p *ReloadPlugin) Name() string        { return "reload" }
func (p *ReloadPlugin) Description() string { return "重载外部插件" }
func (p *ReloadPlugin) DescEN() string      { return "Hot-reload all external plugins" }

func (p *ReloadPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	p.mgr = mgr
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "reload",
		Description: "重载全部外部插件",
		DescEN:      "Reload all external plugins",
		Usage: "reload\n" +
			"\n" +
			"**机制**\n" +
			"• 把已加载的外部插件逐个卸载再加载，重新读取各自的配置\n" +
			"• 不带参数，只有这一种用法；单个插件用 `apt rm` / `apt i`\n" +
			"• 内建插件不受影响\n" +
			"• 注意：Go 插件文件被替换后无法在同一进程里换新代码，更新插件文件后请用 `restart`",
		UsageEN: "reload\n" +
			"\n" +
			"**How it works**\n" +
			"• Unloads and reloads every loaded external plugin, re-reading their settings\n" +
			"• Takes no arguments; for a single plugin use `apt rm` / `apt i`\n" +
			"• Built-in plugins are untouched\n" +
			"• Note: Go cannot swap a replaced .so inside the same process; after updating plugin files use `restart`",
		Plugin:    p.Name(),
		Category:  "admin",
		OwnerOnly: true,
		Handler:   p.handleReload,
	})
}

func (p *ReloadPlugin) Start(_ context.Context) error { return nil }
func (p *ReloadPlugin) Stop(_ context.Context) error  { return nil }

func (p *ReloadPlugin) handleReload(ctx *interfaces.CommandContext) error {
	if ctx.ArgCount() > 0 {
		return ctx.Edit(ctx.Tlocal(
			"reload 不带参数：重新加载所有外部插件。单个插件的装卸用 `apt i` / `apt rm`",
			"reload takes no arguments: it reloads every external plugin. For a single plugin use `apt i` / `apt rm`",
		))
	}

	loaded := p.loader.GetLoaded()
	names := make([]string, 0, len(loaded))
	for name := range loaded {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return ctx.Edit(ctx.Tlocal(
			"没有已加载的外部插件。用 `apt i 名字` 安装，或 `apt s` 看仓库里有什么",
			"No external plugins are loaded. Install with `apt i <name>` or browse with `apt s`",
		))
	}

	var ok, failed []string
	for _, name := range names {
		if err := p.loader.Unload(ctx.Context(), name); err != nil {
			failed = append(failed, failLine(name, esc(err.Error())))
			continue
		}
		if err := p.loader.LoadByName(ctx.Context(), name); err != nil {
			failed = append(failed, failLine(name, esc(err.Error())))
		} else {
			ok = append(ok, okLine(name, ctx.Tlocal("已重载", "reloaded")))
		}
	}
	c := newCard("🔄", ctx.Tlocal(fmt.Sprintf("重载完成 %d/%d", len(ok), len(names)), fmt.Sprintf("Reloaded %d/%d", len(names)-len(failed), len(names)))).blank()
	for _, l := range append(ok, failed...) {
		c.line(l)
	}
	return ctx.Edit(c.String())
}
