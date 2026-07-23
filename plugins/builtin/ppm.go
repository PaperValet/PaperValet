package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/internal/plugin/loader"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// PPMPlugin (Plugin Package Manager) manages external plugins.
type PPMPlugin struct {
	mgr      plugin.Manager
	loader   *loader.Loader
	pluginDir string
}

func NewPPM(loader *loader.Loader, pluginDir string) *PPMPlugin {
	return &PPMPlugin{loader: loader, pluginDir: pluginDir}
}

func (p *PPMPlugin) Name() string        { return "ppm" }
func (p *PPMPlugin) Description() string { return "外部插件包管理器" }

func (p *PPMPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	p.mgr = mgr
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "ppm",
		Aliases:     []string{"plugin", "plugins"},
		Description: "外部插件包管理",
		Usage:       "ppm list | ppm install <name> | ppm enable <name> | ppm disable <name> | ppm reload <name> | ppm remove <name> | ppm update <name> | ppm search <keyword> | ppm info <name>",
		Plugin:      p.Name(),
		Category:    "core",
		Handler:     p.handlePPM,
	})
}

func (p *PPMPlugin) Start(_ context.Context) error { return nil }
func (p *PPMPlugin) Stop(_ context.Context) error  { return nil }

func (p *PPMPlugin) handlePPM(ctx *interfaces.CommandContext) error {
	if ctx.ArgCount() == 0 {
		return ctx.Edit("用法: ppm list | ppm install <name> | ppm enable <name> | ppm disable <name> | ppm reload <name> | ppm remove <name> | ppm update <name> | ppm search <keyword> | ppm info <name>")
	}

	sub := ctx.GetArg(0)

	switch sub {
	case "list":
		return p.handleList(ctx)
	case "install":
		return p.handleInstall(ctx)
	case "enable":
		return p.handleEnable(ctx)
	case "disable":
		return p.handleDisable(ctx)
	case "reload":
		return p.handleReload(ctx)
	case "remove", "uninstall":
		return p.handleRemove(ctx)
	case "update":
		return p.handleUpdate(ctx)
	case "search":
		return p.handleSearch(ctx)
	case "info":
		return p.handleInfo(ctx)
	default:
		return ctx.Edit("未知子命令: " + sub)
	}
}

func (p *PPMPlugin) handleList(ctx *interfaces.CommandContext) error {
	loaded := p.loader.GetLoaded()
	
	// Also scan plugin directory for available .so files
	available := make(map[string]string)
	if p.pluginDir != "" {
		entries, err := os.ReadDir(p.pluginDir)
		if err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".so") {
					name := strings.TrimSuffix(e.Name(), ".so")
					available[name] = filepath.Join(p.pluginDir, e.Name())
				}
			}
		}
	}

	var b strings.Builder
	b.WriteString("📦 外部插件列表:\n\n")

	if len(loaded) == 0 && len(available) == 0 {
		b.WriteString("  (无插件)")
	} else {
		// Show loaded plugins
		if len(loaded) > 0 {
			b.WriteString("  ✅ 已加载:\n")
			for name, lp := range loaded {
				status := "⏸️"
				// Check if plugin has status info
				b.WriteString(fmt.Sprintf("    %s %s — %s\n", status, name, lp.Path))
				if lp.Metadata != nil {
					b.WriteString(fmt.Sprintf("       %s\n", lp.Metadata.Description))
				}
			}
			b.WriteString("\n")
		}

		// Show available but not loaded
		notLoaded := make([]string, 0)
		for name, path := range available {
			if _, ok := loaded[name]; !ok {
				notLoaded = append(notLoaded, fmt.Sprintf("    ⏸️ %s — %s", name, path))
			}
		}
		if len(notLoaded) > 0 {
			b.WriteString("  ⏸️ 可用但未加载:\n")
			b.WriteString(strings.Join(notLoaded, "\n"))
		}
	}

	return ctx.Edit(b.String())
}

func (p *PPMPlugin) handleInstall(ctx *interfaces.CommandContext) error {
	if ctx.ArgCount() < 2 {
		return ctx.Edit("用法: ppm install <name>\n示例: ppm install ping")
	}
	name := ctx.GetArg(1)
	
	// Check if already loaded
	if _, ok := p.loader.GetLoaded()[name]; ok {
		return ctx.Edit(fmt.Sprintf("插件 %s 已加载", name))
	}
	
	// Try to load from plugin directory
	pluginPath := filepath.Join(p.pluginDir, name+".so")
	if _, err := os.Stat(pluginPath); err == nil {
		if err := p.loader.Load(ctx.Context(), pluginPath); err != nil {
			return ctx.Edit(fmt.Sprintf("安装失败: %v", err))
		}
		return ctx.Edit(fmt.Sprintf("✅ 已安装并加载: %s", name))
	}
	
	// Check for known plugins in plugins-external directory
	externalPath := filepath.Join(filepath.Dir(p.pluginDir), "..", "plugins-external", name)
	if _, err := os.Stat(externalPath); err == nil {
		return ctx.Edit(fmt.Sprintf("插件源码位于: %s\n请先编译: go build -buildmode=plugin -o %s/%s.so %s", 
			externalPath, p.pluginDir, name, externalPath))
	}
	
	return ctx.Edit(fmt.Sprintf("未找到插件: %s\n可用插件在 plugins-external/ 目录中", name))
}

func (p *PPMPlugin) handleEnable(ctx *interfaces.CommandContext) error {
	if ctx.ArgCount() < 2 {
		return ctx.Edit("用法: ppm enable <name>")
	}
	name := ctx.GetArg(1)

	pluginPath := filepath.Join(p.pluginDir, name+".so")
	if _, err := os.Stat(pluginPath); err != nil {
		return ctx.Edit(fmt.Sprintf("插件文件不存在: %s", pluginPath))
	}

	if err := p.loader.Load(ctx.Context(), pluginPath); err != nil {
		return ctx.Edit(fmt.Sprintf("启用失败: %v", err))
	}
	return ctx.Edit(fmt.Sprintf("✅ 已启用: %s", name))
}

func (p *PPMPlugin) handleDisable(ctx *interfaces.CommandContext) error {
	if ctx.ArgCount() < 2 {
		return ctx.Edit("用法: ppm disable <name>")
	}
	name := ctx.GetArg(1)
	
	if err := p.loader.Unload(ctx.Context(), name); err != nil {
		return ctx.Edit(fmt.Sprintf("禁用失败: %v", err))
	}
	return ctx.Edit(fmt.Sprintf("⏸️ 已禁用: %s", name))
}

func (p *PPMPlugin) handleReload(ctx *interfaces.CommandContext) error {
	if ctx.ArgCount() < 2 {
		return ctx.Edit("用法: ppm reload <name>")
	}
	name := ctx.GetArg(1)

	// Unload first
	if err := p.loader.Unload(ctx.Context(), name); err != nil {
		// Ignore error if not loaded
	}

	// Reload
	pluginPath := filepath.Join(p.pluginDir, name+".so")
	if _, err := os.Stat(pluginPath); err != nil {
		return ctx.Edit(fmt.Sprintf("插件文件不存在: %s", pluginPath))
	}

	if err := p.loader.Load(ctx.Context(), pluginPath); err != nil {
		return ctx.Edit(fmt.Sprintf("重载失败: %v", err))
	}
	return ctx.Edit(fmt.Sprintf("🔄 已重载: %s", name))
}

func (p *PPMPlugin) handleRemove(ctx *interfaces.CommandContext) error {
	if ctx.ArgCount() < 2 {
		return ctx.Edit("用法: ppm remove <name>")
	}
	name := ctx.GetArg(1)
	
	// Unload first
	if err := p.loader.Unload(ctx.Context(), name); err != nil {
		// Ignore error if not loaded
	}
	
	// Remove .so file
	pluginPath := filepath.Join(p.pluginDir, name+".so")
	if err := os.Remove(pluginPath); err != nil && !os.IsNotExist(err) {
		return ctx.Edit(fmt.Sprintf("删除文件失败: %v", err))
	}
	return ctx.Edit(fmt.Sprintf("🗑 已删除: %s", name))
}

func (p *PPMPlugin) handleUpdate(ctx *interfaces.CommandContext) error {
	if ctx.ArgCount() < 2 {
		return ctx.Edit("用法: ppm update <name>")
	}
	name := ctx.GetArg(1)
	
	externalPath := filepath.Join(filepath.Dir(p.pluginDir), "..", "plugins-external", name)
	if _, err := os.Stat(externalPath); err != nil {
		return ctx.Edit(fmt.Sprintf("插件源码不存在: %s", externalPath))
	}
	
	// Rebuild
	pluginPath := filepath.Join(p.pluginDir, name+".so")
	// We can't run go build here directly - would need to shell out
	return ctx.Edit(fmt.Sprintf("请手动重新编译:\ncd %s && go build -buildmode=plugin -o %s .", externalPath, pluginPath))
}

func (p *PPMPlugin) handleSearch(ctx *interfaces.CommandContext) error {
	if ctx.ArgCount() < 2 {
		return ctx.Edit("用法: ppm search <keyword>")
	}
	keyword := strings.ToLower(ctx.GetArg(1))
	
	// Scan external plugins
	externalDir := filepath.Join(filepath.Dir(p.pluginDir), "..", "plugins-external")
	entries, err := os.ReadDir(externalDir)
	if err != nil {
		return ctx.Edit("无法读取插件目录")
	}
	
	var results []string
	for _, e := range entries {
		if e.IsDir() && strings.Contains(strings.ToLower(e.Name()), keyword) {
			results = append(results, e.Name())
		}
	}
	
	if len(results) == 0 {
		return ctx.Edit("未找到匹配的插件")
	}
	
	var b strings.Builder
	b.WriteString(fmt.Sprintf("🔍 搜索 '%s' 结果:\n", keyword))
	for _, r := range results {
		b.WriteString(fmt.Sprintf("  • %s\n", r))
	}
	return ctx.Edit(b.String())
}

func (p *PPMPlugin) handleInfo(ctx *interfaces.CommandContext) error {
	if ctx.ArgCount() < 2 {
		return ctx.Edit("用法: ppm info <name>")
	}
	name := ctx.GetArg(1)
	
	loaded := p.loader.GetLoaded()
	if lp, ok := loaded[name]; ok {
		var b strings.Builder
		b.WriteString(fmt.Sprintf("📦 %s (已加载)\n", name))
		b.WriteString(fmt.Sprintf("路径: %s\n", lp.Path))
		if lp.Metadata != nil {
			b.WriteString(fmt.Sprintf("描述: %s\n", lp.Metadata.Description))
			b.WriteString(fmt.Sprintf("作者: %s\n", lp.Metadata.Author))
			b.WriteString(fmt.Sprintf("版本: %s\n", lp.Metadata.Version))
		}
		return ctx.Edit(b.String())
	}
	
	// Check available
	pluginPath := filepath.Join(p.pluginDir, name+".so")
	if _, err := os.Stat(pluginPath); err == nil {
		return ctx.Edit(fmt.Sprintf("📦 %s (可用，未加载)\n路径: %s", name, pluginPath))
	}
	
	return ctx.Edit(fmt.Sprintf("未找到插件: %s", name))
}