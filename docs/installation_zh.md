# 安装指南

[English](installation.md) | **中文**

## 准备工作

- Linux（amd64/arm64）或 macOS
- `curl` 和 `tar`
- 一个 Telegram 账号
- API 凭据（`api_id` / `api_hash`），在 [my.telegram.org/apps](https://my.telegram.org/apps) 申请

## 安装

```bash
curl -fsSL https://raw.githubusercontent.com/PaperValet/PaperValet/master/scripts/install.sh | bash
```

选 `1) 安装`，再起个命令名（回车就是 `papervalet`）。安装器只做两件事：

- 把程序放到 `~/.papervalet/bin/papervalet`
- 把命令注册到 `/usr/local/bin`（root）或 `~/.local/bin`（普通用户，会把它加进 `~/.bashrc` 的 `PATH`）

每个命令名就是一个独立实例，数据目录各管各的，多个账号可以并存。`papervalet` 用 `~/.papervalet`，其他名字 `<name>` 用 `~/.<name>`。

## 初始化

```bash
papervalet initialize
```

装完安装器会问要不要马上开始。一共五步，第一步选的语言会贯穿后面所有提示：

1. **语言**：简体中文或 English，同时作为机器人的默认语言
2. **Telegram API**：`api_id` 和 `api_hash`
3. **手机号**：带国家码，如 `+8613800138000`
4. **登录**：输入 Telegram 发来的验证码，开了两步验证再输密码
5. **后台常驻**：可选注册 systemd 服务（回车用命令名作服务名），开机自启

再跑一次 `initialize` 会把旧值当默认值，也能换账号。

随便找个对话（收藏夹就行）给自己发 `.ping`，机器人回 `🏓 Pong!` 就通了。

> 机器人只响应**你自己发出的消息**。

## 运行

没注册服务时：

```bash
papervalet run
```

注册了服务（以 root 为例，用户服务加 `--user`）：

```bash
systemctl status papervalet
journalctl -u papervalet -f
```

```
~/.papervalet
├── bin/papervalet
├── config.json      # initialize 写入（权限 0600）
├── session.json     # Telegram 登录会话
├── sessions.db
├── plugins/
└── data/
```

## 升级 / 卸载

重新运行安装器，选 `2) 升级` 或 `3) 卸载`。

- 升级只换程序，数据全保留，有服务会顺手重启。
- 卸载会移除命令和服务，再问你要不要连数据目录一起删。

无交互用法：`bash install.sh --upgrade --name papervalet -y`。

## Docker

```bash
docker run -d --name papervalet \
  -v "$PWD/config:/app/config" \
  -v "$PWD/data:/app/data" \
  --restart unless-stopped \
  ghcr.io/papervalet/papervalet:latest
```

容器读取 `/app/config/config.json`。首次登录用环境变量传入凭据：

```bash
docker run -it --rm \
  -e PAPERVALET_PHONE=+8613800138000 \
  -e PAPERVALET_CODE=12345 \
  -e PAPERVALET_2FA_PASSWORD=你的密码 \
  -v "$PWD/config:/app/config" \
  -v "$PWD/data:/app/data" \
  ghcr.io/papervalet/papervalet:latest
```

验证码只能用一次，给容器登录要现取一个新的。

## 从源码构建

```bash
git clone https://github.com/PaperValet/PaperValet
cd PaperValet
go build -o papervalet ./cmd/papervalet
./papervalet initialize
./papervalet run
```

需要 Go 1.25+。`-config path/to/config.json` 依旧可用，此时工作目录保持不变。

## 常见问题

**`papervalet: command not found`**：新开一个终端，或执行 `source ~/.bashrc`。

**提示还没登录**：重新跑 `papervalet initialize`。

**机器人不响应**：指令必须**从你自己的账号**发出，且以前缀开头（默认 `.`）。

**arm64 机器**：release 里预编译的 `.so` 插件只有 amd64。可以从 [PaperValet-Plugins](https://github.com/PaperValet/PaperValet-Plugins) 用 `go build -buildmode=plugin` 自行编译。
