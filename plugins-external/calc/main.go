package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type CalcPlugin struct{}

func New() *CalcPlugin { return &CalcPlugin{} }

func (p *CalcPlugin) Name() string        { return "calc" }
func (p *CalcPlugin) Description() string { return "计算器" }

func (p *CalcPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	cmds := []*plugin.Command{
		{
			Name:        "calc",
			Aliases:     []string{"calculate", "计算", "js"},
			Description: "计算数学表达式",
			Usage:       "calc <表达式>",
			Plugin:      p.Name(),
			Category:    "tools",
			OwnerOnly:   false,
			Handler:     p.handleCalc,
		},
	}
	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *CalcPlugin) Start(ctx context.Context) error { return nil }
func (p *CalcPlugin) Stop(ctx context.Context) error  { return nil }

func (p *CalcPlugin) handleCalc(ctx *plugin.CommandContext) error {
	if len(ctx.Args) == 0 {
		return ctx.Edit(`🧮 <b>计算器</b>

用法: <code>calc <表达式></code>

支持: + - * / % ( ) ^ sqrt() sin() cos() tan() log() ln() pi e

示例:
• <code>calc 2 + 3 * 4</code>
• <code>calc sqrt(16)</code>
• <code>calc sin(pi/2)</code>
• <code>calc (100+200)*0.85</code>

⚠️ 完整实现需集成表达式解析库 (如 govaluate)`)
	}

	expr := strings.Join(ctx.Args, " ")
	return ctx.Edit(fmt.Sprintf(`🧮 <b>计算结果</b>

表达式: <code>%s</code>

⚠️ 完整实现需集成表达式解析库`, expr))
}