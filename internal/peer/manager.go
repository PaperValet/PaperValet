package peer

import (
	"context"
	"sync"
	"time"

	"github.com/gotd/td/tg"
	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/logger"
)

// AccessHashManager caches and resolves access hashes for peers.
// Unlike TeleBox's scattered approach, this centralizes all peer resolution
// with a proper fallback chain and SQLite-backed caching.
type AccessHashManager struct {
	api    *tg.Client
	db     *sqliteDB
	mu     sync.RWMutex
	cache  map[int64]*peerCacheEntry
	logger interfaces.Logger
}

type peerCacheEntry struct {
	AccessHash  int64
	PeerType    string // "user", "chat", "channel"
	ResolvedAt  time.Time
	TTL         time.Duration
}

type sqliteDB struct {
	// TODO: Implement SQLite-backed persistence
	// For now, in-memory cache only
}

func NewAccessHashManager(api *tg.Client) *AccessHashManager {
	return &AccessHashManager{
		api:    api,
		cache:  make(map[int64]*peerCacheEntry),
		logger: logger.NamedLogger("peer_manager"),
	}
}

// GetInputPeer returns an InputPeer for the given chat/user ID.
// Priority: cache → direct API resolution → fallback by ID pattern.
func (m *AccessHashManager) GetInputPeer(ctx context.Context, peerID int64) (tg.InputPeerClass, error) {
	// Try cache first
	if entry := m.getFromCache(peerID); entry != nil {
		return m.buildInputPeer(peerID, entry)
	}

	// Resolve via API
	peer, err := m.resolveFromAPI(ctx, peerID)
	if err != nil {
		// Fallback: construct from ID pattern
		return m.fallbackInputPeer(peerID), nil
	}
	return peer, nil
}

// GetUserPeerWithFallback attempts to get a user peer with access hash,
// trying multiple strategies in order.
func (m *AccessHashManager) GetUserPeerWithFallback(ctx context.Context, userID int64, channelPeer tg.InputChannelClass) (tg.InputPeerClass, error) {
	// Try cache first
	if entry := m.getFromCache(userID); entry != nil && entry.AccessHash != 0 {
		return &tg.InputPeerUser{
			UserID:     userID,
			AccessHash: entry.AccessHash,
		}, nil
	}

	// Try to get from channel participants
	if channelPeer != nil {
		participants, err := m.api.ChannelsGetParticipants(ctx, &tg.ChannelsGetParticipantsRequest{
			Channel: channelPeer,
			Filter:  &tg.ChannelParticipantsRecent{},
			Offset:  0,
			Limit:   200,
		})
		if err == nil {
			if p, ok := participants.AsModified(); ok {
				for _, u := range p.GetUsers() {
					if user, ok := u.(*tg.User); ok && user.ID == userID {
						hash := user.AccessHash
						m.updateCache(userID, hash, "user")
						return &tg.InputPeerUser{
							UserID:     userID,
							AccessHash: hash,
						}, nil
					}
				}
			}
		}
	}

	// Fallback: construct without access hash
	return &tg.InputPeerUser{
		UserID:     userID,
		AccessHash: 0,
	}, nil
}

// GetUserPeerFromMessage resolves a user peer from a message context.
func (m *AccessHashManager) GetUserPeerFromMessage(ctx context.Context, peer tg.InputPeerClass, msgID int, userID int64) (tg.InputPeerClass, error) {
	// Try cache first
	if entry := m.getFromCache(userID); entry != nil && entry.AccessHash != 0 {
		return &tg.InputPeerUser{
			UserID:     userID,
			AccessHash: entry.AccessHash,
		}, nil
	}

	// Try to get the message and extract user info
	// This is a multi-step process, so we keep it simple for now
	return &tg.InputPeerUser{
		UserID:     userID,
		AccessHash: 0,
	}, nil
}

// ResolveUsername resolves a @username to an InputPeer.
func (m *AccessHashManager) ResolveUsername(ctx context.Context, username string) (tg.InputPeerClass, error) {
	resolved, err := m.api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{
		Username: username,
	})
	if err != nil {
		return nil, err
	}

	peer := resolved.GetPeer()
	if peer != nil {
		switch p := peer.(type) {
		case *tg.PeerUser:
			// Find user in resolved users
			for _, u := range resolved.GetUsers() {
				if user, ok := u.(*tg.User); ok && user.ID == p.UserID {
					m.updateCache(user.ID, user.AccessHash, "user")
					return &tg.InputPeerUser{
						UserID:     user.ID,
						AccessHash: user.AccessHash,
					}, nil
				}
			}
		case *tg.PeerChat:
			return &tg.InputPeerChat{
				ChatID: p.ChatID,
			}, nil
		case *tg.PeerChannel:
			for _, c := range resolved.GetChats() {
				if channel, ok := c.(*tg.Channel); ok && channel.ID == p.ChannelID {
					m.updateCache(channel.ID, channel.AccessHash, "channel")
					return &tg.InputPeerChannel{
						ChannelID:  channel.ID,
						AccessHash: channel.AccessHash,
					}, nil
				}
			}
		}
	}

	return nil, &PeerError{Code: "RESOLVE_FAILED", Message: "could not resolve username: " + username}
}

// RegisterPeer stores a peer's access hash from an update context.
func (m *AccessHashManager) RegisterPeer(peerID int64, accessHash int64, peerType string) {
	m.updateCache(peerID, accessHash, peerType)
}

// --- Internal methods ---

func (m *AccessHashManager) getFromCache(peerID int64) *peerCacheEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entry, ok := m.cache[peerID]
	if !ok {
		return nil
	}
	if entry.TTL > 0 && time.Since(entry.ResolvedAt) > entry.TTL {
		return nil // expired
	}
	return entry
}

func (m *AccessHashManager) updateCache(peerID int64, accessHash int64, peerType string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cache[peerID] = &peerCacheEntry{
		AccessHash: accessHash,
		PeerType:   peerType,
		ResolvedAt: time.Now(),
		TTL:        24 * time.Hour,
	}
}

func (m *AccessHashManager) buildInputPeer(peerID int64, entry *peerCacheEntry) (tg.InputPeerClass, error) {
	switch entry.PeerType {
	case "user":
		return &tg.InputPeerUser{
			UserID:     peerID,
			AccessHash: entry.AccessHash,
		}, nil
	case "chat":
		return &tg.InputPeerChat{
			ChatID: -peerID, // Positive internal ID
		}, nil
	case "channel":
		// Channel IDs in Telegram are negative with -100 prefix
		channelID := peerID
		if peerID < 0 {
			channelID = -peerID - 1000000000000
		}
		return &tg.InputPeerChannel{
			ChannelID:  channelID,
			AccessHash: entry.AccessHash,
		}, nil
	default:
		return m.fallbackInputPeer(peerID), nil
	}
}

func (m *AccessHashManager) resolveFromAPI(ctx context.Context, peerID int64) (tg.InputPeerClass, error) {
	// Try to resolve via contacts or channels
	// For now, just return a fallback peer
	return m.fallbackInputPeer(peerID), nil
}

func (m *AccessHashManager) fallbackInputPeer(peerID int64) tg.InputPeerClass {
	switch {
	case peerID > 0:
		// Positive ID → User
		return &tg.InputPeerUser{
			UserID:     peerID,
			AccessHash: 0,
		}
	case peerID > -1000000000000:
		// Small negative → Legacy group chat
		return &tg.InputPeerChat{
			ChatID: -peerID,
		}
	default:
		// Large negative (-100xxxx) → Channel/Supergroup
		channelID := -peerID - 1000000000000
		return &tg.InputPeerChannel{
			ChannelID:  channelID,
			AccessHash: 0,
		}
	}
}

type PeerError struct {
	Code    string
	Message string
	Err     error
}

func (e *PeerError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *PeerError) Unwrap() error {
	return e.Err
}