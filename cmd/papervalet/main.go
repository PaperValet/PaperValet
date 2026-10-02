package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/gotd/td/telegram"

	"github.com/TiaraBasori/PaperValet/internal/app"
	"github.com/TiaraBasori/PaperValet/internal/bot"
	"github.com/TiaraBasori/PaperValet/internal/config"
	"github.com/TiaraBasori/PaperValet/internal/i18n"
	"github.com/TiaraBasori/PaperValet/internal/setup"
)

var (
	version   = "0.1.0"
	buildTime = "unknown"
	gitCommit = "unknown"
)

var cat = i18n.SetupCatalog()

// lang picks the CLI language: configured language, else $LANG.
func lang(cfg *config.Config) i18n.Lang {
	if cfg != nil {
		switch i18n.Lang(cfg.I18n.DefaultLanguage) {
		case i18n.ZhCN, i18n.EnUS:
			return i18n.Lang(cfg.I18n.DefaultLanguage)
		}
	}
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(k); v != "" {
			if strings.HasPrefix(strings.ToLower(v), "zh") {
				return i18n.ZhCN
			}
			break
		}
	}
	return i18n.EnUS
}

func fail(l i18n.Lang, key string, args ...any) {
	fmt.Fprintln(os.Stderr, "✗ "+cat.T(l, key, args...))
	os.Exit(1)
}

func main() {
	cmdName := config.CommandName()

	fs := flag.NewFlagSet(cmdName, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configFlag := fs.String("config", "", "")
	versionFlag := fs.Bool("version", false, "")

	// Subcommand may come before or after flags: `papervalet run -config x`
	// and the legacy Docker form `papervalet -config x` both work.
	args := os.Args[1:]
	sub := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		sub, args = args[0], args[1:]
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			sub = "help"
		} else {
			fmt.Fprintln(os.Stderr, "✗ "+err.Error())
			os.Exit(2)
		}
	}
	if sub == "" && fs.NArg() > 0 {
		sub = fs.Arg(0)
	}
	if *versionFlag {
		sub = "version"
	}

	switch sub {
	case "", "run":
		run(cmdName, *configFlag)
	case "initialize", "init":
		initialize(cmdName)
	case "version":
		printVersion()
	case "help":
		fmt.Println(cat.T(lang(nil), "cli.usage", cmdName))
	default:
		fmt.Fprintln(os.Stderr, "✗ "+cat.T(lang(nil), "cli.unknown", sub))
		fmt.Fprintln(os.Stderr, cat.T(lang(nil), "cli.usage", cmdName))
		os.Exit(2)
	}
}

func printVersion() {
	fmt.Printf("PaperValet %s\n", version)
	fmt.Printf("  build: %s\n", buildTime)
	fmt.Printf("  commit: %s\n", gitCommit)
	fmt.Printf("  go: %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
}

func initialize(cmdName string) {
	home, err := config.Home()
	if err != nil {
		fail(lang(nil), "cli.init_failed", err)
	}
	ctx, stop := signalContext()
	defer stop()
	err = setup.Initialize(ctx, setup.Options{
		Home:    home,
		Command: cmdName,
		NewClient: func(cfg *config.Config) *telegram.Client {
			return app.NewTelegramClient(cfg, nil)
		},
		NewBotClient: func(cfg *config.Config) *telegram.Client {
			return bot.NewClient(app.BotOptions(cfg), nil)
		},
	})
	if err != nil {
		// Aborts and login failures were already explained on screen.
		if errors.Is(err, setup.ErrAborted) || errors.Is(err, context.Canceled) {
			os.Exit(130)
		}
		os.Exit(1)
	}
}

// resolveConfig returns the config path. An explicit -config or
// $PAPERVALET_CONFIG keeps the working directory (Docker, custom layouts);
// otherwise the bot runs inside the data home.
func resolveConfig(flagPath string) (string, error) {
	if flagPath != "" {
		return flagPath, nil
	}
	if v := strings.TrimSpace(os.Getenv("PAPERVALET_CONFIG")); v != "" {
		return v, nil
	}
	home, err := config.Home()
	if err != nil {
		return "", err
	}
	if err := os.Chdir(home); err != nil {
		return filepath.Join(home, config.FileName), err
	}
	return filepath.Join(home, config.FileName), nil
}

func run(cmdName, flagPath string) {
	initCmd := cmdName + " initialize"
	path, err := resolveConfig(flagPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fail(lang(nil), "cli.no_config", initCmd)
		}
		fail(lang(nil), "cli.load_failed", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fail(lang(nil), "cli.no_config", initCmd)
		}
		fail(lang(nil), "cli.load_failed", err)
	}
	l := lang(cfg)
	if err := cfg.Validate(); err != nil {
		fail(l, "cli.load_failed", err)
	}

	application, err := app.New(cfg)
	if err != nil {
		fail(l, "cli.run_failed", err)
	}
	application.SetConfigPath(path)

	ctx, stop := signalContext()
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = application.Shutdown(shutdownCtx)
	}()

	if err := application.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		if errors.Is(err, app.ErrNotLoggedIn) {
			fail(l, "cli.not_logged_in", initCmd)
		}
		fail(l, "cli.run_failed", err)
	}
}
