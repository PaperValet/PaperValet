package setup

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
)

var phoneRe = regexp.MustCompile(`^\+[1-9][0-9]{6,14}$`)

// normalizePhone strips spaces, dashes and brackets users paste in.
func normalizePhone(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '-', '(', ')', '\t':
			return -1
		}
		return r
	}, s)
}

func validPhone(s string) bool { return phoneRe.MatchString(normalizePhone(s)) }

// errRetryPhone asks the caller to prompt for the phone number again.
var errRetryPhone = errors.New("retry phone")

// displayName renders "First Last (@user)".
func displayName(u *tg.User) string {
	name := strings.TrimSpace(u.FirstName + " " + u.LastName)
	if name == "" {
		name = fmt.Sprintf("id %d", u.ID)
	}
	if u.Username != "" {
		name += " (@" + u.Username + ")"
	}
	return name
}

// friendlyWait formats a flood wait the way people read it.
func friendlyWait(d time.Duration) string {
	d = d.Round(time.Second)
	if d < time.Minute {
		return d.String()
	}
	return d.Truncate(time.Minute).String()
}

func (u *UI) sentCodeHint(s *tg.AuthSentCode) string {
	switch t := s.Type.(type) {
	case *tg.AuthSentCodeTypeApp:
		return u.T("setup.code_sent_app")
	case *tg.AuthSentCodeTypeSMS, *tg.AuthSentCodeTypeSMSWord, *tg.AuthSentCodeTypeSMSPhrase,
		*tg.AuthSentCodeTypeFragmentSMS, *tg.AuthSentCodeTypeFirebaseSMS:
		return u.T("setup.code_sent_sms")
	case *tg.AuthSentCodeTypeCall, *tg.AuthSentCodeTypeFlashCall, *tg.AuthSentCodeTypeMissedCall:
		return u.T("setup.code_sent_call")
	case *tg.AuthSentCodeTypeEmailCode:
		return u.T("setup.code_sent_email", t.EmailPattern)
	default:
		return u.T("setup.code_sent_other")
	}
}

// loginError maps a Telegram error to a friendly line; ok is false when the
// error is not one we explain.
func (u *UI) loginError(err error) (string, bool) {
	if d, ok := tgerr.AsFloodWait(err); ok {
		return u.T("setup.flood_wait", friendlyWait(d)), true
	}
	switch {
	case tgerr.Is(err, "PHONE_NUMBER_BANNED"):
		return u.T("setup.phone_banned"), true
	case errors.Is(err, &auth.SignUpRequired{}), tgerr.Is(err, "PHONE_NUMBER_UNOCCUPIED"):
		return u.T("setup.signup_required"), true
	}
	return "", false
}

// Login drives the code + 2FA flow on an already running client. askPhone is
// called first and again whenever Telegram rejects the number.
func Login(ctx context.Context, u *UI, client *telegram.Client, askPhone func() (string, error)) (*tg.User, error) {
	for {
		phone, err := askPhone()
		if err != nil {
			return nil, err
		}
		user, err := loginPhone(ctx, u, client, normalizePhone(phone))
		if errors.Is(err, errRetryPhone) {
			continue
		}
		return user, err
	}
}

func loginPhone(ctx context.Context, u *UI, client *telegram.Client, phone string) (*tg.User, error) {
	a := client.Auth()
	sent, err := a.SendCode(ctx, phone, auth.SendCodeOptions{})
	if err != nil {
		if tgerr.Is(err, "PHONE_NUMBER_INVALID") {
			u.Err(u.T("setup.phone_rejected"))
			return nil, errRetryPhone
		}
		if msg, ok := u.loginError(err); ok {
			u.Err(msg)
			return nil, ErrAborted
		}
		return nil, err
	}

	var code *tg.AuthSentCode
	switch s := sent.(type) {
	case *tg.AuthSentCode:
		code = s
	case *tg.AuthSentCodeSuccess:
		return client.Self(ctx)
	default:
		return nil, fmt.Errorf("unexpected sent code %T", sent)
	}

	u.Hint(u.sentCodeHint(code))
	for {
		input, err := u.Ask(u.T("setup.code"), "", nil, "")
		if err != nil {
			return nil, err
		}
		input = strings.ReplaceAll(input, " ", "")
		_, err = a.SignIn(ctx, phone, input, code.PhoneCodeHash)
		switch {
		case err == nil:
			return client.Self(ctx)
		case errors.Is(err, auth.ErrPasswordAuthNeeded):
			if err := password(ctx, u, client); err != nil {
				return nil, err
			}
			return client.Self(ctx)
		case tgerr.Is(err, "PHONE_CODE_INVALID", "PHONE_CODE_EMPTY"):
			u.Err(u.T("setup.code_invalid"))
		case tgerr.Is(err, "PHONE_CODE_EXPIRED"):
			u.Warn(u.T("setup.code_expired"))
			return loginPhone(ctx, u, client, phone)
		default:
			if msg, ok := u.loginError(err); ok {
				u.Err(msg)
				return nil, ErrAborted
			}
			return nil, err
		}
	}
}

func password(ctx context.Context, u *UI, client *telegram.Client) error {
	label := u.T("setup.password")
	if p, err := client.API().AccountGetPassword(ctx); err == nil && p.Hint != "" {
		label = u.T("setup.password_hint", p.Hint)
	}
	for {
		pw, err := u.Secret(label)
		if err != nil {
			return err
		}
		_, err = client.Auth().Password(ctx, pw)
		switch {
		case err == nil:
			return nil
		case errors.Is(err, auth.ErrPasswordInvalid):
			u.Err(u.T("setup.password_invalid"))
		default:
			if msg, ok := u.loginError(err); ok {
				u.Err(msg)
				return ErrAborted
			}
			return err
		}
	}
}
