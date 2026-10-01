# Installation

**English** · [中文](installation_zh.md)

You need a Linux or macOS machine, `curl`, and API credentials from [my.telegram.org/apps](https://my.telegram.org/apps).

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/PaperValet/PaperValet/master/scripts/install.sh | bash
```

Choose **Install** and a command name (Enter keeps `papervalet`). The installer puts the binary in `~/.<name>/bin` and links the command into `/usr/local/bin` (root) or `~/.local/bin`.

Each command name is a separate instance with its own data directory, so several accounts can run side by side.

## Set up

```bash
papervalet initialize
```

Five steps: language, `api_id` / `api_hash`, phone number, login code (plus 2FA password if set), and an optional systemd service that starts on boot. Running it again keeps your previous answers as defaults.

Then send `.ping` in any chat. The bot only reacts to your own messages.

## Run

```bash
papervalet run                 # foreground
systemctl status papervalet    # if you registered the service
journalctl -u papervalet -f
```

Non-root services are user units, so add `--user` to `systemctl` and `journalctl`.

Data directory layout:

```
~/.papervalet
├── bin/papervalet
├── config.json      # 0600, holds api_hash
├── session.json     # Telegram login
├── sessions.db
├── plugins/         # external .so plugins
└── data/            # plugin and runtime state
```

## Upgrade and uninstall

From Telegram, `.update now` downloads the latest release and restarts. `.update -f` reinstalls it even when the version already matches.

From the shell, run the installer again and choose **Upgrade** or **Uninstall**. Upgrading keeps all data. Uninstalling asks before deleting the data directory.

```bash
bash install.sh --upgrade --name papervalet -y
```

## Docker

```bash
docker run -d --name papervalet --restart unless-stopped \
  -v "$PWD/config:/app/config" \
  -v "$PWD/data:/app/data" \
  -e PAPERVALET_PHONE=+8613800138000 \
  -e PAPERVALET_CODE=12345 \
  -e PAPERVALET_2FA_PASSWORD=secret \
  ghcr.io/papervalet/papervalet:latest
```

Put `config.json` in `./config` (see [`config.example.json`](../config.example.json)) and point `session_file` and `database_file` at `data/` so the login survives restarts. The login variables are only needed for the first start. Login codes are single-use.

The image is built without cgo, so it cannot load external plugins.

## From source

```bash
git clone https://github.com/PaperValet/PaperValet && cd PaperValet
make build
./papervalet initialize
./papervalet run
```

## Configuration

`initialize` writes `config.json`. Fields you might edit by hand:

| Field | Default | Meaning |
|---|---|---|
| `bot.command_prefix` | `.` | Main command prefix |
| `bot.command_prefixes` | | Extra prefixes, also managed with `.prefix` |
| `bot.owner_id` | `0` | Owner account; `0` means the logged-in account |
| `bot.plugin_repo` | PaperValet-Plugins latest release | Where `apt` downloads plugins |
| `logger.level` | `INFO` | `DEBUG`, `INFO`, `WARN`, `ERROR` |
| `i18n.default_language` | `zh-CN` | `zh-CN` or `en-US` |

Environment variables:

| Variable | Meaning |
|---|---|
| `PAPERVALET_HOME` | Data directory, default `~/.papervalet` |
| `PAPERVALET_CONFIG` | Config file path; the working directory is not changed |
| `PAPERVALET_PHONE`, `PAPERVALET_CODE`, `PAPERVALET_2FA_PASSWORD` | Login without a terminal |

## Troubleshooting

**`command not found`**: open a new terminal or `source ~/.bashrc`.

**Not logged in**: run `papervalet initialize` again.

**No reaction**: the command must come from your account and start with the prefix.

**Plugin fails with "different version"**: the plugin was built with another Go version. Run `.update -f`, then install it again.

**Plugins won't load at all**: only the linux/amd64 release can load external plugins.
