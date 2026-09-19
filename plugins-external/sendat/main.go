package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type SendAtPlugin struct{}

func New() *SendAtPlugin { return &SendAtPlugin{} }

func (p *SendAtPlugin) Name() string        { return "sendat" }
func (p *SendAtPlugin) Description() string { return "定时发送消息" }

func (p *SendAtPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&plugin.Command{
		Name:        "sendat",
		Aliases:     []string{"定时", "schedule"},
		Description: "在指定时间发送消息（进程重启后失效）",
		Usage:       "sendat <+5m|18:00|2006-01-02 15:04> <消息>",
		Plugin:      p.Name(),
		Category:    "tools",
		OwnerOnly:   true,
		Handler:     p.handleSendAt,
	})
}

func (p *SendAtPlugin) Start(ctx context.Context) error { return nil }
func (p *SendAtPlugin) Stop(ctx context.Context) error  { return nil }

func (p *SendAtPlugin) handleSendAt(ctx *plugin.CommandContext) error {
	if len(ctx.Args) < 2 {
		return ctx.Edit(`⏰ 用法：<code>sendat <时间> <消息></code>

时间格式：
• <code>+5m</code> — 5 分钟后（s/m/h）
• <code>18:00</code> — 今天 18:00（已过则为明天）
• <code>2026-01-01 12:00</code> — 指定日期时间`)
	}

	var when time.Time
	var delay time.Duration
	var err error

	arg := ctx.Args[0]
	msgParts := ctx.Args[1:]

	if strings.HasPrefix(arg, "+") {
		delay, err = time.ParseDuration(strings.TrimPrefix(arg, "+"))
		if err != nil {
			return ctx.Edit(fmt.Sprintf("❌ 无法解析时长 %q", arg))
		}
		when = time.Now().Add(delay)
	} else if strings.Count(arg, ":") == 1 && !strings.Contains(arg, "-") {
		t, perr := time.ParseInLocation("15:04", arg, time.Local)
		if perr != nil {
			return ctx.Edit(fmt.Sprintf("❌ 无法解析时间 %q", arg))
		}
		now := time.Now()
		when = time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, time.Local)
		if !when.After(now) {
			when = when.Add(24 * time.Hour)
		}
		delay = time.Until(when)
	} else {
		full := arg
		if len(ctx.Args) >= 3 && strings.Count(ctx.Args[1], ":") >= 1 {
			full = arg + " " + ctx.Args[1]
			msgParts = ctx.Args[2:]
		}
		t, perr := time.ParseInLocation("2006-01-02 15:04", full, time.Local)
		if perr != nil {
			return ctx.Edit(fmt.Sprintf("❌ 无法解析时间 %q", full))
		}
		when = t
		delay = time.Until(when)
	}

	msg := strings.Join(msgParts, " ")
	if msg == "" {
		return ctx.Edit("❌ 消息内容不能为空")
	}
	if delay <= 0 {
		return ctx.Edit("❌ 时间已过")
	}
	if delay > 7*24*time.Hour {
		return ctx.Edit("❌ 最多支持 7 天内的定时")
	}

	peer, err := ctx.ResolvePeer()
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 会话解析失败: %v", err))
	}
	api := ctx.API

	time.AfterFunc(delay, func() {
		sendCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, _ = api.MessagesSendMessage(sendCtx, &tg.MessagesSendMessageRequest{
			Peer:     peer,
			Message:  msg,
			RandomID: time.Now().UnixNano(),
		})
	})

	return ctx.Edit(fmt.Sprintf("⏰ 已设置定时消息\n\n时间：<code>%s</code>\n内容：<code>%s</code>\n\n注意：进程重启后失效",
		when.Format("2006-01-02 15:04:05"), msg))
}
