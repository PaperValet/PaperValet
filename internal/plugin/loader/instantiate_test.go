package loader

import (
	"context"
	"errors"
	"strings"
	"testing"

	pkgplugin "github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type fakePlugin struct{}

func (*fakePlugin) Name() string                                  { return "fake" }
func (*fakePlugin) Description() string                           { return "" }
func (*fakePlugin) Init(context.Context, pkgplugin.Manager) error { return nil }
func (*fakePlugin) Start(context.Context) error                   { return nil }
func (*fakePlugin) Stop(context.Context) error                    { return nil }

func TestInstantiateAcceptedSignatures(t *testing.T) {
	cases := map[string]interface{}{
		"concrete pointer": func() *fakePlugin { return &fakePlugin{} },
		"plugin, error":    func() (pkgplugin.Plugin, error) { return &fakePlugin{}, nil },
		"interface{}":      func() interface{} { return &fakePlugin{} },
		"pointer, error":   func() (*fakePlugin, error) { return &fakePlugin{}, nil },
		"plugin":           func() pkgplugin.Plugin { return &fakePlugin{} },
	}
	for name, sym := range cases {
		p, err := instantiate(sym)
		if err != nil || p == nil || p.Name() != "fake" {
			t.Errorf("%s: got %v, %v", name, p, err)
		}
	}
}

func TestInstantiateRejects(t *testing.T) {
	cases := map[string]struct {
		sym  interface{}
		want string
	}{
		"not func":       {42, "not a function"},
		"takes args":     {func(int) *fakePlugin { return nil }, "unsupported signature"},
		"bad second":     {func() (*fakePlugin, int) { return nil, 0 }, "unsupported signature"},
		"not a plugin":   {func() string { return "" }, "does not implement"},
		"nil pointer":    {func() *fakePlugin { return nil }, "returned nil"},
		"nil interface":  {func() interface{} { return nil }, "returned nil"},
		"returns error":  {func() (pkgplugin.Plugin, error) { return nil, errors.New("boom") }, "boom"},
		"nil func value": {(func() *fakePlugin)(nil), "not a function"},
	}
	for name, c := range cases {
		_, err := instantiate(c.sym)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want contains %q", name, err, c.want)
		}
	}
}
