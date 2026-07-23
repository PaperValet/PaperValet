package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type SudoPlugin struct{}

func New() (plugin.Plugin, error) {
	return &SudoPlugin{}, nil
}

var Metadata = &plugin.PluginMetadata{
	Name:        "sudo",
	Description: "sudo权限执行",
	Version:     "1.0.0",
	Author:      "PaperValet",
	MinVersion:  "0.1.0",
}

func (p *SudoPlugin) Name() string        { return "sudo" }
func (p *SudoPlugin) Description() string { return "sudo权限执行" }

func (p *SudoPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	cmds := []*plugin.Command{
		{
			Name:        "sudo",
			Description: "以所有者权限执行命令",
			Usage:       "sudo <命令> [参数...]",
			Plugin:      p.Name(),
			Category:    "admin",
			OwnerOnly:   true,
			Handler:     p.handleSudo,
		},
		{
			Name:        "su",
			Description: "切换用户执行 (模拟)",
			Usage:       "su <用户ID> <命令>",
			Plugin:      p.Name(),
			Category:    "admin",
			OwnerOnly:   true,
			Handler:     p.handleSu,
		},
	}

	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *SudoPlugin) handleSudo(ctx *plugin.CommandContext) error {
	args := ctx.Args()
	if len(args) == 0 {
		return ctx.Edit("用法: sudo <命令> [参数...]")
	}

	command := args[0]
	cmdArgs := args[1:]

	// This would typically use the command registry to execute another command
	// For now, we just simulate the concept
	fullCmd := fmt.Sprintf("%s %s", command, strings.Join(cmdArgs, " "))
	
	return ctx.Edit(fmt.Sprintf(`👑 <b>Sudo 执行</b>

<code>%s</code>

<i>需要实现命令分发器调用</i>
⏰ %s`, fullCmd, time.Now().Format("15:04:05")))
}

func (p *SudoPlugin) handleSu(ctx *plugin.CommandContext) error {
	args := ctx.Args()
	if len(args) < 2 {
		return ctx.Edit("用法: su <用户ID> <命令>")
	}

	userID := args[0]
	command := args[1]
	cmdArgs := args[2:]

	fullCmd := fmt.Sprintf("%s %s", command, strings.Join(cmdArgs, " "))

	return ctx.Edit(fmt.Sprintf(`👤 <b>用户模拟</b>

<b>目标用户:</b> <code>%s</code>
<b>命令:</b> <code>%s</code>

<i>需要实现用户上下文切换</i>
⏰ %s`, userID, fullCmd, time.Now().Format("15:04:05")))
}

func (p *SudoPlugin) Start(ctx context.Context) error { return nil }
func (p *SudoPlugin) Stop(ctx context.Context) error  { return nil }