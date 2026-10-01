# PaperValet

[English](README.md) | **中文**

基于 [gotd/td](https://github.com/gotd/td) 的生产级 Telegram 用户机器人 — 纯 Go MTProto，无 CGO。

架构清晰模块化，15 个内置插件，外加可热加载的外部插件生态。

## 快速开始

```bash
curl -fsSL https://raw.githubusercontent.com/PaperValet/PaperValet/master/scripts/install.sh | bash
```

安装器只负责放好程序并注册 `papervalet` 命令，接着：

```bash
papervalet initialize   # 选语言、填 api_id/api_hash、手机号、验证码，可选注册 systemd 服务
papervalet run          # 前台启动（已注册服务可跳过）
```

`api_id` / `api_hash` 在 [my.telegram.org](https://my.telegram.org/apps) 申请。

随便找个对话给自己发 `.ping`，机器人回 `🏓 Pong!` 就跑通了。

完整教程（含 systemd 和 Docker）：**[安装指南](docs/installation_zh.md)**。

## 内置插件

| 插件 | 指令 | 说明 |
|------|------|------|
| `core` | `.ping`、`.restart` | 延迟检测、原地重启 |
| `help` | `.help` | 按分类显示帮助 |
| `status` | `.status`（别名 `.version`） | 版本、运行时长、内存、插件数 |
| `apt` | `.apt list/install/remove/load/unload` | 插件管理器 |
| `info` | `.info`（别名 `.ids`） | ID 与跳转链接，回复可看对方 |
| `re` | `.re [条数] [次数]` | 复读回复的消息 |
| `alias` | `.alias set/del/list` | 运行时命令别名 |
| `exec` | `.exec` | 执行系统命令 |
| `sudo` | `.sudo on/off/add/remove/list` | 权限委派 |
| `reload` | `.reload` | 重新加载全部外部插件 |
| `log` | `.loglevel`、`.sendlog` | 日志级别与发送 |
| `prefix` | `.prefix list/add/del/set` | 前缀管理 |
| `backup` | `.backup`、`.backup restore` | 配置打包发收藏夹，回复恢复 |
| `update` | `.update`、`.update now` | 从 GitHub Release 升级 |
| `dme` | `.dme N`、`.dme all`、`.dme others on/off` | 批量删除，带防撤回 |
| `lang` | `.lang` | 语言切换 |

## 外部插件

第三方插件独立维护在 [PaperValet-Plugins](https://github.com/PaperValet/PaperValet-Plugins)（现有 21 个），一条命令安装：

```
.apt install weather
```

自己写插件：[插件 SDK 文档](docs/plugin-sdk_zh.md)。

## 配置说明

`~/.papervalet/config.json`（由 `papervalet initialize` 生成，模板见 `config.example.json`）：

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

## 环境变量

| 环境变量 | 用途 |
|----------|------|
| `PAPERVALET_HOME` | 数据目录（默认 `~/.papervalet`），每个注册的命令各有一份 |
| `PAPERVALET_CONFIG` | 显式指定配置路径，不切换工作目录（Docker 用） |
| `PAPERVALET_PHONE` | 无终端登录：E.164 手机号，如 `+8613800138000` |
| `PAPERVALET_CODE` | 无终端登录：Telegram 发的一次性验证码 |
| `PAPERVALET_2FA_PASSWORD` | 无终端登录：两步验证密码 |

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
