package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type EncodePlugin struct{}

func New() *EncodePlugin { return &EncodePlugin{} }

func (p *EncodePlugin) Name() string        { return "encode" }
func (p *EncodePlugin) Description() string { return "编码/解码工具" }

func (p *EncodePlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&plugin.Command{
		Name:        "encode",
		Aliases:     []string{"enc", "b64"},
		Description: "编码/解码: base64, url, hex",
		Usage:       "encode <base64|url|hex> <encode|decode> <内容>",
		Plugin:      p.Name(),
		Category:    "tools",
		Handler:     p.handleEncode,
	})
}

func (p *EncodePlugin) Start(ctx context.Context) error { return nil }
func (p *EncodePlugin) Stop(ctx context.Context) error  { return nil }

func (p *EncodePlugin) handleEncode(ctx *plugin.CommandContext) error {
	if len(ctx.Args) < 3 {
		return ctx.Edit(`🔐 <b>编码/解码工具</b>

用法: <code>encode <类型> <encode|decode> <内容></code>

类型: base64 / url / hex
示例: <code>encode base64 encode Hello</code>`)
	}

	typ := strings.ToLower(ctx.Args[0])
	action := strings.ToLower(ctx.Args[1])
	input := strings.Join(ctx.Args[2:], " ")

	var out string
	var err error
	encode := action == "encode" || action == "enc" || action == "e"
	decode := action == "decode" || action == "dec" || action == "d"
	if !encode && !decode {
		return ctx.Edit("❌ 操作必须是 encode 或 decode")
	}

	switch typ {
	case "base64", "b64":
		if encode {
			out = base64.StdEncoding.EncodeToString([]byte(input))
		} else {
			var b []byte
			b, err = base64.StdEncoding.DecodeString(input)
			out = string(b)
		}
	case "url":
		if encode {
			out = url.QueryEscape(input)
		} else {
			out, err = url.QueryUnescape(input)
		}
	case "hex":
		if encode {
			out = hex.EncodeToString([]byte(input))
		} else {
			var b []byte
			b, err = hex.DecodeString(input)
			out = string(b)
		}
	default:
		return ctx.Edit("❌ 未知类型，支持 base64 / url / hex")
	}
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 解码失败: %v", err))
	}

	op := "编码"
	if decode {
		op = "解码"
	}
	return ctx.Edit(fmt.Sprintf("🔐 <b>%s %s</b>\n\n输入: <code>%s</code>\n输出: <code>%s</code>",
		typ, op, input, out))
}
