# Architecture

## Layout

```
cmd/papervalet/        CLI: run, initialize, version
internal/
  app/                 wiring, login, update handler
  bot/                 companion bot: settings panels, plugin pages
  command/             parser, registry, middleware, plugin Host
  config/              config.json, data home, defaults
  eventbus/            pub/sub with priorities
  i18n/                zh-CN / en-US catalogs, per-user language
  media/               file upload and download
  peer/                access hash cache and peer resolver
  plugin/              plugin manager
  plugin/loader/       .so loader and apt downloads
  session/             SQLite session store
  settings/            per-plugin settings store
  setup/               interactive initialize
pkg/
  plugin/              public SDK, the only package plugins import
  logger/              zap wrapper
plugins/builtin/       built-in plugins
examples/              sample external plugin
scripts/               installers
```

## Message flow

1. gotd delivers updates to `app.UpdateHandler`, which records access hashes and emits a `message` event.
2. `command.Parser` subscribes to `message`. It checks the sender (owner, or a sudo user's incoming message), strips a prefix, expands user aliases once, and matches the longest registered command.
3. `command.Registry` builds a `CommandContext` and runs the handler through recovery, logging and rate-limit middleware. `OwnerOnly` commands are checked here.
4. Handlers answer with `ctx.Edit` or `ctx.Reply`. Text is Telegram Markdown, converted to entities by `pkg/plugin.ParseMarkdown`.

## Companion bot

A second gotd client logs in with `telegram.bot_token` and runs beside the userbot; if it fails, the userbot keeps going. It answers only the owner. After login the userbot sends it `/start` once, because a bot cannot open a chat.

The bot is the settings UI. Plugins declare options with `Host().Settings(spec)` (`internal/settings`, stored in `data/<plugin>/settings.json`) and the bot renders them as button panels under `/menu`, which has three panels: system settings and external plugins (only plugins with settings or a page, buttons named after the plugins), and the plugin manager, which lists installed and repository plugins together and installs, removes and reloads them; settings and management never share a screen (`internal/app/catalog.go` feeds them from the manager, registry and loader). Plugins can also add one page each and post messages through `Host().Bot(name)`; callback data is namespaced per plugin. Unloading a plugin removes its panel and page.

Options are never commands. language and prefix have no command at all; sudo and log keep only their actions.

Code: `service.go` (client, send/edit, button colors), `updates.go` (messages, commands, taps, typed answers), `panel.go` (callback data layout, settings screens, plugin pages), `catalog.go` (menu, panels, manager, install/remove/reload with a one-op lock), `progress.go` (live progress panel). Taps slower than 4s are answered early and the view follows. Typed-answer prompts expire after 15 minutes, ops after 10. Stale or unknown buttons redraw a valid screen instead of failing.

Every screen is an icon plus bold title, with lists and values in a block quote. Back is always first in its row. Long operations edit their message with a bar, done/total, elapsed time and per-plugin results, refreshed within 1s of a change and at least every 3s. Results start with `✅`/`⚠️`/`❌`; failures carry a retry button. Button colors: primary for navigation into management and retry, success for install and enabled toggles, danger for remove and failed plugins.

## Plugins

Built-in and external plugins implement the same `plugin.Plugin` interface and register commands through `plugin.Manager`.

Built-ins are compiled in and registered in `app.registerBuiltins`. There are 16: ping, restart, status, info, re, dme, exec, apt, reload, update, backup, sudo, alias, prefix, language, log.

External plugins are `.so` files in `plugins/`, loaded at startup. `apt i` downloads one from the plugin repository release, loads it and starts it right away. `apt rm` stops it and deletes the file. There is no installed-but-disabled state.

Plugins that work outside command handlers (schedulers, restored tasks) take long-lived services from `mgr.Host()`.

Go plugins cannot be unloaded. `reload` re-runs `Init`/`Start` on the loaded code. Replacing a `.so` takes a `restart`.

## Runtime state

Everything lives in the data home (default `~/.papervalet`, the working directory of the process):

- `config.json`: API credentials, bot token and startup defaults, mode 0600
- `session.json`, `bot_session.json`, `sessions.db`: account and bot logins, per-chat session state
- `data/`: peer caches, aliases, sudo list, restart marker, and one `data/<plugin>/` per plugin holding its `settings.json`
- `plugins/`: external plugins

## Restart and update

`restart` stops all plugins, flushes sessions and logs, then re-executes the binary under the same PID. Supervisors (systemd, Docker, tmux) never see an exit. After login the app edits the command message into the elapsed time.

`update now` downloads the matching release bundle, swaps only the binary and uses the same restart path.

## Build and release

- Go 1.25.14 with `-trimpath` everywhere, pinned in CI, the Dockerfile and the plugin repo.
- CI on `master`: vet, test, cross-builds for linux, darwin and windows, and a multi-arch image on GHCR.
- A `v*` tag publishes bundles (`bin/papervalet`, `config.example.json`, `LICENSE`, `README`, `plugins/`) to GitHub Releases.
- The plugin repo CI builds each plugin inside a `go work` workspace with the latest PaperValet and republishes the `latest` release with every `.so` and `plugins.json`.
