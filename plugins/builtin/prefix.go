package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// PrefixPlugin manages command prefixes with persistent storage.
type PrefixPlugin struct {
	mgr      plugin.Manager
	mu       sync.Mutex
	prefixes []string
	file     string
}

func NewPrefix() *PrefixPlugin {
	return &PrefixPlugin{prefixes: []string{"."}, file: "data/prefixes.json"}
}

func (p *PrefixPlugin) Name() string        { return "prefix" }
func (p *PrefixPlugin) Description() string { return "管理命令前缀" }
func (p *PrefixPlugin) DescEN() string      { return "Manage command prefixes" }

func (p *PrefixPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	p.mgr = mgr
	p.load()
	p.mgr.Commands().SetPrefixes(p.prefixes)
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "prefix",
		Description: "管理命令前缀",
		DescEN:      "Manage command prefixes",
		Usage: "prefix · prefix add|del|set <符号>\n" +
			"\n" +
			"**示例**\n" +
			"• `prefix`  看当前前缀\n" +
			"• `prefix add !`  再加一个 !，. 和 ! 都能触发\n" +
			"• `prefix set !`  把 ! 设为主前缀（帮助里显示的那个）\n" +
			"• `prefix del !`  删除\n" +
			"\n" +
			"**机制**\n" +
			"• 可以同时有多个前缀，至少保留一个\n" +
			"• 多个前缀匹配时优先最长的，例如 .. 优先于 .\n" +
			"• 立即生效，保存在 data/prefixes.json",
		UsageEN: "prefix · prefix add|del|set <symbol>\n" +
			"\n" +
			"**Examples**\n" +
			"• `prefix`  show current prefixes\n" +
			"• `prefix add !`  add !, both . and ! trigger\n" +
			"• `prefix set !`  make ! the main prefix (shown in help)\n" +
			"• `prefix del !`  remove\n" +
			"\n" +
			"**How it works**\n" +
			"• Several prefixes can coexist; at least one stays\n" +
			"• The longest matching prefix wins, e.g. .. before .\n" +
			"• Applies immediately, stored in data/prefixes.json",
		Plugin:    p.Name(),
		Category:  "admin",
		OwnerOnly: true,
		Handler:   p.handlePrefix,
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
	if json.Unmarshal(data, &prefixes) == nil && len(prefixes) > 0 {
		p.prefixes = prefixes
	}
}

func (p *PrefixPlugin) save() {
	_ = os.MkdirAll(filepath.Dir(p.file), 0o700)
	data, _ := json.MarshalIndent(p.prefixes, "", "  ")
	_ = os.WriteFile(p.file, data, 0o600)
	p.mgr.Commands().SetPrefixes(p.prefixes)
}

func prefixList(prefixes []string) string {
	parts := make([]string, len(prefixes))
	for i, p := range prefixes {
		parts[i] = plugin.Code(p)
	}
	return strings.Join(parts, " ")
}

func (p *PrefixPlugin) handlePrefix(ctx *interfaces.CommandContext) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	sub := strings.ToLower(ctx.GetArg(0))
	arg := ctx.GetArg(1)

	switch sub {
	case "", "list", "ls":
		c := newCard("🔧", ctx.Tlocal("命令前缀", "Prefixes"))
		c.blank().rawField(ctx.Tlocal("主前缀", "Main"), plugin.Code(p.prefixes[0]))
		c.rawField(ctx.Tlocal("全部", "All"), prefixList(p.prefixes))
		c.hint(ctx.Tlocal(cmdRef(".prefix add !")+" 添加", cmdRef(".prefix add !")+" to add"))
		return ctx.Edit(c.String())

	case "add":
		if arg == "" {
			return ctx.Edit(errText(ctx.Tlocal("用法 ", "Usage ") + cmdRef("prefix add <符号>")))
		}
		for _, x := range p.prefixes {
			if x == arg {
				return ctx.Edit(skipLine(arg, ctx.Tlocal("已经在了", "already present")))
			}
		}
		p.prefixes = append(p.prefixes, arg)
		sort.Slice(p.prefixes, func(i, j int) bool { return len(p.prefixes[i]) > len(p.prefixes[j]) })
		p.save()
		return ctx.Edit(okLine(arg, ctx.Tlocal("已添加", "added")))

	case "del", "rm":
		if arg == "" {
			return ctx.Edit(errText(ctx.Tlocal("用法 ", "Usage ") + cmdRef("prefix del <符号>")))
		}
		if len(p.prefixes) <= 1 {
			return ctx.Edit(errText(ctx.Tlocal("至少保留一个前缀", "at least one prefix must stay")))
		}
		for i, x := range p.prefixes {
			if x == arg {
				p.prefixes = append(p.prefixes[:i], p.prefixes[i+1:]...)
				p.save()
				return ctx.Edit("🗑 **" + esc(arg) + "**  " + ctx.Tlocal("已删除", "removed"))
			}
		}
		return ctx.Edit(errText(ctx.Tlocal("没有这个前缀 ", "no such prefix: ") + cmdRef(arg)))

	case "set", "main":
		if arg == "" {
			return ctx.Edit(errText(ctx.Tlocal("用法 ", "Usage ") + cmdRef("prefix set <符号>")))
		}
		rest := p.prefixes[:0]
		for _, x := range p.prefixes {
			if x != arg {
				rest = append(rest, x)
			}
		}
		p.prefixes = append([]string{arg}, rest...)
		p.save()
		return ctx.Edit(okLine(arg, ctx.Tlocal("已设为主前缀", "is now the main prefix")))
	}
	return ctx.Edit(errText(ctx.Tlocal("未知子命令 ", "Unknown subcommand ") + cmdRef(sub)))
}

var _ = fmt.Sprintf
