# Installation Guide

[English](installation.md) | [中文](installation_zh.md)

## Requirements

- Linux (amd64/arm64) or macOS
- `curl` and `tar` (the installer checks these for you)
- A Telegram account
- API credentials (`api_id` / `api_hash`) from [my.telegram.org](https://my.telegram.org) → *API development tools*

## One-line Install

```bash
curl -fsSL https://raw.githubusercontent.com/PaperValet/PaperValet/master/scripts/install.sh | bash
```

An interactive menu opens. Pick `1) install` and you will be asked for three things:

| Prompt | What to enter |
|--------|---------------|
| `api_id` | Numeric ID from my.telegram.org |
| `api_hash` | Hash string from my.telegram.org |
| `phone` | Your phone number in E.164 format, e.g. `+8613800138000` |

Everything is installed to `~/.papervalet`:

```
~/.papervalet
├── bin/papervalet      # the bot binary
├── plugins/            # external .so plugins land here
├── config.json         # generated from your answers
└── run.sh              # launcher
```

## First Login

Start the bot:

```bash
~/.papervalet/run.sh
```

Because the installer saved your phone number, only two prompts remain:

1. **Login code** — Telegram sends a code to your app; type it in and press Enter. Codes expire in a few minutes, so do this promptly.
2. **2FA password** — only if you have two-step verification enabled.

On success you will see `authenticated` in the log followed by plugin loading lines. The bot is now live — send `.ping` to yourself in any chat (Saved Messages works well) and it should answer `🏓 Pong!`.

> The bot only reacts to **your own outgoing messages**. Other people's messages are ignored.

## Running as a Service (systemd)

```bash
sudo tee /etc/systemd/system/papervalet.service >/dev/null <<EOF
[Unit]
Description=PaperValet userbot
After=network-online.target

[Service]
User=%i
WorkingDirectory=$HOME/.papervalet
EnvironmentFile=$HOME/.papervalet.env
ExecStart=$HOME/.papervalet/bin/papervalet -config $HOME/.papervalet/config.json
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable --now papervalet
journalctl -u papervalet -f        # follow logs
```

Once the session file exists, restarts never ask for a code again — login data is persisted in `session.json`.

## Docker

```bash
docker run -d --name papervalet \
  -v "$PWD/config:/app/config" \
  -v "$PWD/data:/app/data" \
  --restart unless-stopped \
  ghcr.io/papervalet/papervalet:latest
```

For the first login inside a container, pass credentials through the environment:

```bash
docker run -it --rm \
  -e PAPERVALET_PHONE=+8613800138000 \
  -e PAPERVALET_CODE=12345 \
  -e PAPERVALET_2FA_PASSWORD=yourpassword \
  -v "$PWD/config:/app/config" \
  -v "$PWD/data:/app/data" \
  ghcr.io/papervalet/papervalet:latest
```

The code is single-use; generate a fresh one for the container login.

## Manual Install

1. Download the bundle for your platform from [Releases](https://github.com/PaperValet/PaperValet/releases/latest) — e.g. `papervalet-linux-amd64.tar.gz`.
2. Extract: `tar -xzf papervalet-linux-amd64.tar.gz && cd papervalet-linux-amd64`
3. Edit `config.json` (copy from `config.example.json`) and fill in `api_id` / `api_hash`.
4. Run `./run.sh`.

## From Source

```bash
git clone https://github.com/PaperValet/PaperValet
cd PaperValet
go build -o papervalet ./cmd/papervalet
cp config.example.json config.json   # fill in api_id / api_hash
./papervalet -config config.json
```

Go 1.25+ is required.

## Upgrade / Uninstall

Re-run the installer and pick the matching menu item:

```bash
curl -fsSL https://raw.githubusercontent.com/PaperValet/PaperValet/master/scripts/install.sh | bash
```

- `3) upgrade` — replaces the binary, keeps `config.json`, `session.json` and `sessions.db`
- `4) uninstall` — removes the install; you can keep config and session data

## Troubleshooting

**`PHONE_CODE_EXPIRED`** — the code expired before you entered it. Press `Ctrl+C`, start again, and enter the fresh code quickly.

**Bot does not respond** — commands must be sent **from your own account** and start with the prefix (default `.`). Check the log level with `.loglevel debug` if needed.

**arm64 hosts** — prebuilt `.so` plugins in release bundles are amd64-only. The core bot runs natively on arm64; external plugins can be built from the [PaperValet-Plugins](https://github.com/PaperValet/PaperValet-Plugins) repo with `go build -buildmode=plugin`.

**Where are my files?** — everything lives under `~/.papervalet`. `session.json` is your Telegram login; back it up if you want to skip future logins.
