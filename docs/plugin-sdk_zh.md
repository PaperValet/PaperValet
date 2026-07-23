# PaperValet 外部插件 SDK

[English](plugin-sdk.md) | **中文**

本文档说明如何将外部插件以共享库（`.so` 文件）的形式为 PaperValet 开发。

## 插件结构

每个插件是一个 Go 包，需提供 `New()` 函数，返回 `plugin.Plugin` 接口。

### 必需接口

```go
package main

import (
    "context"
    "github.com/PaperValet/PaperValet/internal/plugin"
)

// Plugin 是所有插件必须实现的接口
type Plugin interface {
    Name() string
    Description() string
    Init(ctx context.Context, mgr *plugin.Manager) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}
```

### 插件元数据（可选）

```go
// Metadata 变量（可选，供插件加载器使用）
var Metadata = &loader.PluginMetadata{
    Name:        "my-plugin",
    Description: "我的自定义插件",
    Version:     "1.0.0",
    Author:      "你的名字",
    MinVersion:  "0.1.0",  // 最低 PaperValet 版本
}
```

## 完整示例

### myplugin/main.go

```go
package main

import (
    "context"
    "fmt"
    "strings"

    "github.com/PaperValet/PaperValet/internal/interfaces"
    "github.com/PaperValet/PaperValet/internal/plugin"
)

// MyPlugin 实现 plugin.Plugin
type MyPlugin struct {
    mgr *plugin.Manager
}

func (p *MyPlugin) Name() string        { return "myplugin" }
func (p *MyPlugin) Description() string { return "自定义插件示例" }

// Init 注册指令
func (p *MyPlugin) Init(ctx context.Context, mgr *plugin.Manager) error {
    p.mgr = mgr

    return mgr.RegisterCommand(&interfaces.Command{
        Name:        "hello",
        Aliases:     []string{"hi"},
        Description: "打个招呼",
        Usage:       "hello [name]",
        Plugin:      p.Name(),
        Category:    "tools",
        Handler:     p.handleHello,
    })
}

func (p *MyPlugin) Start(ctx context.Context) error { return nil }
func (p *MyPlugin) Stop(ctx context.Context) error  { return nil }

func (p *MyPlugin) handleHello(ctx *interfaces.CommandContext) error {
    name := "World"
    if ctx.ArgCount() > 0 {
        name = ctx.GetArg(0)
    }
    return ctx.Edit(fmt.Sprintf("Hello, %s! 👋", name))
}

// New 是插件加载器的入口
func New() interface{} {
    return &MyPlugin{}
}

// Metadata 供插件加载器使用
var Metadata = &loader.PluginMetadata{
    Name:        "myplugin",
    Description: "自定义插件示例",
    Version:     "1.0.0",
    Author:      "你的名字",
    MinVersion:  "0.1.0",
}
```

### 编译插件

以 plugin 模式编译（非可执行文件）：

```bash
go build -buildmode=plugin -o myplugin.so ./myplugin
```

复制到 plugins 目录：

```bash
cp myplugin.so /path/to/papervalet/plugins/
```

## 与 PaperValet 一起构建

### 方法 1：使用示例模板

克隆示例：

```bash
git clone https://github.com/PaperValet/plugin-template myplugin
cd myplugin
```

编辑 `main.go` 实现插件逻辑，然后编译：

```bash
go build -buildmode=plugin -o myplugin.so .
```

安装：

```bash
mkdir -p ~/.config/papervalet/plugins
cp myplugin.so ~/.config/papervalet/plugins/
```

### 方法 2：Go module

`go.mod`：

```go
module github.com/yourname/myplugin

go 1.25

require github.com/PaperValet/PaperValet v0.1.0
```

```bash
go mod tidy
go build -buildmode=plugin -o myplugin.so .
```

## 插件生命周期

1. **Load** — PaperValet 通过 `plugin.Open()` 加载 `.so` 文件
2. **New()** — 调用 `New()`，期望返回 `plugin.Plugin`
3. **Init()** — 传入插件管理器，用于注册指令
4. **Start()** — 所有插件初始化完成后调用
5. **Stop()** — 关闭时调用

## Command Context 方法

```go
ctx.Message    // *interfaces.MessageEvent - 触发指令的消息
ctx.Args       // []string - 解析后的参数
ctx.GetArg(i)  // string - 按索引取参数
ctx.ArgCount() // int - 参数数量
ctx.Edit(text) // error - 编辑指令消息（userbot UX）
ctx.Reply(text) // error - 回复消息
ctx.Delete()   // error - 删除指令消息
ctx.API        // *tg.Client - Telegram API 客户端
ctx.PeerResolver // interfaces.PeerResolver - 解析 peer
ctx.Emitter    // interfaces.Emitter - 发射事件
ctx.Session    // *interfaces.SessionContext - 每会话上下文
ctx.Logger     // interfaces.Logger - 日志
```

## 会话用法

```go
func (p *MyPlugin) handleCmd(ctx *interfaces.CommandContext) error {
    session := ctx.Session
    if session == nil {
        return ctx.Edit("无可用会话")
    }

    // 读写会话数据
    count, _ := session.Get("counter")
    if count == nil {
        count = 0
    }
    count = count.(int) + 1
    session.Set("counter", count)

    return ctx.Edit(fmt.Sprintf("Counter: %d", count))
}
```

## 事件发射

```go
func (p *MyPlugin) handleCmd(ctx *interfaces.CommandContext) error {
    ctx.Emitter.Emit(ctx.Context(), "myplugin.custom_event", map[string]any{
        "user_id": ctx.Message.UserID,
        "data":    "custom data",
    })
    return nil
}
```

## 版本兼容性

- 在 Metadata 中设置 `MinVersion`，要求最低 PaperValet 版本
- 插件加载器在加载前校验版本
- 使用语义化版本（如 `"0.1.0"`、`"1.0.0"`）

## 最佳实践

1. **优雅处理错误** — 在 handler 中返回 error，不要 panic
2. **使用 context** — 尊重 `ctx.Context()` 的取消信号
3. **不要阻塞** — 长任务放到 goroutine
4. **清理资源** — 在 `Stop()` 中正确释放
5. **日志** — 使用 `ctx.Logger` 做结构化日志
6. **依赖** — 只依赖 PaperValet 公开接口，不要依赖 internal 包

## 发布插件

1. 为插件创建 GitHub 仓库
2. 在 tag 推送时用 GitHub Actions 构建 `.so`
3. 在 Release 中附带 `.so` 产物
4. 用户下载后放到 `plugins/` 目录

## 示例发布工作流

```yaml
# .github/workflows/release.yml
name: Release
on:
  push:
    tags: ['v*']
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - run: go build -buildmode=plugin -o myplugin.so .
      - uses: softprops/action-gh-release@v2
        with:
          files: myplugin.so
```
