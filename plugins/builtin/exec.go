package builtin

import (
	"context"
	"fmt"
	"html"
	"os/exec"
	"time"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// ExecPlugin executes system commands (owner only).
// The old separate sudo plugin (run-as-root) is merged here via the --sudo flag.
type ExecPlugin struct{}

func NewExec() *ExecPlugin { return &ExecPlugin{} }

func (p *ExecPlugin) Name() string        { return "exec" }
func (p *ExecPlugin) Description() string { return "执行系统命令" }

func (p *ExecPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "exec",
		Aliases:     []string{"sh", "shell", "cmd"},
		Description: "执行系统命令（--sudo 以 root 运行）",
		Usage:       "exec <命令> [参数...]",
		Plugin:      p.Name(),
		Category:    "admin",
		OwnerOnly:   true,
		Handler:     p.handleExec,
	})
}

func (p *ExecPlugin) Start(_ context.Context) error { return nil }
func (p *ExecPlugin) Stop(_ context.Context) error  { return nil }

func (p *ExecPlugin) handleExec(ctx *interfaces.CommandContext) error {
	args := ctx.Args
	if len(args) == 0 {
		return ctx.Edit("用法: exec <命令> [参数...]")
	}

	asRoot := false
	if args[0] == "--sudo" {
		asRoot = true
		args = args[1:]
	}
	if len(args) == 0 {
		return ctx.Edit("用法: exec <命令> [参数...]")
	}

	_ = ctx.Edit("⏳ 执行中...")

	ctxWithTimeout, cancel := context.WithTimeout(ctx.Context(), 30*time.Second)
	defer cancel()

	cmdName := args[0]
	cmdArgs := args[1:]
	if asRoot {
		cmdArgs = append([]string{cmdName}, cmdArgs...)
		cmdName = "sudo"
	}

	execCmd := exec.CommandContext(ctxWithTimeout, cmdName, cmdArgs...)
	output, err := execCmd.CombinedOutput()

	result := string(output)
	if len(result) > 4000 {
		result = result[:4000] + "\n... (输出过长，已截断)"
	}

	tag := "✅ 执行完成"
	if asRoot {
		tag = "✅ 执行完成 (root)"
	}

	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 执行失败: %v\n\n输出:\n<pre>%s</pre>", err, html.EscapeString(result)))
	}
	if result == "" {
		result = "(无输出)"
	}
	return ctx.Edit(fmt.Sprintf("%s\n\n<pre>%s</pre>", tag, html.EscapeString(result)))
}
