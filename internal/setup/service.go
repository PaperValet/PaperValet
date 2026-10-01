package setup

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var serviceNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

func validServiceName(s string) bool { return serviceNameRe.MatchString(s) }

// errNoSystemd and errNeedRoot are surfaced as friendly messages.
var (
	errNoSystemd = errors.New("no systemd")
	errNeedRoot  = errors.New("need root")
)

// Service describes a systemd unit running one PaperValet instance.
type Service struct {
	Name    string // unit name without .service
	Binary  string // absolute path of the papervalet binary
	Home    string // PAPERVALET_HOME
	Command string // PAPERVALET_CMD, for hints printed by the bot
	User    bool   // user unit (~/.config/systemd/user) instead of system unit
}

// unitPath is where the unit file lives.
func (s Service) unitPath() (string, error) {
	if !s.User {
		return filepath.Join("/etc/systemd/system", s.Name+".service"), nil
	}
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg, "systemd", "user", s.Name+".service"), nil
}

// Unit renders the unit file.
func (s Service) Unit() string {
	target := "multi-user.target"
	if s.User {
		target = "default.target"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[Unit]\n")
	fmt.Fprintf(&b, "Description=PaperValet Telegram userbot (%s)\n", s.Name)
	fmt.Fprintf(&b, "Wants=network-online.target\n")
	fmt.Fprintf(&b, "After=network-online.target\n\n")
	fmt.Fprintf(&b, "[Service]\n")
	fmt.Fprintf(&b, "Type=simple\n")
	fmt.Fprintf(&b, "Environment=%s\n", quoteEnv("PAPERVALET_HOME="+s.Home))
	fmt.Fprintf(&b, "Environment=%s\n", quoteEnv("PAPERVALET_CMD="+s.Command))
	fmt.Fprintf(&b, "WorkingDirectory=%s\n", s.Home)
	fmt.Fprintf(&b, "ExecStart=%s run\n", s.Binary)
	fmt.Fprintf(&b, "Restart=on-failure\n")
	fmt.Fprintf(&b, "RestartSec=5\n\n")
	fmt.Fprintf(&b, "[Install]\n")
	fmt.Fprintf(&b, "WantedBy=%s\n", target)
	return b.String()
}

// quoteEnv wraps an Environment= assignment in quotes when it has spaces.
func quoteEnv(kv string) string {
	if !strings.ContainsAny(kv, " \t\"\\") {
		return kv
	}
	kv = strings.ReplaceAll(kv, `\`, `\\`)
	kv = strings.ReplaceAll(kv, `"`, `\"`)
	return `"` + kv + `"`
}

func (s Service) systemctl(args ...string) error {
	if s.User {
		args = append([]string{"--user"}, args...)
	}
	cmd := exec.Command("systemctl", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(out.String())
		if msg == "" {
			return err
		}
		return fmt.Errorf("systemctl %s: %s", strings.Join(args, " "), msg)
	}
	return nil
}

// Exists reports whether the unit file is already present.
func (s Service) Exists() bool {
	p, err := s.unitPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// Install writes the unit, reloads systemd, enables and (re)starts it.
func (s Service) Install() error {
	p, err := s.unitPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(p, []byte(s.Unit()), 0o644); err != nil {
		return err
	}
	if err := s.systemctl("daemon-reload"); err != nil {
		return err
	}
	if err := s.systemctl("enable", s.Name+".service"); err != nil {
		return err
	}
	return s.systemctl("restart", s.Name+".service")
}

// LogsCmd and StatusCmd are the commands users run to inspect the service.
func (s Service) LogsCmd() string {
	if s.User {
		return "journalctl --user -u " + s.Name + " -f"
	}
	return "journalctl -u " + s.Name + " -f"
}

func (s Service) StatusCmd() string {
	if s.User {
		return "systemctl --user status " + s.Name
	}
	return "systemctl status " + s.Name
}

// detectServiceMode picks system units for root and user units otherwise,
// failing early when neither is usable.
func detectServiceMode() (user bool, err error) {
	if _, err := os.Stat("/run/systemd/system"); err != nil {
		return false, errNoSystemd
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return false, errNoSystemd
	}
	if os.Geteuid() == 0 {
		return false, nil
	}
	// Headless boxes often have no user manager; probe before writing files.
	if err := exec.Command("systemctl", "--user", "show-environment").Run(); err != nil {
		return true, errNeedRoot
	}
	return true, nil
}
