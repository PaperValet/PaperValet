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

`New` → `Init` (register commands, settings and pages) → `Start` → … → `Stop`.

Background goroutines start in `Start` and must exit in `Stop`. `apt rm` and `reload` call `Stop` too, not just shutdown.

## Commands

| Field | Meaning |
|---|---|
| `Name`, `Aliases` | Command and extra names; at most one short form for a long name (`ddg`, `st`); users add their own with `.alias` |
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
h.Settings(spec)             // register the settings panel, see below
h.Bot("hello")               // the companion bot, scoped to this plugin
```

## Settings: the bot panel, not commands

**Rule: options are never set with commands.** No `hello set x`, no `hello config`, no on/off subcommands. Declare the options once and PaperValet renders them as a button panel in the companion bot (`/menu`). Commands are for actions.

```go
func (p *Hello) Init(_ context.Context, mgr plugin.Manager) error {
	set, err := mgr.Host().Settings(&plugin.SettingsSpec{
		Plugin: "hello", Title: "👋 打招呼", TitleEN: "👋 Hello",
		Settings: []plugin.Setting{
			{Key: "emoji", Label: "表情", LabelEN: "Emoji", Kind: plugin.SettingToggle, Default: true},
			{Key: "style", Label: "风格", LabelEN: "Style", Kind: plugin.SettingChoice, Default: "plain",
				Choices: []plugin.Choice{{Value: "plain", Label: "普通", LabelEN: "Plain"}, {Value: "bold", Label: "加粗", LabelEN: "Bold"}}},
			{Key: "name", Label: "默认名字", LabelEN: "Default name", Kind: plugin.SettingText,
				Validate: func(s string) (string, error) {
					if len(s) > 32 {
						return "", plugin.Invalid("太长了", "too long")
					}
					return s, nil
				}},
			{Key: "times", Label: "次数", LabelEN: "Times", Kind: plugin.SettingNumber, Default: 1, Min: 1, Max: 5},
		},
	})
	if err != nil {
		return err
	}
	p.set = set // read with set.Bool("emoji"), set.String("style"), set.Int("times")
	...
}
```

| Kind | Panel | Go type |
|---|---|---|
| `SettingToggle` | one tap flips it | `bool` |
| `SettingChoice` | a button per choice | `string` |
| `SettingText` | owner types it, `Validate` checks and normalizes | `string` |
| `SettingNumber` | owner types it, kept within `Min..Max` | `int` |

Values live in `data/<plugin>/settings.json`. Read them when you need them; they change while the plugin runs. `OnChange(key)` fires after each change if you must react at once. `Secret` masks a value in the panel. Validation errors come from `plugin.Invalid(zh, en)` so the owner reads them in their language.

## Bot pages and messages

For more than a setting, a list with per-item buttons, a dashboard, a confirmation, add a page. It appears under the plugin in `/menu`, and `Command` also opens it with `/<command>`.

```go
b := mgr.Host().Bot("hello")
b.SetPage(&plugin.Page{
	Title: "收到的问候", TitleEN: "Greetings",
	Handle: func(c *plugin.BotContext) (*plugin.View, error) {
		switch {
		case strings.HasPrefix(c.Data, "rm:"):
			remove(strings.TrimPrefix(c.Data, "rm:"))
			c.Toast(c.Tlocal("已删除", "Deleted"))
		case c.Data == "add":
			c.Ask("new") // next text the owner sends comes back as Data "new"
			return &plugin.View{Text: c.Tlocal("发送名字", "Send a name")}, nil
		case c.Data == "new":
			add(c.Input)
		}
		v := &plugin.View{Text: "👋 " + c.Tlocal("问候", "Greetings")}
		for _, g := range list() {
			v.Buttons = append(v.Buttons, plugin.Row(plugin.Btn("🗑 "+g, "rm:"+g)))
		}
		v.Buttons = append(v.Buttons, plugin.Row(plugin.Btn("➕", "add")))
		return v, nil
	},
})
```

- `Data` is `""` when the page opens, otherwise the pressed button's data (32 bytes max, scoped to your plugin, so no prefixes needed).
- Color special buttons with `.Primary()` (main action), `.Success()` (add, confirm) or `.Danger()` (remove), e.g. `plugin.Btn("🗑", "rm:x").Danger()`. Clients without button colors show them plain.
- The bot adds the back button. `Toast` and `Alert` answer the tap.
- `b.Notify(ctx, view)` messages the owner, `b.Send` posts to any chat the bot can write to, `b.Edit` updates a message it sent. Buttons on those messages also come back to `Handle`.
- Calls fail with `plugin.ErrBotNotReady` until the bot is online. `SetPage` works any time.
- Pages and settings disappear when the plugin unloads; saved values stay.
- The bot answers only the owner.

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
