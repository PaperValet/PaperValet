# Architecture

## Layout

```
cmd/papervalet/        CLI: run, initialize, version
internal/
  app/                 wiring, login, update handler
  command/             parser, registry, middleware, plugin Host
  config/              config.json, data home, defaults
  eventbus/            pub/sub with priorities
  i18n/                zh-CN / en-US catalogs, per-user language
  media/               file upload and download
  peer/                access hash cache and peer resolver
  plugin/              plugin manager
  plugin/loader/       .so loader and apt downloads
  session/             SQLite session store
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

## Plugins

Built-in and external plugins implement the same `plugin.Plugin` interface and register commands through `plugin.Manager`.

Built-ins are compiled in and registered in `app.registerBuiltins`. There are 16: ping, restart, status, info, re, dme, exec, apt, reload, update, backup, sudo, alias, prefix, lang, log.

External plugins are `.so` files in `plugins/`, loaded at startup. `apt i` downloads one from the plugin repository release, loads it and starts it right away. `apt rm` stops it and deletes the file. There is no installed-but-disabled state.

Plugins that work outside command handlers (schedulers, restored tasks) take long-lived services from `mgr.Host()`.

Go plugins cannot be unloaded. `reload` re-runs `Init`/`Start` on the loaded code. Replacing a `.so` takes a `restart`.

## Runtime state

Everything lives in the data home (default `~/.papervalet`, the working directory of the process):

- `config.json`: API credentials and settings, mode 0600
- `session.json`, `sessions.db`: Telegram login and per-chat session state
- `data/`: peers cache, aliases, prefixes, sudo list, language, restart marker, and one `data/<plugin>/` per plugin
- `plugins/`: external plugins

## Restart and update

`restart` stops all plugins, flushes sessions and logs, then re-executes the binary under the same PID. Supervisors (systemd, Docker, tmux) never see an exit. After login the app edits the command message into the elapsed time.

`update now` downloads the matching release bundle, swaps only the binary and uses the same restart path.

## Build and release

- Go 1.25.14 with `-trimpath` everywhere, pinned in CI, the Dockerfile and the plugin repo.
- CI on `master`: vet, test, cross-builds for linux, darwin and windows, and a multi-arch image on GHCR.
- A `v*` tag publishes bundles (`bin/papervalet`, `config.example.json`, `LICENSE`, `README`, `plugins/`) to GitHub Releases.
- The plugin repo CI builds each plugin inside a `go work` workspace with the latest PaperValet and republishes the `latest` release with every `.so` and `plugins.json`.
