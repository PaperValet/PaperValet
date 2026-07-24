package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type WeatherPlugin struct{}

func New() *WeatherPlugin { return &WeatherPlugin{} }

func (p *WeatherPlugin) Name() string        { return "weather" }
func (p *WeatherPlugin) Description() string { return "天气查询" }

func (p *WeatherPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	cmds := []*plugin.Command{
		{
			Name:        "weather",
			Aliases:     []string{"天气", "wt"},
			Description: "查询天气",
			Usage:       "weather <城市>",
			Plugin:      p.Name(),
			Category:    "tools",
			OwnerOnly:   false,
			Handler:     p.handleWeather,
		},
	}
	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *WeatherPlugin) Start(ctx context.Context) error { return nil }
func (p *WeatherPlugin) Stop(ctx context.Context) error  { return nil }

func (p *WeatherPlugin) handleWeather(ctx *plugin.CommandContext) error {
	if len(ctx.Args) == 0 {
		return ctx.Edit(`🌤 <b>天气查询</b>

用法: <code>weather <城市></code>

示例:
• <code>weather 北京</code>
• <code>weather 上海</code>
• <code>weather 深圳</code>

⚠️ 完整实现需接入天气 API (如和风天气、OpenWeatherMap)`)
	}

	city := strings.Join(ctx.Args, " ")
	return ctx.Edit(fmt.Sprintf(`🌤 <b>天气查询</b>

城市: <code>%s</code>

⚠️ 完整实现需接入天气 API 获取实时数据`, city))
}