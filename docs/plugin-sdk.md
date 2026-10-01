# Plugin SDK

**English** · [中文](plugin-sdk_zh.md)

External plugins are Go plugins (`.so`) that import only `github.com/TiaraBasori/PaperValet/pkg/plugin`. A working example lives in [`examples/plugin-example`](../examples/plugin-example).

## Minimal plugin

```go
package main

import (
	"context"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

var Metadata = &plugin.PluginMetadata{
	Name:        "hello",
	Description: "打招呼",
	DescEN:      "Say hello",
	Version:     "1.0.0",
	Author:      "you",
}

type Hello struct{}

func New() *Hello { return &Hello{} }

func (p *Hello) Name() string        { return "hello" }
func (p *Hello) Description() string { return Metadata.Description }
func (p *Hello) DescEN() string      { return Metadata.DescEN }

func (p *Hello) Init(_ context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&plugin.Command{
		Name:        "hello",
		Description: "打招呼",
		DescEN:      "Say hello",
		Usage:       "hello [名字]",
		UsageEN:     "hello [name]",
		Plugin:      p.Name(),
		Handler: func(ctx *plugin.CommandContext) error {
			return ctx.Edit("👋 " + plugin.Bold(ctx.GetArgs()))
		},
	})
}

func (p *Hello) Start(context.Context) error { return nil }
func (p *Hello) Stop(context.Context) error  { return nil }
```

The loader looks up two symbols:

- `New`: takes no arguments and returns anything that implements `plugin.Plugin`, optionally with an `error` as a second result.
- `Metadata` (optional): `*plugin.PluginMetadata`, shown by `apt ls` and `apt info`.

## Lifecycle

`New` → `Init` (register commands) → `Start` → … → `Stop`.

Background goroutines start in `Start` and must exit in `Stop`. `apt rm` and `reload` call `Stop` too, not just shutdown.

## Commands

| Field | Meaning |
|---|---|
| `Name`, `Aliases` | Command and extra names; plugins leave `Aliases` empty, users add their own with `.alias` |
| `Description`, `DescEN` | One line for `help` |
| `Usage`, `UsageEN` | Shown by `help <command>`; Markdown allowed |
| `Plugin` | Owning plugin, used to group commands in `help` |
| `OwnerOnly` | Only the owner and sudo users can run it |
| `RateLimit` | Minimum seconds between two calls per user |

## Command context

```go
ctx.Args, ctx.GetArg(i), ctx.GetArgs(), ctx.ArgCount()
ctx.Message           // triggering message (ChatID, UserID, ReplyToID, Message)
ctx.Edit(md)          // edit the command message
ctx.Reply(md)         // reply to it
ctx.ReplyMedia(path, caption)
ctx.Delete(), ctx.DeleteMessages(ids...)
ctx.Tlocal(zh, en)    // pick a string by the user's language
ctx.API               // *tg.Client
ctx.PeerResolver, ctx.Media, ctx.Downloader, ctx.Logger
ctx.Context()         // cancelled on shutdown
```

`DeleteMessages` handles channels and supergroups correctly.

## Host services

`mgr.Host()` offers the same services without a triggering message, for schedulers and restored jobs. Keep it from `Init`. Network calls work once `Start` runs.

```go
h := mgr.Host()
h.API(), h.PeerResolver(), h.Media(), h.Downloader()
h.SelfID()                   // logged-in account (0 before login)
h.Logger("hello")
h.DataDir("hello")           // data/hello, created on demand
h.Send(ctx, chatID, md, 0)   // returns the new message id
h.Lang(userID)               // "zh-CN" or "en-US"
```

## Markdown

Message text is Telegram Markdown. Anything that comes from users must go through a helper so stray symbols stay literal:

```go
plugin.Escape(s)   plugin.Code(v)   plugin.Pre(s)
plugin.Bold(s)     plugin.Italic(s) plugin.Link(text, url)
plugin.Mention(text, userID)
```

Backslash escapes do not work inside code spans, so use `plugin.Code` for those.

## Building

Go plugins only load when the plugin and the bot agree on the Go version and every shared package. Build inside a workspace with the same Go version as the release (currently 1.25.14) and `-trimpath`:

```bash
go work init . /path/to/PaperValet
go build -trimpath -buildmode=plugin -o hello.so .
```

Building through a `replace` directive instead fails with "plugin was built with a different version of package".

Copy the `.so` into `plugins/` and run `.restart`. Plugins only load on linux/amd64 builds with cgo, which is what the release ships.

## Publishing

Add a directory under `plugins-external/` in [PaperValet-Plugins](https://github.com/PaperValet/PaperValet-Plugins). Its CI builds every plugin against the latest PaperValet and publishes the `.so` files plus the `plugins.json` index that `apt` reads.
