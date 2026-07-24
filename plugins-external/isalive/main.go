package main

import (
	"context"
	"fmt"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type IsAlivePlugin struct{}

func New() *IsAlivePlugin { return &IsAlivePlugin{} }

func (p *IsAlivePlugin) Name() string        { return "isalive" }
func (p *IsAlivePlugin) Description() string { return "活了么 - 检测 bot 是否在线" }

func (p *IsAlivePlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	cmds := []*plugin.Command{
		{
			Name:        "isalive",
			Aliases:     []string{"alive", "活了么", "在吗"},
			Description: "检测 bot 运行状态",
			Usage:       "isalive",
			Plugin:      p.Name(),
			Category:    "core",
			OwnerOnly:   false,
			Handler:     p.handleIsAlive,
		},
	}
	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *IsAlivePlugin) Start(ctx context.Context) error { return nil }
func (p *IsAlivePlugin) Stop(ctx context.Context) error  { return nil }

func (p *IsAlivePlugin) handleIsAlive(ctx *plugin.CommandContext) error {
	return ctx.Edit(`✅ <b>存活检测</b>

状态: <code>在线</code>
版本: <code>PaperValet 1.0</code>
运行时: <code>gotd/td</code>

💡 Bot 运行正常`)
}