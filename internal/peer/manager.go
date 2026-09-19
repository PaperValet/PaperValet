package peer

import (
	"context"
	"sync"
	"time"

	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/pkg/logger"
)

// AccessHashManager caches and resolves access hashes for peers.
type AccessHashManager struct {
	api    *tg.Client
	mu     sync.RWMutex
	cache  map[int64]*peerCacheEntry
	logger interface {
		Info(string, ...any)
		Debug(string, ...any)
	}
}

type peerCacheEntry struct {
	AccessHash int64
	PeerType   string
	ResolvedAt time.Time
	TTL        time.Duration
}

func NewAccessHashManager(api *tg.Client) *AccessHashManager {
	return &AccessHashManager{
		api:    api,
		cache:  make(map[int64]*peerCacheEntry),
		logger: logger.NamedLogger("peer"),
	}
}

func (m *AccessHashManager) GetInputPeer(ctx context.Context, peerID int64) (tg.InputPeerClass, error) {
	if entry := m.getFromCache(peerID); entry != nil {
		return m.buildInputPeer(peerID, entry), nil
	}
	return m.fallbackInputPeer(peerID), nil
}

func (m *AccessHashManager) GetUserPeerWithFallback(ctx context.Context, userID int64, channelPeer tg.InputChannelClass) (tg.InputPeerClass, error) {
	if entry := m.getFromCache(userID); entry != nil && entry.AccessHash != 0 {
		return &tg.InputPeerUser{UserID: userID, AccessHash: entry.AccessHash}, nil
	}
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
						m.updateCache(userID, user.AccessHash, "user")
						return &tg.InputPeerUser{UserID: userID, AccessHash: user.AccessHash}, nil
					}
				}
			}
		}
	}
	return &tg.InputPeerUser{UserID: userID}, nil
}

func (m *AccessHashManager) GetUserPeerFromMessage(ctx context.Context, peer tg.InputPeerClass, msgID int, userID int64) (tg.InputPeerClass, error) {
	if entry := m.getFromCache(userID); entry != nil && entry.AccessHash != 0 {
		return &tg.InputPeerUser{UserID: userID, AccessHash: entry.AccessHash}, nil
	}
	return &tg.InputPeerUser{UserID: userID}, nil
}

func (m *AccessHashManager) ResolveUsername(ctx context.Context, username string) (tg.InputPeerClass, error) {
	resolved, err := m.api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{Username: username})
	if err != nil {
		return nil, err
	}
	peer := resolved.GetPeer()
	if peer == nil {
		return nil, &PeerError{Code: "RESOLVE_FAILED", Message: "could not resolve: " + username}
	}
	switch p := peer.(type) {
	case *tg.PeerUser:
		for _, u := range resolved.GetUsers() {
			if user, ok := u.(*tg.User); ok && user.ID == p.UserID {
				m.updateCache(user.ID, user.AccessHash, "user")
				return &tg.InputPeerUser{UserID: user.ID, AccessHash: user.AccessHash}, nil
			}
		}
	case *tg.PeerChat:
		return &tg.InputPeerChat{ChatID: p.ChatID}, nil
	case *tg.PeerChannel:
		for _, c := range resolved.GetChats() {
			if ch, ok := c.(*tg.Channel); ok && ch.ID == p.ChannelID {
				m.updateCache(ch.ID, ch.AccessHash, "channel")
				return &tg.InputPeerChannel{ChannelID: ch.ID, AccessHash: ch.AccessHash}, nil
			}
		}
	}
	return nil, &PeerError{Code: "RESOLVE_FAILED", Message: "could not resolve: " + username}
}

func (m *AccessHashManager) RegisterPeer(peerID int64, accessHash int64, peerType string) {
	m.updateCache(peerID, accessHash, peerType)
}

func (m *AccessHashManager) getFromCache(peerID int64) *peerCacheEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entry, ok := m.cache[peerID]
	if !ok {
		return nil
	}
	if entry.TTL > 0 && time.Since(entry.ResolvedAt) > entry.TTL {
		return nil
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

func (m *AccessHashManager) buildInputPeer(peerID int64, entry *peerCacheEntry) tg.InputPeerClass {
	switch entry.PeerType {
	case "user":
		return &tg.InputPeerUser{UserID: peerID, AccessHash: entry.AccessHash}
	case "chat":
		return &tg.InputPeerChat{ChatID: -peerID}
	case "channel":
		channelID := peerID
		if peerID < 0 {
			channelID = -peerID - 1000000000000
		}
		return &tg.InputPeerChannel{ChannelID: channelID, AccessHash: entry.AccessHash}
	default:
		return m.fallbackInputPeer(peerID)
	}
}

func (m *AccessHashManager) fallbackInputPeer(peerID int64) tg.InputPeerClass {
	switch {
	case peerID > 0:
		return &tg.InputPeerUser{UserID: peerID}
	case peerID > -1000000000000:
		return &tg.InputPeerChat{ChatID: -peerID}
	default:
		channelID := -peerID - 1000000000000
		return &tg.InputPeerChannel{ChannelID: channelID}
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

func (e *PeerError) Unwrap() error { return e.Err }
