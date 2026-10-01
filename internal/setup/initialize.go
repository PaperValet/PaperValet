package setup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gotd/td/telegram"

	"github.com/TiaraBasori/PaperValet/internal/config"
	"github.com/TiaraBasori/PaperValet/internal/i18n"
)

const totalSteps = 5

var hashRe = regexp.MustCompile(`^[0-9a-fA-F]{32}$`)

// ClientFactory builds the Telegram client for a config. main passes the
// app's constructor so login uses the same device info and session file.
type ClientFactory func(cfg *config.Config) *telegram.Client

// Options configures Initialize.
type Options struct {
	Home      string // data home, already resolved
	Command   string // registered shell command name
	NewClient ClientFactory
}

// guessLang picks the starting language from the existing config or $LANG.
func guessLang(cfg *config.Config) int {
	if cfg.I18n.DefaultLanguage == string(i18n.EnUS) {
		return 1
	}
	if cfg.I18n.DefaultLanguage == string(i18n.ZhCN) {
		return 0
	}
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(k); v != "" {
			if strings.HasPrefix(strings.ToLower(v), "zh") {
				return 0
			}
			return 1
		}
	}
	return 1
}

// loadExisting reads config.json without Load's side effects so saving it
// back does not bake in runtime defaults.
func loadExisting(path string) (*config.Config, bool, error) {
	cfg := config.Default()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, false, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, true, nil
}

// Initialize runs the interactive setup in the data home.
func Initialize(ctx context.Context, opt Options) error {
	u := NewUI()
	if err := os.MkdirAll(opt.Home, 0o700); err != nil {
		return err
	}
	if err := os.Chdir(opt.Home); err != nil {
		return err
	}
	cfgPath := filepath.Join(opt.Home, config.FileName)
	cfg, existed, err := loadExisting(cfgPath)
	if err != nil {
		return err
	}

	// 1. Language. Shown bilingually because nothing is chosen yet.
	u.Title("PaperValet · " + u.cat.T(i18n.ZhCN, "setup.title") + " / " + u.cat.T(i18n.EnUS, "setup.title"))
	u.Blank()
	u.line(u.paint(ansiBold, "[1/5] 语言 / Language"))
	langs := []i18n.Lang{i18n.ZhCN, i18n.EnUS}
	idx, err := u.Choose([]string{"简体中文", "English"}, guessLang(cfg), "请输入 1 或 2 / Enter 1 or 2")
	if err != nil {
		return err
	}
	u.Lang = langs[idx]
	cfg.I18n.DefaultLanguage = string(u.Lang)
	if existed {
		u.Hint(u.T("setup.existing", cfgPath))
		u.Hint(u.T("setup.existing_hint"))
	}

	// 2. API credentials.
	u.Step(2, totalSteps, "setup.step_api")
	u.Hint(u.T("setup.api_hint"))
	def := ""
	if cfg.Telegram.APIID != 0 {
		def = strconv.Itoa(cfg.Telegram.APIID)
	}
	idStr, err := u.Ask(u.T("setup.api_id"), def, func(s string) bool {
		n, err := strconv.Atoi(s)
		return err == nil && n > 0
	}, "setup.api_id_invalid")
	if err != nil {
		return err
	}
	cfg.Telegram.APIID, _ = strconv.Atoi(idStr)
	defHash := cfg.Telegram.APIHash
	if !hashRe.MatchString(defHash) {
		defHash = ""
	}
	cfg.Telegram.APIHash, err = u.AskMasked(u.T("setup.api_hash"), defHash, hashRe.MatchString, "setup.api_hash_invalid")
	if err != nil {
		return err
	}
	if err := cfg.Save(cfgPath); err != nil {
		return err
	}
	u.OK(u.T("setup.config_saved", cfgPath))

	// 3 + 4. Phone and login, inside one client session.
	if err := login(ctx, u, opt, cfg); err != nil {
		return err
	}

	// 5. Service.
	return offerService(u, opt)
}

func login(ctx context.Context, u *UI, opt Options, cfg *config.Config) error {
	resolved := *cfg
	if !filepath.IsAbs(resolved.Telegram.SessionFile) {
		resolved.Telegram.SessionFile = filepath.Join(opt.Home, resolved.Telegram.SessionFile)
	}
	relogin, err := checkSession(ctx, u, opt, &resolved)
	if err != nil || !relogin {
		return err
	}
	askPhone := func() (string, error) {
		return u.Ask(u.T("setup.phone"), "", validPhone, "setup.phone_invalid")
	}
	u.Step(3, totalSteps, "setup.step_phone")
	phone, err := askPhone()
	if err != nil {
		return err
	}

	u.Step(4, totalSteps, "setup.step_login")
	u.Hint(u.T("setup.connecting"))
	client := opt.NewClient(&resolved)
	err = client.Run(ctx, func(ctx context.Context) error {
		first := true
		user, err := Login(ctx, u, client, func() (string, error) {
			if first {
				first = false
				return phone, nil
			}
			return askPhone()
		})
		if err != nil {
			return err
		}
		u.OK(u.T("setup.logged_in", displayName(user)))
		return nil
	})
	if err != nil && !errors.Is(err, ErrAborted) && !errors.Is(err, context.Canceled) {
		u.Err(u.T("setup.login_failed", err))
	}
	return err
}

// checkSession reports whether a fresh login is needed. An existing valid
// session is kept unless the user asks to switch accounts, in which case it
// is logged out and the session file removed (a logged-out key is dead).
func checkSession(ctx context.Context, u *UI, opt Options, cfg *config.Config) (bool, error) {
	if _, err := os.Stat(cfg.Telegram.SessionFile); err != nil {
		return true, nil
	}
	u.Blank()
	u.Hint(u.T("setup.connecting"))
	client := opt.NewClient(cfg)
	relogin := true
	err := client.Run(ctx, func(ctx context.Context) error {
		status, err := client.Auth().Status(ctx)
		if err != nil || !status.Authorized || status.User == nil {
			return nil
		}
		u.OK(u.T("setup.already_logged", displayName(status.User)))
		again, err := u.Confirm(u.T("setup.relogin"), false)
		if err != nil {
			return err
		}
		relogin = again
		if again {
			_, _ = client.API().AuthLogOut(ctx)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrAborted) || errors.Is(err, context.Canceled) {
			return false, err
		}
		// A broken session file is no reason to stop; start clean.
		relogin = true
	}
	if relogin {
		if err := os.Remove(cfg.Telegram.SessionFile); err != nil && !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
	}
	return relogin, nil
}

func offerService(u *UI, opt Options) error {
	runCmd := opt.Command + " run"
	u.Step(5, totalSteps, "setup.step_service")
	want, err := u.Confirm(u.T("setup.service_ask"), true)
	if err != nil {
		return err
	}
	if !want {
		u.Blank()
		u.OK(u.T("setup.done"))
		u.Hint(u.T("setup.run_hint", u.Cmd(runCmd)))
		u.Blank()
		return nil
	}

	user, err := detectServiceMode()
	switch {
	case errors.Is(err, errNoSystemd):
		u.Warn(u.T("setup.service_no_systemd"))
		u.Hint(u.T("setup.run_hint", u.Cmd(runCmd)))
		return nil
	case errors.Is(err, errNeedRoot):
		u.Warn(u.T("setup.service_need_root", opt.Command))
		u.Hint(u.T("setup.run_hint", u.Cmd(runCmd)))
		return nil
	}

	bin, err := os.Executable()
	if err != nil {
		return err
	}
	if r, err := filepath.EvalSymlinks(bin); err == nil {
		bin = r
	}

	var svc Service
	for {
		name, err := u.Ask(u.T("setup.service_name"), opt.Command, validServiceName, "setup.service_name_bad")
		if err != nil {
			return err
		}
		svc = Service{Name: name, Binary: bin, Home: opt.Home, Command: opt.Command, User: user}
		if !svc.Exists() {
			break
		}
		over, err := u.Confirm(u.T("setup.service_exists", name), false)
		if err != nil {
			return err
		}
		if over {
			break
		}
	}

	if err := svc.Install(); err != nil {
		u.Err(u.T("setup.service_failed", err))
		u.Hint(u.T("setup.run_hint", u.Cmd(runCmd)))
		return nil
	}
	time.Sleep(2 * time.Second)
	if err := svc.systemctl("is-active", "--quiet", svc.Name+".service"); err != nil {
		u.Err(u.T("setup.service_failed", svc.Name))
		u.Hint(u.T("setup.service_logs", u.Cmd(svc.LogsCmd())))
		return nil
	}

	u.Blank()
	u.OK(u.T("setup.service_started", svc.Name))
	u.Hint(u.T("setup.service_status", u.Cmd(svc.StatusCmd())))
	u.Hint(u.T("setup.service_logs", u.Cmd(svc.LogsCmd())))
	if svc.User {
		u.Hint(u.T("setup.service_linger", u.Cmd("loginctl enable-linger "+os.Getenv("USER"))))
	}
	u.Blank()
	u.OK(u.T("setup.done"))
	u.Blank()
	return nil
}
