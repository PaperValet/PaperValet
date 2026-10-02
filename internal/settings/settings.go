// Package settings stores plugin options declared with plugin.SettingsSpec.
// Values live in data/<plugin>/settings.json; the companion bot edits them.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// FileName is the per-plugin settings file inside data/<plugin>/.
const FileName = "settings.json"

var (
	keyRe    = regexp.MustCompile(`^[a-z0-9_]{1,24}$`)
	pluginRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,24}$`)
)

// Registry holds every registered settings spec.
type Registry struct {
	root   string
	mu     sync.RWMutex
	stores map[string]*Store
}

// NewRegistry keeps settings under root/<plugin>/settings.json.
func NewRegistry(root string) *Registry {
	return &Registry{root: root, stores: map[string]*Store{}}
}

// Register validates spec, loads saved values and returns the store. A
// plugin registers once per load; unloading it frees the name again.
func (r *Registry) Register(spec *plugin.SettingsSpec) (*Store, error) {
	if err := Validate(spec); err != nil {
		return nil, err
	}
	s := &Store{spec: spec, path: filepath.Join(r.root, spec.Plugin, FileName), values: map[string]any{}}
	if err := s.load(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, taken := r.stores[spec.Plugin]; taken {
		return nil, fmt.Errorf("settings %s: already registered", spec.Plugin)
	}
	r.stores[spec.Plugin] = s
	return s, nil
}

// Unregister drops a plugin's spec. Saved values stay on disk.
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	delete(r.stores, name)
	r.mu.Unlock()
}

// Get returns a plugin's store.
func (r *Registry) Get(name string) (*Store, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.stores[name]
	return s, ok
}

// Names lists plugins with settings, sorted.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.stores))
	for n := range r.stores {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Validate checks a spec before it is registered.
func Validate(spec *plugin.SettingsSpec) error {
	if spec == nil {
		return errors.New("settings: nil spec")
	}
	if !pluginRe.MatchString(spec.Plugin) {
		return fmt.Errorf("settings: invalid plugin name %q", spec.Plugin)
	}
	if len(spec.Settings) == 0 {
		return fmt.Errorf("settings %s: no settings", spec.Plugin)
	}
	seen := map[string]bool{}
	for i := range spec.Settings {
		st := &spec.Settings[i]
		if !keyRe.MatchString(st.Key) {
			return fmt.Errorf("settings %s: invalid key %q", spec.Plugin, st.Key)
		}
		if seen[st.Key] {
			return fmt.Errorf("settings %s: duplicate key %q", spec.Plugin, st.Key)
		}
		seen[st.Key] = true
		if st.Label == "" {
			return fmt.Errorf("settings %s.%s: empty label", spec.Plugin, st.Key)
		}
		if _, err := coerce(st, st.Default); err != nil && st.Default != nil {
			return fmt.Errorf("settings %s.%s: default: %w", spec.Plugin, st.Key, err)
		}
		if st.Kind == plugin.SettingChoice {
			if len(st.Choices) == 0 {
				return fmt.Errorf("settings %s.%s: choice without choices", spec.Plugin, st.Key)
			}
			if len(st.Choices) > 50 {
				return fmt.Errorf("settings %s.%s: too many choices", spec.Plugin, st.Key)
			}
		}
	}
	return nil
}

// Store is one plugin's settings. It implements plugin.Settings.
type Store struct {
	spec   *plugin.SettingsSpec
	path   string
	mu     sync.RWMutex
	values map[string]any
}

var _ plugin.Settings = (*Store)(nil)

// Spec returns the declared spec.
func (s *Store) Spec() *plugin.SettingsSpec { return s.spec }

// Setting returns the declaration of key.
func (s *Store) Setting(key string) (*plugin.Setting, bool) {
	for i := range s.spec.Settings {
		if s.spec.Settings[i].Key == key {
			return &s.spec.Settings[i], true
		}
	}
	return nil, false
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("%s: %w", s.path, err)
	}
	for k, v := range raw {
		st, ok := s.Setting(k)
		if !ok {
			continue
		}
		if cv, err := coerce(st, v); err == nil {
			s.values[k] = cv
		}
	}
	return nil
}

func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.values, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Get returns the value of key or its default.
func (s *Store) Get(key string) any {
	st, ok := s.Setting(key)
	if !ok {
		return nil
	}
	s.mu.RLock()
	v, set := s.values[key]
	s.mu.RUnlock()
	if set {
		return v
	}
	d, _ := coerce(st, st.Default)
	return d
}

// IsDefault reports whether key still has its default value.
func (s *Store) IsDefault(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, set := s.values[key]
	return !set
}

func (s *Store) Bool(key string) bool {
	v, _ := s.Get(key).(bool)
	return v
}

func (s *Store) String(key string) string {
	v, _ := s.Get(key).(string)
	return v
}

func (s *Store) Int(key string) int {
	v, _ := s.Get(key).(int)
	return v
}

// Set stores a value of the setting's type and runs OnChange.
func (s *Store) Set(key string, value any) error {
	st, ok := s.Setting(key)
	if !ok {
		return fmt.Errorf("unknown setting %q", key)
	}
	v, err := coerce(st, value)
	if err != nil {
		return err
	}
	if err := checkValue(st, v); err != nil {
		return err
	}
	s.mu.Lock()
	s.values[key] = v
	err = s.save()
	s.mu.Unlock()
	if err != nil {
		return err
	}
	s.changed(key)
	return nil
}

// SetText parses typed text for a text or number setting, runs Validate
// and stores it. The error text is meant for the owner.
func (s *Store) SetText(key, text string) error {
	st, ok := s.Setting(key)
	if !ok {
		return fmt.Errorf("unknown setting %q", key)
	}
	text = strings.TrimSpace(text)
	switch st.Kind {
	case plugin.SettingNumber:
		n, err := strconv.Atoi(text)
		if err != nil {
			return plugin.Invalid("不是整数", "not a whole number")
		}
		return s.Set(key, n)
	case plugin.SettingText:
		if st.Validate != nil {
			norm, err := st.Validate(text)
			if err != nil {
				return err
			}
			text = norm
		}
		return s.Set(key, text)
	}
	return fmt.Errorf("setting %q is not typed", key)
}

// Reset restores the default value.
func (s *Store) Reset(key string) error {
	if _, ok := s.Setting(key); !ok {
		return fmt.Errorf("unknown setting %q", key)
	}
	s.mu.Lock()
	delete(s.values, key)
	err := s.save()
	s.mu.Unlock()
	if err != nil {
		return err
	}
	s.changed(key)
	return nil
}

func (s *Store) changed(key string) {
	if s.spec.OnChange != nil {
		s.spec.OnChange(key)
	}
}

// coerce converts v (possibly decoded JSON) into the Kind's Go type.
func coerce(st *plugin.Setting, v any) (any, error) {
	switch st.Kind {
	case plugin.SettingToggle:
		switch x := v.(type) {
		case bool:
			return x, nil
		case nil:
			return false, nil
		}
	case plugin.SettingChoice, plugin.SettingText:
		switch x := v.(type) {
		case string:
			return x, nil
		case nil:
			return "", nil
		}
	case plugin.SettingNumber:
		switch x := v.(type) {
		case int:
			return x, nil
		case int64:
			return int(x), nil
		case float64:
			if x == float64(int(x)) {
				return int(x), nil
			}
		case nil:
			return 0, nil
		}
	default:
		return nil, fmt.Errorf("unknown kind %d", st.Kind)
	}
	return nil, fmt.Errorf("wrong type %T", v)
}

func checkValue(st *plugin.Setting, v any) error {
	switch st.Kind {
	case plugin.SettingChoice:
		for _, c := range st.Choices {
			if c.Value == v {
				return nil
			}
		}
		return plugin.Invalid(fmt.Sprintf("%q 不在选项里", v), fmt.Sprintf("%q is not a choice", v))
	case plugin.SettingNumber:
		n := v.(int)
		if st.Min != 0 || st.Max != 0 {
			if n < st.Min || n > st.Max {
				return plugin.Invalid(fmt.Sprintf("要在 %d 到 %d 之间", st.Min, st.Max), fmt.Sprintf("must be between %d and %d", st.Min, st.Max))
			}
		}
	case plugin.SettingText:
		if len(v.(string)) > 1024 {
			return plugin.Invalid("太长了", "too long")
		}
	}
	return nil
}
