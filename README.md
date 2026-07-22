# PaperValet

A production-grade Telegram Userbot built with [gotd/td](https://github.com/gotd/td).

Clean, modular architecture — no "TeleBox legacy" spaghetti.

## Features

- **Modern gotd/td stack** — Pure Go MTProto, no CGO
- **Plugin system** — Hot-loadable, typed commands with middleware
- **Event bus** — Priority-based pub/sub for updates
- **Peer resolution** — Access hash caching with fallback chain
- **Session management** — SQLite + in-memory cache
- **Structured logging** — Zap with console/JSON output
- **Terminal auth** — Interactive login (phone, code, 2FA)

## Built-in Plugins

| Plugin | Commands | Description |
|--------|----------|-------------|
| `core` | `.help`, `.status` | Core bot management |
| `apt` | `.apt list/enable/disable` | Plugin manager |
| `tools` | `.ping`, `.uptime`, `.info`, `.fwd` | Utility commands |
| `remind` | `.remind` | Reminders (in-memory) |
| `cron` | `.cron` | Scheduled tasks |
| `note` | `.note` | Personal notes |
| `fun` | `.roll`, `.coin`, `.choose`, `.8ball`, `.fact` | Entertainment |
| `admin` | `.restart`, `.shutdown`, `.gc`, `.version` | Owner-only |

## Quick Start

```bash
# 1. Clone & build
git clone https://github.com/TiaraBasori/PaperValet
cd PaperValet
go build -o papervalet ./cmd/papervalet

# 2. Configure
cp config.example.json config.json
# Edit config.json with your api_id/api_hash from my.telegram.org

# 3. Run (first run: interactive login)
./papervalet -config config.json
```

## Configuration

`config.json`:

```json
{
  "telegram": {
    "api_id": 12345,
    "api_hash": "your_api_hash",
    "session_file": "session.json",
    "database_file": "sessions.db"
  },
  "bot": {
    "command_prefix": ".",
    "plugins_dir": "plugins",
    "owner_id": 0
  },
  "logger": {
    "level": "INFO",
    "format": "console"
  }
}
```

- `api_id` / `api_hash` — Get from https://my.telegram.org
- `command_prefix` — Command trigger (default: `.`)
- `owner_id` — Telegram user ID for owner-only commands (0 = first logged-in user)
- `logger.level` — DEBUG, INFO, WARN, ERROR
- `logger.format` — `console` (colored) or `json`

## Usage

```bash
# Run with config
./papervalet -config config.json

# Or default config.json in cwd
./papervalet
```

First run: enter phone (+86...), then code, then 2FA password if enabled.

Commands (prefix `.`):
```
.help           # List all commands
.help <cmd>     # Command detail
.status         # Bot status
.ping           # Latency check
.uptime         # Uptime + memory
.info           # Chat/user IDs
.apt list       # List plugins
.remind 5m drink water
.cron add daily 0 0 9 * * * .status
.note set todo "Buy milk"
.roll 20
.coin
.choose pizza burger sushi
.8ball Will it rain?
.fact
```

## Architecture

```
cmd/papervalet/main.go          # Entry point
internal/
  app/                          # App orchestrator + auth + update handler
  command/                      # Parser, registry, middleware
  config/                       # JSON config with defaults
  core/                         # Types: MessageEvent, CommandContext, interfaces
  cron/                         # Scheduled jobs (robfig/cron)
  eventbus/                     # Priority pub/sub
  media/                        # Download/upload helpers
  peer/                         # AccessHashManager + Resolver
  plugin/                       # Manager + Plugin interface
  session/                      # SQLite session store
plugins/builtin/                # Compiled-in plugins
pkg/logger/                     # Zap wrapper
```

### Key Design Decisions

| Concern | Approach |
|---------|----------|
| Commands | Typed `CommandContext` with `Reply`/`Edit`/`Delete` helpers |
| Plugins | Minimal interface: `Init/Start/Stop` + `RegisterCommand` |
| Events | `EventBus` with priority, filters, async emit |
| Peers | Cache-first `AccessHashManager` → API → ID-pattern fallback |
| Sessions | SQLite (WAL) + in-memory LRU, TTL cleanup |

## Development

```bash
# Add dependency
go get github.com/some/pkg

# Build
go build -o papervalet ./cmd/papervalet

# Run tests
go test ./...

# Lint
go vet ./...
golangci-lint run
```

### Adding a Plugin

```go
// plugins/myplugin/myplugin.go
package myplugin

import (
    "context"
    "github.com/TiaraBasori/PaperValet/internal/command"
    "github.com/TiaraBasori/PaperValet/internal/core"
    "github.com/TiaraBasori/PaperValet/internal/plugin"
)

type MyPlugin struct{}

func New() *MyPlugin { return &MyPlugin{} }
func (p *MyPlugin) Name() string        { return "myplugin" }
func (p *MyPlugin) Description() string { return "My cool plugin" }
func (p *MyPlugin) Init(ctx context.Context, mgr *plugin.Manager) error {
    return mgr.RegisterCommand(&command.Command{
        Name: "hello", Description: "Say hello",
        Plugin: p.Name(), Handler: p.hello,
    })
}
func (p *MyPlugin) Start(ctx context.Context) error { return nil }
func (p *MyPlugin) Stop(ctx context.Context) error  { return nil }

func (p *MyPlugin) hello(ctx *core.CommandContext) error {
    return ctx.Reply("Hello from my plugin!")
}
```

Register in `internal/app/app.go`:
```go
import _ "github.com/TiaraBasori/PaperValet/plugins/myplugin"
```

## External Plugins (Planned)

The `plugins` directory will support loading `.so` plugins dynamically.

## License

MIT — see [LICENSE](LICENSE).