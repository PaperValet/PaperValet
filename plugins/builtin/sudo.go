package builtin

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// SudoPlugin executes commands with sudo (owner only).
type SudoPlugin struct{}

func NewSudo() *SudoPlugin { return &SudoPlugin{} }

func (p *SudoPlugin) Name() string        { return "sudo" }
func (p *SudoPlugin) Description() string { return "以 root 权限执行命令" }

func (p *SudoPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "sudo",
		Description: "以 root 权限执行命令",
		Usage:       "sudo <命令> [参数...]",
		Plugin:      p.Name(),
		Category:    "admin",
		OwnerOnly:   true,
		Handler:     p.handleSudo,
	})
}

func (p *SudoPlugin) Start(_ context.Context) error { return nil }
func (p *SudoPlugin) Stop(_ context.Context) error  { return nil }

func (p *SudoPlugin) handleSudo(ctx *interfaces.CommandContext) error {
	args := ctx.Args
	if len(args) == 0 {
		return ctx.Edit("用法: sudo <命令> [参数...]")
	}

	cmd := args[0]
	cmdArgs := args[1:]

	_ = ctx.Edit("⏳ 以 root 权限执行中...")

	ctxWithTimeout, cancel := context.WithTimeout(ctx.Context(), 30*time.Second)
	defer cancel()

	// Prepend sudo
	sudoArgs := append([]string{cmd}, cmdArgs...)
	execCmd := exec.CommandContext(ctxWithTimeout, "sudo", sudoArgs...)
	output, err := execCmd.CombinedOutput()

	result := string(output)
	if len(result) > 4000 {
		result = result[:4000] + "\n... (输出过长，已截断)"
	}

	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 执行失败: %v\n\n输出:\n<pre>%s</pre>", err, result))
	}

	if result == "" {
		result = "(无输出)"
	}

	return ctx.Edit(fmt.Sprintf("✅ 执行完成 (root)\n\n<pre>%s</pre>", result))
}