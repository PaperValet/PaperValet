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

type DuckDuckGoPlugin struct {
	http *http.Client
}

func New() *DuckDuckGoPlugin {
	return &DuckDuckGoPlugin{http: &http.Client{Timeout: 15 * time.Second}}
}

func (p *DuckDuckGoPlugin) Name() string        { return "duckduckgo" }
func (p *DuckDuckGoPlugin) Description() string { return "DuckDuckGo 搜索" }

func (p *DuckDuckGoPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&plugin.Command{
		Name:        "duckduckgo",
		Aliases:     []string{"ddg", "search", "搜索"},
		Description: "DuckDuckGo 即时答案搜索",
		Usage:       "duckduckgo <查询>",
		Plugin:      p.Name(),
		Category:    "tools",
		Handler:     p.handleSearch,
	})
}

func (p *DuckDuckGoPlugin) Start(ctx context.Context) error { return nil }
func (p *DuckDuckGoPlugin) Stop(ctx context.Context) error  { return nil }

type ddgResp struct {
	Heading      string `json:"Heading"`
	AbstractText string `json:"AbstractText"`
	AbstractURL  string `json:"AbstractURL"`
	Answer       string `json:"Answer"`
	RelatedTopic []struct {
		Text     string `json:"Text"`
		FirstURL string `json:"FirstURL"`
		Topics   []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"Topics"`
	} `json:"RelatedTopics"`
}

func (p *DuckDuckGoPlugin) handleSearch(ctx *plugin.CommandContext) error {
	if len(ctx.Args) == 0 {
		return ctx.Edit("🔍 用法：<code>duckduckgo <查询></code>")
	}
	query := strings.Join(ctx.Args, " ")

	q := url.Values{
		"q":              {query},
		"format":         {"json"},
		"no_html":        {"1"},
		"skip_disambig":  {"1"},
		"t":              {"PaperValet"},
	}
	req, err := http.NewRequestWithContext(ctx.Context(), http.MethodGet,
		"https://api.duckduckgo.com/?"+q.Encode(), nil)
	if err != nil {
		return ctx.Edit("❌ 请求构建失败")
	}
	resp, err := p.http.Do(req)
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 搜索失败: %v", err))
	}
	defer resp.Body.Close()

	var data ddgResp
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return ctx.Edit("❌ 搜索响应解析失败")
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "🔍 <b>%s</b>\n\n", query)

	switch {
	case data.Answer != "":
		fmt.Fprintf(&sb, "%s\n", data.Answer)
	case data.AbstractText != "":
		if data.Heading != "" {
			fmt.Fprintf(&sb, "<b>%s</b>\n", data.Heading)
		}
		fmt.Fprintf(&sb, "%s\n", data.AbstractText)
		if data.AbstractURL != "" {
			fmt.Fprintf(&sb, "<a href=\"%s\">来源</a>\n", data.AbstractURL)
		}
	default:
		sb.WriteString("没有找到即时答案，试试更具体的关键词。\n")
	}

	// Append a few related topics.
	count := 0
	for _, t := range data.RelatedTopic {
		if count >= 3 {
			break
		}
		text, link := t.Text, t.FirstURL
		if text == "" && len(t.Topics) > 0 {
			text, link = t.Topics[0].Text, t.Topics[0].FirstURL
		}
		if text == "" || link == "" {
			continue
		}
		if count == 0 {
			sb.WriteString("\n<b>相关结果</b>\n")
		}
		fmt.Fprintf(&sb, "• <a href=\"%s\">%s</a>\n", link, text)
		count++
	}
	return ctx.Edit(sb.String())
}
