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

### 一键安装（推荐）

```bash
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/PaperValet/PaperValet/main/scripts/install.sh | bash

# Windows (PowerShell)
iwr -useb https://raw.githubusercontent.com/PaperValet/PaperValet/main/scripts/install.ps1 | iex
```

默认进入**交互菜单**，可选项：

| 序号 | 动作 | 用途 |
|------|------|------|
| 1 | `install` | 首次安装到 `~/.papervalet` |
| 2 | `reinstall` | 覆盖重装；默认保留 `config.json` |
| 3 | `upgrade` | 升级到指定版本；保留 `config.json` / `session.json` / `sessions.db` |
| 4 | `uninstall` | 卸载（可保留 config / data） |
| 5 | `status` | 查看安装路径、版本、二进制 SHA256、`.so` 数量 |
| 6 | `latest` | 列出 GitHub 最新 release 与其 bundle |
| 7 | `set-phone` | 持久化 `PAPERVALET_PHONE`（以及可选的 `PAPERVALET_CODE` / `PAPERVALET_2FA_PASSWORD`）到 `~/.papervalet.env` |
| 8 | `run` | 前台启动 bot |
| 9 | `doctor` | 自检（curl / jq / 磁盘 / 架构 / GitHub 可达性） |
| 0 | `exit` | 退出 |
| v | — | 切换版本（latest / vX.Y.Z） |
| h | — | 切换安装目录 |

若传 `--non-interactive` 或任一 `--phone` / `--api-id` / `--api-hash`，脚本跳过菜单
直接 install，便于 CI 使用。

命令行参数：`--version <tag>`、`--home <dir>`、`--phone`、`--api-id`、`--api-hash`、
`--no-config`、`--keep-config`、`--keep-data`、`--repo owner/repo`。完整列表见
`--help`。

任何时候重跑同一行 `curl | bash` 都会再次进入菜单，可升级、卸载、改设置，不必
记命令行。

### 手动下载

到 [GitHub Releases](https://github.com/PaperValet/PaperValet/releases/latest) 拉取
与系统匹配的压缩包，解压后编辑 `config.json`，执行 `./run.sh`（Windows 为 `run.cmd`）。

### Docker

```bash
docker run -d --name papervalet \
  -v "$PWD/config:/app/config" \
  -v "$PWD/data:/app/data" \
  --restart unless-stopped \
  ghcr.io/papervalet/papervalet:latest
```

镜像已包含全部内建插件与预编译的 `.so` 外部插件。

### 从源码构建

适合想改代码或参与开发的用户：

```bash
git clone https://github.com/TiaraBasori/PaperValet
cd PaperValet
go build -o papervalet ./cmd/papervalet
cp config.example.json config.json
# 编辑 config.json，填入 https://my.telegram.org 申请的 api_id / api_hash
./papervalet -config config.json
```

首次运行会进入交互式登录（依次填写手机号、验证码、以及两步验证密码）。

### 无 TTY / 容器场景登录

通过下列环境变量跳过 stdin 提示，便于容器 / systemd / CI 中自动化登录：

| 环境变量 | 用途 |
|----------|------|
| `PAPERVALET_PHONE` | E.164 格式手机号，例如 `+8613800138000` |
| `PAPERVALET_CODE` | Telegram 一次性登录验证码 |
| `PAPERVALET_2FA_PASSWORD` | 云端两步验证密码 |
| `PAPERVALET_NONINTERACTIVE` | 设为 `1`/`true` 时，若上述任一变量缺失则直接报错；未设时缺失会回退到 stdin |

### arm64 注意事项

GitHub Actions 当前只产出 `amd64` 的 `.so`（Go 的 `buildmode=plugin` 在 arm64 上
未受官方支持）。Apple Silicon 或 arm64 Linux 用户拿到的 bundle：主二进制是原生
`arm64`，但 `plugins/*.so` 是 `amd64`。两种处理方式：

1. 借助 Rosetta / x86 兼容运行时加载 `amd64` 的 `.so`。
2. 自己本地编：`cd plugins-external/<name> && go build -buildmode=plugin -o ../<name>.so .`

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
