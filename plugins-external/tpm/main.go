package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type TPMPlugin struct{}

func New() (plugin.Plugin, error) {
	return &TPMPlugin{}, nil
}

var Metadata = &plugin.PluginMetadata{
	Name:        "tpm",
	Description: "Telegram插件管理器",
	Version:     "1.0.0",
	Author:      "PaperValet",
	MinVersion:  "0.1.0",
}

func (p *TPMPlugin) Name() string        { return "tpm" }
func (p *TPMPlugin) Description() string { return "Telegram插件管理器" }

func (p *TPMPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	cmds := []*plugin.Command{
		{
			Name:        "tpm",
			Aliases:     []string{"plugin", "plugins"},
			Description: "插件管理器",
			Usage:       "tpm [list|install|remove|update|enable|disable|search] [插件名]",
			Plugin:      p.Name(),
			Category:    "admin",
			OwnerOnly:   true,
			Handler:     p.handleTPM,
		},
	}

	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *TPMPlugin) handleTPM(ctx *plugin.CommandContext) error {
	args := ctx.Args
	if len(args) == 0 {
		return p.showHelp(ctx)
	}

	sub := args[0]

	switch sub {
	case "list", "ls":
		return p.listPlugins(ctx)
	case "install", "add":
		if len(args) < 2 {
			return ctx.Edit("用法: tpm install <插件名|GitHub URL>")
		}
		return ctx.Edit(fmt.Sprintf("📦 安装插件: %s\n\n⚠️ 需要实现插件仓库下载逻辑", args[1]))
	case "remove", "rm", "uninstall":
		if len(args) < 2 {
			return ctx.Edit("用法: tpm remove <插件名>")
		}
		return ctx.Edit(fmt.Sprintf("🗑 卸载插件: %s\n\n⚠️ 需要实现插件卸载逻辑", args[1]))
	case "update", "up":
		if len(args) < 2 {
			return ctx.Edit("更新所有插件\n\n⚠️ 需要实现插件更新逻辑")
		}
		return ctx.Edit(fmt.Sprintf("🔄 更新插件: %s\n\n⚠️ 需要实现插件更新逻辑", args[1]))
	case "enable":
		if len(args) < 2 {
			return ctx.Edit("用法: tpm enable <插件名>")
		}
		return ctx.Edit(fmt.Sprintf("✅ 启用插件: %s\n\n⚠️ 需要实现插件启用逻辑", args[1]))
	case "disable":
		if len(args) < 2 {
			return ctx.Edit("用法: tpm disable <插件名>")
		}
		return ctx.Edit(fmt.Sprintf("⏸ 禁用插件: %s\n\n⚠️ 需要实现插件禁用逻辑", args[1]))
	case "search", "find":
		if len(args) < 2 {
			return ctx.Edit("用法: tpm search <关键词>")
		}
		return ctx.Edit(fmt.Sprintf("🔍 搜索插件: %s\n\n⚠️ 需要实现插件搜索逻辑", args[1]))
	case "info":
		if len(args) < 2 {
			return ctx.Edit("用法: tpm info <插件名>")
		}
		return ctx.Edit(fmt.Sprintf("ℹ️ 插件信息: %s\n\n⚠️ 需要实现插件信息查询", args[1]))
	default:
		return p.showHelp(ctx)
	}
}

func (p *TPMPlugin) listPlugins(ctx *plugin.CommandContext) error {
	infos := ctx.Manager().GetAllInfo()
	
	var b strings.Builder
	b.WriteString(fmt.Sprintf("📦 <b>已加载插件 (%d)</b>\n\n", len(infos)))

	for _, info := range infos {
		status := "⏸️"
		if info.Status == plugin.StatusActive {
			status = "✅"
		}
		b.WriteString(fmt.Sprintf("%s <b>%s</b> v%s — %s\n", status, info.Name, info.Version, info.Description))
		if info.Author != "" {
			b.WriteString(fmt.Sprintf("   作者: %s\n", info.Author))
		}
		b.WriteString("\n")
	}

	b.WriteString("💡 使用 <code>.tpm [install|remove|enable|disable]</code> 管理插件")
	return ctx.Edit(b.String())
}

func (p *TPMPlugin) showHelp(ctx *plugin.CommandContext) error {
	prefix := ctx.Prefix()
	return ctx.Edit(fmt.Sprintf(`📦 <b>TPM - Telegram插件管理器</b>

<b>用法:</b>
• <code>%stpm list</code> - 列出所有插件
• <code>%stpm install <插件></code> - 安装插件
• <code>%stpm remove <插件></code> - 卸载插件
• <code>%stpm update [插件]</code> - 更新插件
• <code>%stpm enable <插件></code> - 启用插件
• <code>%stpm disable <插件></code> - 禁用插件
• <code>%stpm search <关键词></code> - 搜索插件
• <code>%stpm info <插件></code> - 插件详情

<b>示例:</b>
• <code>%stpm install github.com/user/plugin</code>
• <code>%stpm enable ping</code>
• <code>%stpm update</code>

💡 <i>插件以 .so 文件形式存放在 plugins/ 目录</i>`, prefix, prefix, prefix, prefix, prefix, prefix, prefix, prefix, prefix))
}

func (p *TPMPlugin) Start(ctx context.Context) error { return nil }
func (p *TPMPlugin) Stop(ctx context.Context) error  { return nil }