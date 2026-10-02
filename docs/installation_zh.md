# 安装指南

[English](installation.md) · **中文**

需要一台 Linux 或 macOS 机器、`curl`、在 [my.telegram.org/apps](https://my.telegram.org/apps) 申请的 API 凭据，以及从 [@BotFather](https://t.me/BotFather) 用 `/newbot` 拿到的 bot token。

## 安装

```bash
curl -fsSL https://raw.githubusercontent.com/PaperValet/PaperValet/master/scripts/install.sh | bash
```

选 **安装**，再起一个命令名（直接回车就是 `papervalet`）。安装脚本把程序放进 `~/.<命令名>/bin`，并把命令链接到 `/usr/local/bin`（root）或 `~/.local/bin`。

每个命令名是一个独立实例，有自己的数据目录，多个账号可以同时跑。

## 初始化

```bash
papervalet initialize
```

共六步：选语言、填 `api_id` / `api_hash`、输手机号、输验证码（开了两步验证再输密码）、填 bot token、可选注册开机自启的 systemd 服务。再次运行会把上次填的内容当默认值。

然后在任意聊天发 `.ping`。人形机器人只响应你自己发出的消息。

设置都在配套机器人里。首次启动时你的账号会自动给它发一次消息，这样它才能联系到你。给它发 `/menu` 打开按钮面板，语言、前缀、sudo、日志级别和各插件选项都在里面。除了你它谁都不理。

## 运行

```bash
papervalet run                 # 前台运行
systemctl status papervalet    # 注册了服务时
journalctl -u papervalet -f
```

非 root 用户注册的是用户级服务，`systemctl` 和 `journalctl` 要加 `--user`。

数据目录结构：

```
~/.papervalet
├── bin/papervalet
├── config.json      # 权限 0600，含 api_hash 和 bot_token
├── session.json     # Telegram 登录信息
├── bot_session.json # 配套机器人登录信息
├── sessions.db
├── plugins/         # 外部插件 .so
└── data/            # 运行时数据，data/<插件>/settings.json
```

## 升级与卸载

在 Telegram 里发 `.update now`，会下载最新版并重启。`.update -f` 不管版本号，强制重装最新版。

在终端里再跑一次安装脚本，选 **升级** 或 **卸载**。升级保留全部数据，卸载会先问要不要删数据目录。

```bash
bash install.sh --upgrade --name papervalet -y
```

## Docker

```bash
docker run -d --name papervalet --restart unless-stopped \
  -v "$PWD/config:/app/config" \
  -v "$PWD/data:/app/data" \
  -e PAPERVALET_PHONE=+8613800138000 \
  -e PAPERVALET_CODE=12345 \
  -e PAPERVALET_2FA_PASSWORD=secret \
  ghcr.io/papervalet/papervalet:latest
```

把 `config.json` 放进 `./config`（参考 [`config.example.json`](../config.example.json)），并把 `session_file`、`bot_session_file` 和 `database_file` 指到 `data/` 下，重启后才不用重新登录。登录相关的环境变量只有第一次启动需要，验证码只能用一次。

镜像不带 cgo，加载不了外部插件。

## 从源码构建

```bash
git clone https://github.com/PaperValet/PaperValet && cd PaperValet
make build
./papervalet initialize
./papervalet run
```

## 配置

`config.json` 由 `initialize` 生成，可能需要手改的字段：

| 字段 | 默认 | 含义 |
|---|---|---|
| `telegram.bot_token` | | 配套机器人 token，必填 |
| `bot.command_prefix` | `.` | 主前缀，在机器人面板改过后以面板为准 |
| `bot.command_prefixes` | | 额外前缀，在机器人面板改过后以面板为准 |
| `bot.owner_id` | `0` | 主人账号，`0` 表示当前登录的账号 |
| `bot.plugin_repo` | PaperValet-Plugins 最新 Release | `apt` 下载插件的地址 |
| `logger.level` | `INFO` | 启动时的级别，机器人面板可覆盖 |
| `i18n.default_language` | `zh-CN` | 启动时的语言，机器人面板可覆盖 |

环境变量：

| 变量 | 含义 |
|---|---|
| `PAPERVALET_HOME` | 数据目录，默认 `~/.papervalet` |
| `PAPERVALET_CONFIG` | 配置文件路径，不切换工作目录 |
| `PAPERVALET_PHONE`、`PAPERVALET_CODE`、`PAPERVALET_2FA_PASSWORD` | 没有终端时登录用 |

## 常见问题

**提示找不到命令**：开个新终端，或者 `source ~/.bashrc`。

**提示没登录**：重新跑 `papervalet initialize`。

**发命令没反应**：命令必须由你的账号发出，并以前缀开头。

**机器人不说话**：用自己的账号给它发一次 `/start`。机器人登录出错只会记在 `bot` 日志里，不影响人形机器人。

**插件报 different version**：插件和主程序的 Go 版本不同。先 `.update -f`，再重装插件。

**插件完全加载不了**：只有 linux/amd64 的 Release 能加载外部插件。
