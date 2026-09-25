# PaperValet

**English** | [中文](README_zh.md)

A production-grade Telegram Userbot built with [gotd/td](https://github.com/gotd/td).

Clean, modular architecture — no "TeleBox legacy" spaghetti.

## Features

- **Modern gotd/td stack** — Pure Go MTProto, no CGO
- External plugins are maintained in the separate [PaperValet-Plugins](https://github.com/TiaraBasori/PaperValet-Plugins) repository and installed as `.so` files.
- **Event bus** — Priority-based pub/sub for updates
- **Peer resolution** — Access hash caching with fallback chain
- **Session management** — SQLite + in-memory cache
- **Structured logging** — Zap with console/JSON output
- **Terminal auth** — Interactive login (phone, code, 2FA)

## Built-in Plugins

| Plugin | Commands | Description |
|--------|----------|-------------|
| `core` | `.version`, `.ping`, `.restart` | Core bot management |
| `help` | `.help` | Command help by category |
| `status` | `.status` | Runtime status + metrics |
| `apt` | `.apt list/install/remove/load/unload` | Plugin manager |
| `info` | `.info`, `.fwd` | Chat/user info + forward |
| `alias` | `.alias set/del/list` | Runtime command aliases |
| `exec` | `.exec` | System command execution |
| `sudo` | `.sudo on/off/add/remove/list` | Permission delegation |
| `reload` | `.reload` | External plugin hot reload |
| `log` | `.loglevel`, `.sendlog` | Log level + log delivery |
| `prefix` | `.prefix list/add/del/set` | Command prefix management |
| `backup` | `.backup` | Config/session backup + restore |
| `update` | `.update`, `.autofix` | Git sync + restart |
| `dme` | `.dme`, `.dme all` | Message cleanup |
| `lang` | `.lang` | Language switching |

## Quick Start

### One-line Install (recommended)

```bash
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/PaperValet/PaperValet/main/scripts/install.sh | bash

# Windows (PowerShell)
iwr -useb https://raw.githubusercontent.com/PaperValet/PaperValet/main/scripts/install.ps1 | iex
```

By default the scripts launch an **interactive menu** with the following options:

| # | Action | Purpose |
|---|--------|---------|
| 1 | `install` | First-time install to `~/.papervalet` |
| 2 | `reinstall` | Overwrite install; preserves `config.json` by default |
| 3 | `upgrade` | Upgrade to the selected version; preserves `config.json` + `session.json` + `sessions.db` |
| 4 | `uninstall` | Remove the install (with prompts to keep config / data) |
| 5 | `status` | Show install path, version, binary SHA256, `.so` count |
| 6 | `latest` | List the most recent GitHub release and its bundles |
| 7 | `set-phone` | Persist `PAPERVALET_PHONE` (and optional `PAPERVALET_CODE` / `PAPERVALET_2FA_PASSWORD`) into `~/.papervalet.env` |
| 8 | `run` | Launch the bot in the foreground |
| 9 | `doctor` | Self-check (curl / jq / disk / arch / GitHub reachability) |
| 0 | `exit` | Quit |
| v | — | Switch version (latest / vX.Y.Z) |
| h | — | Switch install directory |

If you pass `--non-interactive` (or any of `--phone`, `--api-id`, `--api-hash`)
the script skips the menu and runs `install` directly — useful for CI.

CLI flags: `--version <tag>`, `--home <dir>`, `--phone`, `--api-id`, `--api-hash`,
`--no-config`, `--keep-config`, `--keep-data`, `--repo owner/repo`.
Full list via `--help`.

Re-running the same `curl | bash` line at any time drops you back into the
menu so you can upgrade, uninstall, or change settings without remembering
command syntax.

### Manual Download

Grab a bundle from [GitHub Releases](https://github.com/PaperValet/PaperValet/releases/latest),
unpack it, edit `config.json`, run `./run.sh` (or `run.cmd` on Windows).

### Docker

```bash
docker run -d --name papervalet \
  -v "$PWD/config:/app/config" \
  -v "$PWD/data:/app/data" \
  --restart unless-stopped \
  ghcr.io/papervalet/papervalet:latest
```

The image ships with all built-in plugins and prebuilt `.so` plugins.

### Build from source

If you prefer to compile yourself or hack on the code:

```bash
git clone https://github.com/TiaraBasori/PaperValet
cd PaperValet
go build -o papervalet ./cmd/papervalet
cp config.example.json config.json
# edit config.json: fill api_id / api_hash from https://my.telegram.org
./papervalet -config config.json
```

First run uses interactive login (phone, code, 2FA if enabled).

### Headless / Container Login

Set these environment variables to skip the TTY prompts:

| Variable | Purpose |
|----------|---------|
| `PAPERVALET_PHONE` | E.164 phone number, e.g. `+8613800138000` |
| `PAPERVALET_CODE` | One-time login code sent by Telegram |
| `PAPERVALET_2FA_PASSWORD` | Cloud 2FA password |
| `PAPERVALET_NONINTERACTIVE` | Set to `1`/`true` to fail fast if any of the above is missing (otherwise the script will fall through to stdin prompts) |

### Architecture Note (arm64)

GitHub Actions currently ships prebuilt `.so` plugins only for `amd64` (Go's
`buildmode=plugin` on arm64 is not officially supported). On an Apple Silicon
or arm64 Linux host the bundle's main binary is native `arm64`, but the
`plugins/*.so` files inside the archive are `amd64`. Two options:

1. Use a Rosetta/x86-compatible runtime to load the `amd64` `.so` files.
2. Build `.so` yourself from the PaperValet-Plugins repo: `cd <name> && go build -buildmode=plugin -o ../<name>.so .`

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

### Run with config

```bash
./papervalet -config config.json
```

### Run with default config.json in cwd

```bash
./papervalet
```

First run: enter phone (+86...), then code, then 2FA password if enabled.

### Commands

Prefix is `.` by default.

| Command | Description |
|---------|-------------|
| `.help` | List all commands |
| `.help <cmd>` | Command detail |
| `.status` | Bot status |
| `.ping` | Latency check |
| `.info` | Chat/user IDs |
| `.apt list` | List plugins |
| `.exec uname -a` | Run system command |
| `.backup` | Create a backup |
| `.dme all 10` | Delete own recent messages |
| `.alias set p ping` | Create a command alias |
| `.prefix add ,` | Add a command prefix |

External plugins (installed via `.apt install`) add more commands, e.g.
`.roll`, `.coin`, `.choose`, `.8ball`, `.fact` (fun), `.username`, `.name`,
`.bio`, `.rmpfp` (account) and `.save` (save).

## Architecture

```
cmd/papervalet/main.go
internal/
  app/
  command/
  config/
  core/
  eventbus/
  media/
  peer/
  plugin/
  session/
plugins/builtin/
pkg/logger/
```

| Path | Role |
|------|------|
| `cmd/papervalet/main.go` | Entry point |
| `internal/app/` | App orchestrator + auth + update handler |
| `internal/command/` | Parser, registry, middleware |
| `internal/config/` | JSON config with defaults |
| `internal/core/` | Types: MessageEvent, CommandContext, interfaces |
| `internal/eventbus/` | Priority pub/sub |
| `internal/media/` | Download/upload helpers |
| `internal/peer/` | AccessHashManager + Resolver |
| `internal/plugin/` | Manager + Plugin interface |
| `internal/session/` | SQLite session store |
| `plugins/builtin/` | Compiled-in plugins |
| `pkg/logger/` | Zap wrapper |

### Key Design Decisions

| Concern | Approach |
|---------|----------|
| Commands | Typed `CommandContext` with `Reply`/`Edit`/`Delete` helpers |
| Plugins | Minimal interface: `Init/Start/Stop` + `RegisterCommand` |
| Events | `EventBus` with priority, filters, async emit |
| Peers | Cache-first `AccessHashManager` → API → ID-pattern fallback |
| Sessions | SQLite (WAL) + in-memory LRU, TTL cleanup |

## Development

### Add dependency

```bash
go get github.com/some/pkg
```

### Build

```bash
go build -o papervalet ./cmd/papervalet
```

### Run tests

```bash
go test ./...
```

### Lint

```bash
go vet ./...
golangci-lint run
```

### Adding a Plugin

Create `plugins/myplugin/myplugin.go`:

```go
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

## External Plugins

The `plugins` directory supports loading external `.so` plugins dynamically. See the [Plugin SDK](docs/plugin-sdk.md) / [中文版](docs/plugin-sdk_zh.md).

External plugins live in the separate [PaperValet-Plugins](https://github.com/TiaraBasori/PaperValet-Plugins) repo.

## License

MIT — see [LICENSE](LICENSE).
