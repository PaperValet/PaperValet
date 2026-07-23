package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
	"github.com/gotd/td/tg"
)

type RePlugin struct{}

func New() (plugin.Plugin, error) {
	return &RePlugin{}, nil
}

var Metadata = &plugin.PluginMetadata{
	Name:        "re",
	Description: "消息复读工具",
	Version:     "1.0.0",
	Author:      "PaperValet",
	MinVersion:  "0.1.0",
}

func (p *RePlugin) Name() string        { return "re" }
func (p *RePlugin) Description() string { return "消息复读工具" }

func (p *RePlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&plugin.Command{
		Name:        "re",
		Description: "复读回复的消息",
		Usage:       "re [数量] [次数]",
		Plugin:      p.Name(),
		Category:    "tools",
		Handler:     p.handleRe,
	})
}

func (p *RePlugin) handleRe(ctx *plugin.CommandContext) error {
	msg := ctx.Message()
	if msg == nil {
		return ctx.Edit("❌ 无消息上下文")
	}

	if !msg.IsReply {
		return ctx.Edit("❌ 你必须回复一条消息才能复读")
	}

	args := ctx.Args()
	count := 1
	repeat := 1

	if len(args) > 0 {
		if c, err := strconv.Atoi(args[0]); err == nil && c > 0 {
			count = c
		}
	}
	if len(args) > 1 {
		if r, err := strconv.Atoi(args[1]); err == nil && r > 0 {
			repeat = r
		}
	}

	// Limit to reasonable values
	if count > 100 {
		count = 100
	}
	if repeat > 10 {
		repeat = 10
	}

	// Get the API client
	client := ctx.API()
	if client == nil {
		return ctx.Edit("❌ 客户端不可用")
	}

	// Try to get the replied message
	// This is a simplified implementation
	return ctx.Edit(fmt.Sprintf("📝 复读功能\n\n数量: %d\n次数: %d\n\n⚠️ 需要完整实现消息获取和转发逻辑", count, repeat))
}

func (p *RePlugin) Start(ctx context.Context) error { return nil }
func (p *RePlugin) Stop(ctx context.Context) error  { return nil }