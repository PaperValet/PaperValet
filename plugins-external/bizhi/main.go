package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type BizhiPlugin struct {
	http *http.Client
}

func New() *BizhiPlugin {
	return &BizhiPlugin{http: &http.Client{Timeout: 30 * time.Second}}
}

func (p *BizhiPlugin) Name() string        { return "bizhi" }
func (p *BizhiPlugin) Description() string { return "发送随机壁纸" }

func (p *BizhiPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&plugin.Command{
		Name:        "bizhi",
		Aliases:     []string{"wallpaper", "壁纸"},
		Description: "获取随机壁纸图片",
		Usage:       "bizhi [分类]",
		Plugin:      p.Name(),
		Category:    "fun",
		Handler:     p.handleBizhi,
	})
}

func (p *BizhiPlugin) Start(ctx context.Context) error { return nil }
func (p *BizhiPlugin) Stop(ctx context.Context) error  { return nil }

var bizhiSources = map[string]string{
	"dongman":  "https://api.btstu.cn/sjbz/api.php?lx=dongman&format=images",
	"meinv":    "https://api.btstu.cn/sjbz/api.php?lx=meinv&format=images",
	"fengjing": "https://api.btstu.cn/sjbz/api.php?lx=fengjing&format=images",
	"suiji":    "https://api.btstu.cn/sjbz/api.php?lx=suiji&format=images",
}

func (p *BizhiPlugin) handleBizhi(ctx *plugin.CommandContext) error {
	if ctx.Media == nil {
		return ctx.Edit("❌ 媒体服务未就绪")
	}

	source := "https://picsum.photos/1920/1080"
	if len(ctx.Args) > 0 {
		cat := strings.ToLower(ctx.Args[0])
		u, ok := bizhiSources[cat]
		if !ok {
			return ctx.Edit("❌ 未知分类，可选：dongman / meinv / fengjing / suiji，留空为随机图")
		}
		source = u
	}

	req, err := http.NewRequestWithContext(ctx.Context(), http.MethodGet, source, nil)
	if err != nil {
		return ctx.Edit("❌ 请求构建失败")
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := p.http.Do(req)
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 壁纸下载失败: %v", err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ctx.Edit(fmt.Sprintf("❌ 壁纸源返回 %d", resp.StatusCode))
	}

	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("pv_bizhi_%d.jpg", ctx.Message.Message.ID))
	f, err := os.Create(tmp)
	if err != nil {
		return ctx.Edit("❌ 临时文件创建失败")
	}
	defer os.Remove(tmp)

	// Cap at 20MB to avoid abuse.
	if _, err := io.Copy(f, io.LimitReader(resp.Body, 20<<20)); err != nil {
		f.Close()
		return ctx.Edit("❌ 壁纸写入失败")
	}
	f.Close()

	if err := ctx.ReplyMedia(tmp, "🖼 随机壁纸"); err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 发送壁纸失败: %v", err))
	}
	return ctx.Delete()
}
