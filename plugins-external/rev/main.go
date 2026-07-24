package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type RevPlugin struct{}

func New() *RevPlugin { return &RevPlugin{} }

func (p *RevPlugin) Name() string        { return "rev" }
func (p *RevPlugin) Description() string { return "反转消息内容" }

func (p *RevPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	cmds := []*plugin.Command{
		{
			Name:        "rev",
			Aliases:     []string{"reverse", "反转"},
			Description: "反转文本内容",
			Usage:       "rev <文本>",
			Plugin:      p.Name(),
			Category:    "fun",
			OwnerOnly:   false,
			Handler:     p.handleRev,
		},
	}
	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *RevPlugin) Start(ctx context.Context) error { return nil }
func (p *RevPlugin) Stop(ctx context.Context) error  { return nil }

func (p *RevPlugin) handleRev(ctx *plugin.CommandContext) error {
	if len(ctx.Args) == 0 {
		return ctx.Edit(`🔄 <b>文本反转</b>

用法: <code>rev <文本></code>

示例:
• <code>rev Hello World</code> → <code>dlroW olleH</code>
• <code>rev 你好世界</code> → <code>界世好你</code>`)
	}

	text := strings.Join(ctx.Args, " ")
	runes := []rune(text)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	reversed := string(runes)

	return ctx.Edit(fmt.Sprintf(`🔄 <b>反转结果</b>

原文: <code>%s</code>
反转: <code>%s</code>`, text, reversed))
}