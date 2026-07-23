# PaperValet 外部插件 SDK

[English](plugin-sdk.md) | **中文**

本文档说明如何以共享库（`.so`）的形式为 PaperValet 开发外部插件。

## 插件结构

每个插件是一个 Go 包，需要提供 `New()` 函数并返回 `plugin.Plugin` 接口。

### 必须实现的接口

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
// Metadata 变量可选，供插件加载器读取
var Metadata = &loader.PluginMetadata{
    Name:        "my-plugin",
    Description: "我的自定义插件",
    Version:     "1.0.0",
    Author:      "你的名字",
    MinVersion:  "0.1.0",  // 所需的最低 PaperValet 版本
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

// New 是插件加载器的入口函数
func New() interface{} {
    return &MyPlugin{}
}

// Metadata 供插件加载器读取
var Metadata = &loader.PluginMetadata{
    Name:        "myplugin",
    Description: "自定义插件示例",
    Version:     "1.0.0",
    Author:      "你的名字",
    MinVersion:  "0.1.0",
}
```

### 编译插件

以 plugin 模式编译（生成 .so，而非可执行文件）：

```bash
go build -buildmode=plugin -o myplugin.so ./myplugin
```

复制到 plugins 目录中：

```bash
cp myplugin.so /path/to/papervalet/plugins/
```

## 与 PaperValet 一起构建

### 方法 1：使用示例模板

克隆模板仓库：

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

### 方法 2：作为 Go module

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

1. **加载（Load）** — PaperValet 通过 `plugin.Open()` 加载 `.so` 文件
2. **创建（New）** — 调用 `New()` 获取 `plugin.Plugin` 实例
3. **初始化（Init）** — 传入插件管理器，用来注册指令
4. **启动（Start）** — 所有插件初始化完成后统一调用
5. **停止（Stop）** — 关闭时调用，释放资源

## Command Context 方法

```go
ctx.Message    // *interfaces.MessageEvent - 触发指令的消息
ctx.Args       // []string - 解析后的参数
ctx.GetArg(i)  // string - 按索引取参数
ctx.ArgCount() // int - 参数个数
ctx.Edit(text) // error - 编辑指令消息（userbot 交互方式）
ctx.Reply(text) // error - 回复消息
ctx.Delete()   // error - 删除指令消息
ctx.API        // *tg.Client - Telegram API 客户端
ctx.PeerResolver // interfaces.PeerResolver - 用于解析账号
ctx.Emitter    // interfaces.Emitter - 触发事件
ctx.Session    // *interfaces.SessionContext - 当前会话上下文
ctx.Logger     // interfaces.Logger - 日志记录器
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

    return ctx.Edit(fmt.Sprintf("计数器: %d", count))
}
```

## 事件触发

```go
func (p *MyPlugin) handleCmd(ctx *interfaces.CommandContext) error {
    ctx.Emitter.Emit(ctx.Context(), "myplugin.custom_event", map[string]any{
        "user_id": ctx.Message.UserID,
        "data":    "自定义数据",
    })
    return nil
}
```

## 版本兼容性

- 在 Metadata 中设置 `MinVersion` 来声明所需的最低 PaperValet 版本
- 插件加载器会在加载前校验版本
- 建议使用语义化版本号（如 `"0.1.0"`、`"1.0.0"`）

## 最佳实践

1. **优雅处理错误** — 在 handler 中返回 error，不要 panic
2. **遵循 context 规范** — 尊重 `ctx.Context()` 的取消信号
3. **不要阻塞主线程** — 耗时任务请放到独立的 goroutine
4. **正确清理资源** — 在 `Stop()` 中释放所有占用的资源
5. **使用结构化日志** — 通过 `ctx.Logger` 记录日志，不要用 fmt.Println
6. **只依赖公开 API** — 只使用 `pkg/plugin` 中的接口，不要依赖 `internal/` 包

## 发布插件

1. 为插件创建 GitHub 仓库
2. 配置 GitHub Actions，在推送 tag 时自动构建 `.so`
3. 在 Release 中附带 `.so` 构建产物
4. 用户下载后放到 `plugins/` 目录即可使用

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
