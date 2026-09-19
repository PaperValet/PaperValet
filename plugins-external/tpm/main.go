package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	goplugin "plugin"
	"strings"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type TPMPlugin struct {
	mgr        plugin.Manager
	pluginsDir string
}

func New() (plugin.Plugin, error) {
	return &TPMPlugin{}, nil
}

var Metadata = &plugin.PluginMetadata{
	Name:        "tpm",
	Description: "Telegram插件管理器",
	Version:     "1.0.0",
	Author:      "PaperValet",
	MinVersion:  "0.1.0",
}

func (p *TPMPlugin) Name() string        { return "tpm" }
func (p *TPMPlugin) Description() string { return "Telegram插件管理器" }

func (p *TPMPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	p.mgr = mgr
	p.pluginsDir = "plugins"
	cmds := []*plugin.Command{
		{
			Name:        "tpm",
			Aliases:     []string{"plugin", "plugins"},
			Description: "插件管理器",
			Usage:       "tpm [list|install|remove|update|enable|disable|search] [插件名]",
			Plugin:      p.Name(),
			Category:    "admin",
			OwnerOnly:   true,
			Handler:     p.handleTPM,
		},
	}

	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *TPMPlugin) handleTPM(ctx *plugin.CommandContext) error {
	args := ctx.Args
	if len(args) == 0 {
		return p.showHelp(ctx)
	}

	sub := args[0]

	switch sub {
	case "list", "ls":
		return p.listPlugins(ctx)
	case "install", "add":
		if len(args) < 2 {
			return ctx.Edit("用法: tpm install <插件名|GitHub URL>")
		}
		return p.install(ctx, args[1])
	case "remove", "rm", "uninstall":
		if len(args) < 2 {
			return ctx.Edit("用法: tpm remove <插件名>")
		}
		return p.remove(ctx, args[1])
	case "update", "up":
		if len(args) < 2 {
			return p.updateAll(ctx)
		}
		return ctx.Edit(p.updateOne(ctx, args[1]))
	case "enable":
		if len(args) < 2 {
			return ctx.Edit("用法: tpm enable <插件名>")
		}
		return p.enable(ctx, args[1])
	case "disable":
		if len(args) < 2 {
			return ctx.Edit("用法: tpm disable <插件名>")
		}
		return p.disable(ctx, args[1])
	case "search", "find":
		if len(args) < 2 {
			return ctx.Edit("用法: tpm search <关键词>")
		}
		return p.search(ctx, args[1])
	case "info":
		if len(args) < 2 {
			return ctx.Edit("用法: tpm info <插件名>")
		}
		return p.info(ctx, args[1])
	default:
		return p.showHelp(ctx)
	}
}

// install downloads a plugin .so from the release registry and loads it.
func (p *TPMPlugin) install(ctx *plugin.CommandContext, name string) error {
	if err := p.download(ctx.Context(), name); err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 安装失败: %v", err))
	}
	if err := p.load(ctx.Context(), name); err != nil {
		return ctx.Edit(fmt.Sprintf("✅ 已下载，但加载失败: %v", err))
	}
	return ctx.Edit(fmt.Sprintf("✅ 插件 <b>%s</b> 安装并加载成功", name))
}

// remove deletes an installed plugin .so.
func (p *TPMPlugin) remove(ctx *plugin.CommandContext, name string) error {
	if p.isLoaded(name) {
		if err := p.unload(ctx.Context(), name); err != nil {
			return ctx.Edit(fmt.Sprintf("❌ 卸载失败: %v", err))
		}
	}
	path := filepath.Join(p.pluginsDir, name+".so")
	if err := os.Remove(path); err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 移除失败: %v", err))
	}
	return ctx.Edit(fmt.Sprintf("🗑 插件 <b>%s</b> 已移除", name))
}

// enable loads a plugin by name.
func (p *TPMPlugin) enable(ctx *plugin.CommandContext, name string) error {
	if p.isLoaded(name) {
		return ctx.Edit(fmt.Sprintf("⚪ 插件 <b>%s</b> 已加载", name))
	}
	if err := p.load(ctx.Context(), name); err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 加载失败: %v", err))
	}
	return ctx.Edit(fmt.Sprintf("✅ 插件 <b>%s</b> 已加载", name))
}

// disable unloads a plugin by name.
func (p *TPMPlugin) disable(ctx *plugin.CommandContext, name string) error {
	if !p.isLoaded(name) {
		return ctx.Edit(fmt.Sprintf("⚪ 插件 <b>%s</b> 未加载", name))
	}
	if err := p.unload(ctx.Context(), name); err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 卸载失败: %v", err))
	}
	return ctx.Edit(fmt.Sprintf("⏹ 插件 <b>%s</b> 已卸载", name))
}

// search lists installed and loaded plugins matching the keyword.
func (p *TPMPlugin) search(ctx *plugin.CommandContext, keyword string) error {
	kw := strings.ToLower(keyword)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("🔍 <b>搜索: %s</b>\n\n", keyword))

	found := 0
	for _, info := range p.mgr.GetAllInfo() {
		if !strings.Contains(strings.ToLower(info.Name), kw) &&
			!strings.Contains(strings.ToLower(info.Description), kw) {
			continue
		}
		b.WriteString(fmt.Sprintf("✅ <b>%s</b> — %s\n", info.Name, info.Description))
		found++
	}

	for _, name := range p.installed() {
		if !strings.Contains(strings.ToLower(name), kw) || p.isLoaded(name) {
			continue
		}
		b.WriteString(fmt.Sprintf("💾 <b>%s</b> — 已安装未加载\n", name))
		found++
	}

	if found == 0 {
		b.WriteString("未找到匹配的插件\n")
	}
	b.WriteString(fmt.Sprintf("\n共 <b>%d</b> 个匹配", found))
	return ctx.Edit(b.String())
}

// info shows details for a loaded or installed plugin.
func (p *TPMPlugin) info(ctx *plugin.CommandContext, name string) error {
	if info, ok := p.mgr.GetInfo(name); ok {
		status := "🔵 内建"
		if info.Status == plugin.StatusActive {
			status = "✅ 活跃"
		}
		return ctx.Edit(fmt.Sprintf("📦 <b>%s</b> %s\n%s", name, status, info.Description))
	}
	if p.isLoaded(name) {
		return ctx.Edit(fmt.Sprintf("📦 <b>%s</b> ✅ 已加载", name))
	}
	for _, n := range p.installed() {
		if n == name {
			return ctx.Edit(fmt.Sprintf("💾 <b>%s</b> ⚪ 已安装未加载\n\n使用 <code>tpm enable %s</code> 加载", name, name))
		}
	}
	return ctx.Edit(fmt.Sprintf("❌ 未找到插件: %s", name))
}

// updateAll re-downloads every installed plugin and reloads it.
func (p *TPMPlugin) updateAll(ctx *plugin.CommandContext) error {
	names := p.installed()
	if len(names) == 0 {
		return ctx.Edit("📦 没有已安装的插件可更新")
	}
	var results []string
	for _, name := range names {
		results = append(results, p.updateOne(ctx, name))
	}
	return ctx.Edit(fmt.Sprintf("🔄 <b>更新结果</b>\n\n%s", strings.Join(results, "\n")))
}

// updateOne re-downloads a plugin and reloads it.
func (p *TPMPlugin) updateOne(ctx *plugin.CommandContext, name string) string {
	if p.isLoaded(name) {
		if err := p.unload(ctx.Context(), name); err != nil {
			return fmt.Sprintf("❌ <b>%s</b> 卸载失败: %v", name, err)
		}
	}
	path := filepath.Join(p.pluginsDir, name+".so")
	if err := os.Remove(path); err != nil {
		return fmt.Sprintf("❌ <b>%s</b> 删除旧文件失败: %v", name, err)
	}
	if err := p.download(ctx.Context(), name); err != nil {
		return fmt.Sprintf("❌ <b>%s</b> 下载失败: %v", name, err)
	}
	if err := p.load(ctx.Context(), name); err != nil {
		return fmt.Sprintf("⚠️ <b>%s</b> 已下载但加载失败: %v", name, err)
	}
	return fmt.Sprintf("✅ <b>%s</b> 已更新", name)
}

// download fetches a plugin .so from the release registry.
func (p *TPMPlugin) download(ctx context.Context, name string) error {
	name = strings.TrimSuffix(name, ".so")
	dest := filepath.Join(p.pluginsDir, name+".so")
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("插件 %s 已安装", name)
	}
	url := fmt.Sprintf("https://github.com/TiaraBasori/PaperValet-Plugins/releases/latest/download/%s.so", name)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败 (HTTP %d)，插件可能不存在", resp.StatusCode)
	}
	if err := os.MkdirAll(p.pluginsDir, 0o755); err != nil {
		return err
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

// load loads a plugin .so from the plugins dir.
func (p *TPMPlugin) load(ctx context.Context, name string) error {
	path := filepath.Join(p.pluginsDir, name+".so")
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("插件文件不存在: %s", path)
	}
	plug, err := goplugin.Open(path)
	if err != nil {
		return err
	}
	newSym, err := plug.Lookup("New")
	if err != nil {
		return fmt.Errorf("插件缺少 New 函数: %w", err)
	}
	var instance plugin.Plugin
	if newFunc, ok := newSym.(func() (plugin.Plugin, error)); ok {
		instance, err = newFunc()
		if err != nil {
			return err
		}
	} else if newFunc, ok := newSym.(func() interface{}); ok {
		instance, _ = newFunc().(plugin.Plugin)
	} else {
		return fmt.Errorf("New 符号类型错误")
	}
	if instance == nil {
		return fmt.Errorf("插件未实现 plugin.Plugin 接口")
	}
	if err := p.mgr.RegisterPlugin(instance); err != nil {
		return err
	}
	if err := instance.Init(ctx, p.mgr); err != nil {
		p.mgr.UnregisterPlugin(instance.Name())
		return err
	}
	if err := instance.Start(ctx); err != nil {
		p.mgr.UnregisterPlugin(instance.Name())
		return err
	}
	return nil
}

// unload stops and unregisters a loaded plugin.
func (p *TPMPlugin) unload(ctx context.Context, name string) error {
	plug, ok := p.mgr.GetPlugin(name)
	if !ok {
		return fmt.Errorf("插件 %s 未加载", name)
	}
	if err := plug.Stop(ctx); err != nil {
		return err
	}
	p.mgr.UnregisterPlugin(name)
	return nil
}

// isLoaded reports whether a plugin is currently loaded.
func (p *TPMPlugin) isLoaded(name string) bool {
	_, ok := p.mgr.GetPlugin(name)
	return ok
}

// installed lists .so files in the plugins dir.
func (p *TPMPlugin) installed() []string {
	entries, err := os.ReadDir(p.pluginsDir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".so") {
			names = append(names, strings.TrimSuffix(e.Name(), ".so"))
		}
	}
	return names
}

func (p *TPMPlugin) listPlugins(ctx *plugin.CommandContext) error {
	infos := p.mgr.GetAllInfo()

	var b strings.Builder
	b.WriteString(fmt.Sprintf("📦 <b>已加载插件 (%d)</b>\n\n", len(infos)))

	for _, info := range infos {
		status := "⏸️"
		if info.Status == plugin.StatusActive {
			status = "✅"
		}
		b.WriteString(fmt.Sprintf("%s <b>%s</b> — %s\n", status, info.Name, info.Description))
		b.WriteString("\n")
	}

	b.WriteString("💡 使用 <code>.tpm [install|remove|enable|disable]</code> 管理插件")
	return ctx.Edit(b.String())
}

func (p *TPMPlugin) showHelp(ctx *plugin.CommandContext) error {
	prefix := p.mgr.Commands().GetPrefix()
	return ctx.Edit(fmt.Sprintf(`📦 <b>TPM - Telegram插件管理器</b>

<b>用法:</b>
• <code>%stpm list</code> - 列出所有插件
• <code>%stpm install <插件></code> - 安装插件
• <code>%stpm remove <插件></code> - 卸载插件
• <code>%stpm update [插件]</code> - 更新插件
• <code>%stpm enable <插件></code> - 启用插件
• <code>%stpm disable <插件></code> - 禁用插件
• <code>%stpm search <关键词></code> - 搜索插件
• <code>%stpm info <插件></code> - 插件详情

<b>示例:</b>
• <code>%stpm install github.com/user/plugin</code>
• <code>%stpm enable ping</code>
• <code>%stpm update</code>

💡 <i>插件以 .so 文件形式存放在 plugins/ 目录</i>`, prefix, prefix, prefix, prefix, prefix, prefix, prefix, prefix, prefix))
}

func (p *TPMPlugin) Start(ctx context.Context) error { return nil }
func (p *TPMPlugin) Stop(ctx context.Context) error  { return nil }
