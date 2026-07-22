package loader

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	goplugin "plugin"
	"strings"

	"github.com/TiaraBasori/PaperValet/internal/plugin"
	"go.uber.org/zap"

	"github.com/TiaraBasori/PaperValet/pkg/logger"
)

// Loader loads external plugins from .so files.
type Loader struct {
	dir     string
	manager *plugin.Manager
	loaded  map[string]*LoadedPlugin
	logger  *zap.Logger
}

type LoadedPlugin struct {
	Plugin   plugin.Plugin
	Handle   *goplugin.Plugin
	Path     string
	Metadata *PluginMetadata
}

type PluginMetadata struct {
	Name        string
	Description string
	Version     string
	Author      string
	MinVersion  string
}

// NewLoader creates a new plugin loader.
func NewLoader(dir string, mgr *plugin.Manager) *Loader {
	return &Loader{
		dir:     dir,
		manager: mgr,
		loaded:  make(map[string]*LoadedPlugin),
		logger:  logger.Named("plugin_loader"),
	}
}

// LoadAll loads all .so plugins from the plugins directory.
func (l *Loader) LoadAll(ctx context.Context) error {
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		if os.IsNotExist(err) {
			l.logger.Info("plugins directory does not exist, skipping", zap.String("dir", l.dir))
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
			l.logger.Error("failed to load plugin", zap.String("path", path), zap.Error(err))
			continue
		}
	}

	l.logger.Info("external plugins loaded", zap.Int("count", len(l.loaded)))
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

	newFunc, ok := newSymbol.(func() interface{})
	if !ok {
		return fmt.Errorf("New symbol has wrong type")
	}

	instance := newFunc()

	plug, ok := instance.(plugin.Plugin)
	if !ok {
		return fmt.Errorf("plugin does not implement plugin.Plugin interface")
	}

	var meta *PluginMetadata
	if metaSym, err := p.Lookup("Metadata"); err == nil {
		if m, ok := metaSym.(*PluginMetadata); ok {
			meta = m
		}
	}

	if err := l.manager.Register(plug); err != nil {
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

	l.logger.Info("plugin loaded", zap.String("name", name), zap.String("path", path))
	return nil
}

// Unload unloads a plugin by name.
func (l *Loader) Unload(ctx context.Context, name string) error {
	loaded, ok := l.loaded[name]
	if !ok {
		return fmt.Errorf("plugin %s not loaded", name)
	}

	if err := loaded.Plugin.Stop(ctx); err != nil {
		l.logger.Warn("plugin stop error", zap.String("name", name), zap.Error(err))
	}

	delete(l.loaded, name)
	l.logger.Info("plugin unloaded", zap.String("name", name))
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