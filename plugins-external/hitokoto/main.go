package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type HitokotoPlugin struct{}

func New() *HitokotoPlugin { return &HitokotoPlugin{} }

func (p *HitokotoPlugin) Name() string        { return "hitokoto" }
func (p *HitokotoPlugin) Description() string { return "获取随机一言" }

func (p *HitokotoPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	cmds := []*plugin.Command{
		{
			Name:        "hitokoto",
			Aliases:     []string{"一言", "yiyan", "quote"},
			Description: "获取随机一言",
			Usage:       "hitokoto [分类]",
			Plugin:      p.Name(),
			Category:    "fun",
			OwnerOnly:   false,
			Handler:     p.handleHitokoto,
		},
	}
	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *HitokotoPlugin) Start(ctx context.Context) error { return nil }
func (p *HitokotoPlugin) Stop(ctx context.Context) error  { return nil }

func (p *HitokotoPlugin) handleHitokoto(ctx *plugin.CommandContext) error {
	category := "a"
	if len(ctx.Args) > 0 {
		category = ctx.Args[0]
	}

	// Simulate API call to hitokoto.cn
	quotes := []string{
		"生活不仅眼前的苟且，还有诗和远方。",
		"所谓无所谓，只是没到心上。",
		"我们都在尘世中迷失，又在回忆里寻找。",
		"愿你眼里有光，心中有爱，脚下有路。",
		"不怕路长，只怕志短。",
		"山水万程，皆有好运。",
	}

	rand.Seed(time.Now().UnixNano())
	quote := quotes[rand.Intn(len(quotes))]

	return ctx.Edit(fmt.Sprintf(`💬 <b>一言</b>

%s

分类: <code>%s</code>

⚠️ 完整实现需接入 hitokoto.cn API`, quote, category))
}