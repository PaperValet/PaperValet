//go:build !windows

package builtin

import (
	"os"
	"os/exec"
	"syscall"
)

// shellCommand runs line through sh in its own process group so a
// deadline kills the whole pipeline, not just the shell.
func shellCommand(line string) *exec.Cmd {
	cmd := exec.Command("sh", "-c", line)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}

func killTree(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

// reexec replaces the process image in place (same PID), so systemd,
// Docker and tmux keep supervising it.
func reexec(exe string) error {
	return syscall.Exec(exe, os.Args, os.Environ())
}
