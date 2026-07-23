package main

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// ExamplePlugin demonstrates the external plugin SDK
type ExamplePlugin struct {
	mgr      plugin.Manager
	startTime time.Time
}

func (p *ExamplePlugin) Name() string        { return "example" }
func (p *ExamplePlugin) Description() string { return "Example external plugin showing SDK usage" }

func (p *ExamplePlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	p.mgr = mgr
	p.startTime = time.Now()

	cmds := []*plugin.Command{
		{
			Name:        "echo",
			Description: "Echo back text",
			Usage:       "echo <text>",
			Plugin:      p.Name(),
			Category:    "tools",
			Handler:     p.handleEcho,
		},
		{
			Name:        "roll",
			Aliases:     []string{"dice"},
			Description: "Roll dice (e.g., 2d6)",
			Usage:       "roll [NdM]",
			Plugin:      p.Name(),
			Category:    "fun",
			Handler:     p.handleRoll,
		},
		{
			Name:        "choose",
			Aliases:     []string{"pick"},
			Description: "Randomly choose from options",
			Usage:       "choose <opt1> <opt2> ...",
			Plugin:      p.Name(),
			Category:    "fun",
			Handler:     p.handleChoose,
		},
		{
			Name:        "uptime",
			Description: "Show plugin uptime",
			Plugin:      p.Name(),
			Category:    "tools",
			Handler:     p.handleUptime,
		},
		{
			Name:        "session",
			Description: "Demo session storage",
			Usage:       "session <get|set|del> [key] [value]",
			Plugin:      p.Name(),
			Category:    "tools",
			Handler:     p.handleSession,
		},
		{
			Name:        "emit",
			Description: "Emit a custom event",
			Usage:       "emit <event_name> [data]",
			Plugin:      p.Name(),
			Category:    "tools",
			Handler:     p.handleEmit,
		},
	}

	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *ExamplePlugin) Start(ctx context.Context) error { return nil }
func (p *ExamplePlugin) Stop(ctx context.Context) error  { return nil }

func (p *ExamplePlugin) handleEcho(ctx *plugin.CommandContext) error {
	if ctx.ArgCount() == 0 {
		return ctx.Edit("用法: echo <文本>")
	}
	return ctx.Edit(ctx.GetArgs())
}

func (p *ExamplePlugin) handleRoll(ctx *plugin.CommandContext) error {
	expr := "1d6"
	if ctx.ArgCount() > 0 {
		expr = ctx.GetArg(0)
	}

	parts := strings.Split(expr, "d")
	if len(parts) != 2 {
		return ctx.Edit("格式: NdM (如 2d6)")
	}

	n := 1
	m := 6
	fmt.Sscanf(parts[0], "%d", &n)
	fmt.Sscanf(parts[1], "%d", &m)

	if n > 100 || m > 10000 {
		return ctx.Edit("数值太大")
	}

	var results []int
	total := 0
	for i := 0; i < n; i++ {
		r := rand.Intn(m) + 1
		results = append(results, r)
		total += r
	}

	return ctx.Edit(fmt.Sprintf("🎲 %s: %v = %d", expr, results, total))
}

func (p *ExamplePlugin) handleChoose(ctx *plugin.CommandContext) error {
	if ctx.ArgCount() < 2 {
		return ctx.Edit("用法: choose <选项1> <选项2> ...")
	}
	choice := ctx.Args[rand.Intn(len(ctx.Args))]
	return ctx.Edit(fmt.Sprintf("🤔 我选: %s", choice))
}

func (p *ExamplePlugin) handleUptime(ctx *plugin.CommandContext) error {
	uptime := time.Since(p.startTime).Truncate(time.Second)
	return ctx.Edit(fmt.Sprintf("⏱ 插件运行时间: %s", uptime))
}

func (p *ExamplePlugin) handleSession(ctx *plugin.CommandContext) error {
	if ctx.ArgCount() == 0 {
		return ctx.Edit("用法: session <get|set|del> [key] [value]")
	}

	sub := ctx.GetArg(0)
	session := ctx.Session
	if session == nil {
		return ctx.Edit("会话不可用")
	}

	switch sub {
	case "get":
		if ctx.ArgCount() < 2 {
			return ctx.Edit("用法: session get <key>")
		}
		key := ctx.GetArg(1)
		val, ok := session.Get(key)
		if !ok {
			return ctx.Edit(fmt.Sprintf("Key '%s' 不存在", key))
		}
		return ctx.Edit(fmt.Sprintf("📋 %s = %v", key, val))

	case "set":
		if ctx.ArgCount() < 3 {
			return ctx.Edit("用法: session set <key> <value>")
		}
		key := ctx.GetArg(1)
		value := strings.Join(ctx.Args[2:], " ")
		session.Set(key, value)
		return ctx.Edit(fmt.Sprintf("✅ 设置 %s = %s", key, value))

	case "del", "delete":
		if ctx.ArgCount() < 2 {
			return ctx.Edit("用法: session del <key>")
		}
		key := ctx.GetArg(1)
		session.Delete(key)
		return ctx.Edit(fmt.Sprintf("🗑 删除 %s", key))

	default:
		return ctx.Edit("未知子命令: " + sub)
	}
}

func (p *ExamplePlugin) handleEmit(ctx *plugin.CommandContext) error {
	if ctx.ArgCount() == 0 {
		return ctx.Edit("用法: emit <event_name> [data]")
	}
	eventName := ctx.GetArg(0)
	data := "{}"
	if ctx.ArgCount() > 1 {
		data = strings.Join(ctx.Args[1:], " ")
	}

	ctx.Emitter.Emit(ctx.Context(), "example.custom", map[string]any{
		"event":  eventName,
		"data":   data,
		"user":   ctx.Message.UserID,
		"chat":   ctx.Message.ChatID,
	})

	return ctx.Edit(fmt.Sprintf("📡 已发送事件: %s", eventName))
}

// New is the entry point for the plugin loader
func New() interface{} {
	return &ExamplePlugin{}
}

// Metadata for the plugin loader
var Metadata = &struct {
	Name        string
	Description string
	Version     string
	Author      string
	MinVersion  string
}{
	Name:        "example",
	Description: "Example external plugin showing SDK usage",
	Version:     "1.0.0",
	Author:      "PaperValet Team",
	MinVersion:  "0.1.0",
}