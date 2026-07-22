package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
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
	CommandPrefix string `json:"command_prefix"`
	PluginsDir    string `json:"plugins_dir"`
	OwnerID       int64  `json:"owner_id,omitempty"` // optional, for owner-only commands
}

type LoggerConfig struct {
	Level  string `json:"level"`
	Format string `json:"format"` // "json" or "console"
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
	if cfg.Logger.Level == "" {
		cfg.Logger.Level = "INFO"
	}
	if cfg.Logger.Format == "" {
		cfg.Logger.Format = "console"
	}

	// Expand paths
	cfg.Telegram.SessionFile = expandPath(cfg.Telegram.SessionFile)
	cfg.Telegram.Database = expandPath(cfg.Telegram.Database)
	cfg.Bot.PluginsDir = expandPath(cfg.Bot.PluginsDir)

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
			CommandPrefix: ".",
			PluginsDir:    "plugins",
			OwnerID:       0,
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
