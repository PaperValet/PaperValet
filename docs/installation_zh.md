# 安装指南

[English](installation.md) | **中文**

## 准备工作

- Linux（amd64/arm64）或 macOS
- `curl` 和 `tar`（安装器会自动检查）
- 一个 Telegram 账号
- API 凭据（`api_id` / `api_hash`），在 [my.telegram.org](https://my.telegram.org) → *API development tools* 申请

## 一键安装

```bash
curl -fsSL https://raw.githubusercontent.com/PaperValet/PaperValet/master/scripts/install.sh | bash
```

会出现交互菜单，选 `1) install`，然后依次回答三个问题：

| 提示 | 填什么 |
|------|--------|
| `api_id` | my.telegram.org 上的数字 ID |
| `api_hash` | my.telegram.org 上的哈希串 |
| `phone` | 手机号，E.164 格式，如 `+8613800138000` |

所有文件装到 `~/.papervalet`：

```
~/.papervalet
├── bin/papervalet      # 主程序
├── plugins/            # 外部 .so 插件安装位置
├── config.json         # 根据你的回答自动生成
└── run.sh              # 启动脚本
```

## 首次登录

启动：

```bash
~/.papervalet/run.sh
```

手机号安装时已经存好，只剩两步：

1. **Login code** — Telegram 会发验证码到你的 App，输入后回车。验证码几分钟就过期，收到后尽快输。
2. **2FA password** — 只有两步验证开启时才会问。

看到日志里出现 `authenticated` 和插件加载信息就是成功了。随便找个对话（收藏夹最方便）给自己发 `.ping`，机器人回 `🏓 Pong!` 就说明跑通了。

> 机器人只响应**你自己发出的消息**，别人的消息一律忽略。

## 注册为系统服务（systemd）

```bash
sudo tee /etc/systemd/system/papervalet.service >/dev/null <<EOF
[Unit]
Description=PaperValet userbot
After=network-online.target

[Service]
User=%i
WorkingDirectory=$HOME/.papervalet
EnvironmentFile=$HOME/.papervalet.env
ExecStart=$HOME/.papervalet/bin/papervalet -config $HOME/.papervalet/config.json
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable --now papervalet
journalctl -u papervalet -f        # 跟踪日志
```

登录成功后凭据保存在 `session.json`，之后重启服务不会再要验证码。

## Docker

```bash
docker run -d --name papervalet \
  -v "$PWD/config:/app/config" \
  -v "$PWD/data:/app/data" \
  --restart unless-stopped \
  ghcr.io/papervalet/papervalet:latest
```

容器内首次登录用环境变量传入凭据：

```bash
docker run -it --rm \
  -e PAPERVALET_PHONE=+8613800138000 \
  -e PAPERVALET_CODE=12345 \
  -e PAPERVALET_2FA_PASSWORD=你的密码 \
  -v "$PWD/config:/app/config" \
  -v "$PWD/data:/app/data" \
  ghcr.io/papervalet/papervalet:latest
```

验证码是一次性的，给容器登录时重新要一个。

## 手动安装

1. 到 [Releases](https://github.com/PaperValet/PaperValet/releases/latest) 下载对应平台的包，如 `papervalet-linux-amd64.tar.gz`。
2. 解压：`tar -xzf papervalet-linux-amd64.tar.gz && cd papervalet-linux-amd64`
3. 复制 `config.example.json` 为 `config.json`，填入 `api_id` / `api_hash`。
4. 运行 `./run.sh`。

## 从源码构建

```bash
git clone https://github.com/PaperValet/PaperValet
cd PaperValet
go build -o papervalet ./cmd/papervalet
cp config.example.json config.json   # 填入 api_id / api_hash
./papervalet -config config.json
```

需要 Go 1.25+。

## 升级 / 卸载

重新运行安装脚本，选对应菜单项：

```bash
curl -fsSL https://raw.githubusercontent.com/PaperValet/PaperValet/master/scripts/install.sh | bash
```

- `3) upgrade` — 只换二进制，保留 `config.json`、`session.json` 和 `sessions.db`
- `4) uninstall` — 卸载，可选保留配置和会话数据

## 常见问题

**`PHONE_CODE_EXPIRED`** — 验证码过期了。按 `Ctrl+C` 重启，重新走一遍，收到新验证码后尽快输入。

**机器人没反应** — 命令必须用**你自己的账号**发送，且以默认前缀 `.` 开头。排查问题可以先用 `.loglevel debug` 调高日志级别。

**arm64 主机** — release 包里的预编译 `.so` 插件只有 amd64 版。主程序在 arm64 上原生运行；外部插件可以在 [PaperValet-Plugins](https://github.com/PaperValet/PaperValet-Plugins) 仓库里用 `go build -buildmode=plugin` 自行编译。

**文件都在哪** — 全部在 `~/.papervalet` 下。`session.json` 是 Telegram 登录凭据，备份它以后就不用重新登录。
