package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type DuckDuckGoPlugin struct{}

func New() *DuckDuckGoPlugin { return &DuckDuckGoPlugin{} }

func (p *DuckDuckGoPlugin) Name() string        { return "duckduckgo" }
func (p *DuckDuckGoPlugin) Description() string { return "DuckDuckGo 搜索" }

func (p *DuckDuckGoPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	cmds := []*plugin.Command{
		{
			Name:        "duckduckgo",
			Aliases:     []string{"ddg", "search", "搜索"},
			Description: "使用 DuckDuckGo 搜索",
			Usage:       "duckduckgo <查询>",
			Plugin:      p.Name(),
			Category:    "tools",
			OwnerOnly:   false,
			Handler:     p.handleSearch,
		},
	}
	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *DuckDuckGoPlugin) Start(ctx context.Context) error { return nil }
func (p *DuckDuckGoPlugin) Stop(ctx context.Context) error  { return nil }

func (p *DuckDuckGoPlugin) handleSearch(ctx *plugin.CommandContext) error {
	if len(ctx.Args) == 0 {
		return ctx.Edit(`🔍 <b>DuckDuckGo 搜索</b>

用法: <code>duckduckgo <查询></code>
示例: <code>duckduckgo Go 语言教程</code>

⚠️ 完整实现需接入 DuckDuckGo Instant Answer API
https://api.duckduckgo.com/?q=query&format=json`)
	}

	query := strings.Join(ctx.Args, " ")
	return ctx.Edit(fmt.Sprintf(`🔍 <b>DuckDuckGo 搜索结果</b>

查询: <code>%s</code>

⚠️ 完整实现需接入 DuckDuckGo API`, query))
}