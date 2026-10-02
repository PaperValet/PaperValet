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

	// Settings panels: only plugins with settings, named as they are,
	// and no management buttons.
	b, _, _ := s.dispatch(ctx, "l:b:0", target{})
	if got := strings.Join(buttonTexts(b), ","); got != "sudo,‹ 返回" {
		t.Fatalf("system settings: %s", got)
	}
	x, _, _ := s.dispatch(ctx, "l:x:0", target{})
	if got := strings.Join(buttonTexts(x), ","); got != "weather,‹ 返回" {
		t.Fatalf("external settings: %s", got)
	}

	// The manager merges installed plugins and the repository.
	g, _, _ := s.dispatch(ctx, "g:0", target{})
	if got := strings.Join(buttonTexts(g), ","); !strings.HasPrefix(got, "bad,broken,gt,weather,") {
		t.Fatalf("manager: %s", got)
	}
	for _, d := range []string{"o:weather", "o:gt", "a:ia", "a:la", "a:ra", "g:0:f", "m"} {
		if !hasButton(g, d) {
			t.Fatalf("manager misses %s", d)
		}
	}
	for _, d := range []string{"e:weather:on", "h:weather"} {
		if hasButton(g, d) {
			t.Fatalf("manager shows a settings button %s", d)
		}
	}
}

func TestPluginScreens(t *testing.T) {
	s, f := newCatalogService(t)
	withSettings(t, s, "sudo", "weather")
	f.external["rev"] = PluginInfo{Name: "rev", Commands: []string{".rev"}}
	ctx := context.Background()
	v, _, _ := s.dispatch(ctx, "h:sudo", target{})
	if !strings.Contains(v.Text, "`.sudo`") || !hasButton(v, "l:b:0") || !hasButton(v, "e:sudo:on") {
		t.Fatalf("builtin screen: %s %+v", v.Text, v.Buttons)
	}
	// Plugins without settings are not reachable from the settings side.
	for _, d := range []string{"h:help", "h:rev"} {
		if v, notice, _ := s.dispatch(ctx, d, target{}); notice == "" || hasButton(v, "a:rm:rev") {
			t.Fatalf("%s: %q %+v", d, notice, v.Buttons)
		}
	}
	// Settings screens carry no management buttons.
	v, _, _ = s.dispatch(ctx, "h:weather", target{})
	if !hasButton(v, "l:x:0") || hasButton(v, "o:weather") || hasButton(v, "a:rm:weather") || hasButton(v, "y:rl:weather") {
		t.Fatalf("external settings screen: %+v", v.Buttons)
	}
	// Manager screens carry no settings buttons.
	v, _, _ = s.dispatch(ctx, "o:weather", target{})
	if hasButton(v, "h:weather") || hasButton(v, "e:weather:on") || !hasButton(v, "y:rl:weather") || !hasButton(v, "a:rm:weather") || !hasButton(v, "g:0") || !strings.Contains(v.Text, "v1.0") {
		t.Fatalf("manage screen: %s %+v", v.Text, v.Buttons)
	}
	v, _, _ = s.dispatch(ctx, "o:rev", target{})
	if !hasButton(v, "a:rm:rev") || !strings.Contains(v.Text, "`.rev`") {
		t.Fatalf("settingless manage: %+v", v.Buttons)
	}
	v, _, _ = s.dispatch(ctx, "o:gt", target{})
	if !hasButton(v, "y:in:gt") || hasButton(v, "a:rm:gt") {
		t.Fatalf("not installed: %+v", v.Buttons)
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

func TestInstallRemove(t *testing.T) {
	s, _ := newCatalogService(t)
	ctx := context.Background()
	v, notice, _ := s.dispatch(ctx, "y:in:gt", target{})
	if notice == "" || !hasButton(v, "a:rm:gt") {
		t.Fatalf("after install: %q %+v", notice, v.Buttons)
	}
	v, _, _ = s.dispatch(ctx, "y:in:bad", target{})
	if !strings.Contains(v.Text, "原因 boom") || !hasButton(v, "o:bad") || !hasButton(v, "y:in:bad") {
		t.Fatalf("install failure: %s %+v", v.Text, v.Buttons)
	}
	// Remove asks first; cancel goes back to the plugin.
	v, _, _ = s.dispatch(ctx, "a:rm:gt", target{})
	if !hasButton(v, "y:rm:gt") || !hasButton(v, "o:gt") {
		t.Fatalf("confirm: %+v", v.Buttons)
	}
	if v, _, _ = s.dispatch(ctx, "y:rm:gt", target{}); !hasButton(v, "o:gt") || !hasButton(v, "g:0:f") {
		t.Fatalf("after remove: %+v", v.Buttons)
	}
	// A second tap on the same confirm reports instead of crashing.
	v, _, _ = s.dispatch(ctx, "y:rm:gt", target{})
	if !strings.Contains(v.Text, "原因 not installed") {
		t.Fatalf("double remove: %s", v.Text)
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
	if !strings.Contains(v.Text, "部分失败") || !strings.Contains(v.Text, "成功 1 · 失败 1") || !strings.Contains(v.Text, "✅ `gt`") || !strings.Contains(v.Text, "原因 boom") {
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
	// Without the index the installed plugins still show.
	f.repoErr = errors.New("offline")
	v, _, _ := s.dispatch(ctx, "g:0", target{})
	if !strings.Contains(v.Text, "offline") || !hasButton(v, "o:weather") || hasButton(v, "a:ia") || !hasButton(v, "g:0:f") {
		t.Fatalf("repo error: %+v", v)
	}
	f.repoErr = nil
	f.external = map[string]PluginInfo{}
	f.repo = nil
	for i := 0; i < 40; i++ {
		f.repo = append(f.repo, RepoEntry{Name: fmt.Sprintf("p%02d", i)})
	}
	v, _, _ = s.dispatch(ctx, "g:1", target{})
	if !hasButton(v, "o:p30") || hasButton(v, "o:p29") || !hasButton(v, "g:0") || !hasButton(v, "n") || hasButton(v, "a:ra") {
		t.Fatalf("page 2: %+v", v.Buttons)
	}
	// Out-of-range pages clamp.
	v, _, _ = s.dispatch(ctx, "g:99", target{})
	if !hasButton(v, "o:p39") || hasButton(v, "g:2") {
		t.Fatalf("clamped: %+v", v.Buttons)
	}
	if v, _, _ := s.dispatch(ctx, "n", target{}); v != nil {
		t.Fatal("counter button changed the panel")
	}
}

func TestStaleButtons(t *testing.T) {
	s, _ := newCatalogService(t)
	ctx := context.Background()
	for _, d := range []string{"zz", "e:", "e:ghost:k", "c:ghost:k:0", "r:ghost", "l:q:abc", "a:zz", "g:abc", "g:1:x", "o:", "a:rm", "s:0", "i:gt"} {
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
