package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type BizhiPlugin struct{}

func New() *BizhiPlugin { return &BizhiPlugin{} }

func (p *BizhiPlugin) Name() string        { return "bizhi" }
func (p *BizhiPlugin) Description() string { return "发送随机壁纸" }

func (p *BizhiPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	cmds := []*plugin.Command{
		{
			Name:        "bizhi",
			Aliases:     []string{"wallpaper", "壁纸"},
			Description: "获取随机壁纸图片",
			Usage:       "bizhi [分类]",
			Plugin:      p.Name(),
			Category:    "fun",
			OwnerOnly:   false,
			Handler:     p.handleBizhi,
		},
	}
	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *BizhiPlugin) Start(ctx context.Context) error { return nil }
func (p *BizhiPlugin) Stop(ctx context.Context) error  { return nil }

func (p *BizhiPlugin) handleBizhi(ctx *plugin.CommandContext) error {
	category := "random"
	if len(ctx.Args) > 0 {
		category = ctx.Args[0]
	}

	// Simulate fetching wallpaper from API
	// Real impl would call API like https://api.btstu.cn/sjbz/?lx=dongman
	urls := []string{
		"https://picsum.photos/1920/1080",
		"https://source.unsplash.com/random/1920x1080",
		"https://api.btstu.cn/sjbz/?lx=dongman",
		"https://api.btstu.cn/sjbz/?lx=meinv",
		"https://api.btstu.cn/sjbz/?lx=fengjing",
	}

	rand.Seed(time.Now().UnixNano())
	url := urls[rand.Intn(len(urls))]

	return ctx.Edit(fmt.Sprintf(`🖼 <b>随机壁纸</b>

分类: <code>%s</code>
链接: %s

⚠️ 完整实现需接入壁纸 API 并使用 client.SendMedia 发送图片`, category, url))
}