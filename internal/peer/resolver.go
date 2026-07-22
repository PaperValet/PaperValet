package peer

import (
	"context"

	"github.com/gotd/td/tg"
)

// Resolver is a lightweight peer resolver that delegates to AccessHashManager.
// This is the public API surface that plugins and commands use.
type Resolver struct {
	manager *AccessHashManager
}

func NewResolver(manager *AccessHashManager) *Resolver {
	return &Resolver{manager: manager}
}

// ResolveFromChatID resolves a chat/user ID to an InputPeer.
// chatID conventions:
//   > 0   → User (private chat)
//   < 0 && > -1000000000000 → Legacy group
//   < -1000000000000 → Supergroup/Channel
func (r *Resolver) ResolveFromChatID(ctx context.Context, chatID int64) (tg.InputPeerClass, error) {
	return r.manager.GetInputPeer(ctx, chatID)
}

// ResolveUserInChannel resolves a user in a channel context with fallback.
func (r *Resolver) ResolveUserInChannel(ctx context.Context, channelPeer tg.InputChannelClass, userID int64) (tg.InputPeerClass, error) {
	return r.manager.GetUserPeerWithFallback(ctx, userID, channelPeer)
}

// ResolveUserFromMessage resolves a user peer from a message context.
func (r *Resolver) ResolveUserFromMessage(ctx context.Context, peer tg.InputPeerClass, msgID int, userID int64) (tg.InputPeerClass, error) {
	return r.manager.GetUserPeerFromMessage(ctx, peer, msgID, userID)
}

// ResolveUsername resolves a @username to InputPeer.
func (r *Resolver) ResolveUsername(ctx context.Context, username string) (tg.InputPeerClass, error) {
	return r.manager.ResolveUsername(ctx, username)
}

// RegisterPeer stores a peer for later use.
func (r *Resolver) RegisterPeer(peerID int64, accessHash int64, peerType string) {
	r.manager.RegisterPeer(peerID, accessHash, peerType)
}