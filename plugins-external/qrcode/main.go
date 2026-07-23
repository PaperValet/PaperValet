package main

import (
	"context"
	"fmt"
	"image/png"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type QRCodePlugin struct{}

func New() (plugin.Plugin, error) {
	return &QRCodePlugin{}, nil
}

var Metadata = &plugin.PluginMetadata{
	Name:        "qrcode",
	Description: "二维码生成器",
	Version:     "1.0.0",
	Author:      "PaperValet",
	MinVersion:  "0.1.0",
}

func (p *QRCodePlugin) Name() string        { return "qrcode" }
func (p *QRCodePlugin) Description() string { return "二维码生成器" }

func (p *QRCodePlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&plugin.Command{
		Name:        "qrcode",
		Aliases:     []string{"qr"},
		Description: "生成二维码",
		Usage:       "qrcode <内容> [大小]",
		Plugin:      p.Name(),
		Category:    "tools",
		Handler:     p.handleQRCode,
	})
}

func (p *QRCodePlugin) handleQRCode(ctx *plugin.CommandContext) error {
	args := ctx.Args()
	if len(args) == 0 {
		return ctx.Edit("用法: qrcode <内容> [大小]\n示例: qrcode https://github.com 256")
	}

	content := args[0]
	size := 256
	if len(args) > 1 {
		if s, err := strconv.Atoi(args[1]); err == nil && s > 0 && s <= 1024 {
			size = s
		}
	}

	_ = ctx.Edit("⏳ 生成二维码中...")

	// Generate QR code
	code, err := qr.Encode(content, qr.M, qr.Auto)
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 生成失败: %v", err))
	}

	// Scale to desired size
	code, err = barcode.Scale(code, size, size)
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 缩放失败: %v", err))
	}

	// Save to temp file
	tmpFile := fmt.Sprintf("/tmp/qrcode_%d.png", time.Now().UnixNano())
	f, err := os.Create(tmpFile)
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 创建文件失败: %v", err))
	}
	defer f.Close()
	defer os.Remove(tmpFile)

	if err := png.Encode(f, code); err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 编码失败: %v", err))
	}

	// Send as file
	return ctx.Reply(fmt.Sprintf("📱 <b>二维码</b>\n内容: <code>%s</code>\n大小: %dx%d", content, size, size))
}

func (p *QRCodePlugin) Start(ctx context.Context) error { return nil }
func (p *QRCodePlugin) Stop(ctx context.Context) error  { return nil }