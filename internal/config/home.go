package config

import (
	"os"
	"path/filepath"
	"strings"
)

// FileName is the config file inside the data home.
const FileName = "config.json"

// Home returns the instance data directory: $PAPERVALET_HOME when set,
// otherwise ~/.papervalet. Every relative path the bot uses (session, db,
// plugins, data/, downloads/) lives under it.
func Home() (string, error) {
	if v := strings.TrimSpace(os.Getenv("PAPERVALET_HOME")); v != "" {
		return filepath.Abs(expandPath(v))
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".papervalet"), nil
}

// CommandName is the shell command this instance was registered under.
// The installer's wrapper exports PAPERVALET_CMD; a bare binary falls back
// to its own file name.
func CommandName() string {
	if v := strings.TrimSpace(os.Getenv("PAPERVALET_CMD")); v != "" {
		return v
	}
	name := filepath.Base(os.Args[0])
	return strings.TrimSuffix(name, ".exe")
}

// Default returns the config initialize writes for a fresh instance.
func Default() *Config {
	return &Config{
		Telegram: TelegramConfig{
			SessionFile: "session.json",
			Database:    "sessions.db",
		},
		Bot: BotConfig{
			CommandPrefix: ".",
			PluginsDir:    "plugins",
		},
		Logger: LoggerConfig{
			Level:  "INFO",
			Format: "console",
		},
	}
}
