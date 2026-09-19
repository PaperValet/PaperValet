package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/internal/config"
	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// BackupPlugin provides config/session backup and restore.
type BackupPlugin struct {
	backupDir  string
	configPath string
	config     *config.Config
}

func NewBackup() *BackupPlugin { return &BackupPlugin{backupDir: "backups"} }

// SetConfig wires the active configuration so backups include configured paths
// instead of silently looking only in the process working directory.
func (p *BackupPlugin) SetConfig(path string, cfg *config.Config) {
	p.configPath = path
	p.config = cfg
}

func (p *BackupPlugin) Name() string        { return "backup" }
func (p *BackupPlugin) Description() string { return "备份与恢复管理" }

func (p *BackupPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&interfaces.Command{
		Name: "backup", Description: "备份管理",
		Usage:  "backup [名称] | backup list|restore <名称> [--force]|clean|info",
		Plugin: p.Name(), Category: "admin", OwnerOnly: true, Handler: p.handleBackup,
	})
}

func (p *BackupPlugin) Start(_ context.Context) error {
	return os.MkdirAll(p.backupDir, 0o700)
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

type backupSet struct {
	name    string
	path    string
	size    int64
	files   int
	modTime time.Time
}

func validBackupName(name string) bool {
	return name != "" && name != "." && name != ".." && filepath.Base(name) == name &&
		!strings.ContainsAny(name, `/\`) && !strings.HasPrefix(name, ".")
}

func (p *BackupPlugin) backupPath(name string) (string, error) {
	if !validBackupName(name) {
		return "", fmt.Errorf("invalid backup name")
	}
	root, err := filepath.Abs(p.backupDir)
	if err != nil {
		return "", err
	}
	path := filepath.Join(root, name)
	if filepath.Dir(path) != root {
		return "", fmt.Errorf("invalid backup path")
	}
	return path, nil
}

func (p *BackupPlugin) getBackups() []backupSet {
	entries, err := os.ReadDir(p.backupDir)
	if err != nil {
		return nil
	}
	var backups []backupSet
	for _, e := range entries {
		if !e.IsDir() || !validBackupName(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		dir := filepath.Join(p.backupDir, e.Name())
		var size int64
		var files int
		_ = filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if fi.Mode().IsRegular() {
				size += fi.Size()
				files++
			}
			return nil
		})
		backups = append(backups, backupSet{name: e.Name(), path: dir, size: size, files: files, modTime: info.ModTime()})
	}
	sort.Slice(backups, func(i, j int) bool { return backups[i].modTime.After(backups[j].modTime) })
	return backups
}

func (p *BackupPlugin) showStatus(ctx *interfaces.CommandContext) error {
	backups := p.getBackups()
	var totalSize int64
	var outdated int
	for _, b := range backups {
		totalSize += b.size
		if time.Since(b.modTime) > 7*24*time.Hour {
			outdated++
		}
	}
	return ctx.Edit(fmt.Sprintf(`📦 <b>备份管理状态</b>

📁 备份目录: <code>%s</code>
📊 备份数量: <b>%d</b>
💾 总大小: <b>%s</b>
⏰ 过期备份: <b>%d</b> (7天以上)`,
		p.backupDir, len(backups), formatBytes(totalSize), outdated))
}

func sqliteCompanionFiles(path string) []string {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".db") || strings.HasSuffix(lower, ".sqlite") {
		return []string{path, path + "-wal", path + "-shm", path + "-journal"}
	}
	return []string{path}
}

func (p *BackupPlugin) sourceFiles() []string {
	seen := map[string]bool{}
	var files []string
	add := func(path string) {
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		files = append(files, path)
	}
	if p.configPath != "" {
		add(p.configPath)
	}
	if p.config != nil {
		for _, path := range sqliteCompanionFiles(p.config.Telegram.Database) {
			add(path)
		}
		add(p.config.Telegram.SessionFile)
	}
	// Compatibility fallback when constructed without application config.
	for _, path := range []string{"config.json", "session.json", "sessions.db", "sessions.db-wal", "sessions.db-shm"} {
		add(path)
	}
	return files
}

func (p *BackupPlugin) doBackup(ctx *interfaces.CommandContext, name string) error {
	if name == "" {
		name = "backup-" + time.Now().Format("20060102-150405")
	}
	dest, err := p.backupPath(name)
	if err != nil {
		return ctx.Edit("❌ 无效备份名称（仅允许目录名，不能包含路径）")
	}
	if _, err := os.Stat(dest); err == nil {
		return ctx.Edit(fmt.Sprintf("❌ 备份已存在: <code>%s</code>", name))
	}

	var copied, missing []string
	for _, src := range p.sourceFiles() {
		info, err := os.Lstat(src)
		if err != nil {
			missing = append(missing, filepath.Base(src))
			continue
		}
		if !info.Mode().IsRegular() {
			missing = append(missing, filepath.Base(src)+" (非普通文件)")
			continue
		}
		if _, err := os.Stat(filepath.Join(dest, filepath.Base(src))); err == nil {
			// Name collision across configured/fallback locations; preserve all files explicitly.
			continue
		}
		data, err := os.ReadFile(src)
		if err != nil {
			missing = append(missing, filepath.Base(src))
			continue
		}
		if err := os.MkdirAll(dest, 0o700); err != nil {
			return ctx.Edit(fmt.Sprintf("❌ 创建备份目录失败: %v", err))
		}
		if err := os.WriteFile(filepath.Join(dest, filepath.Base(src)), data, 0o600); err != nil {
			missing = append(missing, filepath.Base(src))
			continue
		}
		copied = append(copied, filepath.Base(src))
	}
	if len(copied) == 0 {
		_ = os.RemoveAll(dest)
		return ctx.Edit("❌ 备份失败：没有文件写入成功")
	}
	status := fmt.Sprintf(`✅ <b>备份完成</b>

📁 备份路径: <code>%s</code>
📄 已备份: <b>%d</b> 个文件
<code>%s</code>`, dest, len(copied), strings.Join(copied, "\n"))
	if len(missing) > 0 {
		status += fmt.Sprintf("\n⚠️ 缺失或失败: <code>%s</code>", strings.Join(missing, "</code>, <code>"))
	}
	return ctx.Edit(status)
}

func (p *BackupPlugin) listBackups(ctx *interfaces.CommandContext) error {
	backups := p.getBackups()
	if len(backups) == 0 {
		return ctx.Edit("📦 暂无备份\n\n使用 <code>backup</code> 创建第一个备份")
	}
	var b strings.Builder
	b.WriteString("📦 <b>备份列表</b>\n\n")
	for i, bak := range backups {
		b.WriteString(fmt.Sprintf("%d. <code>%s</code>\n   📅 %s | 💾 %s (%d 文件) | ⏰ %s前\n",
			i+1, bak.name, bak.modTime.Format("01-02 15:04"), formatBytes(bak.size), bak.files, formatDuration(time.Since(bak.modTime).Truncate(time.Second))))
	}
	return ctx.Edit(b.String())
}

func (p *BackupPlugin) doRestore(ctx *interfaces.CommandContext, name string, force bool) error {
	backups := p.getBackups()
	var target *backupSet
	for i := range backups {
		if backups[i].name == name {
			target = &backups[i]
			break
		}
	}
	if target == nil {
		return ctx.Edit(fmt.Sprintf("❌ 未找到备份: %s", name))
	}
	if !force {
		return ctx.Edit(fmt.Sprintf("⚠️ 恢复将覆盖现有文件。确认: <code>backup restore %s --force</code>", target.name))
	}
	entries, err := os.ReadDir(target.path)
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 读取备份失败: %v", err))
	}
	var restored, failed []string
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		src := filepath.Join(target.path, e.Name())
		data, err := os.ReadFile(src)
		if err != nil {
			failed = append(failed, e.Name())
			continue
		}
		if err := os.WriteFile(e.Name(), data, 0o600); err != nil {
			failed = append(failed, e.Name())
		} else {
			restored = append(restored, e.Name())
		}
	}
	if len(failed) > 0 {
		return ctx.Edit(fmt.Sprintf("⚠️ 恢复部分完成：成功 %d，失败 %d\n<code>%s</code>", len(restored), len(failed), strings.Join(failed, "\n")))
	}
	return ctx.Edit(fmt.Sprintf("✅ 恢复完成: %d 个文件\n💡 建议重启使配置生效", len(restored)))
}

func (p *BackupPlugin) cleanBackups(ctx *interfaces.CommandContext) error {
	removed := 0
	for _, b := range p.getBackups() {
		if time.Since(b.modTime) > 7*24*time.Hour {
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
		return fmt.Sprintf("%d时%d分", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%d天", int(d.Hours())/24)
}
