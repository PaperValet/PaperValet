package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type EncodePlugin struct{}

func New() *EncodePlugin { return &EncodePlugin{} }

func (p *EncodePlugin) Name() string        { return "encode" }
func (p *EncodePlugin) Description() string { return "编码/解码工具" }

func (p *EncodePlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	cmds := []*plugin.Command{
		{
			Name:        "encode",
			Aliases:     []string{"enc", "base64", "urlencode"},
			Description: "编码/解码: base64, url, hex",
			Usage:       "encode <base64|url|hex> <encode|decode> <内容>",
			Plugin:      p.Name(),
			Category:    "tools",
			OwnerOnly:   false,
			Handler:     p.handleEncode,
		},
	}
	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *EncodePlugin) Start(ctx context.Context) error { return nil }
func (p *EncodePlugin) Stop(ctx context.Context) error  { return nil }

func (p *EncodePlugin) handleEncode(ctx *plugin.CommandContext) error {
	if len(ctx.Args) < 3 {
		return ctx.Edit(`🔐 <b>编码/解码工具</b>

用法: <code>encode <类型> <encode|decode> <内容></code>

类型:
• <code>base64</code> — Base64 编码
• <code>url</code> — URL 编码
• <code>hex</code> — 十六进制

示例:
• <code>encode base64 encode Hello World</code>
• <code>encode url decode https%3A%2F%2Fexample.com</code>`)
	}

	typ := ctx.Args[0]
	action := ctx.Args[1]
	input := strings.Join(ctx.Args[2:], " ")

	return ctx.Edit(fmt.Sprintf(`🔐 <b>编码/解码</b>

类型: <code>%s</code>
操作: <code>%s</code>
输入: <code>%s</code>

⚠️ 完整实现需引入 encoding/base64, net/url, encoding/hex`, typ, action, input))
}