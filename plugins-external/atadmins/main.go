package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type AtAdminsPlugin struct{}

func New() *AtAdminsPlugin { return &AtAdminsPlugin{} }

func (p *AtAdminsPlugin) Name() string        { return "atadmins" }
func (p *AtAdminsPlugin) Description() string { return "一键艾特全部管理员" }

func (p *AtAdminsPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	cmds := []*plugin.Command{
		{
			Name:        "atadmins",
			Aliases:     []string{"calladmins", "管理员"},
			Description: "艾特群组所有管理员",
			Usage:       "atadmins [消息内容]",
			Plugin:      p.Name(),
			Category:    "group",
			OwnerOnly:   false,
			Handler:     p.handleAtAdmins,
		},
	}
	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *AtAdminsPlugin) Start(ctx context.Context) error { return nil }
func (p *AtAdminsPlugin) Stop(ctx context.Context) error  { return nil }

func (p *AtAdminsPlugin) handleAtAdmins(ctx *plugin.CommandContext) error {
	if ctx.Message == nil {
		return ctx.Edit("❌ 仅支持在群组中使用")
	}

	// In real implementation, would fetch admins via API
	// For now, simulate
	msg := strings.Join(ctx.Args, " ")
	if msg == "" {
		msg = "管理员们请注意查收！"
	}

	return ctx.Edit(fmt.Sprintf(`📢 <b>艾特全体管理员</b>

%s

⚠️ 完整实现需要接入 gotd 获取管理员列表并发送 mention`, msg))
}