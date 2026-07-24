package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type QRPlugin struct{}

func New() *QRPlugin { return &QRPlugin{} }

func (p *QRPlugin) Name() string        { return "qr" }
func (p *QRPlugin) Description() string { return "二维码生成/识别" }

func (p *QRPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	cmds := []*plugin.Command{
		{
			Name:        "qr",
			Aliases:     []string{"qrcode", "二维码"},
			Description: "生成二维码",
			Usage:       "qr <内容>",
			Plugin:      p.Name(),
			Category:    "tools",
			OwnerOnly:   false,
			Handler:     p.handleQR,
		},
	}
	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *QRPlugin) Start(ctx context.Context) error { return nil }
func (p *QRPlugin) Stop(ctx context.Context) error  { return nil }

func (p *QRPlugin) handleQR(ctx *plugin.CommandContext) error {
	if len(ctx.Args) == 0 {
		return ctx.Edit(`📱 <b>二维码生成</b>

用法: <code>qr <内容></code>

示例:
• <code>qr https://github.com</code>
• <code>qr Hello World</code>
• <code>qr WIFI:T:WPA;S:MyWiFi;P:password123;;></code>

⚠️ 完整实现需引入 github.com/skip2/go-qrcode`)
	}

	content := strings.Join(ctx.Args, " ")
	return ctx.Edit(fmt.Sprintf(`📱 <b>二维码生成</b>

内容: <code>%s</code>

⚠️ 完整实现需引入 go-qrcode 库生成图片`, content))
}