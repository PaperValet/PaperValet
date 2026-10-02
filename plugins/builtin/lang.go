package builtin

import (
	"context"

	"github.com/TiaraBasori/PaperValet/internal/i18n"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// LangPlugin holds the interface language. It has no command: the
// language is a setting, switched in the bot panel. The choice is global
// (one person's userbot) and persists across restarts.
type LangPlugin struct {
	mgr *i18n.Manager
	set plugin.Settings
}

func NewLang(mgr *i18n.Manager) *LangPlugin {
	return &LangPlugin{mgr: mgr}
}

func (p *LangPlugin) Name() string        { return "lang" }
func (p *LangPlugin) Description() string { return "界面语言" }
func (p *LangPlugin) DescEN() string      { return "Interface language" }

func (p *LangPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	set, err := mgr.Host().Settings(&plugin.SettingsSpec{
		Plugin:  p.Name(),
		Title:   "🌐 语言",
		TitleEN: "🌐 Language",
		Settings: []plugin.Setting{{
			Key:     "language",
			Label:   "界面语言",
			LabelEN: "Interface language",
			Hint:    "所有回复、帮助和这个面板都会切换，sudo 用户看到的也一样",
			HintEN:  "Every reply, help page and this panel switch, sudo users included",
			Kind:    plugin.SettingChoice,
			Default: string(p.mgr.Catalog().Default()),
			Choices: []plugin.Choice{
				{Value: string(i18n.ZhCN), Label: "简体中文", LabelEN: "简体中文"},
				{Value: string(i18n.EnUS), Label: "English", LabelEN: "English"},
			},
		}},
		OnChange: func(string) { p.apply() },
	})
	if err != nil {
		return err
	}
	p.set = set
	p.apply()
	// The old command is gone; a hidden stub points to the panel.
	return mgr.RegisterCommand(&plugin.Command{
		Name:        "lang",
		Description: "界面语言（在机器人面板设置）",
		DescEN:      "Interface language (set in the bot panel)",
		Usage:       "lang",
		UsageEN:     "lang",
		Plugin:      p.Name(),
		Category:    "core",
		OwnerOnly:   true,
		Hidden:      true,
		Handler: func(ctx *plugin.CommandContext) error {
			return ctx.Edit(ctx.Tlocal(
				"🌐 语言设置挪到机器人的 /menu 按钮面板了",
				"🌐 The language now lives in the bot's /menu panel"))
		},
	})
}

func (p *LangPlugin) apply() {
	l := i18n.Lang(p.set.String("language"))
	if l != i18n.ZhCN && l != i18n.EnUS {
		return
	}
	p.mgr.Catalog().SetDefault(l)
	p.mgr.ClearUserLangs()
}

func (p *LangPlugin) Start(_ context.Context) error { return nil }
func (p *LangPlugin) Stop(_ context.Context) error  { return nil }
