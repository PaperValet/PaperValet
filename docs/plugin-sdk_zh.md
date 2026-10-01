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

`New` → `Init`（注册命令）→ `Start` → … → `Stop`。

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
```

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
