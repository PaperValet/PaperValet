---
name: build-plugin
description: Build, test and load-check a PaperValet external plugin (.so). Use when creating or changing a plugin under PaperValet-Plugins/plugins-external, when a plugin fails to load ("different version of package", "plugin was built with a different version", "New symbol has wrong type"), or before pushing plugin changes.
---

# Build a PaperValet plugin

External plugins are Go plugins. The bot only loads a `.so` whose Go toolchain, build flags and every shared package match its own build exactly. Almost every plugin failure comes from breaking one of these.

## Quick path

```bash
.agents/skills/build-plugin/scripts/build-plugin.sh <plugin-dir> [out-dir]
```

It reads the Go version from `.github/workflows/ci.yml`, sets up the `go work` workspace, runs gofmt, vet and tests, builds with `-trimpath -buildmode=plugin`, and loads the result through the real loader (`TestLoadSO` in `internal/plugin/loader`). A pass there means the bot will load it.

Set `PAPERVALET_SRC` if the PaperValet checkout is not at `../../../PaperValet` from the plugin dir. Output defaults to `~/.cache/papervalet-plugins/<name>.so`.

## Rules that must hold

- **Same Go version as the release.** It is pinned as `GO_VERSION` in PaperValet's CI, in the Dockerfile, and in the plugin repo's workflow. Bump all three together. Locally use `GOTOOLCHAIN=go<version>`.
- **Build through a workspace.** `go work init . /path/to/PaperValet`. Building through the `replace` directive in `go.mod` alone records `PaperValet@v0.1.0` paths and the bot rejects the `.so` with "different version of package". `go.work` is gitignored and never committed.
- **`-trimpath` everywhere.** The bot is built with it, so the plugin must be too.
- **cgo on, linux/amd64.** Plugins need `CGO_ENABLED=1`. Only the linux/amd64 release and local builds can load them. The Docker image and cross-compiled bundles cannot.
- **SDK changes rebuild everything.** Any change to `pkg/plugin` (a new `Manager` method, a type change) makes every existing `.so` unloadable. Push PaperValet first, then the plugin repo so CI rebuilds all plugins against it, then redeploy the binary and plugins together.
- **Never build in a small tmpfs.** Each `.so` is 60 to 80 MB. Keep `GOTMPDIR` and outputs on disk.

## Plugin shape

```
plugins-external/<name>/
  main.go        package main, New, Metadata, commands
  *_test.go      unit tests for parsing and pure logic
  go.mod         module + require PaperValet + replace => ../../../PaperValet
```

- `New` takes no arguments and returns a value implementing `plugin.Plugin` (a concrete pointer is fine), optionally with an `error`.
- `var Metadata = &plugin.PluginMetadata{...}` with `Name` equal to `Name()`, `Description`, `DescEN`, `Version`, `Author`. `scripts/gen-registry.py` reads it to build `plugins.json`, the `apt` index.
- Every command sets `Description`, `DescEN`, `Usage`, `UsageEN`. No `Aliases` except one short form for a long name (`ddg`, `st`), and one spelling per subcommand. Users add shortcuts with `.alias`.
- Text is Telegram Markdown. Wrap user input with `plugin.Escape` or `plugin.Code`. Pick language with `ctx.Tlocal(zh, en)`.
- State goes in `data/<name>/` (use `mgr.Host().DataDir(name)`). Goroutines start in `Start` and stop in `Stop`.
- Work outside a command (schedulers, restored jobs) takes the client from `mgr.Host()` in `Init`, never by reflection into host internals.
- Plugins import only `github.com/TiaraBasori/PaperValet/pkg/plugin`, never `internal/...`.

## Diagnosing load failures

| Error | Cause | Fix |
|---|---|---|
| `different version of package .../pkg/plugin` | built via replace, other Go version, or stale against a changed SDK | rebuild with the script against the current PaperValet |
| `plugin was built with a different version of package <dep>` | a shared dependency (gotd/td, zap…) differs | align the plugin's `go.mod` with PaperValet's, `go mod tidy` inside the workspace |
| `New symbol has wrong type` / `plugin missing New` | `New` takes arguments or does not return a Plugin | fix the signature |
| `plugin.Open ... cannot load` on Docker or arm64 | no cgo or no plugin support in that build | use the linux/amd64 release or a local build |
| loads but `apt ls` shows no version | `Metadata` missing or wrong type | export `var Metadata = &plugin.PluginMetadata{...}` |

## Shipping

1. One file per commit in PaperValet-Plugins. Push `main`.
2. CI (`plugins-release.yml`) builds each plugin in a workspace against PaperValet master and republishes the `latest` release with every `.so` and `plugins.json`.
3. A removed plugin keeps its old asset in the release. Delete it with `gh release delete-asset latest <name>.so -R PaperValet/PaperValet-Plugins -y`.
4. On a running bot: `.apt i <name>` for a new plugin. To replace an installed `.so`, copy it into `plugins/` and `.restart`. Go cannot swap code in-process, so `.reload` is not enough.
