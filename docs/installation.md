# Installation Guide

[English](installation.md) | [中文](installation_zh.md)

## Requirements

- Linux (amd64/arm64) or macOS
- `curl` and `tar`
- A Telegram account
- API credentials (`api_id` / `api_hash`) from [my.telegram.org/apps](https://my.telegram.org/apps)

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/PaperValet/PaperValet/master/scripts/install.sh | bash
```

Pick `1) Install` and choose a command name (Enter gives `papervalet`). The installer only does two things:

- puts the binary in `~/.papervalet/bin/papervalet`
- registers the command in `/usr/local/bin` (root) or `~/.local/bin` (other users, added to `PATH` in `~/.bashrc`)

Each command name is its own instance with its own data directory, so several accounts can live side by side. `papervalet` uses `~/.papervalet`, any other name `<name>` uses `~/.<name>`.

## Setup

```bash
papervalet initialize
```

The installer offers to start this right away. Five steps, all in the language you pick first:

1. **Language** — 简体中文 or English; also becomes the bot's default language
2. **Telegram API** — `api_id` and `api_hash`
3. **Phone number** — with country code, e.g. `+8613800138000`
4. **Login** — the code Telegram sends you, plus your 2FA password if you have one
5. **Keep running** — optionally register a systemd service (Enter gives the command name) that starts on boot

Running `initialize` again keeps your old answers as defaults, and lets you switch accounts.

Send `.ping` to yourself in any chat (Saved Messages works well); the bot answers `🏓 Pong!`.

> The bot only reacts to **your own outgoing messages**.

## Running

Without the service:

```bash
papervalet run
```

With the service (root shown; user services add `--user`):

```bash
systemctl status papervalet
journalctl -u papervalet -f
```

```
~/.papervalet
├── bin/papervalet
├── config.json      # written by initialize (0600)
├── session.json     # your Telegram login
├── sessions.db
├── plugins/
└── data/
```

## Upgrade / Uninstall

Re-run the installer and pick `2) Upgrade` or `3) Uninstall`.

- Upgrade replaces the binary, keeps all data, and restarts the instance's service if there is one.
- Uninstall removes the command and service, then asks whether to delete the data directory too.

Non-interactive: `bash install.sh --upgrade --name papervalet -y`.

## Docker

```bash
docker run -d --name papervalet \
  -v "$PWD/config:/app/config" \
  -v "$PWD/data:/app/data" \
  --restart unless-stopped \
  ghcr.io/papervalet/papervalet:latest
```

The container reads `/app/config/config.json`. For the first login, pass credentials through the environment:

```bash
docker run -it --rm \
  -e PAPERVALET_PHONE=+8613800138000 \
  -e PAPERVALET_CODE=12345 \
  -e PAPERVALET_2FA_PASSWORD=yourpassword \
  -v "$PWD/config:/app/config" \
  -v "$PWD/data:/app/data" \
  ghcr.io/papervalet/papervalet:latest
```

The code is single-use; request a fresh one for the container login.

## From Source

```bash
git clone https://github.com/PaperValet/PaperValet
cd PaperValet
go build -o papervalet ./cmd/papervalet
./papervalet initialize
./papervalet run
```

Go 1.25+ is required. `-config path/to/config.json` still works and keeps the current directory as the working directory.

## Troubleshooting

**`papervalet: command not found`** — open a new terminal, or run `source ~/.bashrc`.

**Not logged in** — run `papervalet initialize` again.

**Bot does not respond** — commands must come **from your own account** and start with the prefix (default `.`).

**arm64 hosts** — prebuilt `.so` plugins in release bundles are amd64-only. Build them from [PaperValet-Plugins](https://github.com/PaperValet/PaperValet-Plugins) with `go build -buildmode=plugin`.
