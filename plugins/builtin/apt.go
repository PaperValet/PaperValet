package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/internal/plugin/loader"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// registryEntry is one plugin in the external plugin repository.
type registryEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	DescEN      string `json:"desc_en,omitempty"`
	Version     string `json:"version"`
}

// AptPlugin manages external plugins: list/search/install/remove/load.
type AptPlugin struct {
	loader *loader.Loader
	mgr    plugin.Manager
}

func NewApt(pluginLoader *loader.Loader) *AptPlugin {
	return &AptPlugin{loader: pluginLoader}
}

func (p *AptPlugin) Name() string        { return "apt" }
func (p *AptPlugin) Description() string { return "外部插件管理：搜索、安装、加载" }
func (p *AptPlugin) DescEN() string      { return "Manage external plugins: search, install, load" }

func (p *AptPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	p.mgr = mgr
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "apt",
		Aliases:     []string{"plugin", "plugins", "pkg"},
		Description: "插件管理：<code>search</code> 搜仓库、<code>list</code> 已装、<code>install/remove/load/unload</code>",
		DescEN:      "Plugins: <code>search</code> the repo, <code>list</code> installed, <code>install/remove/load/unload</code>",
		Usage:       "apt search [词] | list | install <名> | remove <名> | load <名> | unload <名>",
		UsageEN:     "apt search [term] | list | install <name> | remove <name> | load <name> | unload <name>",
		Plugin:      p.Name(),
		Category:    "core",
		OwnerOnly:   true,
		Handler:     p.handleApt,
	})
}

func (p *AptPlugin) Start(_ context.Context) error { return nil }
func (p *AptPlugin) Stop(_ context.Context) error  { return nil }

func (p *AptPlugin) help(ctx *interfaces.CommandContext) error {
	return ctx.Edit(ctx.Tlocal(
		`📦 <b>apt 插件管理</b>

<code>apt search [关键词]</code>  搜插件仓库（不带词列出全部）
<code>apt list</code>  已安装和已加载的插件
<code>apt install 名字</code>  从仓库安装
<code>apt load 名字</code> · <code>apt unload 名字</code>  加载/停用
<code>apt remove 名字</code>  删除文件
<code>reload</code>  重新加载全部外部插件`,
		`📦 <b>apt plugin manager</b>

<code>apt search [term]</code>  search the repository (bare search lists all)
<code>apt list</code>  installed and loaded plugins
<code>apt install name</code>  install from the repository
<code>apt load name</code> · <code>apt unload name</code>  start/stop
<code>apt remove name</code>  delete the file
<code>reload</code>  reload every external plugin`))
}

func (p *AptPlugin) handleApt(ctx *interfaces.CommandContext) error {
	if ctx.ArgCount() == 0 {
		return p.help(ctx)
	}
	sub, args := ctx.GetArg(0), ctx.Args[1:]
	switch sub {
	case "help", "h", "?":
		return p.help(ctx)
	case "list", "ls", "installed":
		return p.listInstalled(ctx)
	case "loaded", "active":
		return p.listLoaded(ctx)
	case "search", "find", "repo":
		return p.search(ctx, args)
	case "info":
		if len(args) == 0 {
			return ctx.Edit(ctx.Tlocal("用法: <code>apt info 名字</code>", "Usage: <code>apt info name</code>"))
		}
		return p.pluginInfo(ctx, args[0])
	case "install", "add", "get":
		if len(args) == 0 {
			return ctx.Edit(ctx.Tlocal("用法: <code>apt install 名字</code>，先 <code>apt search</code> 找名字", "Usage: <code>apt install name</code>; find names with <code>apt search</code>"))
		}
		return p.install(ctx, args)
	case "remove", "rm", "delete", "uninstall":
		if len(args) == 0 {
			return ctx.Edit(ctx.Tlocal("用法: <code>apt remove 名字</code>", "Usage: <code>apt remove name</code>"))
		}
		return p.remove(ctx, args)
	case "load", "enable":
		if len(args) == 0 {
			return ctx.Edit(ctx.Tlocal("用法: <code>apt load 名字</code>", "Usage: <code>apt load name</code>"))
		}
		return p.load(ctx, args)
	case "unload", "disable":
		if len(args) == 0 {
			return ctx.Edit(ctx.Tlocal("用法: <code>apt unload 名字</code>", "Usage: <code>apt unload name</code>"))
		}
		return p.unload(ctx, args)
	default:
		return p.help(ctx)
	}
}

// fetchRegistry downloads the live plugin index from the release repo.
func (p *AptPlugin) fetchRegistry(ctx *interfaces.CommandContext) ([]registryEntry, error) {
	url := strings.TrimSuffix(p.loader.RepoURL(), "/") + "/plugins.json"
	req, err := http.NewRequestWithContext(ctx.Context(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var entries []registryEntry
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func (p *AptPlugin) search(ctx *interfaces.CommandContext, args []string) error {
	_ = ctx.Edit("⏳ …")
	entries, err := p.fetchRegistry(ctx)
	if err != nil {
		return ctx.Edit(ctx.Tlocal(
			"❌ 仓库清单拉取失败: "+err.Error()+"\n稍后再试，或直接 <code>apt install 名字</code>",
			"❌ Failed to fetch the registry: "+err.Error()+"\nTry later, or <code>apt install name</code> directly"))
	}
	query := strings.ToLower(strings.Join(args, " "))

	installed, _ := p.loader.GetInstalled()
	installedSet := map[string]bool{}
	for _, n := range installed {
		installedSet[n] = true
	}
	loaded := p.loader.GetLoaded()

	var b strings.Builder
	if query != "" {
		fmt.Fprintf(&b, "🔍 %s <b>%s</b>\n\n", ctx.Tlocal("搜索", "Search"), strings.Join(args, " "))
	} else {
		fmt.Fprintf(&b, "📦 %s (%d)\n\n", ctx.Tlocal("插件仓库", "Plugin repository"), len(entries))
	}
	found := 0
	for _, e := range entries {
		if query != "" &&
			!strings.Contains(strings.ToLower(e.Name), query) &&
			!strings.Contains(strings.ToLower(e.Description), query) &&
			!strings.Contains(strings.ToLower(e.DescEN), query) {
			continue
		}
		found++
		icon := "⬜"
		if loaded[e.Name] != nil {
			icon = "✅"
		} else if installedSet[e.Name] {
			icon = "📦"
		}
		desc := e.Description
		if ctx.Lang == "en-US" && e.DescEN != "" {
			desc = e.DescEN
		}
		fmt.Fprintf(&b, "%s <b>%s</b>", icon, e.Name)
		if e.Version != "" {
			fmt.Fprintf(&b, " <i>v%s</i>", e.Version)
		}
		b.WriteString("\n    " + desc + "\n")
		if found >= 40 && query == "" {
			fmt.Fprintf(&b, "\n… (%d more)\n", len(entries)-found)
			break
		}
	}
	if found == 0 {
		b.WriteString(ctx.Tlocal("没有匹配的插件\n", "No matching plugins\n"))
	}
	b.WriteString("\n" + ctx.Tlocal(
		"安装: <code>apt install 名字</code>（✅ 已加载 · 📦 已装未载 · ⬜ 未装）",
		"Install: <code>apt install name</code> (✅ loaded · 📦 installed · ⬜ available)"))
	return ctx.Edit(b.String())
}

func (p *AptPlugin) listInstalled(ctx *interfaces.CommandContext) error {
	installed, err := p.loader.GetInstalled()
	if err != nil {
		return ctx.Edit("❌ " + err.Error())
	}
	loaded := p.loader.GetLoaded()

	var b strings.Builder
	b.WriteString("📦 " + ctx.Tlocal("已安装插件", "Installed plugins") + "\n\n")
	if len(installed) == 0 {
		b.WriteString(ctx.Tlocal("还没有安装外部插件。", "No external plugins installed yet.\n") + ctx.Tlocal("用 <code>apt search</code> 看仓库", "Browse with <code>apt search</code>"))
		return ctx.Edit(b.String())
	}
	sort.Strings(installed)
	for _, name := range installed {
		icon := "⚪"
		note := ctx.Tlocal("未加载", "not loaded")
		if loaded[name] != nil {
			icon = "🟢"
			note = ctx.Tlocal("已加载", "loaded")
		}
		fmt.Fprintf(&b, "%s <b>%s</b> — %s\n", icon, name, note)
	}
	fmt.Fprintf(&b, "\n⚙️ %s: %d · %s: %d\n", ctx.Tlocal("内建", "built-in"), len(p.mgr.GetAllInfo())-len(loaded),
		ctx.Tlocal("外部已加载", "external loaded"), len(loaded))
	return ctx.Edit(b.String())
}

func (p *AptPlugin) listLoaded(ctx *interfaces.CommandContext) error {
	loaded := p.loader.GetLoaded()
	var names []string
	for name := range loaded {
		names = append(names, name)
	}
	sort.Strings(names)
	var b strings.Builder
	fmt.Fprintf(&b, "🟢 %s (%d)\n\n", ctx.Tlocal("已加载插件", "Loaded plugins"), len(p.mgr.GetAllInfo()))
	for _, info := range p.mgr.GetAllInfo() {
		if info.Status != plugin.StatusActive {
			continue
		}
		desc := info.Description
		if ctx.Lang == "en-US" && info.DescEN != "" {
			desc = info.DescEN
		}
		fmt.Fprintf(&b, "• <b>%s</b> — %s\n", info.Name, desc)
	}
	return ctx.Edit(b.String())
}

func (p *AptPlugin) pluginInfo(ctx *interfaces.CommandContext, name string) error {
	if entry, ok := p.loader.GetLoaded()[name]; ok {
		meta := ""
		if entry.Metadata != nil {
			desc := entry.Metadata.Description
			if ctx.Lang == "en-US" && entry.Metadata.DescEN != "" {
				desc = entry.Metadata.DescEN
			}
			meta = fmt.Sprintf("\n%s\nv%s · %s", desc, entry.Metadata.Version, entry.Metadata.Author)
		}
		cmds := p.mgr.Commands().GetByPlugin(name)
		var cmdNames []string
		for n := range cmds {
			cmdNames = append(cmdNames, n)
		}
		sort.Strings(cmdNames)
		cmdList := strings.Join(cmdNames, ", ")
		if cmdList == "" {
			cmdList = "—"
		}
		return ctx.Edit(fmt.Sprintf("🔌 <b>%s</b> 🟢%s\n%s: <code>%s</code>",
			name, meta, ctx.Tlocal("命令", "Commands"), cmdList))
	}
	if info, ok := p.mgr.GetInfo(name); ok {
		desc := info.Description
		if ctx.Lang == "en-US" && info.DescEN != "" {
			desc = info.DescEN
		}
		return ctx.Edit(fmt.Sprintf("🔵 <b>%s</b> %s\n%s", name, ctx.Tlocal("内建", "built-in"), desc))
	}
	installed, _ := p.loader.GetInstalled()
	for _, n := range installed {
		if n == name {
			return ctx.Edit(fmt.Sprintf("📦 <b>%s</b> ⚪ %s\n<code>apt load %s</code>", name, ctx.Tlocal("已装未载", "installed, not loaded"), name))
		}
	}
	return ctx.Edit(ctx.Tlocal(fmt.Sprintf("没找到 %s。用 apt search 查仓库", name), fmt.Sprintf("%s not found; try apt search", name)))
}

func (p *AptPlugin) install(ctx *interfaces.CommandContext, names []string) error {
	_ = ctx.Edit(fmt.Sprintf("⏳ %s %d…", ctx.Tlocal("安装", "Installing"), len(names)))
	var results []string
	for _, name := range names {
		if err := p.loader.Install(ctx.Context(), name); err != nil {
			results = append(results, fmt.Sprintf("❌ <b>%s</b> %v", name, err))
			continue
		}
		if err := p.loader.LoadByName(ctx.Context(), name); err != nil {
			results = append(results, fmt.Sprintf("📦 <b>%s</b> %s: %v", name, ctx.Tlocal("已下载但加载失败", "downloaded, load failed"), err))
			continue
		}
		results = append(results, fmt.Sprintf("✅ <b>%s</b> %s", name, ctx.Tlocal("已装并加载", "installed and loaded")))
	}
	return ctx.Edit(strings.Join(results, "\n"))
}

func (p *AptPlugin) remove(ctx *interfaces.CommandContext, names []string) error {
	var results []string
	for _, name := range names {
		if err := p.loader.Remove(ctx.Context(), name); err != nil {
			results = append(results, fmt.Sprintf("❌ <b>%s</b> %v", name, err))
		} else {
			results = append(results, fmt.Sprintf("🗑 <b>%s</b> %s", name, ctx.Tlocal("已移除", "removed")))
		}
	}
	return ctx.Edit(strings.Join(results, "\n"))
}

func (p *AptPlugin) load(ctx *interfaces.CommandContext, names []string) error {
	var results []string
	for _, name := range names {
		if p.loader.IsLoaded(name) {
			results = append(results, fmt.Sprintf("⚪ <b>%s</b> %s", name, ctx.Tlocal("本来就加载着", "already loaded")))
			continue
		}
		if err := p.loader.LoadByName(ctx.Context(), name); err != nil {
			results = append(results, fmt.Sprintf("❌ <b>%s</b> %v", name, err))
			continue
		}
		results = append(results, fmt.Sprintf("✅ <b>%s</b> %s", name, ctx.Tlocal("已加载", "loaded")))
	}
	return ctx.Edit(strings.Join(results, "\n"))
}

func (p *AptPlugin) unload(ctx *interfaces.CommandContext, names []string) error {
	var results []string
	for _, name := range names {
		if !p.loader.IsLoaded(name) {
			results = append(results, fmt.Sprintf("⚪ <b>%s</b> %s", name, ctx.Tlocal("本来就没加载", "not loaded")))
			continue
		}
		if err := p.loader.Unload(ctx.Context(), name); err != nil {
			results = append(results, fmt.Sprintf("❌ <b>%s</b> %v", name, err))
			continue
		}
		results = append(results, fmt.Sprintf("⏹ <b>%s</b> %s", name, ctx.Tlocal("已停用", "unloaded")))
	}
	return ctx.Edit(strings.Join(results, "\n"))
}
