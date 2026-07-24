package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type GtPlugin struct{}

func New() *GtPlugin { return &GtPlugin{} }

func (p *GtPlugin) Name() string        { return "gt" }
func (p *GtPlugin) Description() string { return "谷歌翻译" }

func (p *GtPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	cmds := []*plugin.Command{
		{
			Name:        "gt",
			Aliases:     []string{"translate", "翻译", "tr"},
			Description: "谷歌中英文互译",
			Usage:       "gt <文本>",
			Plugin:      p.Name(),
			Category:    "tools",
			OwnerOnly:   false,
			Handler:     p.handleTranslate,
		},
	}
	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *GtPlugin) Start(ctx context.Context) error { return nil }
func (p *GtPlugin) Stop(ctx context.Context) error  { return nil }

func (p *GtPlugin) handleTranslate(ctx *plugin.CommandContext) error {
	if len(ctx.Args) == 0 {
		return ctx.Edit(`🌐 <b>谷歌翻译</b>

用法: <code>gt <文本></code>

示例:
• <code>gt Hello World</code>
• <code>gt 你好世界</code>
• <code>gt これはテストです</code>

⚠️ 完整实现需接入 Google Translate API`)
	}

	text := strings.Join(ctx.Args, " ")
	return ctx.Edit(fmt.Sprintf(`🌐 <b>翻译结果</b>

原文: <code>%s</code>

⚠️ 完整实现需接入 Google Translate API`, text))
}