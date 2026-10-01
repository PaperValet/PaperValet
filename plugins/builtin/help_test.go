package builtin

import (
	"context"
	"strings"
	"testing"

	"github.com/TiaraBasori/PaperValet/internal/command"
	"github.com/TiaraBasori/PaperValet/internal/eventbus"
	"github.com/TiaraBasori/PaperValet/internal/i18n"
	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	pluginmgr "github.com/TiaraBasori/PaperValet/internal/plugin"
)

// newHelpFixture registers the real built-ins whose Init needs no I/O.
func newHelpFixture(t *testing.T) (*HelpPlugin, *pluginmgr.Manager) {
	t.Helper()
	t.Chdir(t.TempDir()) // plugins persist to data/
	bus := eventbus.New(nil)
	reg := command.NewRegistry([]string{"."}, bus, nil, nil, 1, i18n.NewManager(i18n.CoreCatalog()))
	mgr := pluginmgr.NewManager(reg, bus)
	help := NewHelp()
	for _, p := range []interfaces.Plugin{help, NewCore("test"), NewExec(), NewPrune(), NewRe(), NewInfo(), NewAlias()} {
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
	for name := range mgr.Commands().GetAll() {
		if !strings.Contains(out, "<code>."+name+"</code>") {
			t.Errorf("overview misses .%s", name)
		}
	}
	for _, alias := range []string{".h", ".p"} {
		if !strings.Contains(out, "<code>"+alias+"</code>") {
			t.Errorf("overview misses alias %s", alias)
		}
	}
	// Overview must stay terse: no multi-line usage leaks into it.
	if strings.Contains(out, "<b>机制</b>") {
		t.Error("overview must not include detailed usage")
	}
	if strings.Index(out, "<b>help</b>") > strings.Index(out, "<b>exec</b>") {
		t.Error("built-ins must follow builtinOrder")
	}
}

func TestHelpCommandPageIsDetailedAndLocalized(t *testing.T) {
	help, mgr := newHelpFixture(t)
	cmd, _ := mgr.Commands().Get("dme")
	zh := help.commandPage(&interfaces.CommandContext{Lang: "zh-CN"}, ".", cmd)
	if !strings.Contains(zh, "<b>机制</b>") || !strings.Contains(zh, "-f") {
		t.Errorf("zh page lacks mechanism section:\n%s", zh)
	}
	en := help.commandPage(&interfaces.CommandContext{Lang: "en-US"}, ".", cmd)
	if !strings.Contains(en, "How it works") || strings.Contains(en, "机制") {
		t.Errorf("en page not localized:\n%s", en)
	}
	h, _ := mgr.Commands().Get("h")
	page := help.commandPage(&interfaces.CommandContext{Lang: "zh-CN"}, ".", h)
	if !strings.Contains(page, "<code>.h</code>") || !strings.Contains(page, "<b>.help</b>") {
		t.Error("alias lookup must land on the command page and list aliases")
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
