<div align="center">

# PaperValet

纯 Go 编写的 Telegram 人形机器人，基于 [gotd/td](https://github.com/gotd/td)。

[English](README.md) · **中文**

</div>

## 安装

```bash
curl -fsSL https://raw.githubusercontent.com/PaperValet/PaperValet/master/scripts/install.sh | bash
papervalet initialize
```

`initialize` 会带你选语言、填 API、登录、接入配套机器人，并可选注册 systemd 服务。`api_id` / `api_hash` 在 [my.telegram.org](https://my.telegram.org/apps) 申请，bot token 找 [@BotFather](https://t.me/BotFather) 要。装好后在任意聊天发 `.ping`。

Docker、升级和常见问题见 **[安装指南](docs/installation_zh.md)**。

## 命令

只响应你自己发出的消息。`.help` 列出全部命令，`.help 命令` 看详细说明。

设置不走命令。语言、前缀、sudo、日志级别和各插件的选项都在配套机器人里，给它发 `/menu` 点按钮就行。里面有三个面板：系统设置、外置插件设置，还有插件管理，用来浏览仓库、安装、卸载、重载。

| 插件 | 命令 | 作用 |
|---|---|---|
| ping | `.ping [主机]`、`.pingdc` | 测 Telegram、指定主机或各数据中心的延迟 |
| status | `.status` | 版本、主机、资源、运行状态 |
| info | `.info` | 用户、聊天和消息信息 |
| re | `.re [条数] [次数]` | 复读回复的消息 |
| dme | `.dme <数量\|all> [-f]` | 删除自己最近的消息 |
| exec | `.exec <命令>` | 执行 shell 命令 |
| apt | `.apt s / i / rm / ls / info` | 安装和卸载外部插件 |
| reload | `.reload` | 重载外部插件 |
| restart | `.restart` | 原地重启 |
| update | `.update [now\|-f]` | 从 GitHub Release 升级 |
| backup | `.backup [restore]` | 把配置备份到收藏夹 |
| sudo | `.sudo add / remove` | 授权其他人使用命令 |
| alias | `.alias 名字=命令` | 自定义快捷命令 |
| log | `.sendlog [tail\|clean]` | 日志文件 |

## 插件

外部插件来自 [PaperValet-Plugins](https://github.com/PaperValet/PaperValet-Plugins)：

```
.apt s           浏览仓库
.apt i weather   安装一个
.apt i -all      全部安装
```

自己写插件看 [插件 SDK](docs/plugin-sdk_zh.md)。

## 开发

```bash
make build     # 生成 ./papervalet
make test
make lint
```

需要 Go 1.25。外部插件必须和主程序用同一版本 Go、同样带 `-trimpath` 编译。

## 许可

[MIT](LICENSE)
