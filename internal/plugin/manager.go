package plugin

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"

	"github.com/TiaraBasori/PaperValet/internal/command"
	"github.com/TiaraBasori/PaperValet/internal/eventbus"
	"github.com/TiaraBasori/PaperValet/pkg/logger"
)

type Status int

const (
	StatusInactive Status = iota
	StatusActive
	StatusError
)

// Plugin is the minimal interface every plugin implements.
type Plugin interface {
	Name() string
	Description() string
	Init(ctx context.Context, mgr *Manager) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

type Info struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      Status `json:"status"`
	Error       string `json:"error,omitempty"`
}

type Manager struct {
	mu         sync.RWMutex
	plugins    map[string]Plugin
	info       map[string]*Info
	commandReg *command.Registry
	bus        *eventbus.Bus
	startOrder []string
	logger     *zap.Logger
}

func NewManager(cmdReg *command.Registry, bus *eventbus.Bus) *Manager {
	return &Manager{
		plugins:    make(map[string]Plugin),
		info:       make(map[string]*Info),
		commandReg: cmdReg,
		bus:        bus,
		startOrder: make([]string, 0),
		logger:     logger.Named("plugin"),
	}
}

func (m *Manager) Register(p Plugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	name := p.Name()
	if name == "" {
		return fmt.Errorf("plugin name is required")
	}
	if _, exists := m.plugins[name]; exists {
		return fmt.Errorf("plugin %s already registered", name)
	}
	m.plugins[name] = p
	m.info[name] = &Info{Name: name, Description: p.Description(), Status: StatusInactive}
	m.startOrder = append(m.startOrder, name)
	m.logger.Info("registered", zap.String("name", name))
	return nil
}

func (m *Manager) InitAll(ctx context.Context) error {
	m.mu.RLock()
	order := append([]string(nil), m.startOrder...)
	m.mu.RUnlock()
	for _, name := range order {
		m.mu.RLock()
		p := m.plugins[name]
		m.mu.RUnlock()
		m.logger.Info("init", zap.String("name", name))
		if err := p.Init(ctx, m); err != nil {
			m.mu.Lock()
			m.info[name].Status = StatusError
			m.info[name].Error = err.Error()
			m.mu.Unlock()
			return fmt.Errorf("plugin %s init: %w", name, err)
		}
	}
	return nil
}

func (m *Manager) StartAll(ctx context.Context) error {
	m.mu.RLock()
	order := append([]string(nil), m.startOrder...)
	m.mu.RUnlock()
	for _, name := range order {
		p := m.plugins[name]
		m.logger.Info("start", zap.String("name", name))
		if err := p.Start(ctx); err != nil {
			m.mu.Lock()
			m.info[name].Status = StatusError
			m.info[name].Error = err.Error()
			m.mu.Unlock()
			return fmt.Errorf("plugin %s start: %w", name, err)
		}
		m.mu.Lock()
		m.info[name].Status = StatusActive
		m.mu.Unlock()
	}
	return nil
}

func (m *Manager) StopAll(ctx context.Context) error {
	m.mu.RLock()
	order := append([]string(nil), m.startOrder...)
	m.mu.RUnlock()
	for i := len(order) - 1; i >= 0; i-- {
		name := order[i]
		p := m.plugins[name]
		m.logger.Info("stop", zap.String("name", name))
		if err := p.Stop(ctx); err != nil {
			m.logger.Error("stop failed", zap.String("name", name), zap.Error(err))
		}
		m.commandReg.UnregisterPlugin(name)
		m.mu.Lock()
		m.info[name].Status = StatusInactive
		m.mu.Unlock()
	}
	return nil
}

func (m *Manager) GetPlugin(name string) (Plugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.plugins[name]
	return p, ok
}

func (m *Manager) GetInfo(name string) (*Info, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	i, ok := m.info[name]
	return i, ok
}

func (m *Manager) GetAllInfo() []*Info {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Info, 0, len(m.info))
	for _, name := range m.startOrder {
		if i, ok := m.info[name]; ok {
			out = append(out, i)
		}
	}
	return out
}

func (m *Manager) Bus() *eventbus.Bus             { return m.bus }
func (m *Manager) Commands() *command.Registry     { return m.commandReg }
func (m *Manager) RegisterCommand(cmd *command.Command) error {
	return m.commandReg.Register(cmd)
}
