package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
	"go.uber.org/zap"

	"github.com/TiaraBasori/PaperValet/internal/command"
	"github.com/TiaraBasori/PaperValet/internal/config"
	"github.com/TiaraBasori/PaperValet/internal/cron"
	"github.com/TiaraBasori/PaperValet/internal/eventbus"
	"github.com/TiaraBasori/PaperValet/internal/peer"
	"github.com/TiaraBasori/PaperValet/internal/plugin"
	"github.com/TiaraBasori/PaperValet/internal/session"
	"github.com/TiaraBasori/PaperValet/pkg/logger"
	"github.com/TiaraBasori/PaperValet/plugins/builtin"
)

const Version = "0.1.0"

// App is the top-level orchestrator.
type App struct {
	cfg        *config.Config
	client     *telegram.Client
	api        *tg.Client
	bus        *eventbus.Bus
	commands   *command.Registry
	parser     *command.Parser
	plugins    *plugin.Manager
	sessions   *session.Manager
	peers      *peer.Resolver
	accessHash *peer.AccessHashManager
	updates    *UpdateHandler
	cron       *cron.Manager
	logger     *zap.Logger
}

func New(cfg *config.Config) (*App, error) {
	if err := logger.Init(cfg.Logger.Level, cfg.Logger.Format); err != nil {
		return nil, fmt.Errorf("logger: %w", err)
	}
	log := logger.Named("app")

	if err := os.MkdirAll(filepath.Dir(cfg.Telegram.Database), 0o755); err != nil && filepath.Dir(cfg.Telegram.Database) != "." {
		return nil, fmt.Errorf("database dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Telegram.SessionFile), 0o755); err != nil && filepath.Dir(cfg.Telegram.SessionFile) != "." {
		return nil, fmt.Errorf("session dir: %w", err)
	}

	sessMgr, err := session.NewManager(cfg.Telegram.Database)
	if err != nil {
		return nil, fmt.Errorf("session manager: %w", err)
	}

	bus := eventbus.New()
	updates := NewUpdateHandler(bus)

	client := telegram.NewClient(cfg.Telegram.APIID, cfg.Telegram.APIHash, telegram.Options{
		SessionStorage: &telegram.FileSessionStorage{Path: cfg.Telegram.SessionFile},
		UpdateHandler:  updates,
		Device: telegram.DeviceConfig{
			DeviceModel:    "PaperValet",
			SystemVersion:  "Linux",
			AppVersion:     Version,
			SystemLangCode: "en",
			LangCode:       "en",
		},
		RetryInterval: time.Second,
		MaxRetries:    -1,
		DialTimeout:   15 * time.Second,
	})

	api := client.API()
	accessHash := peer.NewAccessHashManager(api)
	resolver := peer.NewResolver(accessHash)

	cmdReg := command.NewRegistry(cfg.Bot.CommandPrefix, bus, api, resolver, cfg.Bot.OwnerID)
	parser := command.NewParser(cmdReg, bus)
	pluginMgr := plugin.NewManager(cmdReg, bus)
	cronMgr := cron.NewManager()

	app := &App{
		cfg:        cfg,
		client:     client,
		api:        api,
		bus:        bus,
		commands:   cmdReg,
		parser:     parser,
		plugins:    pluginMgr,
		sessions:   sessMgr,
		peers:      resolver,
		accessHash: accessHash,
		updates:    updates,
		cron:       cronMgr,
		logger:     log,
	}
	return app, nil
}

func (a *App) registerBuiltins() error {
	for _, p := range []plugin.Plugin{
		builtin.NewCore(Version),
		builtin.NewApt(),
		builtin.NewPing(),
		builtin.NewUptime(),
		builtin.NewInfo(),
		builtin.NewForward(),
		builtin.NewRemind(),
		builtin.NewNote(),
		builtin.NewFun(),
		builtin.NewAdmin(),
		builtin.NewCron(a.cron),
	} {
		if err := a.plugins.Register(p); err != nil {
			return err
		}
	}
	return nil
}

// Run connects, authenticates, loads plugins, and blocks until ctx is cancelled.
func (a *App) Run(ctx context.Context) error {
	if err := a.registerBuiltins(); err != nil {
		return fmt.Errorf("register builtins: %w", err)
	}

	a.parser.Start()
	a.cron.Start()

	return a.client.Run(ctx, func(ctx context.Context) error {
		if err := EnsureAuth(ctx, a.client, ""); err != nil {
			return fmt.Errorf("auth: %w", err)
		}

		self, err := a.client.Self(ctx)
		if err != nil {
			return fmt.Errorf("self: %w", err)
		}
		a.updates.SetSelfUserID(self.ID)
		if a.cfg.Bot.OwnerID == 0 {
			a.cfg.Bot.OwnerID = self.ID
		}
		a.logger.Info("authenticated", zap.Int64("user_id", self.ID), zap.String("username", self.Username))

		if err := a.plugins.InitAll(ctx); err != nil {
			return fmt.Errorf("plugin init: %w", err)
		}
		if err := a.plugins.StartAll(ctx); err != nil {
			return fmt.Errorf("plugin start: %w", err)
		}
		_ = a.bus.Emit(ctx, eventbus.EventStart, map[string]any{"version": Version, "user_id": self.ID})

		a.logger.Info("PaperValet ready", zap.String("version", Version), zap.String("prefix", a.cfg.Bot.CommandPrefix))
		<-ctx.Done()
		return ctx.Err()
	})
}

func (a *App) Shutdown(ctx context.Context) error {
	a.logger.Info("shutting down")
	a.cron.Stop()
	_ = a.plugins.StopAll(ctx)
	_ = a.bus.Shutdown(ctx)
	if a.sessions != nil {
		_ = a.sessions.Close()
	}
	_ = logger.Sync()
	return nil
}

// GetCronManager returns the cron manager for plugins.
func (a *App) GetCronManager() *cron.Manager {
	return a.cron
}