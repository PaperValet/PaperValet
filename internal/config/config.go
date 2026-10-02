package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/pkg/logger"
)

// BotTokenRe matches a @BotFather token: numeric bot id, colon, secret.
var BotTokenRe = regexp.MustCompile(`^[0-9]{5,20}:[A-Za-z0-9_-]{30,}$`)

type Config struct {
	Telegram TelegramConfig `json:"telegram"`
	Bot      BotConfig      `json:"bot"`
	Logger   LoggerConfig   `json:"logger"`
	I18n     I18nConfig     `json:"i18n"`
}

// I18nConfig controls language behavior.
type I18nConfig struct {
	DefaultLanguage string `json:"default_language"`
}

type TelegramConfig struct {
	APIID       int    `json:"api_id"`
	APIHash     string `json:"api_hash"`
	SessionFile string `json:"session_file"`
	Database    string `json:"database_file"`
	// BotToken logs in the companion bot that hosts settings panels.
	BotToken       string `json:"bot_token"`
	BotSessionFile string `json:"bot_session_file,omitempty"`
}

type BotConfig struct {
	CommandPrefix   string   `json:"command_prefix"`
	CommandPrefixes []string `json:"command_prefixes,omitempty"`
	PluginsDir      string   `json:"plugins_dir"`
	PluginRepo      string   `json:"plugin_repo,omitempty"`
	OwnerID         int64    `json:"owner_id,omitempty"`
	MaxMessageLen   int      `json:"max_message_len,omitempty"`
	RateLimit       int      `json:"rate_limit,omitempty"`
}

type LoggerConfig struct {
	Level  string `json:"level"`
	Format string `json:"format"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Set defaults
	if cfg.Bot.CommandPrefix == "" {
		cfg.Bot.CommandPrefix = "."
	}
	if cfg.Bot.PluginsDir == "" {
		cfg.Bot.PluginsDir = "plugins"
	}
	if cfg.Bot.PluginRepo == "" {
		cfg.Bot.PluginRepo = "https://github.com/PaperValet/PaperValet-Plugins/releases/latest/download"
	}
	if cfg.Bot.MaxMessageLen == 0 {
		cfg.Bot.MaxMessageLen = 4000
	}
	if cfg.Bot.RateLimit == 0 {
		cfg.Bot.RateLimit = 3
	}
	if cfg.Telegram.BotSessionFile == "" {
		cfg.Telegram.BotSessionFile = "bot_session.json"
	}
	if cfg.Logger.Level == "" {
		cfg.Logger.Level = "INFO"
	}
	if cfg.Logger.Format == "" {
		cfg.Logger.Format = "console"
	}

	// Ensure command_prefix is in command_prefixes
	if len(cfg.Bot.CommandPrefixes) == 0 {
		cfg.Bot.CommandPrefixes = []string{cfg.Bot.CommandPrefix}
	} else {
		hasMain := false
		for _, p := range cfg.Bot.CommandPrefixes {
			if p == cfg.Bot.CommandPrefix {
				hasMain = true
				break
			}
		}
		if !hasMain {
			cfg.Bot.CommandPrefixes = append([]string{cfg.Bot.CommandPrefix}, cfg.Bot.CommandPrefixes...)
		}
	}

	// Expand paths
	cfg.Telegram.SessionFile = expandPath(cfg.Telegram.SessionFile)
	cfg.Telegram.Database = expandPath(cfg.Telegram.Database)
	cfg.Telegram.BotSessionFile = expandPath(cfg.Telegram.BotSessionFile)
	cfg.Bot.PluginsDir = expandPath(cfg.Bot.PluginsDir)

	// Init logger
	if err := logger.Init(cfg.Logger.Level, cfg.Logger.Format); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func expandPath(path string) string {
	if path == "" {
		return ""
	}
	if path[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}

func Example() *Config {
	return &Config{
		Telegram: TelegramConfig{
			APIID:       12345,
			APIHash:     "your_api_hash",
			SessionFile: "session.json",
			Database:    "sessions.db",
			BotToken:    "123456:your_bot_token",
		},
		Bot: BotConfig{
			CommandPrefix:   ".",
			CommandPrefixes: []string{".", "!", "/"},
			PluginsDir:      "plugins",
			OwnerID:         0,
			MaxMessageLen:   4000,
			RateLimit:       3,
		},
		Logger: LoggerConfig{
			Level:  "INFO",
			Format: "console",
		},
	}
}

// Save writes the config atomically with owner-only permissions; it holds
// api_hash, so it must never be world-readable.
func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func (c *Config) GetUpdateTimeout() time.Duration {
	return 30 * time.Second
}

// GetPrefixes returns all configured command prefixes.
func (c *Config) GetPrefixes() []string {
	return c.Bot.CommandPrefixes
}

// Validate checks if the config has required fields.
func (c *Config) Validate() error {
	var errs []string
	if c.Telegram.APIID == 0 {
		errs = append(errs, "telegram.api_id is required")
	}
	if c.Telegram.APIHash == "" {
		errs = append(errs, "telegram.api_hash is required")
	}
	if !BotTokenRe.MatchString(c.Telegram.BotToken) {
		errs = append(errs, "telegram.bot_token is required (from @BotFather)")
	}
	if c.Bot.CommandPrefix == "" {
		errs = append(errs, "bot.command_prefix is required")
	}
	if len(errs) > 0 {
		return &ConfigError{Errors: errs}
	}
	return nil
}

type ConfigError struct {
	Errors []string
}

func (e *ConfigError) Error() string {
	return "config validation failed: " + strings.Join(e.Errors, "; ")
}
