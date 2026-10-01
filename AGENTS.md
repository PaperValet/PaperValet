# AGENTS.md

Guide for coding agents working on PaperValet, a Telegram userbot in Go built on gotd/td. Read [ARCHITECTURE.md](ARCHITECTURE.md) for how the pieces fit.

## Repositories

- `PaperValet/PaperValet` (this repo, branch `master`): the bot, built-in plugins, the public SDK in `pkg/plugin`.
- `PaperValet/PaperValet-Plugins` (branch `main`): every external plugin, under `plugins-external/<name>/`. Third-party plugin code never goes in this repo.

The Go module path is `github.com/TiaraBasori/PaperValet` even though the GitHub org is `PaperValet`. Do not rename it.

## Toolchain

- Go **1.25.14** exactly (`GO_VERSION` in `.github/workflows/ci.yml`). If the system Go is older, set `GOTOOLCHAIN=go1.25.14`.
- Release builds use `-trimpath`. Plugins must match, see below.
- Builds are large. Put `GOTMPDIR` and build outputs on disk, not in a small `/tmp` tmpfs.

## Verify before you commit

```bash
gofmt -l .                  # must print nothing
go vet ./...
go test ./...
GOOS=windows go build ./... # Windows needs *_unix.go / *_windows.go splits
```

Some tests `t.Chdir` into a temp dir because plugins persist to `data/`. Do the same in new tests that touch the filesystem.

## Conventions

- **Small commits.** One file per commit where practical, with conventional messages (`feat(apt): …`, `fix(loader): …`, `test(…)`, `docs: …`).
- **No compatibility shims.** Remove replaced code and docs outright. No deprecation paths, no "legacy" flags unless asked.
- **Bilingual.** User-facing text comes in zh-CN and en-US. Commands set `Description` + `DescEN` and `Usage` + `UsageEN`. Handlers choose with `ctx.Tlocal(zh, en)`. CLI and setup strings live in `internal/i18n` and have a parity test.
- **Markdown output.** Replies are Telegram Markdown. Escape anything user-controlled with `plugin.Escape`, `plugin.Code` or `esc` in built-ins. Backslash escapes do not work inside code spans.
- **Shared card style.** Built-ins format output with the helpers in `plugins/builtin/ui.go` (`newCard`, `field`, `hint`, `okLine`…). Keep new output consistent with them.
- **Minimal aliases.** Commands carry no aliases unless the name is long (`help` → `h`, `duckduckgo` → `ddg`, `speedtest` → `st`). Users add their own with `.alias`.
- **State under `data/`.** Built-ins write `data/<file>`; plugins use `data/<plugin>/` (`mgr.Host().DataDir`). Files with secrets are `0600`.
- **Channels need channel APIs.** In supergroups and channels use `channels.deleteMessages` and friends; `messages.*` variants silently do nothing there. `ctx.DeleteMessages` already handles it.

## Built-in plugins

`plugins/builtin/`, registered in `internal/app/app.go` (`registerBuiltins`): ping, restart, status, info, re, dme, exec, apt, reload, update, backup, sudo, alias, prefix, lang, log. Adding one means also adding it to `builtinOrder` in `help.go` and the fixture in `help_test.go`, which checks that `help` lists every command.

## The SDK and external plugins

`pkg/plugin` is the only package external plugins import. Treat it as a public API:

- Any change to an exported type or interface (for example a new `Manager` method) breaks every built `.so`. After such a change, push this repo first, then push PaperValet-Plugins so its CI rebuilds all plugins, then redeploy binary and plugins together.
- Plugins never reach into `internal/` or use reflection on host types. If a plugin needs something, add it to the SDK (`plugin.Host` is the place for services outside command handlers).

To build, test or debug a plugin, use the skill in [`.agents/skills/build-plugin`](.agents/skills/build-plugin/SKILL.md). It encodes the workspace build and runs a real load check. The short version: `go work init . /path/to/PaperValet`, then `go build -trimpath -buildmode=plugin`. A `replace`-only build compiles fine and then fails to load.

## Releases

- Pushing `master` runs CI: vet, test, five platform builds, and a GHCR image.
- A `v*` tag also publishes bundles to GitHub Releases. Do not tag or re-cut a release unless the maintainer asks. To replace one: `gh release delete <tag> --cleanup-tag -y`, then tag and push again.
- The plugin repo publishes on every push to `main` into a rolling `latest` release. Removed plugins leave stale assets there; delete them by hand.

## Docs

`README.md`, `README_zh.md`, `docs/installation*.md`, `docs/plugin-sdk*.md` and `ARCHITECTURE.md` describe the current code. When behavior, commands, flags or config change, update the English and Chinese docs in the same change. Keep them short.

## Secrets and accounts

Never commit `config.json`, `session.json`, `sessions.db`, tokens or phone numbers. Live testing happens on a real Telegram account; ask before sending messages, deleting anything or logging in.
