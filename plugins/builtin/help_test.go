package builtin

import (
	"context"
	"strings"
	"testing"

	"github.com/TiaraBasori/PaperValet/internal/bot"
	"github.com/TiaraBasori/PaperValet/internal/command"
	"github.com/TiaraBasori/PaperValet/internal/eventbus"
	"github.com/TiaraBasori/PaperValet/internal/i18n"
	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	pluginmgr "github.com/TiaraBasori/PaperValet/internal/plugin"
	"github.com/TiaraBasori/PaperValet/internal/settings"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// newHelpFixture registers the real built-ins whose Init needs no I/O.
func newHelpFixture(t *testing.T) (*HelpPlugin, *pluginmgr.Manager) {
	t.Helper()
	t.Chdir(t.TempDir()) // plugins persist to data/
	bus := eventbus.New(nil)
	reg := command.NewRegistry([]string{"."}, bus, nil, nil, 1, i18n.NewManager(i18n.CoreCatalog()))
	reg.SetBot(bot.New(bot.Options{}, settings.NewRegistry("data"), nil))
	mgr := pluginmgr.NewManager(reg, bus)
	help := NewHelp()
	for _, p := range []interfaces.Plugin{help, NewPing(), NewRestart(), NewExec(), NewPrune(), NewRe(), NewInfo(), NewAlias(),
		NewApt(nil), NewBackup(), NewLog(), NewPrefix(), NewReload(nil), NewStatus("test", nil), NewSudo(), NewUpdate("test", nil)} {
		if err := mgr.RegisterPlugin(p); err != nil {
			t.Fatal(err)
		}
		if err := p.Init(context.Background(), mgr); err != nil {
			t.Fatal(err)
		}
	}
	reg.AddUserAlias("p", "ping")
	return help, mgr
}

func TestHelpOverviewListsEveryCommandAndAlias(t *testing.T) {
	help, mgr := newHelpFixture(t)
	ctx := &interfaces.CommandContext{Lang: "zh-CN"}
	out := help.overview(ctx, ".")
	for name, cmd := range mgr.Commands().GetAll() {
		// Hidden commands are redirects and never appear in help.
		if cmd.Hidden {
			continue
		}
		// Single-command plugins whose command shares the plugin name may be
		// collapsed into the plugin header line.
		if strings.Contains(out, "**"+name+"** ·") {
			continue
		}
		if !strings.Contains(out, "`."+name+"`") {
			t.Errorf("overview misses .%s", name)
		}
	}
	for _, alias := range []string{".h", ".p"} {
		if !strings.Contains(out, "`"+alias+"`") {
			t.Errorf("overview misses alias %s", alias)
		}
	}
	// Overview must stay terse: no multi-line usage leaks into it.
	if strings.Contains(out, "**机制**") {
		t.Error("overview must not include detailed usage")
	}
	if strings.Index(out, "**help**") > strings.Index(out, "**exec**") {
		t.Error("built-ins must follow builtinOrder")
	}
}

func TestHelpCommandPageIsDetailedAndLocalized(t *testing.T) {
	help, mgr := newHelpFixture(t)
	cmd, _ := mgr.Commands().Get("dme")
	zh := help.commandPage(&interfaces.CommandContext{Lang: "zh-CN"}, ".", cmd)
	if !strings.Contains(zh, "**机制**") || !strings.Contains(zh, "-f") {
		t.Errorf("zh page lacks mechanism section:\n%s", zh)
	}
	en := help.commandPage(&interfaces.CommandContext{Lang: "en-US"}, ".", cmd)
	if !strings.Contains(en, "How it works") || strings.Contains(en, "机制") {
		t.Errorf("en page not localized:\n%s", en)
	}
	h, _ := mgr.Commands().Get("h")
	page := help.commandPage(&interfaces.CommandContext{Lang: "zh-CN"}, ".", h)
	if !strings.Contains(page, "`.h`") || !strings.Contains(page, ".help") {
		t.Errorf("command page must mention help and its alias:\n%s", page)
	}
}

func TestEveryBuiltinCommandHasBilingualDocs(t *testing.T) {
	_, mgr := newHelpFixture(t)
	for name, cmd := range mgr.Commands().GetAll() {
		if cmd.Description == "" || cmd.DescEN == "" || cmd.Usage == "" || cmd.UsageEN == "" {
			t.Errorf("%s: missing description or usage in one language", name)
		}
		if strings.ContainsAny(cmd.Description, "，；") {
			t.Errorf("%s: description should be one short phrase, got %q", name, cmd.Description)
		}
	}
}

// Every page must parse as Telegram Markdown without leaking raw markup.
func TestHelpPagesRenderAsMarkdown(t *testing.T) {
	help, mgr := newHelpFixture(t)
	check := func(where, md string) {
		plain, _ := plugin.ParseMarkdown(md, nil)
		for _, bad := range []string{"**", "<b>", "<code>", "&lt;", "```"} {
			if strings.Contains(plain, bad) {
				t.Errorf("%s leaks %q:\n%s", where, bad, plain)
			}
		}
		if strings.Count(plain, "`") > 0 {
			t.Errorf("%s leaks a backtick:\n%s", where, plain)
		}
	}
	for _, lang := range []string{"zh-CN", "en-US"} {
		ctx := &interfaces.CommandContext{Lang: lang}
		check("overview "+lang, help.overview(ctx, "."))
		for name, cmd := range mgr.Commands().GetAll() {
			check(name+" "+lang, help.commandPage(ctx, ".", cmd))
		}
	}
}
