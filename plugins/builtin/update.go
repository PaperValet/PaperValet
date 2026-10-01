package builtin

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

const releaseAPI = "https://api.github.com/repos/PaperValet/PaperValet/releases/latest"

// UpdatePlugin upgrades the running binary from GitHub Releases.
type UpdatePlugin struct {
	version string
	restart func(ctx *interfaces.CommandContext) error
}

func NewUpdate(version string, restart func(*interfaces.CommandContext) error) *UpdatePlugin {
	return &UpdatePlugin{version: version, restart: restart}
}

func (p *UpdatePlugin) Name() string        { return "update" }
func (p *UpdatePlugin) Description() string { return "升级版本" }
func (p *UpdatePlugin) DescEN() string      { return "Check for and install new versions" }

func (p *UpdatePlugin) Init(_ context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "update",
		Description: "升级版本",
		DescEN:      "Upgrade",
		Usage: `update · update now

<b>示例</b>
• <code>update</code>  检查有没有新版本
• <code>update now</code>  下载并安装，然后自动重启

<b>机制</b>
• 查询 GitHub 上 PaperValet 的最新 Release
• 下载与本机系统和架构匹配的安装包，只替换程序本身
• 先写到临时文件再原子替换，下载失败不会弄坏现有程序
• 配置、登录和插件都不受影响`,
		UsageEN: `update · update now

<b>Examples</b>
• <code>update</code>  check for a new version
• <code>update now</code>  download, install and restart

<b>How it works</b>
• Queries the latest PaperValet release on GitHub
• Downloads the bundle for this OS/arch and replaces only the binary
• Writes to a temp file then swaps atomically; a failed download leaves the old binary intact
• Config, login and plugins are untouched`,
		Plugin:    p.Name(),
		Category:  "admin",
		OwnerOnly: true,
		Handler:   p.handleUpdate,
	})
}

func (p *UpdatePlugin) Start(_ context.Context) error { return nil }
func (p *UpdatePlugin) Stop(_ context.Context) error  { return nil }

type release struct {
	Tag    string `json:"tag_name"`
	Assets []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func fetchRelease(ctx context.Context) (*release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseAPI, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var r release
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	return &r, nil
}

func (p *UpdatePlugin) handleUpdate(ctx *interfaces.CommandContext) error {
	_ = ctx.Edit("⏳ …")
	rel, err := fetchRelease(ctx.Context())
	if err != nil {
		return ctx.Edit(ctx.Tlocal("❌ 查询新版本失败: "+err.Error(), "❌ Version check failed: "+err.Error()))
	}
	latest := strings.TrimPrefix(rel.Tag, "v")
	if latest == p.version {
		return ctx.Edit(ctx.Tlocal(fmt.Sprintf("✅ 已经是最新版 %s", p.version), fmt.Sprintf("✅ Up to date (%s)", p.version)))
	}
	if strings.ToLower(ctx.GetArg(0)) != "now" {
		return ctx.Edit(ctx.Tlocal(
			fmt.Sprintf("🔔 有新版本 %s（当前 %s）\n发 <code>update now</code> 安装并重启", latest, p.version),
			fmt.Sprintf("🔔 New version %s (current %s)\nSend <code>update now</code> to install and restart", latest, p.version)))
	}

	want := fmt.Sprintf("papervalet-%s-%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	var url string
	for _, a := range rel.Assets {
		if a.Name == want {
			url = a.URL
		}
	}
	if url == "" {
		return ctx.Edit(ctx.Tlocal("❌ 这个版本没有适合本机的安装包: "+want, "❌ No build for this platform: "+want))
	}

	_ = ctx.Edit(ctx.Tlocal(fmt.Sprintf("⬇️ 下载 %s…", latest), fmt.Sprintf("⬇️ Downloading %s…", latest)))
	if err := replaceBinary(ctx.Context(), url); err != nil {
		return ctx.Edit(ctx.Tlocal("❌ 更新失败: "+err.Error(), "❌ Update failed: "+err.Error()))
	}
	if p.restart == nil {
		return ctx.Edit(ctx.Tlocal("✅ 已更新，发 restart 生效", "✅ Updated; send restart to apply"))
	}
	return p.restart(ctx)
}

// replaceBinary downloads the bundle and swaps bin/papervalet atomically.
func replaceBinary(ctx context.Context, url string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, _ = filepath.EvalSymlinks(exe)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("binary not found in bundle")
		}
		if err != nil {
			return err
		}
		if filepath.Base(hdr.Name) != "papervalet" || hdr.Typeflag != tar.TypeReg {
			continue
		}
		tmp := exe + ".new"
		f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, io.LimitReader(tr, 200<<20)); err != nil {
			f.Close()
			os.Remove(tmp)
			return err
		}
		if err := f.Close(); err != nil {
			os.Remove(tmp)
			return err
		}
		return os.Rename(tmp, exe)
	}
}
