# PaperValet

[English](README.md) | **中文**

基于 [gotd/td](https://github.com/gotd/td) 的生产级 Telegram 用户机器人 — 纯 Go MTProto，无 CGO。

架构清晰模块化，15 个内置插件，外加可热加载的外部插件生态。

## 快速开始

```bash
curl -fsSL https://raw.githubusercontent.com/PaperValet/PaperValet/master/scripts/install.sh | bash
```

安装器会依次询问 `api_id`、`api_hash`（在 [my.telegram.org](https://my.telegram.org) 申请）和手机号，装到 `~/.papervalet`，然后：

```bash
~/.papervalet/run.sh    # 输入 Telegram 验证码即可
```

随便找个对话给自己发 `.ping`，机器人回 `🏓 Pong!` 就跑通了。

完整教程（含 systemd 和 Docker）：**[安装指南](docs/installation_zh.md)**。

## 内置插件

| 插件 | 指令 | 说明 |
|------|------|------|
| `core` | `.version`、`.ping`、`.restart` | 核心管理 |
| `help` | `.help` | 按分类显示帮助 |
| `status` | `.status` | 运行状态 |
| `apt` | `.apt list/install/remove/load/unload` | 插件管理器 |
| `info` | `.info`、`.fwd` | 信息查询与转发 |
| `alias` | `.alias set/del/list` | 运行时命令别名 |
| `exec` | `.exec` | 执行系统命令 |
| `sudo` | `.sudo on/off/add/remove/list` | 权限委派 |
| `reload` | `.reload` | 外部插件热重载 |
| `log` | `.loglevel`、`.sendlog` | 日志级别与发送 |
| `prefix` | `.prefix list/add/del/set` | 前缀管理 |
| `backup` | `.backup` | 备份与恢复 |
| `update` | `.update`、`.autofix` | 代码同步与重启 |
| `dme` | `.dme`、`.dme all` | 消息清理 |
| `lang` | `.lang` | 语言切换 |

## 外部插件

第三方插件独立维护在 [PaperValet-Plugins](https://github.com/PaperValet/PaperValet-Plugins)（现有 21 个），一条命令安装：

```
.apt install weather
```

自己写插件：[插件 SDK 文档](docs/plugin-sdk_zh.md)。

## 配置说明

`config.json`（安装器自动生成，模板见 `config.example.json`）：

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

- `command_prefix` — 指令前缀（默认 `.`）
- `owner_id` — 所有者用户 ID，仅所有者指令需要（`0` 则自动以首个登录用户为准）
- `logger.level` — DEBUG、INFO、WARN、ERROR

## 无终端 / 容器登录

| 环境变量 | 用途 |
|----------|------|
| `PAPERVALET_PHONE` | E.164 手机号，如 `+8613800138000` |
| `PAPERVALET_CODE` | Telegram 发的一次性验证码 |
| `PAPERVALET_2FA_PASSWORD` | 两步验证密码 |
| `PAPERVALET_NONINTERACTIVE` | 设为 `1`/`true` 时缺值直接报错，不再等 stdin |

## 架构

```
cmd/papervalet/       入口
internal/
  app/                编排 + 鉴权 + 更新处理
  command/            解析器、注册表、中间件
  config/             JSON 配置与默认值
  eventbus/           优先级发布订阅
  media/              上传下载
  peer/               Access Hash 缓存与解析
  plugin/             管理器 + .so 加载器
  session/            SQLite 会话存储
  i18n/               zh-CN / en-US 语言目录
plugins/builtin/      编译期内置插件
pkg/plugin/           外部插件公共 SDK
pkg/logger/           zap 封装
```

| 设计点 | 方案 |
|--------|------|
| 命令 | 强类型 `CommandContext`，带 `Reply`/`Edit`/`Delete` 助手 |
| 插件 | 最小接口 `Init/Start/Stop` + `RegisterCommand` |
| 事件 | `EventBus` 支持优先级、过滤、异步分发 |
| Peer | 缓存优先 `AccessHashManager` → API → ID 规则回退 |
| 会话 | SQLite（WAL）+ 内存缓存，TTL 清理 |

## 开发

```bash
go build -o papervalet ./cmd/papervalet   # 构建
go test ./...                             # 测试
go vet ./... && golangci-lint run         # 静态检查
```

需要 Go 1.25+。

## 许可证

MIT — 见 [LICENSE](LICENSE)。
