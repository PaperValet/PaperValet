package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type SendAtPlugin struct{}

func New() *SendAtPlugin { return &SendAtPlugin{} }

func (p *SendAtPlugin) Name() string        { return "sendat" }
func (p *SendAtPlugin) Description() string { return "定时消息发送" }

func (p *SendAtPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	cmds := []*plugin.Command{
		{
			Name:        "sendat",
			Aliases:     []string{"定时", "at"},
			Description: "定时发送消息",
			Usage:       "sendat <时间> <消息>",
			Plugin:      p.Name(),
			Category:    "tools",
			OwnerOnly:   false,
			Handler:     p.handleSendAt,
		},
	}
	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *SendAtPlugin) Start(ctx context.Context) error { return nil }
func (p *SendAtPlugin) Stop(ctx context.Context) error  { return nil }

func (p *SendAtPlugin) handleSendAt(ctx *plugin.CommandContext) error {
	if len(ctx.Args) < 2 {
		return ctx.Edit(`⏰ <b>定时发送</b>

用法: <code>sendat <时间> <消息></code>

时间格式:
• <code>10:30</code> — 今天 10:30
• <code>+5m</code> — 5 分钟后
• <code>+1h</code> — 1 小时后
• <code>2026-01-01 12:00</code> — 指定日期时间

示例:
• <code>sendat +10m 该喝水了</code>
• <code>sendat 18:00 下班啦</code>`)
	}

	when := ctx.Args[0]
	msg := strings.Join(ctx.Args[1:], " ")

	return ctx.Edit(fmt.Sprintf(`⏰ <b>定时消息已设置</b>

时间: <code>%s</code>
内容: <code>%s</code>

⚠️ 完整实现需集成 cron 调度器`, when, msg))
}