package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/logger"
)

type Data map[string]any

type Record struct {
	UserID    int64
	ChatID    int64
	State     string
	Data      Data
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Manager manages session state with SQLite + in-memory cache.
type Manager struct {
	db      *sql.DB
	cache   map[string]*Record
	mu      sync.RWMutex
	ttl     time.Duration
	logger  interfaces.Logger
	cleanup *time.Ticker
	done    chan struct{}
}

// NewManager creates a new session manager.
func NewManager(dbPath string) (*Manager, error) {
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("failed to open session database: %w", err)
	}

	// Create tables
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			user_id INTEGER NOT NULL,
			chat_id INTEGER NOT NULL,
			state TEXT NOT NULL DEFAULT '',
			data TEXT NOT NULL DEFAULT '{}',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			PRIMARY KEY (user_id, chat_id)
		)
	`); err != nil {
		return nil, fmt.Errorf("failed to create sessions table: %w", err)
	}

	m := &Manager{
		db:      db,
		cache:   make(map[string]*Record),
		ttl:     24 * time.Hour,
		logger:  logger.NamedLogger("session_manager"),
		cleanup: time.NewTicker(1 * time.Hour),
		done:    make(chan struct{}),
	}

	go m.cleanupLoop()

	return m, nil
}

// GetOrCreate retrieves a session or creates a new one.
func (m *Manager) GetOrCreate(ctx context.Context, userID, chatID int64) (*Record, error) {
	key := m.key(userID, chatID)

	// Check cache first
	m.mu.RLock()
	if cached, ok := m.cache[key]; ok {
		m.mu.RUnlock()
		return cached, nil
	}
	m.mu.RUnlock()

	// Query database
	record, err := m.queryDB(ctx, userID, chatID)
	if err != nil {
		// Create new session
		record = &Record{
			UserID:    userID,
			ChatID:    chatID,
			State:     "",
			Data:      make(Data),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := m.insertDB(ctx, record); err != nil {
			return nil, err
		}
	}

	// Update cache
	m.mu.Lock()
	m.cache[key] = record
	m.mu.Unlock()

	return record, nil
}

// Save persists a session record.
func (m *Manager) Save(ctx context.Context, record *Record) error {
	record.UpdatedAt = time.Now()
	key := m.key(record.UserID, record.ChatID)

	// Update cache
	m.mu.Lock()
	m.cache[key] = record
	m.mu.Unlock()

	// Update database
	return m.updateDB(ctx, record)
}

// Delete removes a session record.
func (m *Manager) Delete(ctx context.Context, userID, chatID int64) error {
	key := m.key(userID, chatID)

	m.mu.Lock()
	delete(m.cache, key)
	m.mu.Unlock()

	_, err := m.db.ExecContext(ctx, "DELETE FROM sessions WHERE user_id = ? AND chat_id = ?", userID, chatID)
	return err
}

// GetDB returns the underlying database connection for advanced use.
func (m *Manager) GetDB() *sql.DB {
	return m.db
}

// Close closes the session manager.
func (m *Manager) Close() error {
	m.cleanup.Stop()
	close(m.done)

	m.mu.Lock()
	m.cache = nil
	m.mu.Unlock()

	return m.db.Close()
}

// --- Internal ---

func (m *Manager) key(userID, chatID int64) string {
	return fmt.Sprintf("%d:%d", userID, chatID)
}

func (m *Manager) queryDB(ctx context.Context, userID, chatID int64) (*Record, error) {
	row := m.db.QueryRowContext(ctx,
		"SELECT user_id, chat_id, state, data, created_at, updated_at FROM sessions WHERE user_id = ? AND chat_id = ?",
		userID, chatID,
	)

	var record Record
	var dataStr string
	var createdAt, updatedAt int64

	if err := row.Scan(&record.UserID, &record.ChatID, &record.State, &dataStr, &createdAt, &updatedAt); err != nil {
		return nil, err
	}

	record.CreatedAt = time.Unix(createdAt, 0)
	record.UpdatedAt = time.Unix(updatedAt, 0)

	if err := json.Unmarshal([]byte(dataStr), &record.Data); err != nil {
		record.Data = make(Data)
	}

	return &record, nil
}

func (m *Manager) insertDB(ctx context.Context, record *Record) error {
	data, err := json.Marshal(record.Data)
	if err != nil {
		return err
	}

	_, err = m.db.ExecContext(ctx,
		"INSERT INTO sessions (user_id, chat_id, state, data, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		record.UserID, record.ChatID, record.State, string(data),
		record.CreatedAt.Unix(), record.UpdatedAt.Unix(),
	)
	return err
}

func (m *Manager) updateDB(ctx context.Context, record *Record) error {
	data, err := json.Marshal(record.Data)
	if err != nil {
		return err
	}

	_, err = m.db.ExecContext(ctx,
		"UPDATE sessions SET state = ?, data = ?, updated_at = ? WHERE user_id = ? AND chat_id = ?",
		record.State, string(data), record.UpdatedAt.Unix(),
		record.UserID, record.ChatID,
	)
	return err
}

func (m *Manager) cleanupLoop() {
	for {
		select {
		case <-m.done:
			return
		case <-m.cleanup.C:
			m.mu.Lock()
			if m.cache == nil {
				m.mu.Unlock()
				return
			}
			for key, record := range m.cache {
				if time.Since(record.UpdatedAt) > m.ttl {
					delete(m.cache, key)
				}
			}
			m.mu.Unlock()
		}
	}
}