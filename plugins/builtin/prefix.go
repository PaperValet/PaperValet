package builtin

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// PrefixPlugin holds the command prefixes. It has no command: prefixes
// are settings, changed in the bot panel. Defaults come from config.json.
type PrefixPlugin struct {
	mgr plugin.Manager
	set plugin.Settings
}

func NewPrefix() *PrefixPlugin { return &PrefixPlugin{} }

func (p *PrefixPlugin) Name() string        { return "prefix" }
func (p *PrefixPlugin) Description() string { return "命令前缀" }
func (p *PrefixPlugin) DescEN() string      { return "Command prefixes" }

// validPrefix accepts 1–3 visible characters with no spaces.
func validPrefix(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" || utf8.RuneCountInString(s) > 3 || strings.ContainsAny(s, " \t\n") {
		return "", plugin.Invalid("1–3 个字符，不能有空格", "1–3 characters, no spaces")
	}
	return s, nil
}

func validExtra(s string) (string, error) {
	fields := strings.Fields(s)
	for _, f := range fields {
		if _, err := validPrefix(f); err != nil {
			return "", err
		}
	}
	return strings.Join(fields, " "), nil
}

func (p *PrefixPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	p.mgr = mgr
	cur := mgr.Commands().GetPrefixes()
	main, extra := ".", ""
	if len(cur) > 0 {
		main, extra = cur[0], strings.Join(cur[1:], " ")
	}
	set, err := mgr.Host().Settings(&plugin.SettingsSpec{
		Plugin:  p.Name(),
		Title:   "🔧 命令前缀",
		TitleEN: "🔧 Prefixes",
		Settings: []plugin.Setting{
			{
				Key: "main", Label: "主前缀", LabelEN: "Main prefix",
				Hint:   "帮助里显示的那个，例如 .",
				HintEN: "The one shown in help, e.g. .",
				Kind:   plugin.SettingText, Default: main, Validate: validPrefix,
			},
			{
				Key: "extra", Label: "其他前缀", LabelEN: "Extra prefixes",
				Hint:   "用空格分开，例如 ! ,，都能触发命令；匹配时优先最长的",
				HintEN: "Space separated, e.g. ! , — all trigger commands; the longest match wins",
				Kind:   plugin.SettingText, Default: extra, Validate: validExtra,
			},
		},
		OnChange: func(string) { p.apply() },
	})
	if err != nil {
		return err
	}
	p.set = set
	p.apply()
	return nil
}

// Prefixes returns main first, then the extras without duplicates.
func (p *PrefixPlugin) Prefixes() []string {
	main := p.set.String("main")
	if main == "" {
		main = "."
	}
	out := []string{main}
	for _, f := range strings.Fields(p.set.String("extra")) {
		dup := false
		for _, x := range out {
			dup = dup || x == f
		}
		if !dup {
			out = append(out, f)
		}
	}
	return out
}

func (p *PrefixPlugin) apply() { p.mgr.Commands().SetPrefixes(p.Prefixes()) }

// panelHint is the reply of the hidden redirect command.
func panelHint(ctx *plugin.CommandContext) error {
	return ctx.Edit(ctx.Tlocal(
		"🔧 前缀设置挪到机器人的 /menu 按钮面板了",
		"🔧 Prefixes now live in the bot's /menu panel"))
}

func (p *PrefixPlugin) Start(_ context.Context) error { return nil }
func (p *PrefixPlugin) Stop(_ context.Context) error  { return nil }
