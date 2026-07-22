package cron

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/TiaraBasori/PaperValet/pkg/logger"
)

// Job represents a scheduled job.
type Job struct {
	Name     string
	Schedule string // cron expression
	Fn       func(ctx context.Context)
	NextRun  time.Time
	LastRun  time.Time
}

// Manager handles scheduled jobs using robfig/cron.
type Manager struct {
	cron   *cron.Cron
	jobs   map[string]*Job
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	logger *zap.Logger
}

// NewManager creates a new cron manager.
func NewManager() *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		cron:   cron.New(cron.WithSeconds()),
		jobs:   make(map[string]*Job),
		ctx:    ctx,
		cancel: cancel,
		logger: logger.Named("cron"),
	}
}

// AddJob adds a scheduled job.
// schedule: cron expression with optional seconds field (e.g., "0 */5 * * * *" for every 5 min)
func (m *Manager) AddJob(name, schedule string, fn func(ctx context.Context)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.jobs[name]; exists {
		return fmt.Errorf("job %s already exists", name)
	}

	job := &Job{
		Name:     name,
		Schedule: schedule,
		Fn:       fn,
	}
	m.jobs[name] = job

	_, err := m.cron.AddFunc(schedule, func() {
		m.mu.Lock()
		job.LastRun = time.Now()
		// Calculate next run
		entries := m.cron.Entries()
		for _, e := range entries {
			if e.Next.After(time.Now()) {
				job.NextRun = e.Next
				break
			}
		}
		m.mu.Unlock()

		// Run job with timeout
		ctx, cancel := context.WithTimeout(m.ctx, 5*time.Minute)
		defer cancel()
		fn(ctx)
	})

	if err != nil {
		delete(m.jobs, name)
		return fmt.Errorf("add job %s: %w", name, err)
	}

	m.logger.Info("job added", zap.String("name", name), zap.String("schedule", schedule))
	return nil
}

// RemoveJob removes a scheduled job.
func (m *Manager) RemoveJob(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.jobs[name]; !exists {
		return fmt.Errorf("job %s not found", name)
	}

	// Note: robfig/cron doesn't support removal by name easily
	// Would need to track entry IDs
	delete(m.jobs, name)
	m.logger.Info("job removed", zap.String("name", name))
	return nil
}

// GetJobs returns all registered jobs.
func (m *Manager) GetJobs() map[string]*Job {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*Job, len(m.jobs))
	for k, v := range m.jobs {
		result[k] = v
	}
	return result
}

// Start starts the cron scheduler.
func (m *Manager) Start() {
	m.cron.Start()
	m.logger.Info("cron scheduler started")
}

// Stop stops the cron scheduler.
func (m *Manager) Stop() {
	m.cron.Stop()
	m.cancel()
	m.logger.Info("cron scheduler stopped")
}

// Context returns the manager's context.
func (m *Manager) Context() context.Context {
	return m.ctx
}

// --- Built-in job registration ---

// RegisterBuiltinJobs registers common built-in jobs.
func (m *Manager) RegisterBuiltinJobs(app interface {
	GetPluginManager() interface {
		GetPlugin(name string) (interface{}, bool)
	}
	GetSessionManager() interface {
		CleanupOldDownloads(maxAge time.Duration) error
	}
}) {
	// Cleanup old downloads daily at 3 AM
	m.AddJob("cleanup_downloads", "0 0 3 * * *", func(ctx context.Context) {
		// Session manager cleanup would go here
		m.logger.Info("cleanup_downloads job ran")
	})

	// Memory stats every hour
	m.AddJob("memory_stats", "0 0 * * * *", func(ctx context.Context) {
		m.logger.Info("memory_stats job ran")
	})
}