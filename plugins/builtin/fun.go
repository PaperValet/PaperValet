package builtin

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/TiaraBasori/PaperValet/internal/command"
	"github.com/TiaraBasori/PaperValet/internal/core"
	"github.com/TiaraBasori/PaperValet/internal/plugin"
)

// FunPlugin provides entertainment commands.
type FunPlugin struct{}

func NewFun() *FunPlugin { return &FunPlugin{} }

func (p *FunPlugin) Name() string        { return "fun" }
func (p *FunPlugin) Description() string { return "娱乐命令" }

func (p *FunPlugin) Init(_ context.Context, mgr *plugin.Manager) error {
	cmds := []*command.Command{
		{
			Name:        "roll",
			Aliases:     []string{"dice"},
			Description: "掷骰子",
			Usage:       "roll [最大值]",
			Plugin:      p.Name(),
			Category:    "fun",
			Handler:     p.handleRoll,
		},
		{
			Name:        "coin",
			Aliases:     []string{"flip"},
			Description: "抛硬币",
			Plugin:      p.Name(),
			Category:    "fun",
			Handler:     p.handleCoin,
		},
		{
			Name:        "choose",
			Aliases:     []string{"pick"},
			Description: "从选项中随机选择",
			Usage:       "choose <选项1> <选项2> ...",
			Plugin:      p.Name(),
			Category:    "fun",
			Handler:     p.handleChoose,
		},
		{
			Name:        "8ball",
			Description: "魔法八球占卜",
			Usage:       "8ball <问题>",
			Plugin:      p.Name(),
			Category:    "fun",
			Handler:     p.handle8ball,
		},
		{
			Name:        "fact",
			Description: "随机冷知识",
			Plugin:      p.Name(),
			Category:    "fun",
			Handler:     p.handleFact,
		},
	}
	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *FunPlugin) Start(_ context.Context) error { return nil }
func (p *FunPlugin) Stop(_ context.Context) error  { return nil }

func (p *FunPlugin) handleRoll(ctx *core.CommandContext) error {
	max := 6
	if ctx.ArgCount() > 0 {
		var err error
		max, err = parseInt(ctx.GetArg(0))
		if err != nil || max < 2 {
			return ctx.Edit("最大值必须 >= 2")
		}
	}
	result := rand.Intn(max) + 1
	return ctx.Edit(fmt.Sprintf("🎲 掷骰子 (1-%d): %d", max, result))
}

func (p *FunPlugin) handleCoin(ctx *core.CommandContext) error {
	result := "正面"
	if rand.Intn(2) == 0 {
		result = "反面"
	}
	return ctx.Edit(fmt.Sprintf("🪙 抛硬币: %s", result))
}

func (p *FunPlugin) handleChoose(ctx *core.CommandContext) error {
	if ctx.ArgCount() < 2 {
		return ctx.Edit("用法: choose <选项1> <选项2> ...")
	}
	choice := ctx.Args[rand.Intn(len(ctx.Args))]
	return ctx.Edit(fmt.Sprintf("🤔 我选: %s", choice))
}

func (p *FunPlugin) handle8ball(ctx *core.CommandContext) error {
	if ctx.ArgCount() == 0 {
		return ctx.Edit("用法: 8ball <问题>")
	}
	answers := []string{
		"确定无疑 🎯", "非常有可能 👍", "不用怀疑 ✅", "是的 👌",
		"大概率 🤷", "看起来不错 👀", "是的 😊", "迹象表明会 📈",
		"回复模糊 🌫️", "稍后再问 ⏳", "最好不告诉你 🤐", "别指望了 🙅",
		"答案是否定 ❌", "非常不可能 👎", "前景不佳 📉", "极不可能 🚫",
	}
	return ctx.Edit(fmt.Sprintf("🎱 %s", answers[rand.Intn(len(answers))]))
}

func (p *FunPlugin) handleFact(ctx *core.CommandContext) error {
	facts := []string{
		"章鱼有三颗心脏和蓝色的血液 🐙",
		"蜂蜜永远不会变质 🍯",
		"香蕉是浆果，但草莓不是 🍌",
		"章鱼有九个大脑 🧠",
		"一只蜗牛可以睡三年 🐌",
		"海獭睡觉时会牵手 🦦",
		"企鹅会向伴侣送鹅卵石求婚 🐧",
		"猫的鼻纹像指纹一样独一无二 🐱",
		"海豚睡觉时只睡半个大脑 🐬",
		"鲨鱼比树还古老 🦈",
	}
	return ctx.Edit(fmt.Sprintf("💡 %s", facts[rand.Intn(len(facts))]))
}

func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}