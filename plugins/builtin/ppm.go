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

// PPMPlugin — Plugin Package Manager.
// Full lifecycle: search, install, remove, load, unload, reload, list, info.
type PPMPlugin struct {
	loader *loader.Loader
	mgr    plugin.Manager
}

func NewPPM(pluginLoader *loader.Loader) *PPMPlugin {
	return &PPMPlugin{loader: pluginLoader}
}

func (p *PPMPlugin) Name() string        { return "ppm" }
func (p *PPMPlugin) Description() string { return "📦 插件包管理器（动态插拔）" }

func (p *PPMPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	p.mgr = mgr
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "ppm",
		Aliases:     []string{"plugin", "plugins", "pkg"},
		Description: "插件包管理器 — 动态安装/卸载/加载/管理插件",
		Usage:       "ppm <子命令> [参数...]",
		Plugin:      p.Name(),
		Category:    "core",
		OwnerOnly:   true,
		Handler:     p.handlePPM,
	})
}

func (p *PPMPlugin) Start(_ context.Context) error { return nil }
func (p *PPMPlugin) Stop(_ context.Context) error  { return nil }

func (p *PPMPlugin) handlePPM(ctx *interfaces.CommandContext) error {
	args := ctx.Args
	if len(args) == 0 {
		return p.showHelp(ctx)
	}

	sub := args[0]
	subArgs := args[1:]

	switch sub {
	case "help", "h", "?":
		return p.showHelp(ctx)

	case "list", "ls", "installed":
		return p.listInstalled(ctx)

	case "loaded", "active":
		return p.listLoaded(ctx)

	case "info", "status":
		if len(subArgs) == 0 {
			return ctx.Edit("用法: ppm info <插件名>")
		}
		return p.pluginInfo(ctx, subArgs[0])

	case "search", "find":
		return p.searchRegistry(ctx, subArgs)

	case "install", "add", "get":
		if len(subArgs) == 0 {
			return ctx.Edit("用法: ppm install <插件名> [插件名...]")
		}
		return p.installPlugins(ctx, subArgs)

	case "remove", "rm", "delete", "uninstall":
		if len(subArgs) == 0 {
			return ctx.Edit("用法: ppm remove <插件名> [插件名...]")
		}
		return p.removePlugins(ctx, subArgs)

	case "load", "enable":
		if len(subArgs) == 0 {
			return ctx.Edit("用法: ppm load <插件名> [插件名...]")
		}
		return p.loadPlugins(ctx, subArgs)

	case "unload", "disable":
		if len(subArgs) == 0 {
			return ctx.Edit("用法: ppm unload <插件名> [插件名...]")
		}
		return p.unloadPlugins(ctx, subArgs)

	case "reload", "restart", "refresh":
		if len(subArgs) == 0 {
			return ctx.Edit("用法: ppm reload <插件名> [插件名...]")
		}
		return p.reloadPlugins(ctx, subArgs)

	default:
		return ctx.Edit(fmt.Sprintf("❌ 未知子命令: %s\n\n%s", sub, p.helpText()))
	}
}

func (p *PPMPlugin) showHelp(ctx *interfaces.CommandContext) error {
	return ctx.Edit(p.helpText())
}

func (p *PPMPlugin) helpText() string {
	return `📦 <b>PPM — Plugin Package Manager</b>

<b>管理命令:</b>
• <code>ppm list</code> — 列出已安装的插件
• <code>ppm loaded</code> — 列出已加载的插件
• <code>ppm info &lt;name&gt;</code> — 查看插件详情

<b>安装与移除:</b>
• <code>ppm install &lt;name&gt;</code> — 下载并安装插件
• <code>ppm remove &lt;name&gt;</code> — 删除已安装的插件

<b>生命周期:</b>
• <code>ppm load &lt;name&gt;</code> — 加载插件（启动）
• <code>ppm unload &lt;name&gt;</code> — 卸载插件（停止）
• <code>ppm reload &lt;name&gt;</code> — 热重载插件

<b>示例:</b>
• <code>ppm list</code>
• <code>ppm install ping</code>
• <code>ppm load ping</code>
• <code>ppm unload ping</code>
• <code>ppm remove ping</code>`
}

func (p *PPMPlugin) listInstalled(ctx *interfaces.CommandContext) error {
	installed, err := p.loader.GetInstalled()
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 读取插件目录失败: %v", err))
	}

	loaded := p.loader.GetLoaded()

	if len(installed) == 0 && len(loaded) == 0 {
		return ctx.Edit("📦 暂无已安装的插件\n\n使用 <code>ppm install &lt;name&gt;</code> 安装插件")
	}

	// Collect all unique plugin names
	seen := make(map[string]bool)
	var allPlugins []string

	// Built-in plugins from manager
	for _, info := range p.mgr.GetAllInfo() {
		if !seen[info.Name] {
			allPlugins = append(allPlugins, info.Name)
			seen[info.Name] = true
		}
	}
	for _, name := range installed {
		if !seen[name] {
			allPlugins = append(allPlugins, name)
			seen[name] = true
		}
	}
	sort.Strings(allPlugins)

	var b strings.Builder
	b.WriteString("📦 <b>插件列表</b>\n\n")

	// Count by type
	var builtinCount, loadedCount int
	for _, name := range allPlugins {
		isLoaded := loaded[name] != nil
		_, isBuiltin := p.mgr.GetInfo(name)
		if isLoaded {
			loadedCount++
		}
		if isBuiltin && !isLoaded {
			builtinCount++
		}
	}

	b.WriteString(fmt.Sprintf("内建: <b>%d</b> | 外部: <b>%d</b> | 已加载: <b>%d</b>\n\n", builtinCount, len(installed), loadedCount))

	for _, name := range allPlugins {
		isLoaded := loaded[name] != nil
		_, isBuiltin := p.mgr.GetInfo(name)
		hasFile := false
		for _, n := range installed {
			if n == name {
				hasFile = true
				break
			}
		}

		icon := "📦"
		if isLoaded {
			icon = "✅"
		} else if hasFile {
			icon = "💾"
		}
		status := "未加载"
		if isLoaded {
			status = "🟢 已加载"
		} else if isBuiltin {
			status = "🔵 内建"
		} else if hasFile {
			status = "⚪ 未加载"
		}

		b.WriteString(fmt.Sprintf("%s <b>%s</b> — %s\n", icon, name, status))
	}

	return ctx.Edit(b.String())
}

func (p *PPMPlugin) listLoaded(ctx *interfaces.CommandContext) error {
	loaded := p.loader.GetLoaded()
	builtins := p.mgr.GetAllInfo()

	if len(loaded) == 0 && len(builtins) == 0 {
		return ctx.Edit("无已加载的插件")
	}

	var b strings.Builder
	b.WriteString("✅ <b>已加载插件</b>\n\n")

	// Built-in plugins
	var builtinNames []string
	seen := make(map[string]bool)
	for _, info := range builtins {
		if info.Status == plugin.StatusActive {
			builtinNames = append(builtinNames, info.Name)
			seen[info.Name] = true
		}
	}
	sort.Strings(builtinNames)

	if len(builtinNames) > 0 {
		b.WriteString("<b>🔵 内建插件:</b>\n")
		for _, name := range builtinNames {
			info, _ := p.mgr.GetInfo(name)
			b.WriteString(fmt.Sprintf("  • <code>%s</code> — %s\n", name, info.Description))
		}
		b.WriteString("\n")
	}

	// External plugins
	if len(loaded) > 0 {
		b.WriteString("<b>🔌 外部插件:</b>\n")
		var names []string
		for name := range loaded {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			entry := loaded[name]
			loadTime := entry.LoadedAt.Format("15:04:05")
			meta := ""
			if entry.Metadata != nil {
				meta = fmt.Sprintf(" v%s", entry.Metadata.Version)
			}
			b.WriteString(fmt.Sprintf("  • <code>%s</code>%s — %s\n", name, meta, loadTime))
		}
	}

	return ctx.Edit(b.String())
}

func (p *PPMPlugin) pluginInfo(ctx *interfaces.CommandContext, name string) error {
	// Check loaded
	loaded := p.loader.GetLoaded()
	if entry, ok := loaded[name]; ok {
		meta := ""
		if entry.Metadata != nil {
			meta = fmt.Sprintf(`📌 <b>版本:</b> %s
👤 <b>作者:</b> %s
📅 <b>加载时间:</b> %s
📁 <b>路径:</b> <code>%s</code>`,
				entry.Metadata.Version, entry.Metadata.Author,
				entry.LoadedAt.Format("2006-01-02 15:04:05"),
				entry.Path)
		}
		cmds := p.mgr.Commands().GetByPlugin(name)
		cmdList := ""
		if len(cmds) > 0 {
			var names []string
			for n := range cmds {
				names = append(names, n)
			}
			sort.Strings(names)
			cmdList = "\n<b>命令:</b> " + strings.Join(names, ", ")
		}
		return ctx.Edit(fmt.Sprintf("✅ <b>%s</b> 🟢 已加载\n%s%s", name, meta, cmdList))
	}

	// Check built-in
	if info, ok := p.mgr.GetInfo(name); ok {
		cmds := p.mgr.Commands().GetByPlugin(name)
		var cmdList []string
		for n := range cmds {
			cmdList = append(cmdList, n)
		}
		sort.Strings(cmdList)
		statusStr := "🔵 内建"
		if info.Status == plugin.StatusActive {
			statusStr = "✅ 活跃"
		}
		return ctx.Edit(fmt.Sprintf("📦 <b>%s</b> %s\n%s\n命令: %s", name, statusStr, info.Description, strings.Join(cmdList, ", ")))
	}

	// Check installed but not loaded
	installed, _ := p.loader.GetInstalled()
	for _, n := range installed {
		if n == name {
			return ctx.Edit(fmt.Sprintf("💾 <b>%s</b> ⚪ 已安装但未加载\n\n使用 <code>ppm load %s</code> 加载", name, name))
		}
	}

	return ctx.Edit(fmt.Sprintf("❌ 未找到插件: %s\n\n使用 <code>ppm list</code> 查看已安装的插件", name))
}

func (p *PPMPlugin) searchRegistry(ctx *interfaces.CommandContext, args []string) error {
	_ = ctx.Edit("⏳ 正在查询插件注册表...")

	// Real external plugins available from PaperValet-Plugins registry
	knownPlugins := []struct {
		name        string
		description string
		version     string
	}{
		{"ping", "网络延迟测试工具 (TCP/HTTP/ICMP/DC)", "1.0.0"},
		{"leech", "媒体下载工具 (yt-dlp)", "1.0.0"},
		{"qrcode", "二维码生成与解码", "1.0.0"},
		{"bf", "Brainfuck 解释器", "1.0.0"},
		{"re", "消息复读机", "1.0.0"},
		{"sendlog", "日志发送工具", "1.0.0"},
		{"tpm", "Telegram 插件管理器 (旧版)", "1.0.0"},
		// New plugins from TeleBox-Plugins migration
		{"atadmins", "一键艾特全部管理员", "1.0.0"},
		{"ids", "显示用户/群组/消息 ID 及跳转链接", "1.0.0"},
		{"isalive", "活了么 - 检测 bot 是否在线", "1.0.0"},
		{"calc", "计算器 - 支持基本数学运算", "1.0.0"},
		{"encode", "编码/解码工具 (base64/url/hex)", "1.0.0"},
		{"hitokoto", "获取随机一言", "1.0.0"},
		{"qr", "二维码生成", "1.0.0"},
		{"rev", "反转消息内容", "1.0.0"},
		{"sendat", "定时消息发送", "1.0.0"},
		{"gt", "谷歌翻译", "1.0.0"},
		{"bizhi", "随机壁纸", "1.0.0"},
		{"weather", "天气查询", "1.0.0"},
		{"speedtest", "网络速度测试", "1.0.0"},
		{"duckduckgo", "DuckDuckGo 搜索", "1.0.0"},
	}

	installed, _ := p.loader.GetInstalled()
	installedSet := make(map[string]bool)
	for _, n := range installed {
		installedSet[n] = true
	}

	query := ""
	if len(args) > 0 {
		query = strings.ToLower(strings.Join(args, " "))
	}

	var b strings.Builder
	if query != "" {
		b.WriteString(fmt.Sprintf("🔍 <b>搜索: %s</b>\n\n", query))
	} else {
		b.WriteString("📦 <b>插件注册表</b>\n\n")
		b.WriteString("使用 <code>ppm install <name></code> 安装\n\n")
	}

	var found int
	for _, plug := range knownPlugins {
		if query != "" && !strings.Contains(strings.ToLower(plug.name), query) &&
			!strings.Contains(strings.ToLower(plug.description), query) {
			continue
		}
		status := "📦"
		if installedSet[plug.name] {
			status = "✅"
		}
		b.WriteString(fmt.Sprintf("%s <b>%s</b> v%s\n", status, plug.name, plug.version))
		b.WriteString(fmt.Sprintf("   %s\n", plug.description))
		found++
	}

	if found == 0 {
		b.WriteString("未找到匹配的插件\n")
	}

	b.WriteString(fmt.Sprintf("\n共 <b>%d</b> 个插件", found))

	return ctx.Edit(b.String())
}

func (p *PPMPlugin) installPlugins(ctx *interfaces.CommandContext, names []string) error {
	_ = ctx.Edit(fmt.Sprintf("⏳ 正在安装 %d 个插件...", len(names)))

	var results []string
	for _, name := range names {
		if err := p.loader.Install(ctx.Context(), name); err != nil {
			results = append(results, fmt.Sprintf("❌ <b>%s</b> 安装失败: %v", name, err))
			continue
		}
		results = append(results, fmt.Sprintf("✅ <b>%s</b> 安装成功", name))
	}

	return ctx.Edit(fmt.Sprintf("📦 <b>安装结果</b>\n\n%s", strings.Join(results, "\n")))
}

func (p *PPMPlugin) removePlugins(ctx *interfaces.CommandContext, names []string) error {
	_ = ctx.Edit(fmt.Sprintf("⏳ 正在移除 %d 个插件...", len(names)))

	var results []string
	for _, name := range names {
		if err := p.loader.Remove(ctx.Context(), name); err != nil {
			results = append(results, fmt.Sprintf("❌ <b>%s</b> 移除失败: %v", name, err))
			continue
		}
		results = append(results, fmt.Sprintf("🗑 <b>%s</b> 已移除", name))
	}

	return ctx.Edit(fmt.Sprintf("🗑 <b>移除结果</b>\n\n%s", strings.Join(results, "\n")))
}

func (p *PPMPlugin) loadPlugins(ctx *interfaces.CommandContext, names []string) error {
	_ = ctx.Edit(fmt.Sprintf("⏳ 正在加载 %d 个插件...", len(names)))

	var results []string
	for _, name := range names {
		if p.loader.IsLoaded(name) {
			results = append(results, fmt.Sprintf("⚪ <b>%s</b> 已加载", name))
			continue
		}
		if err := p.loader.LoadByName(ctx.Context(), name); err != nil {
			results = append(results, fmt.Sprintf("❌ <b>%s</b> 加载失败: %v", name, err))
			continue
		}
		results = append(results, fmt.Sprintf("✅ <b>%s</b> 已加载", name))
	}

	return ctx.Edit(fmt.Sprintf("🔌 <b>加载结果</b>\n\n%s", strings.Join(results, "\n")))
}

func (p *PPMPlugin) unloadPlugins(ctx *interfaces.CommandContext, names []string) error {
	_ = ctx.Edit(fmt.Sprintf("⏳ 正在卸载 %d 个插件...", len(names)))

	var results []string
	for _, name := range names {
		if !p.loader.IsLoaded(name) {
			results = append(results, fmt.Sprintf("⚪ <b>%s</b> 未加载", name))
			continue
		}
		if err := p.loader.Unload(ctx.Context(), name); err != nil {
			results = append(results, fmt.Sprintf("❌ <b>%s</b> 卸载失败: %v", name, err))
			continue
		}
		results = append(results, fmt.Sprintf("⏹ <b>%s</b> 已卸载", name))
	}

	return ctx.Edit(fmt.Sprintf("🔌 <b>卸载结果</b>\n\n%s", strings.Join(results, "\n")))
}

func (p *PPMPlugin) reloadPlugins(ctx *interfaces.CommandContext, names []string) error {
	_ = ctx.Edit(fmt.Sprintf("🔄 正在重载 %d 个插件...", len(names)))

	var results []string
	for _, name := range names {
		// Unload if loaded
		if p.loader.IsLoaded(name) {
			if err := p.loader.Unload(ctx.Context(), name); err != nil {
				results = append(results, fmt.Sprintf("❌ <b>%s</b> 卸载失败: %v", name, err))
				continue
			}
		}

		// Reload: unload old + load fresh
		if err := p.loader.LoadByName(ctx.Context(), name); err != nil {
			results = append(results, fmt.Sprintf("❌ <b>%s</b> 重载失败: %v", name, err))
			continue
		}
		results = append(results, fmt.Sprintf("🔄 <b>%s</b> 已重载", name))
	}

	return ctx.Edit(fmt.Sprintf("🔄 <b>重载结果</b>\n\n%s", strings.Join(results, "\n")))
}