package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type HitokotoPlugin struct {
	http *http.Client
}

func New() *HitokotoPlugin {
	return &HitokotoPlugin{http: &http.Client{Timeout: 10 * time.Second}}
}

func (p *HitokotoPlugin) Name() string        { return "hitokoto" }
func (p *HitokotoPlugin) Description() string { return "获取随机一言" }

func (p *HitokotoPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&plugin.Command{
		Name:        "hitokoto",
		Aliases:     []string{"一言", "yiyan"},
		Description: "获取随机一言",
		Usage:       "hitokoto [分类 a-l]",
		Plugin:      p.Name(),
		Category:    "fun",
		Handler:     p.handleHitokoto,
	})
}

func (p *HitokotoPlugin) Start(ctx context.Context) error { return nil }
func (p *HitokotoPlugin) Stop(ctx context.Context) error  { return nil }

type hitokotoResp struct {
	Hitokoto string `json:"hitokoto"`
	From     string `json:"from"`
	FromWho  string `json:"from_who"`
}

var validCategories = map[string]string{
	"a": "动画", "b": "漫画", "c": "游戏", "d": "文学", "e": "原创", "f": "网络",
	"g": "其他", "h": "影视", "i": "诗词", "j": "网易云", "k": "哲学", "l": "抖机灵",
}

func (p *HitokotoPlugin) handleHitokoto(ctx *plugin.CommandContext) error {
	cat := ""
	if len(ctx.Args) > 0 {
		cat = strings.ToLower(ctx.Args[0])
		if _, ok := validCategories[cat]; !ok {
			return ctx.Edit("❌ 无效分类，可选 a-l：<code>" + catList() + "</code>")
		}
	}

	q := url.Values{"encode": {"json"}, "charset": {"utf-8"}}
	if cat != "" {
		q.Set("c", cat)
	}
	req, err := http.NewRequestWithContext(ctx.Context(), http.MethodGet,
		"https://v1.hitokoto.cn/?"+q.Encode(), nil)
	if err != nil {
		return ctx.Edit("❌ 请求构建失败")
	}
	resp, err := p.http.Do(req)
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 请求一言 API 失败: %v", err))
	}
	defer resp.Body.Close()

	var data hitokotoResp
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil || data.Hitokoto == "" {
		return ctx.Edit("❌ 一言 API 返回异常")
	}

	source := data.From
	if data.FromWho != "" && data.FromWho != "null" {
		source = data.FromWho + "《" + data.From + "》"
	} else if source != "" {
		source = "《" + source + "》"
	}

	out := "💬 " + data.Hitokoto
	if source != "" {
		out += "\n\n—— " + source
	}
	return ctx.Edit(out)
}

func catList() string {
	names := make([]string, 0, len(validCategories))
	for k, v := range validCategories {
		names = append(names, k+":"+v)
	}
	return strings.Join(names, " ")
}
