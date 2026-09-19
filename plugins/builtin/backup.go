package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// BackupPlugin provides config/session backup and restore.
type BackupPlugin struct {
	backupDir string
}

func NewBackup() *BackupPlugin {
	return &BackupPlugin{backupDir: "backups"}
}

func (p *BackupPlugin) Name() string        { return "backup" }
func (p *BackupPlugin) Description() string { return "备份与恢复管理" }

func (p *BackupPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "backup",
		Description: "备份管理",
		Usage:       "backup [名称] | backup list|restore <名称> [--force]|clean|info",
		Plugin:      p.Name(),
		Category:    "admin",
		OwnerOnly:   true,
		Handler:     p.handleBackup,
	})
}

func (p *BackupPlugin) Start(_ context.Context) error {
	if err := os.MkdirAll(p.backupDir, 0o755); err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}
	return nil
}

func (p *BackupPlugin) Stop(_ context.Context) error { return nil }

func (p *BackupPlugin) handleBackup(ctx *interfaces.CommandContext) error {
	args := ctx.Args
	if len(args) == 0 {
		return p.doBackup(ctx, "")
	}

	switch args[0] {
	case "list", "ls":
		return p.listBackups(ctx)
	case "restore", "recover":
		if len(args) < 2 {
			return ctx.Edit("用法: backup restore <名称> [--force]")
		}
		force := false
		for _, a := range args[2:] {
			if a == "--force" || a == "-f" {
				force = true
			}
		}
		return p.doRestore(ctx, args[1], force)
	case "clean", "prune":
		return p.cleanBackups(ctx)
	case "info", "status":
		return p.showStatus(ctx)
	case "backup", "create", "new":
		name := ""
		if len(args) > 1 {
			name = strings.Join(args[1:], "-")
		}
		return p.doBackup(ctx, name)
	default:
		// Single unknown token is treated as a backup name.
		if len(args) == 1 {
			return p.doBackup(ctx, args[0])
		}
		return ctx.Edit(fmt.Sprintf("未知子命令: %s\n\n%s", args[0], p.usage()))
	}
}

func (p *BackupPlugin) usage() string {
	return `📦 <b>备份管理</b>

<b>用法:</b>
• <code>backup</code> — 创建时间戳备份
• <code>backup &lt;名称&gt;</code> — 创建命名备份
• <code>backup list</code> — 列出所有备份
• <code>backup restore &lt;名称&gt; --force</code> — 恢复备份
• <code>backup clean</code> — 清理 7 天前的备份
• <code>backup info</code> — 查看备份统计`
}

// backupSet is one named backup directory under backups/.
type backupSet struct {
	name    string
	path    string
	size    int64
	files   int
	modTime time.Time
}

func (p *BackupPlugin) getBackups() []backupSet {
	entries, err := os.ReadDir(p.backupDir)
	if err != nil {
		return nil
	}
	var backups []backupSet
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		dir := filepath.Join(p.backupDir, e.Name())
		var size int64
		var files int
		_ = filepath.Walk(dir, func(_ string, fi os.FileInfo, err error) error {
			if err == nil && !fi.IsDir() {
				size += fi.Size()
				files++
			}
			return nil
		})
		backups = append(backups, backupSet{
			name:    e.Name(),
			path:    dir,
			size:    size,
			files:   files,
			modTime: info.ModTime(),
		})
	}
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].modTime.After(backups[j].modTime)
	})
	return backups
}

func (p *BackupPlugin) showStatus(ctx *interfaces.CommandContext) error {
	backups := p.getBackups()
	var totalSize int64
	var outdated int
	now := time.Now()
	for _, b := range backups {
		totalSize += b.size
		if now.Sub(b.modTime) > 7*24*time.Hour {
			outdated++
		}
	}

	return ctx.Edit(fmt.Sprintf(`📦 <b>备份管理状态</b>

📁 备份目录: <code>%s</code>
📊 备份数量: <b>%d</b>
💾 总大小: <b>%s</b>
⏰ 过期备份: <b>%d</b> (7天以上)

<b>常用命令:</b>
<code>backup</code> — 快速备份
<code>backup list</code> — 查看列表
<code>backup restore &lt;名称&gt; --force</code> — 恢复`,
		p.backupDir, len(backups), formatBytes(totalSize), outdated))
}

// backupPatterns are the file globs copied into each backup set.
var backupPatterns = []string{"config.json", "config.yaml", "*.db", "*.session", "*.key", "data/*.json"}

func (p *BackupPlugin) doBackup(ctx *interfaces.CommandContext, name string) error {
	if name == "" {
		name = "backup-" + time.Now().Format("20060102-150405")
	}
	dest := filepath.Join(p.backupDir, name)

	var files []string
	seen := make(map[string]bool)
	for _, pattern := range backupPatterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, m := range matches {
			if !seen[m] {
				seen[m] = true
				files = append(files, m)
			}
		}
	}
	if len(files) == 0 {
		return ctx.Edit("❌ 未找到可备份的文件（config.json, *.db 等）")
	}

	_ = ctx.Edit(fmt.Sprintf("⏳ 正在备份 %d 个文件到 <code>%s</code>...", len(files), dest))

	var backedUp []string
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		out := filepath.Join(dest, f)
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			continue
		}
		if err := os.WriteFile(out, data, 0o600); err != nil {
			continue
		}
		backedUp = append(backedUp, f)
	}

	if len(backedUp) == 0 {
		return ctx.Edit("❌ 备份失败：没有文件写入成功")
	}
	return ctx.Edit(fmt.Sprintf(`✅ <b>备份完成</b>

📁 备份路径: <code>%s</code>
📄 已备份: <b>%d</b> / %d 个文件
<code>%s</code>`,
		dest, len(backedUp), len(files), strings.Join(backedUp, "\n")))
}

func (p *BackupPlugin) listBackups(ctx *interfaces.CommandContext) error {
	backups := p.getBackups()
	if len(backups) == 0 {
		return ctx.Edit("📦 暂无备份\n\n使用 <code>backup</code> 创建第一个备份")
	}

	var b strings.Builder
	b.WriteString("📦 <b>备份列表</b>\n\n")
	for i, bak := range backups {
		age := time.Since(bak.modTime).Truncate(time.Second)
		b.WriteString(fmt.Sprintf("%d. <code>%s</code>\n", i+1, bak.name))
		b.WriteString(fmt.Sprintf("   📅 %s | 💾 %s (%d 文件) | ⏰ %s前\n",
			bak.modTime.Format("01-02 15:04"),
			formatBytes(bak.size),
			bak.files,
			formatDuration(age)))
	}
	return ctx.Edit(b.String())
}

func (p *BackupPlugin) doRestore(ctx *interfaces.CommandContext, name string, force bool) error {
	backups := p.getBackups()

	// Find by exact/prefix name or 1-based index.
	var target *backupSet
	for i := range backups {
		if backups[i].name == name || strings.HasPrefix(backups[i].name, name) {
			target = &backups[i]
			break
		}
	}
	if target == nil {
		var idx int
		if n, err := fmt.Sscanf(name, "%d", &idx); n == 1 && err == nil && idx > 0 && idx <= len(backups) {
			target = &backups[idx-1]
		}
	}
	if target == nil {
		return ctx.Edit(fmt.Sprintf("❌ 未找到备份: %s\n使用 <code>backup list</code> 查看可用备份", name))
	}

	if !force {
		return ctx.Edit(fmt.Sprintf("⚠️ <b>恢复确认</b>\n\n备份: <code>%s</code>\n时间: %s\n文件: %d 个\n\n‼️ 恢复将覆盖现有文件，确认请使用 <code>backup restore %s --force</code>",
			target.name, target.modTime.Format("2006-01-02 15:04:05"), target.files, target.name))
	}

	var restored, failed []string
	err := filepath.Walk(target.path, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(target.path, path)
		if err != nil {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			failed = append(failed, rel)
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(rel), 0o755); err != nil && filepath.Dir(rel) != "." {
			failed = append(failed, rel)
			return nil
		}
		if err := os.WriteFile(rel, data, 0o600); err != nil {
			failed = append(failed, rel)
			return nil
		}
		restored = append(restored, rel)
		return nil
	})
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 恢复失败: %v", err))
	}

	out := fmt.Sprintf("✅ <b>恢复完成</b>\n\n备份: <code>%s</code>\n恢复: %d 个文件", target.name, len(restored))
	if len(failed) > 0 {
		out += fmt.Sprintf("\n失败: %d 个\n<code>%s</code>", len(failed), strings.Join(failed, "\n"))
	}
	out += "\n\n💡 建议重启使配置生效"
	return ctx.Edit(out)
}

func (p *BackupPlugin) cleanBackups(ctx *interfaces.CommandContext) error {
	backups := p.getBackups()
	now := time.Now()
	var removed int
	for _, b := range backups {
		if now.Sub(b.modTime) > 7*24*time.Hour {
			if err := os.RemoveAll(b.path); err == nil {
				removed++
			}
		}
	}
	return ctx.Edit(fmt.Sprintf("🧹 清理完成，已移除 %d 个过期备份", removed))
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d秒", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d分", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		return fmt.Sprintf("%d时%d分", h, m)
	}
	days := int(d.Hours()) / 24
	return fmt.Sprintf("%d天", days)
}
