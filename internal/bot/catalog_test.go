package bot

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type fakeCatalog struct {
	mu       sync.Mutex
	builtins []PluginInfo
	external map[string]PluginInfo
	repo     []RepoEntry
	repoErr  error
	block    chan struct{}
	calls    []string
}

func (f *fakeCatalog) Builtins() []PluginInfo { return append([]PluginInfo(nil), f.builtins...) }

func (f *fakeCatalog) Externals() []PluginInfo {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []PluginInfo
	for _, p := range f.external {
		out = append(out, p)
	}
	return out
}

func (f *fakeCatalog) Repo(context.Context, bool) ([]RepoEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.repoErr != nil {
		return nil, f.repoErr
	}
	out := append([]RepoEntry(nil), f.repo...)
	for i := range out {
		_, out[i].Installed = f.external[out[i].Name]
	}
	return out, nil
}

func (f *fakeCatalog) record(op, name string) {
	if f.block != nil {
		<-f.block
	}
	f.mu.Lock()
	f.calls = append(f.calls, op+":"+name)
	f.mu.Unlock()
}

func (f *fakeCatalog) Install(_ context.Context, name string) error {
	f.record("in", name)
	if name == "bad" {
		return errors.New("boom")
	}
	f.mu.Lock()
	f.external[name] = PluginInfo{Name: name}
	f.mu.Unlock()
	return nil
}

func (f *fakeCatalog) Remove(_ context.Context, name string) error {
	f.record("rm", name)
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.external[name]; !ok {
		return errors.New("not installed")
	}
	delete(f.external, name)
	return nil
}

func (f *fakeCatalog) Reload(_ context.Context, name string) error {
	f.record("rl", name)
	return nil
}

func (f *fakeCatalog) Explain(err error) (string, string) {
	return "原因 " + err.Error(), err.Error()
}

func newCatalogService(t *testing.T) (*Service, *fakeCatalog) {
	t.Helper()
	s := newTestService(t)
	f := &fakeCatalog{
		builtins: []PluginInfo{{Name: "sudo", Desc: "授权", Commands: []string{".sudo"}}, {Name: "help", Commands: []string{".help", ".h"}}},
		external: map[string]PluginInfo{
			"weather": {Name: "weather", Desc: "天气", Version: "1.0"},
			"broken":  {Name: "broken", Failed: "plugin.Open: bad ELF"},
		},
		repo: []RepoEntry{{Name: "weather"}, {Name: "gt", Desc: "翻译"}, {Name: "bad"}},
	}
	s.SetCatalog(f)
	return s, f
}

func buttonTexts(v *View) []string {
	var out []string
	for _, row := range v.Buttons {
		for _, b := range row {
			out = append(out, b.Text)
		}
	}
	return out
}

// withSettings gives a plugin a one-toggle settings panel.
func withSettings(t *testing.T, s *Service, names ...string) {
	t.Helper()
	for _, n := range names {
		if _, err := s.Settings(&plugin.SettingsSpec{Plugin: n, Settings: []plugin.Setting{{Key: "on", Label: "On"}}}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMenuHasThreePanels(t *testing.T) {
	s, _ := newCatalogService(t)
	withSettings(t, s, "sudo", "weather")
	ctx := context.Background()
	m := s.menuView()
	for _, d := range []string{"l:b:0", "l:x:0", "g:0"} {
		if !hasButton(m, d) {
			t.Fatalf("menu misses %s: %+v", d, m.Buttons)
		}
	}
	if !strings.Contains(m.Text, "系统设置") || !strings.Contains(m.Text, "⚠️ 1") {
		t.Fatalf("menu text: %s", m.Text)
	}

	// Only plugins with settings show, named as they are.
	b, _, _ := s.dispatch(ctx, "l:b:0", target{})
	if got := strings.Join(buttonTexts(b), ","); got != "sudo,« 返回" {
		t.Fatalf("system settings: %s", got)
	}
	x, _, _ := s.dispatch(ctx, "l:x:0", target{})
	if got := strings.Join(buttonTexts(x), ","); !strings.HasPrefix(got, "weather,") || hasButton(x, "h:broken") || hasButton(x, "a:ra") {
		t.Fatalf("external panel: %s", got)
	}
	if !hasButton(x, "g:0") {
		t.Fatal("external panel lacks a link to the manager")
	}

	// The manager lists every installed plugin, broken ones too.
	g, _, _ := s.dispatch(ctx, "g:0", target{})
	if got := strings.Join(buttonTexts(g), ","); !strings.HasPrefix(got, "broken,weather,") {
		t.Fatalf("manager: %s", got)
	}
	for _, d := range []string{"o:weather", "s:0", "a:la", "a:ra", "m"} {
		if !hasButton(g, d) {
			t.Fatalf("manager misses %s", d)
		}
	}
}

func TestPluginScreens(t *testing.T) {
	s, f := newCatalogService(t)
	withSettings(t, s, "sudo", "weather")
	f.external["rev"] = PluginInfo{Name: "rev", Commands: []string{".rev"}}
	ctx := context.Background()
	v, _, _ := s.dispatch(ctx, "h:sudo", target{})
	if !strings.Contains(v.Text, "`.sudo`") || hasButton(v, "o:sudo") || !hasButton(v, "l:b:0") || !hasButton(v, "e:sudo:on") {
		t.Fatalf("builtin screen: %s %+v", v.Text, v.Buttons)
	}
	// A system plugin without settings is not reachable.
	if v, notice, _ := s.dispatch(ctx, "h:help", target{}); notice == "" || !hasButton(v, "l:b:0") {
		t.Fatalf("settingless builtin: %q", notice)
	}
	v, _, _ = s.dispatch(ctx, "h:weather", target{})
	if !hasButton(v, "o:weather") || !hasButton(v, "l:x:0") || hasButton(v, "a:rm:weather") {
		t.Fatalf("external settings screen: %+v", v.Buttons)
	}
	v, _, _ = s.dispatch(ctx, "o:weather", target{})
	if !hasButton(v, "h:weather") || !hasButton(v, "y:rl:weather") || !hasButton(v, "a:rm:weather") || !hasButton(v, "g:0") || !strings.Contains(v.Text, "v1.0") {
		t.Fatalf("manage screen: %s %+v", v.Text, v.Buttons)
	}
	// Settingless external: manage screen without a settings button.
	v, _, _ = s.dispatch(ctx, "o:rev", target{})
	if hasButton(v, "h:rev") || !hasButton(v, "a:rm:rev") || !strings.Contains(v.Text, "`.rev`") {
		t.Fatalf("settingless manage: %+v", v.Buttons)
	}
	if v, _, _ := s.dispatch(ctx, "h:rev", target{}); !hasButton(v, "a:rm:rev") {
		t.Fatal("h: on a settingless external should land on its manage screen")
	}
	v, _, _ = s.dispatch(ctx, "o:broken", target{})
	if !strings.Contains(v.Text, "原因 plugin.Open") || !hasButton(v, "y:rl:broken") || !hasButton(v, "a:rm:broken") {
		t.Fatalf("broken screen: %s %+v", v.Text, v.Buttons)
	}
	for _, d := range []string{"h:ghost", "o:ghost"} {
		if v, notice, _ := s.dispatch(ctx, d, target{}); notice == "" || v == nil {
			t.Fatalf("%s: gone plugin not reported", d)
		}
	}
}

func TestRepoInstallRemove(t *testing.T) {
	s, _ := newCatalogService(t)
	ctx := context.Background()
	v, _, _ := s.dispatch(ctx, "s:0", target{})
	if !hasButton(v, "o:weather") || !hasButton(v, "i:gt") || !hasButton(v, "a:ia") || !hasButton(v, "g:0") {
		t.Fatalf("repo: %+v", v.Buttons)
	}
	v, _, _ = s.dispatch(ctx, "i:gt", target{})
	if !hasButton(v, "y:in:gt") {
		t.Fatalf("install screen: %+v", v.Buttons)
	}
	v, notice, _ := s.dispatch(ctx, "y:in:gt", target{})
	if notice == "" || !hasButton(v, "a:rm:gt") {
		t.Fatalf("after install: %q %+v", notice, v.Buttons)
	}
	// Installed entries in the repo open the manage screen.
	if v, _, _ := s.dispatch(ctx, "i:gt", target{}); !hasButton(v, "a:rm:gt") {
		t.Fatal("installed repo entry did not open the manage screen")
	}
	v, _, _ = s.dispatch(ctx, "y:in:bad", target{})
	if !strings.Contains(v.Text, "原因 boom") {
		t.Fatalf("install failure: %s", v.Text)
	}
	// Remove asks first; cancel goes back to the plugin.
	v, _, _ = s.dispatch(ctx, "a:rm:gt", target{})
	if !hasButton(v, "y:rm:gt") || !hasButton(v, "o:gt") {
		t.Fatalf("confirm: %+v", v.Buttons)
	}
	if v, _, _ = s.dispatch(ctx, "y:rm:gt", target{}); !hasButton(v, "s:0") {
		t.Fatalf("after remove: %+v", v.Buttons)
	}
	// A second tap on the same confirm reports instead of crashing.
	v, _, _ = s.dispatch(ctx, "y:rm:gt", target{})
	if !strings.Contains(v.Text, "原因 not installed") {
		t.Fatalf("double remove: %s", v.Text)
	}
	if v, notice, _ := s.dispatch(ctx, "i:nope", target{}); notice == "" || v == nil {
		t.Fatal("vanished repo entry")
	}
	if _, _, err := s.dispatch(ctx, "y:in", target{}); err == nil {
		t.Fatal("install without a name accepted")
	}
	if _, _, err := s.dispatch(ctx, "y:zz:x", target{}); err == nil {
		t.Fatal("unknown op accepted")
	}
}

func TestBulkOps(t *testing.T) {
	s, f := newCatalogService(t)
	ctx := context.Background()
	v, _, _ := s.dispatch(ctx, "y:ia", target{})
	if !strings.Contains(v.Text, "1/2") || !strings.Contains(v.Text, "`gt`") || !strings.Contains(v.Text, "原因 boom") {
		t.Fatalf("install all: %s", v.Text)
	}
	v, _, _ = s.dispatch(ctx, "y:la", target{})
	if strings.Contains(v.Text, "broken") {
		t.Fatalf("reload all touched a failed file: %s", v.Text)
	}
	v, _, _ = s.dispatch(ctx, "y:ra", target{})
	if len(f.Externals()) != 0 {
		t.Fatalf("remove all left %v: %s", f.Externals(), v.Text)
	}
	v, _, _ = s.dispatch(ctx, "y:ra", target{})
	if !strings.Contains(v.Text, "没有要处理") {
		t.Fatalf("empty bulk: %s", v.Text)
	}
}

func TestOneOperationAtATime(t *testing.T) {
	s, f := newCatalogService(t)
	f.block = make(chan struct{})
	ctx := context.Background()
	done := make(chan struct{})
	go func() {
		s.dispatch(ctx, "y:in:gt", target{})
		close(done)
	}()
	deadline := time.Now().Add(time.Second)
	for !s.busy.Load() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	v, notice, err := s.dispatch(ctx, "y:rm:weather", target{})
	if err != nil || v != nil || !strings.HasPrefix(notice, "!") {
		t.Fatalf("second op: %v %v %q", err, v, notice)
	}
	close(f.block)
	<-done
	if s.busy.Load() {
		t.Fatal("busy flag stuck")
	}
}

func TestRepoErrorAndPaging(t *testing.T) {
	s, f := newCatalogService(t)
	ctx := context.Background()
	f.repoErr = errors.New("offline")
	v, _, _ := s.dispatch(ctx, "s:0", target{})
	if !strings.Contains(v.Text, "offline") || !hasButton(v, "s:0:f") {
		t.Fatalf("repo error: %+v", v)
	}
	f.repoErr = nil
	f.repo = nil
	for i := 0; i < 40; i++ {
		f.repo = append(f.repo, RepoEntry{Name: fmt.Sprintf("p%02d", i)})
	}
	v, _, _ = s.dispatch(ctx, "s:1", target{})
	if !hasButton(v, "i:p15") || hasButton(v, "i:p14") || !hasButton(v, "s:0") || !hasButton(v, "s:2") || !hasButton(v, "n") {
		t.Fatalf("page 2: %+v", v.Buttons)
	}
	// Out-of-range pages clamp.
	v, _, _ = s.dispatch(ctx, "s:99", target{})
	if !hasButton(v, "i:p39") || hasButton(v, "s:3") {
		t.Fatalf("clamped: %+v", v.Buttons)
	}
	if v, _, _ := s.dispatch(ctx, "n", target{}); v != nil {
		t.Fatal("counter button changed the panel")
	}
}

func TestStaleButtons(t *testing.T) {
	s, _ := newCatalogService(t)
	ctx := context.Background()
	for _, d := range []string{"zz", "e:", "e:ghost:k", "c:ghost:k:0", "r:ghost", "l:q:abc", "a:zz", "g:abc", "o:", "a:rm"} {
		v, _, err := s.dispatch(ctx, d, target{})
		if err != nil || v == nil {
			t.Fatalf("%q: %v %v", d, err, v)
		}
	}
	// A setting removed after the panel was drawn.
	if _, err := s.Settings(&plugin.SettingsSpec{Plugin: "weather", Settings: []plugin.Setting{{Key: "city", Label: "City", Kind: plugin.SettingText}}}); err != nil {
		t.Fatal(err)
	}
	if v, notice, _ := s.dispatch(ctx, "e:weather:gone", target{}); notice == "" || !hasButton(v, "e:weather:city") {
		t.Fatalf("gone setting: %q", notice)
	}
}

func TestPendingPrompts(t *testing.T) {
	s, _ := newCatalogService(t)
	st, err := s.Settings(&plugin.SettingsSpec{Plugin: "weather", Settings: []plugin.Setting{{Key: "city", Label: "City", Kind: plugin.SettingText}}})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	s.dispatch(ctx, "e:weather:city", target{msgID: 5})
	// Media (empty text) keeps the prompt.
	s.onMessage(42, 9, "")
	if s.currentPending() == nil {
		t.Fatal("media dropped the prompt")
	}
	// A stale prompt is ignored.
	s.mu.Lock()
	s.pending.at = time.Now().Add(-pendingTTL - time.Minute)
	s.mu.Unlock()
	s.onMessage(42, 10, "Paris")
	if st.String("city") != "" || s.currentPending() != nil {
		t.Fatal("expired prompt stored text")
	}
	// /cancel drops a prompt.
	s.dispatch(ctx, "e:weather:city", target{msgID: 5})
	s.onMessage(42, 11, "/cancel")
	if s.currentPending() != nil {
		t.Fatal("cancel kept the prompt")
	}
	// Commands addressed to another bot are ignored.
	s.dispatch(ctx, "e:weather:city", target{msgID: 5})
	s.onMessage(42, 12, "/menu@otherbot")
	if s.currentPending() == nil {
		t.Fatal("foreign command cleared the prompt")
	}
	s.onMessage(42, 13, "Tokyo")
	if st.String("city") != "Tokyo" {
		t.Fatalf("city = %q", st.String("city"))
	}
}

func TestClip(t *testing.T) {
	if clip("短", 5) != "短" || clip("一二三四五六", 4) != "一二三…" {
		t.Fatal("clip")
	}
}
