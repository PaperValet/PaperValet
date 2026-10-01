package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

// ErrNotLoggedIn means the session is not authorized and no headless
// credentials were supplied; the user should run `<cmd> initialize`.
var ErrNotLoggedIn = errors.New("not logged in")

// envAuth logs in from environment variables only, for containers and CI
// where nobody can type: PAPERVALET_PHONE, PAPERVALET_CODE and optionally
// PAPERVALET_2FA_PASSWORD. Interactive login lives in `initialize`.
type envAuth struct{}

func env(k string) string { return strings.TrimSpace(os.Getenv(k)) }

func (envAuth) Phone(context.Context) (string, error) {
	return env("PAPERVALET_PHONE"), nil
}

func (envAuth) Password(context.Context) (string, error) {
	if v := os.Getenv("PAPERVALET_2FA_PASSWORD"); v != "" {
		return v, nil
	}
	return "", fmt.Errorf("2FA enabled but PAPERVALET_2FA_PASSWORD is not set")
}

func (envAuth) Code(context.Context, *tg.AuthSentCode) (string, error) {
	if v := env("PAPERVALET_CODE"); v != "" {
		return v, nil
	}
	return "", fmt.Errorf("PAPERVALET_CODE is not set")
}

func (envAuth) AcceptTermsOfService(_ context.Context, tos tg.HelpTermsOfService) error {
	return &auth.SignUpRequired{TermsOfService: tos}
}

func (envAuth) SignUp(context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, &auth.SignUpRequired{}
}

// EnsureAuth returns nil when the session is authorized, logs in headlessly
// when PAPERVALET_PHONE is set, and returns ErrNotLoggedIn otherwise.
func EnsureAuth(ctx context.Context, client *telegram.Client) error {
	status, err := client.Auth().Status(ctx)
	if err != nil {
		return err
	}
	if status.Authorized {
		return nil
	}
	if env("PAPERVALET_PHONE") == "" {
		return ErrNotLoggedIn
	}
	return auth.NewFlow(envAuth{}, auth.SendCodeOptions{}).Run(ctx, client.Auth())
}
