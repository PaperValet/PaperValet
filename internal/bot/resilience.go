package bot

import (
	"context"
	"time"

	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// SelfErrorHook returns a telegram.Options.OnSelfError handler, shared by the
// userbot and bot clients. gotd calls Self() after every (re)connect to
// subscribe the fresh connection for updates, then swallows the error unless
// the handler acts: one transient failure would leave the client connected
// (pings flow, presence looks online) while receiving nothing at all.
//
// Returning the error here makes gotd drop the connection and reconnect with
// backoff, retrying the subscription. Authorization errors are passed
// through: before login they are routine, and gotd already treats them as
// permanent connection errors.
func SelfErrorHook(log plugin.Logger) func(context.Context, error) error {
	return func(_ context.Context, err error) error {
		if auth.IsUnauthorized(err) {
			return nil
		}
		log.Error("update subscription failed, reconnecting", "error", err)
		return err
	}
}

// stateAPI is the part of *tg.Client the subscription loop needs.
type stateAPI interface {
	UpdatesGetState(ctx context.Context) (*tg.UpdatesState, error)
}

// subscribeUpdates keeps asking Telegram for the bot's update state until it
// answers. A bot that never gets an answer never receives messages, so one
// failed call must not be final — the loop backs off up to a minute and keeps
// trying while the run context is alive.
func subscribeUpdates(ctx context.Context, api stateAPI, log plugin.Logger, sleep func(time.Duration)) {
	delay := 5 * time.Second
	for {
		_, err := api.UpdatesGetState(ctx)
		if err == nil {
			return
		}
		if ctx.Err() != nil {
			return
		}
		log.Warn("get state failed, retrying", "error", err)
		sleep(delay)
		if delay < time.Minute {
			delay *= 2
		}
	}
}
