package builtin

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/TiaraBasori/PaperValet/internal/i18n"
	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

const langFile = "data/lang"

// LangPlugin switches the bot language. The choice is global (it is one
// person's userbot) and persists across restarts.
type LangPlugin struct {
	mgr *i18n.Manager
}

func NewLang(mgr *i18n.Manager) *LangPlugin {
	return &LangPlugin{mgr: mgr}
}

func (p *LangPlugin) Name() string        { return "lang" }
func (p *LangPlugin) Description() string { return "切换界面语言" }
func (p *LangPlugin) DescEN() string      { return "Switch the interface language" }

func (p *LangPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	if data, err := os.ReadFile(langFile); err == nil {
		if l := normalizeLang(strings.TrimSpace(string(data))); l != "" {
			p.mgr.Catalog().SetDefault(l)
		}
	}
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "lang",
		Aliases:     []string{"language"},
		Description: "切换语言：lang en / lang zh",
		DescEN:      "Switch language: lang en / lang zh",
		Usage:       "lang [zh|en]",
		UsageEN:     "lang [zh|en]",
		Plugin:      p.Name(),
		Category:    "core",
		OwnerOnly:   true,
		Handler:     p.handleLang,
	})
}

func (p *LangPlugin) Start(_ context.Context) error { return nil }
func (p *LangPlugin) Stop(_ context.Context) error  { return nil }

// normalizeLang accepts short forms (zh, en, cn) and full codes.
func normalizeLang(s string) i18n.Lang {
	switch strings.ToLower(s) {
	case "zh", "cn", "zh-cn", "chinese", "中文":
		return i18n.ZhCN
	case "en", "en-us", "english":
		return i18n.EnUS
	}
	return ""
}

func (p *LangPlugin) handleLang(ctx *interfaces.CommandContext) error {
	if ctx.ArgCount() == 0 {
		cur := p.mgr.Catalog().Default()
		return ctx.Edit(ctx.Tlocal(
			"🌐 当前: <code>"+string(cur)+"</code>\n<code>lang en</code> 切英文 · <code>lang zh</code> 切中文",
			"🌐 Current: <code>"+string(cur)+"</code>\n<code>lang en</code> English · <code>lang zh</code> Chinese"))
	}
	l := normalizeLang(ctx.GetArg(0))
	if l == "" {
		return ctx.Edit(ctx.Tlocal("只支持 zh 和 en", "Only zh and en are supported"))
	}
	p.mgr.Catalog().SetDefault(l)
	p.mgr.ClearUserLangs()
	_ = os.MkdirAll(filepath.Dir(langFile), 0o700)
	_ = os.WriteFile(langFile, []byte(l), 0o600)
	if l == i18n.EnUS {
		return ctx.Edit("✅ Language: English")
	}
	return ctx.Edit("✅ 语言：中文")
}
