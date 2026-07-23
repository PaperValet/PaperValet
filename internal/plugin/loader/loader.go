package loader

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	goplugin "plugin"
	"strings"

	pkgplugin "github.com/TiaraBasori/PaperValet/pkg/plugin"
	"github.com/TiaraBasori/PaperValet/pkg/logger"
)

// Loader loads external plugins from .so files.
type Loader struct {
	dir     string
	manager pkgplugin.Manager
	loaded  map[string]*LoadedPlugin
	logger  pkgplugin.Logger
}

type LoadedPlugin struct {
	Plugin   pkgplugin.Plugin
	Handle   *goplugin.Plugin
	Path     string
	Metadata *pkgplugin.PluginMetadata
}

// NewLoader creates a new plugin loader.
func NewLoader(dir string, mgr pkgplugin.Manager) *Loader {
	return &Loader{
		dir:     dir,
		manager: mgr,
		loaded:  make(map[string]*LoadedPlugin),
		logger:  logger.NamedLogger("plugin_loader"),
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

// Load loads a single plugin from a .so file.
func (l *Loader) Load(ctx context.Context, path string) error {
	name := strings.TrimSuffix(filepath.Base(path), ".so")

	if _, ok := l.loaded[name]; ok {
		return fmt.Errorf("plugin %s already loaded", name)
	}

	p, err := goplugin.Open(path)
	if err != nil {
		return fmt.Errorf("open plugin: %w", err)
	}

	newSymbol, err := p.Lookup("New")
	if err != nil {
		return fmt.Errorf("plugin missing New function: %w", err)
	}

	var instance interface{}
	var plug pkgplugin.Plugin

	// Try New() (plugin.Plugin, error) signature first
	if newFunc, ok := newSymbol.(func() (pkgplugin.Plugin, error)); ok {
		var err error
		instance, err = newFunc()
		if err != nil {
			return fmt.Errorf("plugin New failed: %w", err)
		}
		plug, _ = instance.(pkgplugin.Plugin)
	} else if newFunc, ok := newSymbol.(func() interface{}); ok {
		// Fallback to New() interface{} signature
		instance = newFunc()
		plug, _ = instance.(pkgplugin.Plugin)
	} else {
		return fmt.Errorf("New symbol has wrong type")
	}

	if plug == nil {
		return fmt.Errorf("plugin does not implement plugin.Plugin interface")
	}

	var meta *pkgplugin.PluginMetadata
	if metaSym, err := p.Lookup("Metadata"); err == nil {
		if m, ok := metaSym.(*pkgplugin.PluginMetadata); ok {
			meta = m
		}
	}

	if err := l.manager.RegisterPlugin(plug); err != nil {
		return fmt.Errorf("register plugin: %w", err)
	}

	if err := plug.Init(ctx, l.manager); err != nil {
		return fmt.Errorf("init plugin: %w", err)
	}

	if err := plug.Start(ctx); err != nil {
		return fmt.Errorf("start plugin: %w", err)
	}

	loaded := &LoadedPlugin{
		Plugin:   plug,
		Handle:   p,
		Path:     path,
		Metadata: meta,
	}
	l.loaded[name] = loaded

	l.logger.Info("plugin loaded", "name", name, "path", path)
	return nil
}

// Unload unloads a plugin by name.
func (l *Loader) Unload(ctx context.Context, name string) error {
	loaded, ok := l.loaded[name]
	if !ok {
		return fmt.Errorf("plugin %s not loaded", name)
	}

	if err := loaded.Plugin.Stop(ctx); err != nil {
		l.logger.Warn("plugin stop error", "name", name, "error", err)
	}

	delete(l.loaded, name)
	l.logger.Info("plugin unloaded", "name", name)
	return nil
}

// GetLoaded returns all loaded plugins.
func (l *Loader) GetLoaded() map[string]*LoadedPlugin {
	result := make(map[string]*LoadedPlugin, len(l.loaded))
	for k, v := range l.loaded {
		result[k] = v
	}
	return result
}

// GetPluginDir returns the plugin directory.
func (l *Loader) GetPluginDir() string {
	return l.dir
}