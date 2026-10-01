package loader

import (
	"os"
	goplugin "plugin"
	"testing"
)

// TestLoadSO proves an external plugin loads into this build: same Go
// toolchain, same package versions, a valid New and Metadata. It only runs
// when PAPERVALET_PLUGIN_SO points at a .so file:
//
//	GOWORK=off PAPERVALET_PLUGIN_SO=/abs/weather.so \
//	  go test -count=1 -trimpath -run TestLoadSO ./internal/plugin/loader
func TestLoadSO(t *testing.T) {
	path := os.Getenv("PAPERVALET_PLUGIN_SO")
	if path == "" {
		t.Skip("PAPERVALET_PLUGIN_SO not set")
	}
	p, err := goplugin.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	sym, err := p.Lookup("New")
	if err != nil {
		t.Fatalf("lookup New: %v", err)
	}
	pl, err := instantiate(sym)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	if pl.Name() == "" {
		t.Fatal("plugin Name() is empty")
	}
	if sym, err := p.Lookup("Metadata"); err == nil {
		meta := metadataOf(sym)
		if meta == nil {
			t.Fatalf("Metadata has type %T, want *plugin.PluginMetadata", sym)
		}
		if meta.Name != pl.Name() {
			t.Errorf("Metadata.Name %q != Name() %q", meta.Name, pl.Name())
		}
		t.Logf("loaded %s v%s", pl.Name(), meta.Version)
		return
	}
	t.Logf("loaded %s (no Metadata)", pl.Name())
}
