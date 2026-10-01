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

// AptPlugin manages external plugins. Installed always means loaded.
type AptPlugin struct {
	loader *loader.Loader
	mgr    plugin.Manager
}

func NewApt(pluginLoader *loader.Loader) *AptPlugin {
	return &AptPlugin{loader: pluginLoader}
}

func (p *AptPlugin) Name() string        { return "apt" }
func (p *AptPlugin) Description() string { return "外部插件管理" }
func (p *AptPlugin) DescEN() string      { return "External plugin manager" }

func (p *AptPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	p.mgr = mgr
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "apt",
		Description: "外部插件管理",
		DescEN:      "External plugin manager",
		Usage: `apt &lt;子命令&gt; [参数]

<b>子命令</b>
• <code>search</code> · <code>s</code>  [关键词]  搜索插件仓库，不带词列出全部
• <code>install</code> · <code>i</code>  &lt;名字…&gt;  安装并立即启用
• <code>remove</code> · <code>rm</code>  &lt;名字…&gt;  停用并删除
• <code>list</code> · <code>ls</code>  已安装的插件
• <code>info</code>  &lt;名字&gt;  版本、作者、提供的命令

<b>示例</b>
• <code>apt s</code>
• <code>apt i weather calc</code>
• <code>apt rm weather</code>

<b>机制</b>
• 安装即启用，卸载即删除，没有「装了但没启用」的状态
• 插件清单实时从 PaperValet-Plugins 的 Release 拉取
• 插件是 plugins/ 下的 .so 文件，启动时自动加载
• 插件必须和主程序用同一版本 Go 编译，否则会提示版本不符，此时运行 <code>update</code>`,
		UsageEN: `apt &lt;subcommand&gt; [args]

<b>Subcommands</b>
• <code>search</code> · <code>s</code>  [term]  search the repository; bare lists all
• <code>install</code> · <code>i</code>  &lt;name…&gt;  install and enable
• <code>remove</code> · <code>rm</code>  &lt;name…&gt;  disable and delete
• <code>list</code> · <code>ls</code>  installed plugins
• <code>info</code>  &lt;name&gt;  version, author, commands

<b>Examples</b>
• <code>apt s</code>
• <code>apt i weather calc</code>
• <code>apt rm weather</code>

<b>How it works</b>
• Installed means enabled, removed means deleted; there is no "installed but off"
• The index is fetched live from the PaperValet-Plugins release
• Plugins are .so files in plugins/, loaded on startup
• Plugins must be built with the same Go version as the bot; on a mismatch run <code>update</code>`,
		Plugin:    p.Name(),
		Category:  "core",
		OwnerOnly: true,
		Handler:   p.handleApt,
	})
}

func (p *AptPlugin) Start(_ context.Context) error { return nil }
func (p *AptPlugin) Stop(_ context.Context) error  { return nil }

// aptSubcommands maps every accepted spelling to its canonical action.
var aptSubcommands = map[string]string{
	"search": "search", "s": "search", "find": "search",
	"install": "install", "i": "install", "in": "install", "add": "install",
	"remove": "remove", "rm": "remove", "r": "remove", "uninstall": "remove", "del": "remove",
	"list": "list", "ls": "list", "l": "list",
	"info": "info", "show": "info",
	"help": "help", "h": "help",
}

func (p *AptPlugin) help(ctx *interfaces.CommandContext) error {
	prefix := p.mgr.Commands().GetPrefix()
	c := newCard("📦", ctx.Tlocal("apt 插件管理", "apt plugins"))
	c.blank()
	c.line(cmdRef(prefix+"apt s") + "  " + ctx.Tlocal("搜索仓库", "search"))
	c.line(cmdRef(prefix+"apt i 名字") + "  " + ctx.Tlocal("安装", "install"))
	c.line(cmdRef(prefix+"apt rm 名字") + "  " + ctx.Tlocal("卸载", "remove"))
	c.line(cmdRef(prefix+"apt ls") + "  " + ctx.Tlocal("已安装", "installed"))
	c.line(cmdRef(prefix+"apt info 名字") + "  " + ctx.Tlocal("详情", "details"))
	c.hint(ctx.Tlocal("完整说明 ", "Full guide ") + cmdRef(prefix+"help apt"))
	return ctx.Edit(c.String())
}

func (p *AptPlugin) handleApt(ctx *interfaces.CommandContext) error {
	if ctx.ArgCount() == 0 {
		return p.help(ctx)
	}
	sub, ok := aptSubcommands[strings.ToLower(ctx.GetArg(0))]
	args := ctx.Args[1:]
	if !ok {
		return p.help(ctx)
	}
	need := func(usage string) error {
		return ctx.Edit(ctx.Tlocal("用法 ", "Usage ") + cmdRef(p.mgr.Commands().GetPrefix()+usage))
	}
	switch sub {
	case "search":
		return p.search(ctx, args)
	case "install":
		if len(args) == 0 {
			return need("apt i <name>")
		}
		return p.install(ctx, args)
	case "remove":
		if len(args) == 0 {
			return need("apt rm <name>")
		}
		return p.remove(ctx, args)
	case "list":
		return p.list(ctx)
	case "info":
		if len(args) == 0 {
			return need("apt info <name>")
		}
		return p.info(ctx, args[0])
	}
	return p.help(ctx)
}

// fetchRegistry downloads the live plugin index from the release repo.
func (p *AptPlugin) fetchRegistry(ctx *interfaces.CommandContext) ([]registryEntry, error) {
	url := strings.TrimSuffix(p.loader.RepoURL(), "/") + "/plugins.json"
	req, err := http.NewRequestWithContext(ctx.Context(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
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

func entryDesc(ctx *interfaces.CommandContext, e registryEntry) string {
	if ctx.Lang == "en-US" && e.DescEN != "" {
		return e.DescEN
	}
	return e.Description
}

func (p *AptPlugin) search(ctx *interfaces.CommandContext, args []string) error {
	_ = ctx.Edit("🔍 …")
	entries, err := p.fetchRegistry(ctx)
	if err != nil {
		return ctx.Edit(errText(ctx.Tlocal("拉取插件清单失败：", "Could not fetch the index: ") + htmlEscape(err.Error())))
	}
	query := strings.ToLower(strings.Join(args, " "))
	loaded := p.loader.GetLoaded()

	title := ctx.Tlocal(fmt.Sprintf("插件仓库 · %d 个", len(entries)), fmt.Sprintf("Repository · %d plugins", len(entries)))
	if query != "" {
		title = ctx.Tlocal("搜索 ", "Search ") + htmlEscape(strings.Join(args, " "))
	}
	c := newCard("📦", title).blank()
	found := 0
	for _, e := range entries {
		desc := entryDesc(ctx, e)
		if query != "" && !strings.Contains(strings.ToLower(e.Name+" "+e.Description+" "+e.DescEN), query) {
			continue
		}
		found++
		mark := "▫️"
		if loaded[e.Name] != nil {
			mark = "✅"
		}
		c.line(fmt.Sprintf("%s <code>%s</code>  %s", mark, e.Name, htmlEscape(desc)))
	}
	if found == 0 {
		c.line(ctx.Tlocal("没有匹配的插件", "No matching plugins"))
	}
	prefix := p.mgr.Commands().GetPrefix()
	c.hint(ctx.Tlocal("✅ 已安装 · 安装用 ", "✅ installed · install with ") + cmdRef(prefix+"apt i 名字"))
	return ctx.Edit(c.String())
}

func (p *AptPlugin) list(ctx *interfaces.CommandContext) error {
	loaded := p.loader.GetLoaded()
	prefix := p.mgr.Commands().GetPrefix()
	if len(loaded) == 0 {
		c := newCard("📦", ctx.Tlocal("已安装插件", "Installed plugins"))
		c.blank().line(ctx.Tlocal("还没有安装外部插件", "No external plugins yet"))
		c.hint(ctx.Tlocal("去仓库看看 ", "Browse ") + cmdRef(prefix+"apt s"))
		return ctx.Edit(c.String())
	}
	names := make([]string, 0, len(loaded))
	for n := range loaded {
		names = append(names, n)
	}
	sort.Strings(names)
	c := newCard("📦", ctx.Tlocal(fmt.Sprintf("已安装插件 · %d 个", len(names)), fmt.Sprintf("Installed · %d", len(names)))).blank()
	for _, n := range names {
		e := loaded[n]
		desc, ver := "", ""
		if e.Metadata != nil {
			desc = e.Metadata.Description
			if ctx.Lang == "en-US" && e.Metadata.DescEN != "" {
				desc = e.Metadata.DescEN
			}
			ver = " <i>v" + htmlEscape(e.Metadata.Version) + "</i>"
		}
		c.line(fmt.Sprintf("✅ <code>%s</code>%s  %s", n, ver, htmlEscape(desc)))
	}
	c.hint(ctx.Tlocal("卸载用 ", "Remove with ") + cmdRef(prefix+"apt rm 名字"))
	return ctx.Edit(c.String())
}

func (p *AptPlugin) info(ctx *interfaces.CommandContext, name string) error {
	prefix := p.mgr.Commands().GetPrefix()
	commands := func(plugin string) string {
		var out []string
		for n := range p.mgr.Commands().GetByPlugin(plugin) {
			out = append(out, cmdRef(prefix+n))
		}
		sort.Strings(out)
		if len(out) == 0 {
			return "—"
		}
		return strings.Join(out, " ")
	}
	if e, ok := p.loader.GetLoaded()[name]; ok {
		c := newCard("🔌", htmlEscape(name))
		if e.Metadata != nil {
			desc := e.Metadata.Description
			if ctx.Lang == "en-US" && e.Metadata.DescEN != "" {
				desc = e.Metadata.DescEN
			}
			c.line(htmlEscape(desc)).blank()
			c.field(ctx.Tlocal("版本", "Version"), e.Metadata.Version)
			c.field(ctx.Tlocal("作者", "Author"), e.Metadata.Author)
		} else {
			c.blank()
		}
		c.field(ctx.Tlocal("安装于", "Loaded"), e.LoadedAt.Format("2006-01-02 15:04"))
		c.rawField(ctx.Tlocal("命令", "Commands"), commands(name))
		return ctx.Edit(c.String())
	}
	if info, ok := p.mgr.GetInfo(name); ok {
		c := newCard("🔵", htmlEscape(name)+ctx.Tlocal(" · 内建", " · built-in"))
		c.line(htmlEscape(pluginDesc(ctx, info))).blank()
		c.rawField(ctx.Tlocal("命令", "Commands"), commands(name))
		return ctx.Edit(c.String())
	}
	return ctx.Edit(errText(ctx.Tlocal("没装 ", "Not installed: ") + cmdRef(name) +
		ctx.Tlocal("，用 ", ". Try ") + cmdRef(prefix+"apt s "+name)))
}

// loadError turns Go plugin loader errors into an actionable message.
func loadError(ctx *interfaces.CommandContext, err error) string {
	msg := err.Error()
	if strings.Contains(msg, "different version of package") {
		return ctx.Tlocal("和主程序的 Go 版本不一致，先 <code>update now</code> 升级主程序再装",
			"built with a different Go version; run <code>update now</code> first")
	}
	if strings.Contains(msg, "status 404") || strings.Contains(msg, "may not exist") {
		return ctx.Tlocal("仓库里没有这个插件", "not in the repository")
	}
	return htmlEscape(msg)
}

func (p *AptPlugin) install(ctx *interfaces.CommandContext, names []string) error {
	_ = ctx.Edit("⏳ " + ctx.Tlocal("安装中…", "Installing…"))
	var rows []string
	for _, name := range names {
		if p.loader.IsLoaded(name) {
			rows = append(rows, skipLine(name, ctx.Tlocal("已经装过了", "already installed")))
			continue
		}
		if err := p.loader.Install(ctx.Context(), name); err != nil && !strings.Contains(err.Error(), "already installed") {
			rows = append(rows, failLine(name, loadError(ctx, err)))
			continue
		}
		if err := p.loader.LoadByName(ctx.Context(), name); err != nil {
			// Installed means loaded: never leave a dead file behind.
			_ = p.loader.Remove(ctx.Context(), name)
			rows = append(rows, failLine(name, loadError(ctx, err)))
			continue
		}
		cmds := p.mgr.Commands().GetByPlugin(name)
		var refs []string
		for n := range cmds {
			refs = append(refs, cmdRef(p.mgr.Commands().GetPrefix()+n))
		}
		sort.Strings(refs)
		rows = append(rows, okLine(name, strings.Join(refs, " ")))
	}
	return ctx.Edit(strings.Join(rows, "\n"))
}

func (p *AptPlugin) remove(ctx *interfaces.CommandContext, names []string) error {
	var rows []string
	for _, name := range names {
		if _, builtin := p.mgr.GetInfo(name); builtin && !p.loader.IsLoaded(name) {
			rows = append(rows, skipLine(name, ctx.Tlocal("内建插件不能卸载", "built-in, cannot remove")))
			continue
		}
		if err := p.loader.Remove(ctx.Context(), name); err != nil {
			rows = append(rows, failLine(name, ctx.Tlocal("没装这个插件", "not installed")))
			continue
		}
		rows = append(rows, "🗑 <b>"+htmlEscape(name)+"</b>  "+ctx.Tlocal("已卸载", "removed"))
	}
	return ctx.Edit(strings.Join(rows, "\n"))
}
