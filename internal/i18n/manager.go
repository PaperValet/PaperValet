package i18n

import (
	"context"
	"sync"
)

// ctxKey is the context key for the active language.
type ctxKey struct{}

// WithLang returns a context carrying the active language.
func WithLang(ctx context.Context, l Lang) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// LangFrom returns the language stored in ctx, or the fallback.
func LangFrom(ctx context.Context, fallback Lang) Lang {
	if ctx == nil {
		return fallback
	}
	if l, ok := ctx.Value(ctxKey{}).(Lang); ok && l != "" {
		return l
	}
	return fallback
}

// Manager holds the catalog plus per-user language preferences.
type Manager struct {
	cat      *Catalog
	mu       sync.RWMutex
	userLang map[int64]Lang // userID -> preferred language
}

// NewManager creates a language manager over a catalog.
func NewManager(cat *Catalog) *Manager {
	return &Manager{
		cat:      cat,
		userLang: make(map[int64]Lang),
	}
}

// Catalog returns the underlying catalog.
func (m *Manager) Catalog() *Catalog { return m.cat }

// SetUserLang records a user's preferred language.
func (m *Manager) SetUserLang(userID int64, l Lang) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.userLang[userID] = l
}

// UserLang returns a user's preferred language, or the catalog default.
func (m *Manager) UserLang(userID int64) Lang {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if l, ok := m.userLang[userID]; ok {
		return l
	}
	return m.cat.Default()
}

// T translates for a user, falling back to the catalog default.
func (m *Manager) T(userID int64, key string, args ...any) string {
	return m.cat.T(m.UserLang(userID), key, args...)
}

// TContext translates using the language carried in ctx (if any), else user pref.
func (m *Manager) TContext(ctx context.Context, userID int64, key string, args ...any) string {
	l := LangFrom(ctx, m.UserLang(userID))
	return m.cat.T(l, key, args...)
}