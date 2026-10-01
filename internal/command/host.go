package command

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/logger"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// dataRoot is where plugin data directories live, relative to the
// working directory like every other runtime file.
const dataRoot = "data"

var pluginNameRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// Host returns the plugin.Host view of the registry: the same services a
// CommandContext carries, usable outside command handlers.
func (r *Registry) Host() plugin.Host { return registryHost{r} }

type registryHost struct{ r *Registry }

func (h registryHost) API() *tg.Client { return h.r.api }

func (h registryHost) PeerResolver() plugin.PeerResolver { return h.r.resolver }

func (h registryHost) Media() plugin.MediaSender {
	h.r.mu.RLock()
	defer h.r.mu.RUnlock()
	return h.r.media
}

func (h registryHost) Downloader() plugin.MediaDownloader {
	h.r.mu.RLock()
	defer h.r.mu.RUnlock()
	return h.r.mediaDownloader
}

func (h registryHost) SelfID() int64 { return h.r.getSelfID() }

func (h registryHost) Logger(name string) plugin.Logger {
	return logger.NamedLogger("plugin").Named(name)
}

func (h registryHost) DataDir(name string) (string, error) {
	if !pluginNameRe.MatchString(name) {
		return "", fmt.Errorf("invalid plugin name %q", name)
	}
	dir := filepath.Join(dataRoot, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func (h registryHost) Lang(userID int64) string {
	if h.r.i18n == nil {
		return "zh-CN"
	}
	return string(h.r.i18n.UserLang(userID))
}

func (h registryHost) Send(ctx context.Context, chatID int64, text string, replyTo int) (int, error) {
	api, resolver := h.r.api, h.r.resolver
	if api == nil || resolver == nil {
		return 0, interfaces.ErrNoMessage
	}
	peer, err := resolver.ResolveFromChatID(ctx, chatID)
	if err != nil {
		return 0, err
	}
	plain, entities := plugin.ParseMarkdown(text, func(id int64) (tg.InputUserClass, error) {
		return resolveInputUser(ctx, api, id)
	})
	req := &tg.MessagesSendMessageRequest{Peer: peer, Message: plain, RandomID: randomID()}
	if len(entities) > 0 {
		req.SetEntities(entities)
	}
	if replyTo > 0 {
		req.SetReplyTo(&tg.InputReplyToMessage{ReplyToMsgID: replyTo})
	}
	upd, err := api.MessagesSendMessage(ctx, req)
	if err != nil {
		return 0, err
	}
	return sentMessageID(upd), nil
}

func resolveInputUser(ctx context.Context, api *tg.Client, id int64) (tg.InputUserClass, error) {
	users, err := api.UsersGetUsers(ctx, []tg.InputUserClass{&tg.InputUser{UserID: id}})
	if err != nil || len(users) == 0 {
		return nil, fmt.Errorf("resolve user %d failed", id)
	}
	u, ok := users[0].(*tg.User)
	if !ok {
		return nil, fmt.Errorf("unexpected user type %T", users[0])
	}
	return u.AsInput(), nil
}

// sentMessageID digs the new message id out of a send result; 0 if absent.
func sentMessageID(u tg.UpdatesClass) int {
	switch v := u.(type) {
	case *tg.UpdateShortSentMessage:
		return v.ID
	case *tg.Updates:
		return idFromUpdates(v.Updates)
	case *tg.UpdatesCombined:
		return idFromUpdates(v.Updates)
	}
	return 0
}

func idFromUpdates(list []tg.UpdateClass) int {
	for _, up := range list {
		switch x := up.(type) {
		case *tg.UpdateMessageID:
			return x.ID
		case *tg.UpdateNewMessage:
			if m, ok := x.Message.(*tg.Message); ok {
				return m.ID
			}
		case *tg.UpdateNewChannelMessage:
			if m, ok := x.Message.(*tg.Message); ok {
				return m.ID
			}
		}
	}
	return 0
}

func randomID() int64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UnixNano()
	}
	return int64(binary.LittleEndian.Uint64(b[:]))
}
