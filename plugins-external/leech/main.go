package main

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type LeechPlugin struct{}

func New() (plugin.Plugin, error) {
	return &LeechPlugin{}, nil
}

var Metadata = &plugin.PluginMetadata{
	Name:        "leech",
	Description: "媒体下载工具",
	Version:     "1.0.0",
	Author:      "PaperValet",
	MinVersion:  "0.1.0",
}

func (p *LeechPlugin) Name() string        { return "leech" }
func (p *LeechPlugin) Description() string { return "媒体下载工具" }

func (p *LeechPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	cmds := []*plugin.Command{
		{
			Name:        "leech",
			Aliases:     []string{"dl", "download"},
			Description: "下载媒体文件",
			Usage:       "leech <URL> [选项]",
			Plugin:      p.Name(),
			Category:    "tools",
			Handler:     p.handleLeech,
		},
		{
			Name:        "yt",
			Description: "显示下载进度",
			Plugin:      p.Name(),
			Category:    "tools",
			Handler:     p.handleProgress,
		},
	}

	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *LeechPlugin) handleLeech(ctx *plugin.CommandContext) error {
	args := ctx.Args
	if len(args) == 0 {
		return ctx.Edit(`📥 <b>Leech 下载工具</b>

<b>用法:</b>
• <code>.leech <URL></code> - 下载媒体
• <code>.leech <URL> -f mp4</code> - 指定格式
• <code>.leech list</code> - 显示下载队列

<b>支持的站点:</b>
• YouTube, Bilibili, Twitter/X
• Instagram, TikTok
• 通用视频/音频链接`)
	}

	target := args[0]

	if target == "list" || target == "queue" {
		return ctx.Edit("📋 下载队列为空")
	}

	// Validate URL
	if _, err := url.ParseRequestURI(target); err != nil {
		return ctx.Edit("❌ 无效的URL")
	}

	// This would integrate with yt-dlp or similar
	return ctx.Edit(fmt.Sprintf(`📥 <b>开始下载</b>

🔗 <b>链接:</b> <code>%s</code>
📊 <b>状态:</b> 排队中...

💡 <i>需要集成 yt-dlp 或类似工具</i>
⏰ <i>%s</i>`, target, time.Now().Format("15:04:05")))
}

func (p *LeechPlugin) handleProgress(ctx *plugin.CommandContext) error {
	return ctx.Edit("📊 暂无活跃下载")
}

func (p *LeechPlugin) Start(ctx context.Context) error { return nil }
func (p *LeechPlugin) Stop(ctx context.Context) error  { return nil }