package builtin

import (
	"context"
	"fmt"
	"html"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

const (
	execDefaultTimeout = 60 * time.Second
	execMaxTimeout     = 600 * time.Second
	execOutputLimit    = 3500 // fits one Telegram message with headers
	execBufferLimit    = 1 << 20
	execProgressEvery  = 3 * time.Second
)

// ExecPlugin executes shell commands (owner only).
type ExecPlugin struct{}

func NewExec() *ExecPlugin { return &ExecPlugin{} }

func (p *ExecPlugin) Name() string        { return "exec" }
func (p *ExecPlugin) Description() string { return "在服务器上执行 shell 命令" }
func (p *ExecPlugin) DescEN() string      { return "Run shell commands on the server" }

func (p *ExecPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "exec",
		Aliases:     []string{"sh", "shell"},
		Description: "执行 shell 命令，实时回显输出，默认 60 秒后自动终止",
		DescEN:      "Run a shell command with live output; killed after 60s by default",
		Usage:       "exec [-t 秒数] <命令>",
		UsageEN:     "exec [-t seconds] <command>",
		Plugin:      p.Name(),
		Category:    "admin",
		OwnerOnly:   true,
		Handler:     p.handleExec,
	})
}

func (p *ExecPlugin) Start(_ context.Context) error { return nil }
func (p *ExecPlugin) Stop(_ context.Context) error  { return nil }

func (p *ExecPlugin) help(ctx *interfaces.CommandContext) error {
	return ctx.Edit(ctx.Tlocal(
		`💻 <b>exec</b> 执行 shell 命令

<code>exec uptime</code>
<code>exec df -h | head</code>
<code>exec -t 120 apt update</code>  最长跑 120 秒

不会自己结束的命令（如 ping）到时间自动终止，已有输出照常返回。默认 60 秒，最多 600 秒。`,
		`💻 <b>exec</b> run a shell command

<code>exec uptime</code>
<code>exec df -h | head</code>
<code>exec -t 120 apt update</code>  run for up to 120s

Commands that never exit (like ping) are killed at the deadline and their output is still returned. Default 60s, max 600s.`))
}

// parseExecArgs splits an optional -t/--timeout flag from the shell line.
func parseExecArgs(raw string) (time.Duration, string, error) {
	line := strings.TrimSpace(raw)
	timeout := execDefaultTimeout
	for _, flag := range []string{"-t ", "--timeout "} {
		if strings.HasPrefix(line, flag) {
			rest := strings.TrimSpace(line[len(flag):])
			num, cmd, _ := strings.Cut(rest, " ")
			var n int
			if _, err := fmt.Sscanf(num, "%d", &n); err != nil || n < 1 || time.Duration(n)*time.Second > execMaxTimeout {
				return 0, "", fmt.Errorf("bad timeout")
			}
			timeout = time.Duration(n) * time.Second
			line = strings.TrimSpace(cmd)
			break
		}
	}
	return timeout, line, nil
}

// limitedBuffer keeps at most limit bytes, dropping the oldest output.
type limitedBuffer struct {
	mu      sync.Mutex
	data    []byte
	limit   int
	dropped bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = append(b.data, p...)
	if over := len(b.data) - b.limit; over > 0 {
		b.data = b.data[over:]
		b.dropped = true
	}
	return len(p), nil
}

func (b *limitedBuffer) String() (string, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.data), b.dropped
}

// tail returns the last limit bytes of s, cut at a line boundary.
func tail(s string, limit int) (string, bool) {
	if len(s) <= limit {
		return s, false
	}
	s = s[len(s)-limit:]
	if i := strings.IndexByte(s, '\n'); i >= 0 && i < len(s)-1 {
		s = s[i+1:]
	}
	return s, true
}

func (p *ExecPlugin) handleExec(ctx *interfaces.CommandContext) error {
	if strings.TrimSpace(ctx.RawArgs) == "" {
		return p.help(ctx)
	}
	timeout, line, err := parseExecArgs(ctx.RawArgs)
	if err != nil {
		return ctx.Edit(ctx.Tlocal("❌ 超时要写 1–600 之间的秒数，比如 <code>exec -t 120 …</code>",
			"❌ Timeout must be 1-600 seconds, e.g. <code>exec -t 120 …</code>"))
	}
	if line == "" {
		return p.help(ctx)
	}

	runCtx, cancel := context.WithTimeout(ctx.Context(), timeout)
	defer cancel()

	cmd := exec.Command("sh", "-c", line)
	// Own process group so the deadline kills the whole pipeline, not just sh.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	out := &limitedBuffer{limit: execBufferLimit}
	cmd.Stdout = out
	cmd.Stderr = out

	header := "<code>$ " + html.EscapeString(line) + "</code>"
	_ = ctx.Edit(header + "\n⏳ " + ctx.Tlocal("运行中…", "running…"))

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return ctx.Edit(header + "\n❌ " + html.EscapeString(err.Error()))
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	ticker := time.NewTicker(execProgressEvery)
	defer ticker.Stop()
	var waitErr error
	timedOut := false
loop:
	for {
		select {
		case waitErr = <-done:
			break loop
		case <-runCtx.Done():
			timedOut = true
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			waitErr = <-done
			break loop
		case <-ticker.C:
			s, _ := out.String()
			s, _ = tail(s, 800)
			body := ""
			if strings.TrimSpace(s) != "" {
				body = "\n<pre>" + html.EscapeString(s) + "</pre>"
			}
			_ = ctx.Edit(fmt.Sprintf("%s\n⏳ %s %ds/%ds%s", header,
				ctx.Tlocal("运行中", "running"), int(time.Since(start).Seconds()), int(timeout.Seconds()), body))
		}
	}

	elapsed := time.Since(start).Round(100 * time.Millisecond)
	s, dropped := out.String()
	s, cut := tail(s, execOutputLimit)

	var status string
	switch {
	case timedOut:
		status = ctx.Tlocal(fmt.Sprintf("⏱ 已运行 %s，到时自动终止", timeout), fmt.Sprintf("⏱ Stopped at the %s limit", timeout))
	case waitErr != nil:
		code := -1
		if ee, ok := waitErr.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
		status = ctx.Tlocal(fmt.Sprintf("❌ 退出码 %d · %s", code, elapsed), fmt.Sprintf("❌ Exit code %d · %s", code, elapsed))
	default:
		status = fmt.Sprintf("✅ %s", elapsed)
	}

	body := ctx.Tlocal("（无输出）", "(no output)")
	if strings.TrimSpace(s) != "" {
		body = "<pre>" + html.EscapeString(strings.TrimRight(s, "\n")) + "</pre>"
	}
	if cut || dropped {
		body = ctx.Tlocal("…只显示最后一部分输出\n", "…showing the last part of the output\n") + body
	}
	return ctx.Edit(header + "\n" + status + "\n" + body)
}
