package peer

import (
	"context"

	"github.com/gotd/td/tg"
)

// Resolver is a lightweight peer resolver that delegates to AccessHashManager.
type Resolver struct {
	manager *AccessHashManager
}

func NewResolver(manager *AccessHashManager) *Resolver {
	return &Resolver{manager: manager}
}

func (r *Resolver) ResolveFromChatID(ctx context.Context, chatID int64) (tg.InputPeerClass, error) {
	return r.manager.GetInputPeer(ctx, chatID)
}

func (r *Resolver) ResolveUserInChannel(ctx context.Context, channelPeer tg.InputChannelClass, userID int64) (tg.InputPeerClass, error) {
	return r.manager.GetUserPeerWithFallback(ctx, userID, channelPeer)
}

func (r *Resolver) ResolveUserFromMessage(ctx context.Context, peer tg.InputPeerClass, msgID int, userID int64) (tg.InputPeerClass, error) {
	return r.manager.GetUserPeerFromMessage(ctx, peer, msgID, userID)
}

func (r *Resolver) ResolveUsername(ctx context.Context, username string) (tg.InputPeerClass, error) {
	return r.manager.ResolveUsername(ctx, username)
}

func (r *Resolver) RegisterPeer(peerID int64, accessHash int64, peerType string) {
	r.manager.RegisterPeer(peerID, accessHash, peerType)
}
