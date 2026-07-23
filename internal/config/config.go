package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/pkg/logger"
)

type Config struct {
	Telegram TelegramConfig `json:"telegram"`
	Bot      BotConfig      `json:"bot"`
	Logger   LoggerConfig   `json:"logger"`
}

type TelegramConfig struct {
	APIID       int    `json:"api_id"`
	APIHash     string `json:"api_hash"`
	SessionFile string `json:"session_file"`
	Database    string `json:"database_file"`
}

type BotConfig struct {
	CommandPrefix  string   `json:"command_prefix"`
	CommandPrefixes []string `json:"command_prefixes,omitempty"`
	PluginsDir     string   `json:"plugins_dir"`
	PluginRepo     string   `json:"plugin_repo,omitempty"`
	OwnerID        int64    `json:"owner_id,omitempty"`
	MaxMessageLen  int      `json:"max_message_len,omitempty"`
	RateLimit      int      `json:"rate_limit,omitempty"`
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
		cfg.Bot.PluginRepo = "https://github.com/TiaraBasori/PaperValet-Plugins/releases/latest/download"
	}
	if cfg.Bot.MaxMessageLen == 0 {
		cfg.Bot.MaxMessageLen = 4000
	}
	if cfg.Bot.RateLimit == 0 {
		cfg.Bot.RateLimit = 3
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

func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
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