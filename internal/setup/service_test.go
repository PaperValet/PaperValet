package setup

import (
	"strings"
	"testing"
)

func TestServiceUnit(t *testing.T) {
	s := Service{Name: "pv2", Binary: "/root/.pv2/bin/papervalet", Home: "/root/.pv2", Command: "pv2"}
	u := s.Unit()
	for _, want := range []string{
		"ExecStart=/root/.pv2/bin/papervalet run\n",
		"Environment=PAPERVALET_HOME=/root/.pv2\n",
		"Environment=PAPERVALET_CMD=pv2\n",
		"WorkingDirectory=/root/.pv2\n",
		"Restart=on-failure\n",
		"WantedBy=multi-user.target\n",
	} {
		if !strings.Contains(u, want) {
			t.Errorf("unit missing %q\n%s", want, u)
		}
	}
	s.User = true
	if !strings.Contains(s.Unit(), "WantedBy=default.target\n") {
		t.Error("user unit must target default.target")
	}
	if got := s.LogsCmd(); got != "journalctl --user -u pv2 -f" {
		t.Errorf("logs cmd %q", got)
	}
}

func TestQuoteEnv(t *testing.T) {
	if got := quoteEnv("A=/x/y"); got != "A=/x/y" {
		t.Errorf("plain: %q", got)
	}
	if got := quoteEnv(`A=/my dir/"q"`); got != `"A=/my dir/\"q\""` {
		t.Errorf("quoted: %q", got)
	}
}

func TestValidServiceName(t *testing.T) {
	for _, ok := range []string{"papervalet", "pv-2", "bot_main"} {
		if !validServiceName(ok) {
			t.Errorf("%q should be valid", ok)
		}
	}
	for _, bad := range []string{"", "-x", "a b", "a/b", "x.service"} {
		if validServiceName(bad) {
			t.Errorf("%q should be invalid", bad)
		}
	}
}

func TestPhone(t *testing.T) {
	if !validPhone("+86 138-0013-8000") {
		t.Error("spaced phone should be valid")
	}
	if validPhone("13800138000") || validPhone("+0123456") {
		t.Error("phone without + or with leading 0 should be invalid")
	}
	if got := normalizePhone(" +1 (415) 555-0123 "); got != "+14155550123" {
		t.Errorf("normalize: %q", got)
	}
}

func TestMask(t *testing.T) {
	if got := mask("0123456789abcdef"); got != "0123…cdef" {
		t.Errorf("mask: %q", got)
	}
	if got := mask("abc"); got != "•••" {
		t.Errorf("short mask: %q", got)
	}
}
