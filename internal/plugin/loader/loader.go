package loader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	goplugin "plugin"
	"strings"
	"time"

	pkgplugin "github.com/TiaraBasori/PaperValet/pkg/plugin"
	"github.com/TiaraBasori/PaperValet/pkg/logger"
)

// Loader loads/unloads/manages external plugins from .so files.
type Loader struct {
	dir     string
	manager pkgplugin.Manager
	loaded  map[string]*LoadedPlugin
	logger  pkgplugin.Logger
	http    *http.Client
	repoURL string // base URL for plugin registry
}

// LoadedPlugin represents a loaded .so plugin.
type LoadedPlugin struct {
	Plugin   pkgplugin.Plugin
	Handle   *goplugin.Plugin
	Path     string
	Metadata *pkgplugin.PluginMetadata
	LoadedAt time.Time
}

// NewLoader creates a new plugin loader.
func NewLoader(dir string, mgr pkgplugin.Manager) *Loader {
	return &Loader{
		dir:     dir,
		manager: mgr,
		loaded:  make(map[string]*LoadedPlugin),
		logger:  logger.NamedLogger("plugin_loader"),
		http: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				ResponseHeaderTimeout: 10 * time.Second,
			},
		},
		repoURL: "https://github.com/TiaraBasori/PaperValet-Plugins/releases/latest/download",
	}
}

// LoadAll loads all .so plugins from the plugins directory.
func (l *Loader) LoadAll(ctx context.Context) error {
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		if os.IsNotExist(err) {
			l.logger.Info("plugins directory does not exist, skipping", "dir", l.dir)
			return nil
		}
		return fmt.Errorf("read plugins dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".so") {
			continue
		}
		path := filepath.Join(l.dir, entry.Name())
		if err := l.Load(ctx, path); err != nil {
			l.logger.Error("failed to load plugin", "path", path, "error", err)
			continue
		}
	}
	l.logger.Info("external plugins loaded", "count", len(l.loaded))
	return nil
}

// Load loads a single plugin from a .so file path.
func (l *Loader) Load(ctx context.Context, path string) error {
	name := strings.TrimSuffix(filepath.Base(path), ".so")

	if _, ok := l.loaded[name]; ok {
		return fmt.Errorf("plugin %s already loaded", name)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("plugin file not found: %s", path)
	}

	p, err := goplugin.Open(path)
	if err != nil {
		return fmt.Errorf("open plugin: %w", err)
	}

	// Lookup New() function
	newSymbol, err := p.Lookup("New")
	if err != nil {
		return fmt.Errorf("plugin missing New function: %w", err)
	}

	var instance interface{}
	var plug pkgplugin.Plugin

	if newFunc, ok := newSymbol.(func() (pkgplugin.Plugin, error)); ok {
		instance, err = newFunc()
		if err != nil {
			return fmt.Errorf("plugin New failed: %w", err)
		}
		plug, _ = instance.(pkgplugin.Plugin)
	} else if newFunc, ok := newSymbol.(func() interface{}); ok {
		instance = newFunc()
		plug, _ = instance.(pkgplugin.Plugin)
	} else {
		return fmt.Errorf("New symbol has wrong type (expected func()(Plugin, error) or func()interface{})")
	}

	if plug == nil {
		return fmt.Errorf("plugin does not implement plugin.Plugin interface")
	}

	// Optional metadata
	var meta *pkgplugin.PluginMetadata
	if metaSym, err := p.Lookup("Metadata"); err == nil {
		if m, ok := metaSym.(*pkgplugin.PluginMetadata); ok {
			meta = m
		}
	}

	// Register & init
	if err := l.manager.RegisterPlugin(plug); err != nil {
		return fmt.Errorf("register plugin: %w", err)
	}
	if err := plug.Init(ctx, l.manager); err != nil {
		l.manager.UnregisterPlugin(plug.Name())
		return fmt.Errorf("init plugin: %w", err)
	}
	if err := plug.Start(ctx); err != nil {
		l.manager.UnregisterPlugin(plug.Name())
		return fmt.Errorf("start plugin: %w", err)
	}

	l.loaded[name] = &LoadedPlugin{
		Plugin:   plug,
		Handle:   p,
		Path:     path,
		Metadata: meta,
		LoadedAt: time.Now(),
	}
	l.logger.Info("plugin loaded", "name", name, "path", path)
	return nil
}

// LoadByName loads a plugin by name from the plugins directory.
func (l *Loader) LoadByName(ctx context.Context, name string) error {
	if !strings.HasSuffix(name, ".so") {
		name = name + ".so"
	}
	path := filepath.Join(l.dir, name)
	return l.Load(ctx, path)
}

// Unload unloads a plugin by name (stops it, unregisters commands, removes from manager).
func (l *Loader) Unload(ctx context.Context, name string) error {
	loaded, ok := l.loaded[name]
	if !ok {
		return fmt.Errorf("plugin %s not loaded", name)
	}

	if err := loaded.Plugin.Stop(ctx); err != nil {
		l.logger.Warn("plugin stop error", "name", name, "error", err)
	}
	l.manager.UnregisterPlugin(name)
	delete(l.loaded, name)
	l.logger.Info("plugin unloaded", "name", name)
	return nil
}

// Install downloads a plugin .so from the registry and places it in the plugins dir.
func (l *Loader) Install(ctx context.Context, name string) error {
	if strings.HasSuffix(name, ".so") {
		name = strings.TrimSuffix(name, ".so")
	}

	dest := filepath.Join(l.dir, name+".so")
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("plugin %s already installed at %s", name, dest)
	}

	url := fmt.Sprintf("%s/%s.so", l.repoURL, name)
	l.logger.Info("downloading plugin", "name", name, "url", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := l.http.Do(req)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d (plugin %s may not exist)", resp.StatusCode, name)
	}

	if err := os.MkdirAll(l.dir, 0o755); err != nil {
		return fmt.Errorf("create plugins dir: %w", err)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	written, err := io.Copy(f, resp.Body)
	if err != nil {
		os.Remove(dest)
		return fmt.Errorf("write plugin: %w", err)
	}
	l.logger.Info("plugin downloaded", "name", name, "size", written, "path", dest)
	return nil
}

// Remove deletes a plugin .so file from the plugins directory.
func (l *Loader) Remove(ctx context.Context, name string) error {
	if strings.HasSuffix(name, ".so") {
		name = strings.TrimSuffix(name, ".so")
	}

	// Unload if loaded
	if _, ok := l.loaded[name]; ok {
		if err := l.Unload(ctx, name); err != nil {
			l.logger.Warn("unload before remove", "name", name, "error", err)
		}
	}

	path := filepath.Join(l.dir, name+".so")
	if err := os.Remove(path); os.IsNotExist(err) {
		return fmt.Errorf("plugin %s not found at %s", name, path)
	} else if err != nil {
		return fmt.Errorf("remove plugin file: %w", err)
	}
	l.logger.Info("plugin removed", "name", name)
	return nil
}

// GetLoaded returns all currently loaded plugins.
func (l *Loader) GetLoaded() map[string]*LoadedPlugin {
	result := make(map[string]*LoadedPlugin, len(l.loaded))
	for k, v := range l.loaded {
		result[k] = v
	}
	return result
}

// IsLoaded checks if a plugin is loaded.
func (l *Loader) IsLoaded(name string) bool {
	_, ok := l.loaded[name]
	return ok
}

// GetAvailable returns .so files in the plugins dir that are NOT loaded.
func (l *Loader) GetAvailable() ([]string, error) {
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var available []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".so") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".so")
		if _, ok := l.loaded[name]; !ok {
			available = append(available, name)
		}
	}
	return available, nil
}

// GetInstalled returns all .so files in the plugins dir.
func (l *Loader) GetInstalled() ([]string, error) {
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var installed []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".so") {
			continue
		}
		installed = append(installed, strings.TrimSuffix(e.Name(), ".so"))
	}
	return installed, nil
}

// GetPluginDir returns the plugin directory path.
func (l *Loader) GetPluginDir() string {
	return l.dir
}

// SetRepoURL sets the plugin registry URL.
func (l *Loader) SetRepoURL(url string) {
	l.repoURL = url
}