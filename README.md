<div align="center">

# PaperValet

A Telegram userbot in pure Go, built on [gotd/td](https://github.com/gotd/td).

**English** · [中文](README_zh.md)

</div>

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/PaperValet/PaperValet/master/scripts/install.sh | bash
papervalet initialize
```

`initialize` walks you through language, API credentials, login, a companion bot and an optional systemd service. Get `api_id` / `api_hash` at [my.telegram.org](https://my.telegram.org/apps) and a bot token from [@BotFather](https://t.me/BotFather). Then send `.ping` in any chat.

Docker, upgrades and troubleshooting: **[Installation](docs/installation.md)**.

## Commands

Everything is triggered by your own messages. `.help` lists all commands, `.help <command>` explains one.

Settings are not commands. Language, prefixes, sudo, log level and every plugin's options live in the companion bot: send it `/menu` and tap.

| Plugin | Command | What it does |
|---|---|---|
| ping | `.ping [host]`, `.pingdc` | Latency to Telegram, a host, or every DC |
| status | `.status` | Version, host, resources, runtime |
| info | `.info` | User, chat and message details |
| re | `.re [count] [times]` | Repeat the replied message |
| dme | `.dme <n\|all> [-f]` | Delete your recent messages |
| exec | `.exec <cmd>` | Run a shell command |
| apt | `.apt s / i / rm / ls / info` | Install and remove external plugins |
| reload | `.reload` | Reload external plugins |
| restart | `.restart` | Restart in place |
| update | `.update [now\|-f]` | Upgrade from GitHub Releases |
| backup | `.backup [restore]` | Back up config to Saved Messages |
| sudo | `.sudo add / remove` | Let other users run commands |
| alias | `.alias name=command` | Your own shortcuts |
| log | `.sendlog [tail\|clean]` | Log files |

## Plugins

External plugins come from [PaperValet-Plugins](https://github.com/PaperValet/PaperValet-Plugins):

```
.apt s           browse
.apt i weather   install one
.apt i -all      install everything
```

To write one, see the [Plugin SDK](docs/plugin-sdk.md).

## Development

```bash
make build     # ./papervalet
make test
make lint
```

Go 1.25. External plugins must be built with the same Go version and `-trimpath` as the binary.

## License

[MIT](LICENSE)
