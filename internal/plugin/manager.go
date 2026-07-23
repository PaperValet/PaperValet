package plugin

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/TiaraBasori/PaperValet/internal/command"
	"github.com/TiaraBasori/PaperValet/internal/eventbus"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
	"github.com/TiaraBasori/PaperValet/pkg/logger"
)

// Manager implements the plugin.Manager interface for internal use.
type Manager struct {
	mu       sync.RWMutex
	plugins  map[string]plugin.Plugin
	infos    map[string]plugin.PluginInfo
	commands *command.Registry
	bus      *eventbus.Bus
	loaded   bool
}

// NewManager creates a new plugin manager.
func NewManager(commands *command.Registry, bus *eventbus.Bus) *Manager {
	return &Manager{
		plugins:  make(map[string]plugin.Plugin),
		infos:    make(map[string]plugin.PluginInfo),
		commands: commands,
		bus:      bus,
	}
}

// RegisterPlugin registers a plugin with the manager.
func (m *Manager) RegisterPlugin(p plugin.Plugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	name := p.Name()
	if _, exists := m.plugins[name]; exists {
		return fmt.Errorf("plugin %s already registered", name)
	}
	m.plugins[name] = p
	m.infos[name] = plugin.PluginInfo{
		Name:        name,
		Description: p.Description(),
		Status:      plugin.StatusInactive,
	}
	return nil
}

// RegisterCommand registers a command via the command registry.
func (m *Manager) RegisterCommand(cmd *plugin.Command) error {
	return m.commands.Register(cmd)
}

// UnregisterCommand removes a command.
func (m *Manager) UnregisterCommand(name string) {
	m.commands.Unregister(name)
}

// UnregisterPlugin removes a plugin and its commands.
func (m *Manager) UnregisterPlugin(name string) {
	m.commands.UnregisterPlugin(name)
	m.mu.Lock()
	delete(m.plugins, name)
	delete(m.infos, name)
	m.mu.Unlock()
}

// InitPlugin initializes a single registered plugin.
func (m *Manager) InitPlugin(ctx context.Context, name string) error {
	m.mu.RLock()
	p, ok := m.plugins[name]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("plugin %s not registered", name)
	}
	if err := p.Init(ctx, m); err != nil {
		m.mu.Lock()
		m.infos[name] = plugin.PluginInfo{
			Name:        name,
			Description: p.Description(),
			Status:      plugin.StatusError,
		}
		m.mu.Unlock()
		return err
	}
	return nil
}

// StartPlugin starts a single plugin.
func (m *Manager) StartPlugin(ctx context.Context, name string) error {
	m.mu.RLock()
	p, ok := m.plugins[name]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("plugin %s not registered", name)
	}
	if err := p.Start(ctx); err != nil {
		m.mu.Lock()
		m.infos[name] = plugin.PluginInfo{
			Name:        name,
			Description: p.Description(),
			Status:      plugin.StatusError,
		}
		m.mu.Unlock()
		return err
	}
	m.mu.Lock()
	m.infos[name] = plugin.PluginInfo{
		Name:        name,
		Description: p.Description(),
		Status:      plugin.StatusActive,
	}
	m.mu.Unlock()
	return nil
}

// StopPlugin stops a single plugin.
func (m *Manager) StopPlugin(ctx context.Context, name string) error {
	m.mu.RLock()
	p, ok := m.plugins[name]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("plugin %s not registered", name)
	}
	if err := p.Stop(ctx); err != nil {
		return err
	}
	m.mu.Lock()
	m.infos[name] = plugin.PluginInfo{
		Name:        name,
		Description: p.Description(),
		Status:      plugin.StatusInactive,
	}
	m.mu.Unlock()
	return nil
}

// Commands returns the registry provider.
func (m *Manager) Commands() plugin.RegistryProvider {
	return m.commands
}

// GetInfo returns plugin info.
func (m *Manager) GetInfo(name string) (plugin.PluginInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	info, ok := m.infos[name]
	return info, ok
}

// GetAllInfo returns info for all plugins.
func (m *Manager) GetAllInfo() []plugin.PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	infos := make([]plugin.PluginInfo, 0, len(m.infos))
	for _, info := range m.infos {
		infos = append(infos, info)
	}
	return infos
}

// Emit sends an event through the event bus.
func (m *Manager) Emit(ctx context.Context, eventType string, data any) error {
	return m.bus.Emit(ctx, eventType, data)
}

// InitAll calls Init on all registered plugins. Collects errors from all
// plugins instead of aborting on the first failure.
func (m *Manager) InitAll(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var errs []string
	for name, p := range m.plugins {
		if err := p.Init(ctx, m); err != nil {
			m.infos[name] = plugin.PluginInfo{
				Name:        name,
				Description: p.Description(),
				Status:      plugin.StatusError,
			}
			errs = append(errs, fmt.Sprintf("plugin %s init: %s", name, err))
			continue
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("plugin init errors:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

// StartAll calls Start on all registered plugins. Collects errors from all
// plugins instead of aborting on the first failure.
func (m *Manager) StartAll(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var errs []string
	for name, p := range m.plugins {
		if err := p.Start(ctx); err != nil {
			m.infos[name] = plugin.PluginInfo{
				Name:        name,
				Description: p.Description(),
				Status:      plugin.StatusError,
			}
			errs = append(errs, fmt.Sprintf("plugin %s start: %s", name, err))
			continue
		}
		m.infos[name] = plugin.PluginInfo{
			Name:        name,
			Description: p.Description(),
			Status:      plugin.StatusActive,
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("plugin start errors:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

// StopAll calls Stop on all registered plugins.
func (m *Manager) StopAll(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for name, p := range m.plugins {
		if err := p.Stop(ctx); err != nil {
			logger.NamedLogger("plugin").Error("plugin stop error", "name", name, "error", err)
		}
		m.infos[name] = plugin.PluginInfo{
			Name:        name,
			Description: p.Description(),
			Status:      plugin.StatusInactive,
		}
	}
	return nil
}