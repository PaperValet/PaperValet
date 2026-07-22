package app

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
	"golang.org/x/term"
)

// TerminalAuth implements auth.UserAuthenticator for interactive login.
type TerminalAuth struct {
	phoneNumber string
}

func (a TerminalAuth) Phone(_ context.Context) (string, error) {
	if a.phoneNumber != "" {
		return a.phoneNumber, nil
	}
	fmt.Print("Phone (+86...): ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func (a TerminalAuth) Password(_ context.Context) (string, error) {
	fmt.Print("2FA password: ")
	b, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (a TerminalAuth) Code(_ context.Context, _ *tg.AuthSentCode) (string, error) {
	fmt.Print("Login code: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func (a TerminalAuth) AcceptTermsOfService(_ context.Context, tos tg.HelpTermsOfService) error {
	return &auth.SignUpRequired{TermsOfService: tos}
}

func (a TerminalAuth) SignUp(_ context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, &auth.SignUpRequired{}
}

func EnsureAuth(ctx context.Context, client *telegram.Client, phone string) error {
	status, err := client.Auth().Status(ctx)
	if err != nil {
		return err
	}
	if status.Authorized {
		return nil
	}
	flow := auth.NewFlow(TerminalAuth{phoneNumber: phone}, auth.SendCodeOptions{})
	return flow.Run(ctx, client.Auth())
}
