package builtin

import (
	"context"

	"github.com/TiaraBasori/PaperValet/internal/i18n"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// LanguagePlugin holds the interface language. It has no command: the
// language is a setting, switched in the bot panel. The choice is global
// (one person's userbot) and persists across restarts.
type LanguagePlugin struct {
	mgr *i18n.Manager
	set plugin.Settings
}

func NewLanguage(mgr *i18n.Manager) *LanguagePlugin {
	return &LanguagePlugin{mgr: mgr}
}

func (p *LanguagePlugin) Name() string        { return "language" }
func (p *LanguagePlugin) Description() string { return "界面语言" }
func (p *LanguagePlugin) DescEN() string      { return "Interface language" }

func (p *LanguagePlugin) Init(_ context.Context, mgr plugin.Manager) error {
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
	return nil
}

func (p *LanguagePlugin) apply() {
	l := i18n.Lang(p.set.String("language"))
	if l != i18n.ZhCN && l != i18n.EnUS {
		return
	}
	p.mgr.Catalog().SetDefault(l)
	p.mgr.ClearUserLangs()
}

func (p *LanguagePlugin) Start(_ context.Context) error { return nil }
func (p *LanguagePlugin) Stop(_ context.Context) error  { return nil }
