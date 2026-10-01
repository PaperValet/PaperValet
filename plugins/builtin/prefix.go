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
		Usage: `prefix · prefix add|del|set &lt;符号&gt;

<b>示例</b>
• <code>prefix</code>  看当前前缀
• <code>prefix add !</code>  再加一个 !，. 和 ! 都能触发
• <code>prefix set !</code>  把 ! 设为主前缀（帮助里显示的那个）
• <code>prefix del !</code>  删除

<b>机制</b>
• 可以同时有多个前缀，至少保留一个
• 多个前缀匹配时优先最长的，例如 .. 优先于 .
• 立即生效，保存在 data/prefixes.json`,
		UsageEN: `prefix · prefix add|del|set &lt;symbol&gt;

<b>Examples</b>
• <code>prefix</code>  show current prefixes
• <code>prefix add !</code>  add !, both . and ! trigger
• <code>prefix set !</code>  make ! the main prefix (shown in help)
• <code>prefix del !</code>  remove

<b>How it works</b>
• Several prefixes can coexist; at least one stays
• The longest matching prefix wins, e.g. .. before .
• Applies immediately, stored in data/prefixes.json`,
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
		parts[i] = "<code>" + htmlEscape(p) + "</code>"
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
		c.blank().rawField(ctx.Tlocal("主前缀", "Main"), "<code>"+htmlEscape(p.prefixes[0])+"</code>")
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
				return ctx.Edit("🗑 <b>" + htmlEscape(arg) + "</b>  " + ctx.Tlocal("已删除", "removed"))
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
