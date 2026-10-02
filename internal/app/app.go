package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/internal/bot"
	"github.com/TiaraBasori/PaperValet/internal/command"
	"github.com/TiaraBasori/PaperValet/internal/config"
	"github.com/TiaraBasori/PaperValet/internal/eventbus"
	"github.com/TiaraBasori/PaperValet/internal/i18n"
	"github.com/TiaraBasori/PaperValet/internal/media"
	"github.com/TiaraBasori/PaperValet/internal/peer"
	"github.com/TiaraBasori/PaperValet/internal/plugin"
	"github.com/TiaraBasori/PaperValet/internal/plugin/loader"
	"github.com/TiaraBasori/PaperValet/internal/session"
	"github.com/TiaraBasori/PaperValet/internal/settings"
	"github.com/TiaraBasori/PaperValet/pkg/logger"
	pkgplugin "github.com/TiaraBasori/PaperValet/pkg/plugin"
	"github.com/TiaraBasori/PaperValet/plugins/builtin"
)

const Version = "0.1.0"

// App is the top-level orchestrator.
type App struct {
	cfg          *config.Config
	client       *telegram.Client
	api          *tg.Client
	bus          *eventbus.Bus
	commands     *command.Registry
	parser       *command.Parser
	plugins      pkgplugin.Manager
	pluginLoader *loader.Loader
	sessions     *session.Manager
	peers        *peer.Resolver
	accessHash   *peer.AccessHashManager
	updates      *UpdateHandler
	i18n         *i18n.Manager
	bot          *bot.Service
	logger       pkgplugin.Logger
	configPath   string
}

func New(cfg *config.Config) (*App, error) {
	if err := logger.Init(cfg.Logger.Level, cfg.Logger.Format); err != nil {
		return nil, fmt.Errorf("logger: %w", err)
	}
	log := logger.NamedLogger("app")

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

	// i18n: build catalog from core + builtin plugin catalogs.
	i18nCat := i18n.CoreCatalog()
	i18nCat.SetDefault(i18n.Lang(cfg.I18n.DefaultLanguage))
	i18nMgr := i18n.NewManager(i18nCat)

	bus := eventbus.New(log)
	updates := NewUpdateHandler(bus)

	client := NewTelegramClient(cfg, updates)

	api := client.API()
	accessHash := peer.NewAccessHashManager(api, peer.NewStore(filepath.Join("data", "peers.json")))
	resolver := peer.NewResolver(accessHash)
	updates.SetPeerRegistry(accessHash)

	cmdReg := command.NewRegistry(cfg.GetPrefixes(), bus, api, resolver, cfg.Bot.OwnerID, i18nMgr)
	mediaMgr := media.NewManager(api, resolver, "downloads")
	cmdReg.SetMediaSender(mediaMgr)
	botSvc := bot.New(BotOptions(cfg), settings.NewRegistry("data"), i18nMgr)
	cmdReg.SetBot(botSvc)
	parser := command.NewParser(cmdReg, bus)
	pluginMgr := plugin.NewManager(cmdReg, bus)

	pluginsDir := cfg.Bot.PluginsDir
	if pluginsDir == "" {
		pluginsDir = "plugins"
	}
	pluginLoader := loader.NewLoader(pluginsDir, pluginMgr)
	pluginLoader.SetRepoURL(cfg.Bot.PluginRepo)
	botSvc.SetCatalog(&catalog{mgr: pluginMgr, commands: cmdReg, loader: pluginLoader})

	app := &App{
		cfg:          cfg,
		client:       client,
		api:          api,
		bus:          bus,
		commands:     cmdReg,
		parser:       parser,
		plugins:      pluginMgr,
		pluginLoader: pluginLoader,
		sessions:     sessMgr,
		peers:        resolver,
		accessHash:   accessHash,
		updates:      updates,
		i18n:         i18nMgr,
		bot:          botSvc,
		logger:       log,
	}
	return app, nil
}

// device is the client identity both accounts present to Telegram.
var device = telegram.DeviceConfig{
	DeviceModel:    "PaperValet",
	SystemVersion:  "Linux",
	AppVersion:     Version,
	SystemLangCode: "en",
	LangCode:       "en",
}

// BotOptions derives the companion bot settings from cfg. initialize uses
// it too, so its login lands in the session file run reads.
func BotOptions(cfg *config.Config) bot.Options {
	return bot.Options{
		APIID:       cfg.Telegram.APIID,
		APIHash:     cfg.Telegram.APIHash,
		Token:       cfg.Telegram.BotToken,
		SessionFile: cfg.Telegram.BotSessionFile,
		PeersFile:   filepath.Join("data", "bot_peers.json"),
		Device:      device,
	}
}

// NewTelegramClient builds the gotd client with PaperValet's device info.
// initialize uses it too, so the login session matches what run expects.
func NewTelegramClient(cfg *config.Config, h telegram.UpdateHandler) *telegram.Client {
	return telegram.NewClient(cfg.Telegram.APIID, cfg.Telegram.APIHash, telegram.Options{
		SessionStorage: &telegram.FileSessionStorage{Path: cfg.Telegram.SessionFile},
		UpdateHandler:  h,
		Device:         device,
		RetryInterval:  time.Second,
		MaxRetries:     -1,
		DialTimeout:    15 * time.Second,
	})
}

// SetConfigPath tells the backup plugin which config file to include.
func (a *App) SetConfigPath(path string) { a.configPath = path }

func (a *App) registerBuiltins() error {
	backup := builtin.NewBackup()
	cfgPath := a.configPath
	if cfgPath == "" {
		cfgPath = config.FileName
	}
	backup.SetConfig(cfgPath)
	restart := builtin.NewRestart()
	restart.BeforeRestart = func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = a.plugins.StopAll(ctx)
		if a.sessions != nil {
			_ = a.sessions.Close()
		}
		_ = logger.Sync()
	}
	for _, p := range []pkgplugin.Plugin{
		builtin.NewPing(),
		restart,
		builtin.NewApt(a.pluginLoader),
		builtin.NewInfo(),
		builtin.NewRe(),
		builtin.NewAlias(),
		builtin.NewExec(),
		builtin.NewSudo(),
		builtin.NewReload(a.pluginLoader),
		builtin.NewLog(),
		builtin.NewPrefix(),
		builtin.NewHelp(),
		builtin.NewStatus(Version, a.pluginLoader),
		backup,
		builtin.NewUpdate(Version, restart.Restart),
		builtin.NewPrune(),
		builtin.NewLang(a.i18n),
	} {
		if err := a.plugins.RegisterPlugin(p); err != nil {
			return err
		}
	}
	// Wire the sudo plugin's permission checker into the command registry.
	if sudo, ok := a.plugins.GetPlugin("sudo"); ok {
		if s, ok := sudo.(*builtin.SudoPlugin); ok {
			a.commands.SetSudoChecker(s.IsSudoUser)
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

	// The bot runs beside the userbot; losing it never stops the userbot.
	botDone := make(chan struct{})
	go func() {
		defer close(botDone)
		_ = a.bot.Run(ctx)
	}()
	defer func() { <-botDone }()

	return a.client.Run(ctx, func(ctx context.Context) error {
		if err := EnsureAuth(ctx, a.client); err != nil {
			return fmt.Errorf("auth: %w", err)
		}

		self, err := a.client.Self(ctx)
		if err != nil {
			return fmt.Errorf("self: %w", err)
		}
		a.updates.SetSelfUserID(self.ID)
		// Seed the peer store with the account itself so self references
		// (Saved Messages, tg://user links) resolve instantly.
		a.accessHash.RegisterPeer(self.ID, self.AccessHash, "user")
		if a.cfg.Bot.OwnerID == 0 {
			a.cfg.Bot.OwnerID = self.ID
		}
		a.commands.SetOwnerID(a.cfg.Bot.OwnerID)
		a.commands.SetSelfID(self.ID)
		a.bot.SetOwner(self.ID)
		a.logger.Info("authenticated", "user_id", self.ID, "username", self.Username)

		if err := a.plugins.InitAll(ctx); err != nil {
			return fmt.Errorf("plugin init: %w", err)
		}
		if err := a.plugins.StartAll(ctx); err != nil {
			return fmt.Errorf("plugin start: %w", err)
		}

		if err := a.pluginLoader.LoadAll(ctx); err != nil {
			a.logger.Warn("external plugin load", "error", err)
		}

		builtin.FinishRestart(ctx, a.api, a.peers.ResolveFromChatID)
		go a.introduceBot(ctx)

		a.bus.Emit(ctx, eventbus.EventStart, map[string]any{"version": Version})

		<-ctx.Done()
		return nil
	})
}

// introduceBot has the account message its bot once. Bots cannot start a
// chat, so without this the bot could not reach the owner.
func (a *App) introduceBot(ctx context.Context) {
	wait, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if !a.bot.WaitReady(wait) || a.bot.KnowsOwner() {
		return
	}
	p, err := a.peers.ResolveUsername(ctx, a.bot.Username())
	if err != nil {
		a.logger.Warn("resolve bot", "error", err)
		return
	}
	if _, err := a.api.MessagesStartBot(ctx, &tg.MessagesStartBotRequest{
		Bot:        inputUser(p),
		Peer:       p,
		RandomID:   time.Now().UnixNano(),
		StartParam: "papervalet",
	}); err != nil {
		a.logger.Warn("start bot", "error", err)
	}
}

func inputUser(p tg.InputPeerClass) tg.InputUserClass {
	if u, ok := p.(*tg.InputPeerUser); ok {
		return &tg.InputUser{UserID: u.UserID, AccessHash: u.AccessHash}
	}
	return &tg.InputUserEmpty{}
}

// Shutdown gracefully stops the app.
func (a *App) Shutdown(ctx context.Context) error {
	a.logger.Info("shutting down")
	_ = a.plugins.StopAll(ctx)
	_ = a.bus.Shutdown(ctx)
	if a.sessions != nil {
		_ = a.sessions.Close()
	}
	_ = logger.Sync()
	return nil
}

// GetPluginLoader returns the external plugin loader.
func (a *App) GetPluginLoader() *loader.Loader {
	return a.pluginLoader
}

// GetI18n returns the i18n manager.
func (a *App) GetI18n() *i18n.Manager {
	return a.i18n
}
