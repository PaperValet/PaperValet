package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHomeDefault(t *testing.T) {
	t.Setenv("PAPERVALET_HOME", "")
	t.Setenv("HOME", "/tmp/pv-home-test")
	got, err := Home()
	if err != nil {
		t.Fatal(err)
	}
	if want := "/tmp/pv-home-test/.papervalet"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestHomeOverride(t *testing.T) {
	t.Setenv("HOME", "/tmp/pv-home-test")
	t.Setenv("PAPERVALET_HOME", "~/bot2")
	got, err := Home()
	if err != nil {
		t.Fatal(err)
	}
	if want := "/tmp/pv-home-test/bot2"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}

	t.Setenv("PAPERVALET_HOME", "rel")
	got, err = Home()
	if err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	if want := filepath.Join(wd, "rel"); got != want {
		t.Fatalf("relative: got %q want %q", got, want)
	}
}

func TestCommandName(t *testing.T) {
	t.Setenv("PAPERVALET_CMD", "pv2")
	if got := CommandName(); got != "pv2" {
		t.Fatalf("env: got %q", got)
	}
	t.Setenv("PAPERVALET_CMD", "")
	old := os.Args[0]
	defer func() { os.Args[0] = old }()
	os.Args[0] = "/opt/x/bin/papervalet.exe"
	if got := CommandName(); got != "papervalet" {
		t.Fatalf("argv0: got %q", got)
	}
}

func TestDefaultLoadsBack(t *testing.T) {
	cfg := Default()
	cfg.Telegram.APIID = 1
	cfg.Telegram.APIHash = "0123456789abcdef0123456789abcdef"
	path := filepath.Join(t.TempDir(), FileName)
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := got.Validate(); err != nil {
		t.Fatal(err)
	}
	if got.Telegram.SessionFile != "session.json" || got.Bot.PluginsDir != "plugins" {
		t.Fatalf("relative paths changed: %+v", got.Telegram)
	}
}
