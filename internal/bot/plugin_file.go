package bot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/internal/media"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// Plugin files the owner sends to the bot: download to a scratch dir,
// confirm, install. One pending file at a time, keyed by the source
// message id (carried through the confirm button's callback data).

const (
	pluginFileDir   = "data/plugin-files"
	pluginFileLimit = 100 << 20
)

// onPluginFile downloads the .so the owner sent and offers to install it.
func (s *Service) onPluginFile(from int64, msgID int, d *tg.Document) {
	if !s.isOwner(from) {
		return
	}
	ctx, cancel := opCtx()
	defer cancel()
	if s.cat() == nil {
		return
	}
	if d.Size > pluginFileLimit {
		s.sendView(ctx, &View{Text: "❌ " + s.tl(
			fmt.Sprintf("文件太大（%s），上限 %s", media.FormatSize(d.Size), media.FormatSize(pluginFileLimit)),
			fmt.Sprintf("File too large (%s), limit %s", media.FormatSize(d.Size), media.FormatSize(pluginFileLimit)))})
		return
	}
	name := ""
	for _, attr := range d.Attributes {
		if fn, ok := attr.(*tg.DocumentAttributeFilename); ok {
			name = fn.FileName
		}
	}
	v := &View{
		Text: "📦 " + s.tl("收到插件文件", "Plugin file received") + "  " + plugin.Code(plugin.Escape(name)) +
			"\n" + s.tl("安装后会按插件内部的名字存放。确定要装吗？", "It installs under the plugin's own name. Install it?"),
		Buttons: [][]plugin.Button{plugin.Row(
			plugin.Btn(s.tl("取消", "Cancel"), "m"),
			plugin.Btn(s.tl("📥 安装", "📥 Install"), fmt.Sprintf("y:if:%d", msgID)).Success(),
		)},
	}
	if _, err := s.send(ctx, from, v); err != nil {
		s.log.Warn("plugin file offer", "error", err)
		_ = ctx
		return
	}
	// Download after showing the offer, so the tap never waits on the wire.
	path, err := s.downloadPluginFile(ctx, d)
	if err != nil {
		s.log.Warn("plugin file download", "error", err)
		return
	}
	s.mu.Lock()
	s.pluginFiles[msgID] = path
	s.mu.Unlock()
}

// downloadPluginFile pulls the document into data/plugin-files/<id>.so.
func (s *Service) downloadPluginFile(ctx context.Context, d *tg.Document) (string, error) {
	if err := os.MkdirAll(pluginFileDir, 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(pluginFileDir, fmt.Sprintf("%d.so", d.ID))
	if fi, err := os.Stat(dst); err == nil && fi.Size() == d.Size {
		return dst, nil // already downloaded
	}
	if !s.Ready() {
		return "", plugin.ErrBotNotReady
	}
	loc := &tg.InputDocumentFileLocation{
		ID:            d.ID,
		AccessHash:    d.AccessHash,
		FileReference: d.FileReference,
	}
	f, err := os.CreateTemp(pluginFileDir, ".part-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	const chunk = 128 * 1024
	for off := int64(0); ; off += chunk {
		res, err := s.api.UploadGetFile(ctx, &tg.UploadGetFileRequest{
			Location: loc,
			Offset:   off,
			Limit:    chunk,
		})
		if err != nil {
			f.Close()
			return "", err
		}
		file, ok := res.(*tg.UploadFile)
		if !ok {
			f.Close()
			return "", fmt.Errorf("unexpected upload result %T", res)
		}
		if _, err := f.Write(file.GetBytes()); err != nil {
			f.Close()
			return "", err
		}
		if len(file.GetBytes()) < chunk {
			break
		}
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(f.Name(), dst); err != nil {
		return "", err
	}
	return dst, nil
}

// installPluginFile runs the confirmed install for a sent file.
func (s *Service) installPluginFile(ctx context.Context, msgID int) (*View, string, error) {
	c := s.cat()
	if c == nil {
		return s.menuView(), "", nil
	}
	s.mu.RLock()
	path := s.pluginFiles[msgID]
	s.mu.RUnlock()
	if path == "" {
		return s.menuView(), s.tl("文件还没下完，稍等再点", "Still downloading, try again shortly"), nil
	}
	if !s.busy.CompareAndSwap(false, true) {
		return nil, "!" + s.tl("上一个操作还没完成，稍等一下", "Another operation is still running"), nil
	}
	defer s.busy.Store(false)
	pr := s.startProgress(ctx, target{chatID: s.owner()}, s.tl("正在安装插件文件", "Installing plugin file"), 0)
	err := c.InstallFile(ctx, path)
	pr.finish()
	s.mu.Lock()
	delete(s.pluginFiles, msgID)
	s.mu.Unlock()
	_ = os.Remove(path)
	zh, en := c.Explain(err)
	reason := s.tl(zh, en)
	if err == nil {
		v := s.managerView(ctx, 0, false)
		v.Text = "✅ " + s.tl("已安装", "Installed") + "\n\n" + v.Text
		return v, "", nil
	}
	return &View{
		Text: "❌ **" + s.tl("安装失败", "Install failed") + "**\n" + quote([]string{plugin.Escape(reason)}),
		Buttons: [][]plugin.Button{plugin.Row(
			s.back("m"),
			plugin.Btn(s.tl("🔁 重试", "🔁 Retry"), fmt.Sprintf("y:if:%d", msgID)).Primary(),
		)},
	}, "", nil
}
