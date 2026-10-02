# 插件 SDK

[English](plugin-sdk.md) · **中文**

外部插件是 Go plugin（`.so`），只引用 `github.com/TiaraBasori/PaperValet/pkg/plugin`。完整可运行的例子在 [`examples/plugin-example`](../examples/plugin-example)。

## 最小插件

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

加载器查找两个符号：

- `New`：不带参数，返回任何实现了 `plugin.Plugin` 的值，第二个返回值可以是 `error`。
- `Metadata`（可选）：`*plugin.PluginMetadata`，`apt ls` 和 `apt info` 会显示。

## 生命周期

`New` → `Init`（注册命令、设置和页面）→ `Start` → … → `Stop`。

后台 goroutine 在 `Start` 里启动，在 `Stop` 里退出。`apt rm` 和 `reload` 也会调用 `Stop`，不只是关机时。

## 命令

| 字段 | 含义 |
|---|---|
| `Name`、`Aliases` | 命令名和别名；长命令最多带一个简写（`ddg`、`st`），其他让用户用 `.alias` 加 |
| `Description`、`DescEN` | `help` 里的一行说明 |
| `Usage`、`UsageEN` | `help 命令` 显示的用法，可用 Markdown |
| `Plugin` | 所属插件，`help` 按它分组 |
| `OwnerOnly` | 只有主人和 sudo 用户能用 |
| `RateLimit` | 同一用户两次调用的最短间隔（秒） |

## 命令上下文

```go
ctx.Args, ctx.GetArg(i), ctx.GetArgs(), ctx.ArgCount()
ctx.Message           // 触发命令的消息（ChatID、UserID、ReplyToID、Message）
ctx.Edit(md)          // 编辑命令消息
ctx.Reply(md)         // 回复命令消息
ctx.ReplyMedia(path, caption)
ctx.Delete(), ctx.DeleteMessages(ids...)
ctx.Tlocal(zh, en)    // 按用户语言选字符串
ctx.API               // *tg.Client
ctx.PeerResolver, ctx.Media, ctx.Downloader, ctx.Logger
ctx.Context()         // 关机时取消
```

`DeleteMessages` 在频道和超级群里也能正确删除。

## Host 服务

`mgr.Host()` 提供同样的服务，不需要触发消息，适合定时任务和重启后恢复的任务。在 `Init` 里拿到，网络调用要等 `Start` 之后。

```go
h := mgr.Host()
h.API(), h.PeerResolver(), h.Media(), h.Downloader()
h.SelfID()                   // 当前账号（登录前为 0）
h.Logger("hello")
h.DataDir("hello")           // data/hello，按需创建
h.Send(ctx, chatID, md, 0)   // 返回新消息 id
h.Lang(userID)               // "zh-CN" 或 "en-US"
h.Settings(spec)             // 注册设置面板，见下文
h.Bot("hello")               // 配套机器人，限定在本插件范围
h.Prefixes()                 // 命令前缀，主前缀在前
```

### 监听消息

自动回复、验证码、自动删除这类功能要看到所有消息，不只是命令。`Listen` 会在命令执行前收到每条新消息和编辑（收发都有，命令消息也在内）：

```go
func (p *Hello) Start(context.Context) error {
	p.stop = p.host.Listen("hello", func(ctx context.Context, m *plugin.MessageEvent, edited bool) {
		if edited || m.IsOut || !strings.Contains(m.Text, "hi") {
			return
		}
		go p.host.Send(context.Background(), m.ChatID, "👋", m.Message.ID)
	})
	return nil
}

func (p *Hello) Stop(context.Context) error { p.stop(); return nil }
```

监听器在更新链路上依次执行，要尽快返回，慢活放进 goroutine。卸载插件时会自动移除。

`h.RunCommand(ctx, ev)` 以主人身份在 `ev` 所在聊天执行 `ev.Text` 命令，比如定时跑 `.status`：先发出文本，再传 `plugin.EventFromMessage(sent)`。

### 消息工具

```go
ctx.ReplyMessage()                              // 被回复的 *tg.Message
plugin.GetMessages(ctx, api, peer, ids...)      // 自动区分频道的拉取
plugin.DeleteMessages(ctx, api, peer, ids...)   // 自动区分频道的删除，不限条数
plugin.EventFromMessage(msg)                    // *tg.Message → *MessageEvent（给 Downloader、RunCommand 用）
plugin.ChatIDOf(peer), plugin.SenderID(msg)
```

## 设置：走机器人面板，不走命令

**规范：选项一律不用命令设置。** 不要 `hello set x`、`hello config`，也不要开关子命令。把选项声明一次，PaperValet 会在配套机器人（`/menu`）里渲染成按钮面板。命令只负责执行动作。

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
	p.set = set // 用 set.Bool("emoji")、set.String("style")、set.Int("times") 读
	...
}
```

| Kind | 面板里 | Go 类型 |
|---|---|---|
| `SettingToggle` | 点一下切换 | `bool` |
| `SettingChoice` | 每个选项一个按钮 | `string` |
| `SettingText` | 主人直接发文字，`Validate` 校验并规范化 | `string` |
| `SettingNumber` | 主人发数字，限制在 `Min..Max` | `int` |

值存在 `data/<插件>/settings.json`。用到时再读，运行中随时会变。需要立即响应就用 `OnChange(key)`，每次改动后调用。`Secret` 会在面板里打码。校验错误用 `plugin.Invalid(中文, 英文)`，主人看到的是自己的语言。

## 机器人页面和消息

比一个选项复杂的东西，比如带逐项按钮的列表、仪表盘、确认框，就加一个页面。它出现在 `/menu` 里该插件下面，设置了 `Command` 还能用 `/<命令>` 直接打开。

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
			c.Ask("new") // 主人接下来发的文字会以 Data "new" 回到这里
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

- 页面打开时 `Data` 为空，之后是被点按钮的 data（最多 32 字节，已按插件隔离，不用自己加前缀）。
- 特殊按钮可以上色：`.Primary()` 主操作，`.Success()` 新增或确认，`.Danger()` 删除，例如 `plugin.Btn("🗑", "rm:x").Danger()`。不支持按钮颜色的客户端照常显示。
- 返回按钮由机器人自动加。`Toast` 和 `Alert` 用来回应点击。
- `b.Notify(ctx, view)` 发给主人，`b.Send` 发到机器人能发言的任意聊天，`b.Edit` 修改它发过的消息。这些消息上的按钮同样回到 `Handle`。
- 机器人上线前调用会返回 `plugin.ErrBotNotReady`。`SetPage` 随时可调。
- 插件卸载时页面和设置自动移除，保存的值保留。
- 机器人只回应主人。

## Markdown

消息文本是 Telegram Markdown。来自用户的内容一律用辅助函数包一下，免得符号被当成格式：

```go
plugin.Escape(s)   plugin.Code(v)   plugin.Pre(s)
plugin.Bold(s)     plugin.Italic(s) plugin.Link(text, url)
plugin.Mention(text, userID)
```

代码片段里反斜杠转义无效，要用 `plugin.Code`。

## 构建

Go plugin 要求插件和主程序的 Go 版本、所有共享包完全一致。用和 Release 相同的 Go 版本（目前 1.25.14），在 workspace 里带 `-trimpath` 编译：

```bash
go work init . /path/to/PaperValet
go build -trimpath -buildmode=plugin -o hello.so .
```

如果靠 `replace` 指令编译，加载时会报 plugin was built with a different version of package。

把 `.so` 放进 `plugins/`，再 `.restart`。只有带 cgo 的 linux/amd64 构建能加载插件，Release 里的就是这种。

## 发布

在 [PaperValet-Plugins](https://github.com/PaperValet/PaperValet-Plugins) 的 `plugins-external/` 下新建目录。仓库 CI 会对着最新的 PaperValet 编译所有插件，发布 `.so` 和 `apt` 读取的 `plugins.json` 索引。
