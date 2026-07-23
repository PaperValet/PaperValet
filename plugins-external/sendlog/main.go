package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type SendLogPlugin struct{}

func New() (plugin.Plugin, error) {
	return &SendLogPlugin{}, nil
}

var Metadata = &plugin.PluginMetadata{
	Name:        "sendlog",
	Description: "发送日志",
	Version:     "1.0.0",
	Author:      "PaperValet",
	MinVersion:  "0.1.0",
}

func (p *SendLogPlugin) Name() string        { return "sendlog" }
func (p *SendLogPlugin) Description() string { return "发送日志" }

func (p *SendLogPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&plugin.Command{
		Name:        "sendlog",
		Aliases:     []string{"slog", "log"},
		Description: "发送运行日志到当前聊天",
		Usage:       "sendlog [行数]",
		Plugin:      p.Name(),
		Category:    "tools",
		Handler:     p.handleSendLog,
	})
}

func (p *SendLogPlugin) handleSendLog(ctx *plugin.CommandContext) error {
	args := ctx.Args
	lines := 100
	if len(args) > 0 {
		if n, err := strconv.Atoi(args[0]); err == nil && n > 0 && n <= 1000 {
			lines = n
		}
	}

	return ctx.Edit(fmt.Sprintf(`📋 <b>发送日志</b>

行数: <code>%d</code>

⚠️ <i>需要实现日志文件读取和发送逻辑</i>
⏰ <i>%s</i>`, lines, time.Now().Format("15:04:05")))
}

func (p *SendLogPlugin) Start(ctx context.Context) error { return nil }
func (p *SendLogPlugin) Stop(ctx context.Context) error  { return nil }