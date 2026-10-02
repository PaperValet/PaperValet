package settings

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

func spec(onChange func(string)) *plugin.SettingsSpec {
	return &plugin.SettingsSpec{
		Plugin: "demo",
		Settings: []plugin.Setting{
			{Key: "on", Label: "On", Kind: plugin.SettingToggle, Default: true},
			{Key: "mode", Label: "Mode", Kind: plugin.SettingChoice, Default: "a",
				Choices: []plugin.Choice{{Value: "a"}, {Value: "b"}}},
			{Key: "name", Label: "Name", Kind: plugin.SettingText, Default: "x",
				Validate: func(s string) (string, error) {
					if s == "bad" {
						return "", errors.New("nope")
					}
					return strings.ToUpper(s), nil
				}},
			{Key: "n", Label: "N", Kind: plugin.SettingNumber, Default: 5, Min: 1, Max: 10},
		},
		OnChange: onChange,
	}
}

func TestDefaultsSetPersistReset(t *testing.T) {
	root := t.TempDir()
	var changed []string
	reg := NewRegistry(root)
	st, err := reg.Register(spec(func(k string) { changed = append(changed, k) }))
	if err != nil {
		t.Fatal(err)
	}
	if !st.Bool("on") || st.String("mode") != "a" || st.String("name") != "x" || st.Int("n") != 5 {
		t.Fatal("defaults not returned")
	}
	if err := st.Set("on", false); err != nil {
		t.Fatal(err)
	}
	if err := st.Set("mode", "c"); err == nil {
		t.Fatal("unknown choice accepted")
	}
	if err := st.SetText("name", "hi"); err != nil || st.String("name") != "HI" {
		t.Fatalf("validate/normalize: %v %q", err, st.String("name"))
	}
	if err := st.SetText("name", "bad"); err == nil || err.Error() != "nope" {
		t.Fatalf("validator error lost: %v", err)
	}
	if err := st.SetText("n", "11"); err == nil {
		t.Fatal("out of range accepted")
	}
	if err := st.SetText("n", "7"); err != nil {
		t.Fatal(err)
	}
	if err := st.Set("n", "7"); err == nil {
		t.Fatal("wrong type accepted")
	}

	info, err := os.Stat(filepath.Join(root, "demo", FileName))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("perm %o", info.Mode().Perm())
	}

	// A fresh registry reads the saved values back (JSON numbers too).
	st2, err := NewRegistry(root).Register(spec(nil))
	if err != nil {
		t.Fatal(err)
	}
	if st2.Bool("on") || st2.String("name") != "HI" || st2.Int("n") != 7 {
		t.Fatalf("not persisted: %v %q %d", st2.Bool("on"), st2.String("name"), st2.Int("n"))
	}
	if err := st2.Reset("on"); err != nil || !st2.Bool("on") || !st2.IsDefault("on") {
		t.Fatal("reset failed")
	}
	if strings.Join(changed, ",") != "on,name,n" {
		t.Fatalf("OnChange calls = %v", changed)
	}
}

func TestValidateRejectsBadSpecs(t *testing.T) {
	bad := []*plugin.SettingsSpec{
		nil,
		{Plugin: "x y", Settings: []plugin.Setting{{Key: "a", Label: "A"}}},
		{Plugin: "x"},
		{Plugin: "x", Settings: []plugin.Setting{{Key: "A", Label: "A"}}},
		{Plugin: "x", Settings: []plugin.Setting{{Key: "a", Label: "A"}, {Key: "a", Label: "B"}}},
		{Plugin: "x", Settings: []plugin.Setting{{Key: "a"}}},
		{Plugin: "x", Settings: []plugin.Setting{{Key: "a", Label: "A", Kind: plugin.SettingChoice}}},
		{Plugin: "x", Settings: []plugin.Setting{{Key: "a", Label: "A", Kind: plugin.SettingToggle, Default: "yes"}}},
	}
	for i, s := range bad {
		if err := Validate(s); err == nil {
			t.Errorf("spec %d accepted", i)
		}
	}
}

func TestUnregisterKeepsValues(t *testing.T) {
	root := t.TempDir()
	reg := NewRegistry(root)
	st, _ := reg.Register(spec(nil))
	_ = st.Set("mode", "b")
	if _, err := reg.Register(spec(nil)); err == nil {
		t.Fatal("second registration accepted")
	}
	reg.Unregister("demo")
	if _, ok := reg.Get("demo"); ok || len(reg.Names()) != 0 {
		t.Fatal("still registered")
	}
	st, _ = reg.Register(spec(nil))
	if st.String("mode") != "b" {
		t.Fatal("value lost across re-register")
	}
}
