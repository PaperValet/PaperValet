package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type GtPlugin struct {
	http *http.Client
}

func New() *GtPlugin {
	return &GtPlugin{http: &http.Client{Timeout: 15 * time.Second}}
}

func (p *GtPlugin) Name() string        { return "gt" }
func (p *GtPlugin) Description() string { return "谷歌翻译" }

func (p *GtPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&plugin.Command{
		Name:        "gt",
		Aliases:     []string{"translate", "翻译"},
		Description: "谷歌翻译（默认自动检测，中文↔英文互译）",
		Usage:       "gt [目标语言] <文本>",
		Plugin:      p.Name(),
		Category:    "tools",
		Handler:     p.handleTranslate,
	})
}

func (p *GtPlugin) Start(ctx context.Context) error { return nil }
func (p *GtPlugin) Stop(ctx context.Context) error  { return nil }

var langCodeRe = regexp.MustCompile(`^[a-z]{2,3}(-[a-zA-Z]{2,4})?$`)

func (p *GtPlugin) handleTranslate(ctx *plugin.CommandContext) error {
	if len(ctx.Args) == 0 {
		return ctx.Edit("🌐 用法：<code>gt [目标语言] <文本></code>\n示例：<code>gt en 你好</code>、<code>gt hello world</code>")
	}

	args := ctx.Args
	target := ""
	if langCodeRe.MatchString(args[0]) && len(args) > 1 {
		target = args[0]
		args = args[1:]
	}
	text := strings.Join(args, " ")

	if target == "" {
		if containsCJK(text) {
			target = "en"
		} else {
			target = "zh-CN"
		}
	}

	q := url.Values{
		"client": {"gtx"},
		"sl":     {"auto"},
		"tl":     {target},
		"dt":     {"t"},
		"q":      {text},
	}
	req, err := http.NewRequestWithContext(ctx.Context(), http.MethodGet,
		"https://translate.googleapis.com/translate_a/single?"+q.Encode(), nil)
	if err != nil {
		return ctx.Edit("❌ 请求构建失败")
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := p.http.Do(req)
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 翻译请求失败: %v", err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ctx.Edit(fmt.Sprintf("❌ 翻译服务返回 %d", resp.StatusCode))
	}

	// Response: [[["translated","source",...],...], "detected_lang", ...]
	var raw []any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil || len(raw) == 0 {
		return ctx.Edit("❌ 翻译响应解析失败")
	}

	var sb strings.Builder
	if segs, ok := raw[0].([]any); ok {
		for _, seg := range segs {
			if parts, ok := seg.([]any); ok && len(parts) > 0 {
				if s, ok := parts[0].(string); ok {
					sb.WriteString(s)
				}
			}
		}
	}
	translated := sb.String()
	if translated == "" {
		return ctx.Edit("❌ 翻译结果为空")
	}

	detected := ""
	if len(raw) > 2 {
		detected, _ = raw[2].(string)
	}

	return ctx.Edit(fmt.Sprintf("🌐 <b>翻译</b>（%s → %s）\n\n%s", detected, target, translated))
}

func containsCJK(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) ||
			unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hangul, r) {
			return true
		}
	}
	return false
}
