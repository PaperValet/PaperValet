package i18n

import (
	"slices"
	"testing"
)

// Every setup prompt must exist in both languages, otherwise a user who
// picked one language would silently see the other mid-flow.
func TestSetupCatalogParity(t *testing.T) {
	c := SetupCatalog()
	zh, en := c.Keys(ZhCN), c.Keys(EnUS)
	if !slices.Equal(zh, en) {
		for _, k := range zh {
			if !slices.Contains(en, k) {
				t.Errorf("missing en-US key %q", k)
			}
		}
		for _, k := range en {
			if !slices.Contains(zh, k) {
				t.Errorf("missing zh-CN key %q", k)
			}
		}
	}
	if len(zh) == 0 {
		t.Fatal("setup catalog is empty")
	}
}
