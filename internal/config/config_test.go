package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveOwnerOnlyAndRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	// Pre-existing world-readable file must end up 0600 after Save.
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := Example()
	cfg.Telegram.APIID = 42
	cfg.Telegram.APIHash = "secret"
	cfg.I18n.DefaultLanguage = "en-US"
	if err := cfg.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := st.Mode().Perm(); perm != 0o600 {
		t.Fatalf("perm = %o, want 600", perm)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp file left behind: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Telegram.APIID != 42 || got.Telegram.APIHash != "secret" || got.I18n.DefaultLanguage != "en-US" {
		t.Fatalf("round trip mismatch: %+v", got.Telegram)
	}
}
