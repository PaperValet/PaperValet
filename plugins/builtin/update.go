package builtin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// UpdatePlugin merges update and autofix: git sync + restart.
// Absorbs the old autofix plugin.
type UpdatePlugin struct {
	stateFile string
}

func NewUpdate() *UpdatePlugin {
	return &UpdatePlugin{stateFile: "data/update_state.json"}
}

func (p *UpdatePlugin) Name() string        { return "update" }
func (p *UpdatePlugin) Description() string { return "更新与一键修复（git 同步 + 重启）" }

func (p *UpdatePlugin) Init(_ context.Context, mgr plugin.Manager) error {
	cmds := []*interfaces.Command{
		{
			Name:        "update",
			Aliases:     []string{"upgrade"},
			Description: "从远程同步代码并重启",
			Usage:       "update [check]",
			Plugin:      p.Name(),
			Category:    "admin",
			OwnerOnly:   true,
			Handler:     p.handleUpdate,
		},
		{
			Name:        "autofix",
			Description: "一键修复：移除冲突插件 → 同步远程 → 重启",
			Usage:       "autofix",
			Plugin:      p.Name(),
			Category:    "admin",
			OwnerOnly:   true,
			Handler:     p.handleAutofix,
		},
	}
	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *UpdatePlugin) Start(_ context.Context) error { return nil }
func (p *UpdatePlugin) Stop(_ context.Context) error  { return nil }

func (p *UpdatePlugin) handleUpdate(ctx *interfaces.CommandContext) error {
	if ctx.ArgCount() > 0 && ctx.GetArg(0) == "check" {
		return p.checkUpdate(ctx)
	}
	return p.doUpdate(ctx)
}

func (p *UpdatePlugin) checkUpdate(ctx *interfaces.CommandContext) error {
	_ = ctx.Edit("⏳ 检查更新...")

	out, err := exec.Command("git", "fetch", "origin").CombinedOutput()
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ fetch 失败: %v\n%s", err, string(out)))
	}

	local, err := gitRevision("HEAD")
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 读取本地版本失败: %v", err))
	}
	remote, err := gitRevision("@{upstream}")
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 读取远程版本失败: %v", err))
	}
	if local == remote {
		return ctx.Edit("✅ 已是最新版本")
	}
	return ctx.Edit(fmt.Sprintf("🔔 有可用更新\n\n本地: <code>%s</code>\n远程: <code>%s</code>\n\n发送 <code>update</code> 应用更新", local, remote))
}

func (p *UpdatePlugin) doUpdate(ctx *interfaces.CommandContext) error {
	_ = ctx.Edit("⏳ 正在同步远程代码...")

	cmds := [][]string{
		{"git", "fetch", "origin"},
		{"git", "merge", "--ff-only", "@{upstream}"},
		{"go", "build", "./cmd/papervalet"},
	}
	for _, args := range cmds {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			return ctx.Edit(fmt.Sprintf("❌ %s 失败: %v\n%s", strings.Join(args, " "), err, string(out)))
		}
	}

	_ = ctx.Edit("✅ 代码已同步并构建，正在重启...")
	go func() {
		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()
	return nil
}

func (p *UpdatePlugin) handleAutofix(ctx *interfaces.CommandContext) error {
	_ = ctx.Edit("🔧 正在修复：移除冲突插件…")
	removed := p.removeCollidingPlugins()

	_ = ctx.Edit("🔧 正在同步远程代码…")
	cmds := [][]string{
		{"git", "fetch", "origin"},
		{"git", "merge", "--ff-only", "@{upstream}"},
		{"go", "build", "./cmd/papervalet"},
	}
	for _, args := range cmds {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			return ctx.Edit(fmt.Sprintf("❌ %s 失败: %v\n%s", strings.Join(args, " "), err, string(out)))
		}
	}

	_ = ctx.Edit(fmt.Sprintf("✅ 修复完成并构建，移除了 %d 个冲突插件，正在重启…", len(removed)))
	go func() {
		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()
	return nil
}

func (p *UpdatePlugin) removeCollidingPlugins() []string {
	pluginsDir := "plugins"
	builtins := map[string]bool{
		"core": true, "apt": true, "info": true, "alias": true,
		"exec": true, "sudo": true, "reload": true, "log": true,
		"prefix": true, "help": true, "status": true, "backup": true,
		"update": true, "dme": true, "lang": true,
	}

	var removed []string
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return removed
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".so")
		if builtins[name] {
			if err := os.Remove(filepath.Join(pluginsDir, e.Name())); err == nil {
				removed = append(removed, name)
			}
		}
	}
	return removed
}

func gitRevision(ref string) (string, error) {
	out, err := exec.Command("git", "rev-parse", "--short=8", ref).Output()
	return strings.TrimSpace(string(out)), err
}
