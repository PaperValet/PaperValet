package builtin

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

const backupStamp = "20060102-150405"

// BackupPlugin packs bot configuration (config + plugin data) and ships it
// to Saved Messages; restore reads an archive from a replied message.
type BackupPlugin struct {
	configPath string
	dataDirs   []string
}

func NewBackup() *BackupPlugin {
	return &BackupPlugin{dataDirs: []string{"data", "plugins"}}
}

// SetConfig wires the active config path into backups.
func (p *BackupPlugin) SetConfig(path string) { p.configPath = path }

func (p *BackupPlugin) Name() string { return "backup" }
func (p *BackupPlugin) Description() string {
	return "备份/恢复机器人配置（发到收藏夹）"
}
func (p *BackupPlugin) DescEN() string {
	return "Back up / restore bot configuration (to Saved Messages)"
}

func (p *BackupPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "backup",
		Description: "打包配置发到收藏夹；回复那个文件 backup restore 即可恢复",
		DescEN:      "Pack config to Saved Messages; reply to it with backup restore",
		Usage:       "backup | backup restore（回复备份文件）",
		UsageEN:     "backup | backup restore (reply to the archive)",
		Plugin:      p.Name(),
		Category:    "admin",
		OwnerOnly:   true,
		Handler:     p.handleBackup,
	})
}

func (p *BackupPlugin) Start(_ context.Context) error { return nil }
func (p *BackupPlugin) Stop(_ context.Context) error  { return nil }

func (p *BackupPlugin) handleBackup(ctx *interfaces.CommandContext) error {
	switch strings.ToLower(ctx.GetArg(0)) {
	case "", "create", "new":
		return p.doBackup(ctx)
	case "restore", "recover":
		return p.doRestore(ctx)
	default:
		return ctx.Edit(ctx.Tlocal(
			"用法: <code>backup</code> 打包配置发到收藏夹；回复备份文件发 <code>backup restore</code> 恢复",
			"Usage: <code>backup</code> packs config to Saved Messages; reply to the archive with <code>backup restore</code>",
		))
	}
}

// backupEntry is one file inside the archive, relative path included.
type backupEntry struct {
	path string
	data []byte
}

func (p *BackupPlugin) collect() ([]backupEntry, error) {
	var out []backupEntry
	seen := map[string]bool{}
	addFile := func(rel, abs string) {
		if seen[rel] {
			return
		}
		info, err := os.Lstat(abs)
		if err != nil || !info.Mode().IsRegular() {
			return
		}
		if info.Size() > 2<<20 { // 2 MB cap per file
			return
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return
		}
		seen[rel] = true
		out = append(out, backupEntry{path: rel, data: data})
	}

	if p.configPath != "" {
		addFile(filepath.Base(p.configPath), p.configPath)
	}
	for _, dir := range p.dataDirs {
		_ = filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
			if err != nil || !fi.Mode().IsRegular() {
				return nil
			}
			if fi.Size() > 2<<20 {
				return nil
			}
			rel, err := filepath.Rel(".", path)
			if err != nil || strings.HasPrefix(rel, "..") {
				return nil
			}
			// Exclude caches and the session/peers state: a backup must
			// restore onto a fresh login, and peers re-learn themselves.
			if rel == "data/peers.json" || strings.HasSuffix(rel, "-wal") || strings.HasSuffix(rel, "-shm") {
				return nil
			}
			addFile(rel, path)
			return nil
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("nothing to back up")
	}
	return out, nil
}

func (p *BackupPlugin) doBackup(ctx *interfaces.CommandContext) error {
	entries, err := p.collect()
	if err != nil {
		return ctx.Edit("❌ " + err.Error())
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		hdr := &tar.Header{Name: e.path, Mode: 0o600, Size: int64(len(e.data)), ModTime: time.Now()}
		if err := tw.WriteHeader(hdr); err != nil {
			return ctx.Edit("❌ " + err.Error())
		}
		if _, err := tw.Write(e.data); err != nil {
			return ctx.Edit("❌ " + err.Error())
		}
	}
	if err := tw.Close(); err != nil {
		return ctx.Edit("❌ " + err.Error())
	}
	if err := gz.Close(); err != nil {
		return ctx.Edit("❌ " + err.Error())
	}

	path := filepath.Join(os.TempDir(), fmt.Sprintf("papervalet-backup-%s.tar.gz", time.Now().Format(backupStamp)))
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		return ctx.Edit("❌ " + err.Error())
	}
	defer os.Remove(path)

	if err := ctx.Media.SendFile(ctx.Context(), 0, path,
		fmt.Sprintf("PaperValet backup · %s · %d files", time.Now().Format("2006-01-02 15:04"), len(entries)), 0); err != nil {
		return ctx.Edit(ctx.Tlocal("❌ 发送失败: "+err.Error(), "❌ Send failed: "+err.Error()))
	}
	return ctx.Edit(ctx.Tlocal(
		fmt.Sprintf("✅ 备份已发到收藏夹（%d 个文件）。恢复时回复它发 backup restore", len(entries)),
		fmt.Sprintf("✅ Backup sent to Saved Messages (%d files). Reply to it with backup restore to recover", len(entries))))
}

// restoreTarget fetches the replied document and its bytes.
func (p *BackupPlugin) restoreTarget(ctx *interfaces.CommandContext) (string, []byte, error) {
	if !ctx.Message.IsReply {
		return "", nil, fmt.Errorf("%s", ctx.Tlocal("回复那条备份消息再发 backup restore", "Reply to the backup message first"))
	}
	msgs, err := ctx.API.MessagesGetMessages(ctx.Context(),
		[]tg.InputMessageClass{&tg.InputMessageID{ID: ctx.Message.ReplyToID}})
	if err != nil {
		return "", nil, err
	}
	for _, m := range historyMessages(msgs) {
		msg, ok := m.(*tg.Message)
		if !ok || msg.ID != ctx.Message.ReplyToID {
			continue
		}
		doc, ok := msg.Media.(*tg.MessageMediaDocument)
		if !ok {
			return "", nil, fmt.Errorf("%s", ctx.Tlocal("回复的消息没有附件", "The replied message has no attachment"))
		}
		d, ok := doc.Document.(*tg.Document)
		if !ok {
			return "", nil, fmt.Errorf("%s", ctx.Tlocal("回复的不是文件", "Not a file"))
		}
		var name string
		for _, a := range d.Attributes {
			if f, ok := a.(*tg.DocumentAttributeFilename); ok {
				name = f.FileName
			}
		}
		// Stream the document through gzip+tar straight into files.
		loc := d.AsInputDocumentFileLocation("")
		data, err := downloadAll(ctx, *loc, d.Size)
		if err != nil {
			return "", nil, err
		}
		return name, data, nil
	}
	return "", nil, fmt.Errorf("%s", ctx.Tlocal("找不到那条消息", "Message not found"))
}

func (p *BackupPlugin) doRestore(ctx *interfaces.CommandContext) error {
	_, raw, err := p.restoreTarget(ctx)
	if err != nil {
		return ctx.Edit("❌ " + err.Error())
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return ctx.Edit(ctx.Tlocal("❌ 不是有效的备份文件", "❌ Not a valid backup archive"))
	}
	tr := tar.NewReader(gz)

	var restored []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ctx.Edit(ctx.Tlocal("❌ 备份损坏: "+err.Error(), "❌ Corrupt archive: "+err.Error()))
		}
		clean := filepath.Clean(hdr.Name)
		if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			continue
		}
		if !isRestoreable(clean) {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(clean), 0o700); err != nil {
			continue
		}
		if err := os.WriteFile(clean, data, 0o600); err == nil {
			restored = append(restored, clean)
		}
	}
	if len(restored) == 0 {
		return ctx.Edit(ctx.Tlocal("❌ 没有恢复任何文件", "❌ Nothing restored"))
	}

	msg := ctx.Tlocal(
		fmt.Sprintf("✅ 恢复了 %d 个文件:\n<code>%s</code>\n发 restart 生效", len(restored), strings.Join(restored, "\n")),
		fmt.Sprintf("✅ Restored %d files:\n<code>%s</code>\nSend restart to apply", len(restored), strings.Join(restored, "\n")))
	return ctx.Edit(msg)
}

// isRestoreable whitelists what a backup may overwrite. Config and plugin
// data are safe; binaries, session and db state never come back this way.
func isRestoreable(rel string) bool {
	if rel == "config.json" {
		return true
	}
	return strings.HasPrefix(rel, "data/") || strings.HasPrefix(rel, "plugins/")
}

// downloadAll pulls a whole document in one pass.
func downloadAll(ctx *interfaces.CommandContext, loc tg.InputDocumentFileLocation, size int64) ([]byte, error) {
	var buf bytes.Buffer
	limit := int64(512 * 1024)
	for offset := int64(0); offset < size || size == 0; offset += limit {
		req := &tg.UploadGetFileRequest{
			Location: &loc,
			Offset:   offset,
			Limit:    int(limit),
		}
		res, err := ctx.API.UploadGetFile(ctx.Context(), req)
		if err != nil {
			return nil, err
		}
		f, ok := res.(*tg.UploadFile)
		if !ok || len(f.Bytes) == 0 {
			break
		}
		buf.Write(f.Bytes)
		if int64(len(f.Bytes)) < limit {
			break
		}
	}
	if buf.Len() == 0 {
		return nil, fmt.Errorf("%s", ctx.Tlocal("下载失败", "download failed"))
	}
	return buf.Bytes(), nil
}
