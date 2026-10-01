package builtin

import (
	"context"
	"fmt"
	"sort"
	"strings"

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
func (p *ReloadPlugin) Description() string { return "热重载全部外部插件" }
func (p *ReloadPlugin) DescEN() string      { return "Hot-reload all external plugins" }

func (p *ReloadPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	p.mgr = mgr
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "reload",
		Aliases:     []string{"rl"},
		Description: "重新加载所有外部插件（先卸载再加载）",
		DescEN:      "Reload every external plugin (unload then load)",
		Usage:       "reload",
		UsageEN:     "reload",
		Plugin:      p.Name(),
		Category:    "admin",
		OwnerOnly:   true,
		Handler:     p.handleReload,
	})
}

func (p *ReloadPlugin) Start(_ context.Context) error { return nil }
func (p *ReloadPlugin) Stop(_ context.Context) error  { return nil }

func (p *ReloadPlugin) handleReload(ctx *interfaces.CommandContext) error {
	if ctx.ArgCount() > 0 {
		return ctx.Edit(ctx.Tlocal(
			"reload 不带参数：重新加载所有外部插件。单个插件的装卸用 <code>apt load/unload</code>",
			"reload takes no arguments: it reloads every external plugin. For a single plugin use <code>apt load/unload</code>",
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
			"没有已加载的外部插件。用 <code>apt install &lt;名字&gt;</code> 安装，或 <code>apt search</code> 看仓库里有什么",
			"No external plugins are loaded. Install with <code>apt install &lt;name&gt;</code> or browse with <code>apt search</code>",
		))
	}

	var ok, failed []string
	for _, name := range names {
		if err := p.loader.Unload(ctx.Context(), name); err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		if err := p.loader.LoadByName(ctx.Context(), name); err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", name, err))
		} else {
			ok = append(ok, name)
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "🔄 %s %d/%d\n", ctx.Tlocal("重载完成", "reload finished"), len(ok), len(names))
	if len(ok) > 0 {
		b.WriteString("✅ " + strings.Join(ok, ", ") + "\n")
	}
	for _, f := range failed {
		b.WriteString("❌ " + f + "\n")
	}
	return ctx.Edit(b.String())
}
