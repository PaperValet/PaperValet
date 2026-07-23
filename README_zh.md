# PaperValet

[English](README.md) | **中文**

基于 [gotd/td](https://github.com/gotd/td) 构建的生产级 Telegram 用户机器人。

架构清晰、模块化 —— 告别 TeleBox 式的混乱代码。

## 功能特性

- **现代 gotd/td 技术栈** — 纯 Go MTProto，无需 CGO
- **插件系统** — 热加载、强类型指令与中间件
- **事件总线** — 带优先级的发布订阅
- **账号解析** — Access Hash 缓存 + 多级回退
- **会话管理** — SQLite + 内存缓存
- **结构化日志** — 基于 Zap，支持控制台和 JSON 输出
- **终端鉴权** — 交互式登录（手机号、验证码、两步验证）

## 内置插件

| 插件 | 指令 | 说明 |
|------|------|------|
| `core` | `.help`、`.status` | 机器人核心管理 |
| `apt` | `.apt list/enable/disable` | 插件管理器 |
| `tools` | `.ping`、`.uptime`、`.info`、`.fwd` | 实用工具指令 |
| `remind` | `.remind` | 提醒功能（仅内存） |
| `cron` | `.cron` | 定时任务 |
| `note` | `.note` | 个人笔记 |
| `fun` | `.roll`、`.coin`、`.choose`、`.8ball`、`.fact` | 娱乐指令 |
| `admin` | `.restart`、`.shutdown`、`.gc`、`.version` | 仅所有者可用 |

## 快速开始

### 1. 克隆并编译

```bash
git clone https://github.com/TiaraBasori/PaperValet
cd PaperValet
go build -o papervalet ./cmd/papervalet
```

### 2. 配置

```bash
cp config.example.json config.json
```

编辑 `config.json`，填入 [my.telegram.org](https://my.telegram.org) 获取的 `api_id` / `api_hash`。

### 3. 运行

```bash
./papervalet -config config.json
```

首次运行会进入交互式登录（依次填写手机号、验证码、以及两步验证密码）。

## 配置说明

`config.json`：

```json
{
  "telegram": {
    "api_id": 12345,
    "api_hash": "your_api_hash",
    "session_file": "session.json",
    "database_file": "sessions.db"
  },
  "bot": {
    "command_prefix": ".",
    "plugins_dir": "plugins",
    "owner_id": 0
  },
  "logger": {
    "level": "INFO",
    "format": "console"
  }
}
```

- `api_id` / `api_hash` — 从 https://my.telegram.org 获取
- `command_prefix` — 指令前缀（默认为 `.`）
- `owner_id` — 所有者的用户 ID（仅所有者指令需要；设为 `0` 则自动以首个登录用户为所有者）
- `logger.level` — 日志级别：DEBUG、INFO、WARN、ERROR
- `logger.format` — 输出格式：`console`（彩色）或 `json`

## 使用方式

### 指定配置文件运行

```bash
./papervalet -config config.json
```

### 使用当前目录下的默认 config.json

```bash
./papervalet
```

首次运行：输入手机号（格式如 +86...），然后输入验证码，若开启了两步验证还需输入密码。

### 指令

默认前缀为 `.`。

| 指令 | 说明 |
|------|------|
| `.help` | 列出全部指令 |
| `.help <cmd>` | 指令详情 |
| `.status` | 机器人状态 |
| `.ping` | 延迟检测 |
| `.uptime` | 运行时长和内存 |
| `.info` | 对话和用户 ID |
| `.apt list` | 列出插件 |
| `.remind 5m 喝水` | 设置提醒 |
| `.cron add daily 0 0 9 * * * .status` | 定时任务 |
| `.note set todo "买牛奶"` | 个人笔记 |
| `.roll 20` | 掷骰 |
| `.coin` | 抛硬币 |
| `.choose 披萨 汉堡 寿司` | 随机选择 |
| `.8ball 今天会下雨吗？` | 魔法 8 球 |
| `.fact` | 随机冷知识 |

## 架构

```
cmd/papervalet/main.go
internal/
  app/
  command/
  config/
  core/
  cron/
  eventbus/
  media/
  peer/
  plugin/
  session/
plugins/builtin/
pkg/logger/
```

| 路径 | 职责 |
|------|------|
| `cmd/papervalet/main.go` | 入口 |
| `internal/app/` | 应用编排 + 鉴权 + 更新处理 |
| `internal/command/` | 解析器、注册表、中间件 |
| `internal/config/` | JSON 配置与默认值 |
| `internal/core/` | 核心类型定义 |
| `internal/cron/` | 定时任务（基于 robfig/cron） |
| `internal/eventbus/` | 带优先级的事件总线 |
| `internal/media/` | 文件下载与上传 |
| `internal/peer/` | AccessHash 管理器与解析器 |
| `internal/plugin/` | 插件管理器与接口定义 |
| `internal/session/` | SQLite 会话存储 |
| `plugins/builtin/` | 内置插件（编译进主程序） |
| `pkg/logger/` | Zap 日志封装 |

### 关键设计决策

| 关注点 | 方案 |
|--------|------|
| 指令 | 强类型 `CommandContext`，内置 `Reply`/`Edit`/`Delete` 等便捷方法 |
| 插件 | 精简接口：`Init/Start/Stop` + `RegisterCommand` |
| 事件 | 带优先级、过滤器和异步发送的 `EventBus` |
| 账号解析 | 缓存优先 `AccessHashManager` → API 查询 → ID 模式回退 |
| 会话 | SQLite（WAL 模式）+ 内存 LRU 缓存，自动 TTL 清理 |

## 开发

### 添加依赖

```bash
go get github.com/some/pkg
```

### 编译

```bash
go build -o papervalet ./cmd/papervalet
```

### 运行测试

```bash
go test ./...
```

### 代码检查

```bash
go vet ./...
golangci-lint run
```

### 添加插件

创建 `plugins/myplugin/myplugin.go`：

```go
package myplugin

import (
    "context"
    "github.com/TiaraBasori/PaperValet/internal/command"
    "github.com/TiaraBasori/PaperValet/internal/core"
    "github.com/TiaraBasori/PaperValet/internal/plugin"
)

type MyPlugin struct{}

func New() *MyPlugin { return &MyPlugin{} }
func (p *MyPlugin) Name() string        { return "myplugin" }
func (p *MyPlugin) Description() string { return "我的插件" }
func (p *MyPlugin) Init(ctx context.Context, mgr *plugin.Manager) error {
    return mgr.RegisterCommand(&command.Command{
        Name: "hello", Description: "打个招呼",
        Plugin: p.Name(), Handler: p.hello,
    })
}
func (p *MyPlugin) Start(ctx context.Context) error { return nil }
func (p *MyPlugin) Stop(ctx context.Context) error  { return nil }

func (p *MyPlugin) hello(ctx *core.CommandContext) error {
    return ctx.Reply("Hello from my plugin!")
}
```

在 `internal/app/app.go` 中注册：

```go
import _ "github.com/TiaraBasori/PaperValet/plugins/myplugin"
```

## 外部插件

项目支持通过 `plugins` 目录动态加载外部 `.so` 插件。详见 [插件 SDK 文档](docs/plugin-sdk.md) / [中文版](docs/plugin-sdk_zh.md)。

`plugins-external/` 目录提供了多个 TeleBox 风格的外部插件示例（包括 `ping`、`help`、`tpm`、`alias`、`sudo` 等）。

## 许可证

MIT — 详见 [LICENSE](LICENSE)。
