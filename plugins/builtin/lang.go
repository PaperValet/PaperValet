package builtin

import (
	"context"
	"fmt"
	"strings"

	"github.com/TiaraBasori/PaperValet/internal/i18n"
	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// LangPlugin provides runtime language switching (native i18n).
type LangPlugin struct {
	mgr *i18n.Manager
}

func NewLang(mgr *i18n.Manager) *LangPlugin {
	return &LangPlugin{mgr: mgr}
}

func (p *LangPlugin) Name() string        { return "lang" }
func (p *LangPlugin) Description() string { return "语言切换（i18n）" }

func (p *LangPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "lang",
		Aliases:     []string{"language", "i18n", "语言"},
		Description: "查看/切换语言",
		Usage:       "lang [zh-CN|en-US]",
		Plugin:      p.Name(),
		Category:    "core",
		Handler:     p.handleLang,
	})
}

func (p *LangPlugin) Start(_ context.Context) error { return nil }
func (p *LangPlugin) Stop(_ context.Context) error  { return nil }

func (p *LangPlugin) handleLang(ctx *interfaces.CommandContext) error {
	if ctx.ArgCount() == 0 {
		current := p.mgr.UserLang(ctx.Message.UserID)
		langs := p.mgr.Catalog().Languages()
		var names []string
		for _, l := range langs {
			names = append(names, string(l))
		}
		return ctx.Edit(fmt.Sprintf("%s\n%s",
			ctx.T("i18n.current", current),
			ctx.T("i18n.available", strings.Join(names, ", ")),
		))
	}

	target := i18n.Lang(ctx.GetArg(0))
	if !p.mgr.Catalog().Has(target, "core.ping_start") {
		langs := p.mgr.Catalog().Languages()
		var names []string
		for _, l := range langs {
			names = append(names, string(l))
		}
		return ctx.Edit(ctx.T("i18n.invalid", target, strings.Join(names, ", ")))
	}

	p.mgr.SetUserLang(ctx.Message.UserID, target)
	return ctx.Edit(ctx.T("i18n.set", target))
}
