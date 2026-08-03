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

// 环境变量回退：
//   PAPERVALET_PHONE         优先于 TerminalAuth.phoneNumber 与交互 prompt
//   PAPERVALET_CODE          一次性登录验证码（仅脚本/容器场景使用）
//   PAPERVALET_2FA_PASSWORD  2FA 密码
//   PAPERVALET_NONINTERACTIVE=true 时禁止任何 stdin 提示，缺值直接报错
//
// 设计意图：
//   1. 普通用户：什么都不设，走原有交互流程
//   2. 容器 / systemd：把 phone/code/2FA 都用环境变量喂入，全自动登录
//   3. 一键脚本 install.sh：当 PAPERVALET_NONINTERACTIVE 未设但 PAPERVALET_PHONE
//      已设时仍允许后续 code/2FA 走 stdin，避免脚本写到一半卡住

// TerminalAuth implements auth.UserAuthenticator with env-var fallback for
// non-interactive environments (containers, CI, automated installs).
type TerminalAuth struct {
	phoneNumber string
}

// envNonInteractive reports whether the user has opted out of stdin prompts.
func envNonInteractive() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("PAPERVALET_NONINTERACTIVE")))
	return v == "1" || v == "true" || v == "yes"
}

func (a TerminalAuth) Phone(_ context.Context) (string, error) {
	if v := strings.TrimSpace(os.Getenv("PAPERVALET_PHONE")); v != "" {
		return v, nil
	}
	if a.phoneNumber != "" {
		return a.phoneNumber, nil
	}
	if envNonInteractive() {
		return "", fmt.Errorf("PAPERVALET_PHONE not set and PAPERVALET_NONINTERACTIVE=true")
	}
	fmt.Print("Phone (+86...): ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func (a TerminalAuth) Password(_ context.Context) (string, error) {
	if v := os.Getenv("PAPERVALET_2FA_PASSWORD"); v != "" {
		return v, nil
	}
	if envNonInteractive() {
		return "", fmt.Errorf("PAPERVALET_2FA_PASSWORD not set and PAPERVALET_NONINTERACTIVE=true")
	}
	fmt.Print("2FA password: ")
	b, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (a TerminalAuth) Code(_ context.Context, _ *tg.AuthSentCode) (string, error) {
	if v := os.Getenv("PAPERVALET_CODE"); v != "" {
		return v, nil
	}
	if envNonInteractive() {
		return "", fmt.Errorf("PAPERVALET_CODE not set and PAPERVALET_NONINTERACTIVE=true")
	}
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