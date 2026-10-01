package builtin

import (
	"strings"
	"testing"
	"time"
)

func TestParseExecArgs(t *testing.T) {
	cases := []struct {
		in      string
		timeout time.Duration
		line    string
		bad     bool
	}{
		{"uptime", execDefaultTimeout, "uptime", false},
		{"-t 5 ping 1.1.1.1", 5 * time.Second, "ping 1.1.1.1", false},
		{"--timeout 120 apt update", 120 * time.Second, "apt update", false},
		{"-t 0 ls", 0, "", true},
		{"-t 9999 ls", 0, "", true},
		{"-t abc ls", 0, "", true},
		{"echo -t 5", execDefaultTimeout, "echo -t 5", false},
	}
	for _, c := range cases {
		to, line, err := parseExecArgs(c.in)
		if c.bad {
			if err == nil {
				t.Errorf("%q: want error", c.in)
			}
			continue
		}
		if err != nil || to != c.timeout || line != c.line {
			t.Errorf("%q: got %v %q %v", c.in, to, line, err)
		}
	}
}

func TestLimitedBufferKeepsTail(t *testing.T) {
	b := &limitedBuffer{limit: 10}
	_, _ = b.Write([]byte("0123456789abcdef"))
	s, dropped := b.String()
	if s != "6789abcdef" || !dropped {
		t.Fatalf("got %q %v", s, dropped)
	}
}

func TestTailCutsAtLine(t *testing.T) {
	in := strings.Repeat("line\n", 100)
	out, cut := tail(in, 23)
	if !cut || strings.HasPrefix(out, "ine") || len(out) > 23 {
		t.Fatalf("got %q", out)
	}
	if s, cut := tail("short", 100); s != "short" || cut {
		t.Fatal("short input must pass through")
	}
}
