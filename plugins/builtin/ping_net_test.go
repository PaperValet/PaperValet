package builtin

import (
	"context"
	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"net"
	"strconv"
	"testing"
	"time"
)

func TestParseTarget(t *testing.T) {
	cases := map[string]string{
		"dc1":                   "dc",
		"8.8.8.8":               "ip",
		"2001:4860:4860::8888":  "ip",
		"google.com":            "domain",
		"https://example.com/x": "domain",
		"-c":                    "invalid",
		"a;b":                   "invalid",
	}
	for in, want := range cases {
		if got := parseTarget(in).typ; got != want {
			t.Errorf("parseTarget(%q)=%s want %s", in, got, want)
		}
	}
	if parseTarget("https://example.com/x").value != "example.com" {
		t.Error("url host not extracted")
	}
}

func TestParsePingOutput(t *testing.T) {
	out := `3 packets transmitted, 3 received, 0% packet loss, time 2003ms
rtt min/avg/max/mdev = 0.031/0.046/0.060/0.011 ms`
	r := parsePingOutput(out)
	if r.avg != 0 || r.loss != 0 {
		t.Errorf("got %+v", r)
	}
	r = parsePingOutput("3 packets transmitted, 2 received, 33.3333% packet loss\nround-trip min/avg/max/stddev = 10.1/20.6/30.0/1 ms")
	if r.avg != 21 || r.loss != 33 {
		t.Errorf("got %+v", r)
	}
	r = parsePingOutput("3 packets transmitted, 0 received, 100% packet loss")
	if r.avg != -1 || r.loss != 100 {
		t.Errorf("got %+v", r)
	}
}

func TestParseProxyURL(t *testing.T) {
	p := parseProxyURL("socks5h://u:p%40ss@127.0.0.1")
	if p == nil || p.kind != "socks5" || p.port != 1080 || p.user != "u" || p.password != "p@ss" {
		t.Fatalf("got %+v", p)
	}
	if p := parseProxyURL("http://proxy:3128"); p == nil || p.kind != "http" || p.port != 3128 {
		t.Fatalf("got %+v", p)
	}
	if parseProxyURL("ftp://x") != nil {
		t.Fatal("ftp accepted")
	}
}

func TestDcNumber(t *testing.T) {
	if n, ok := dcNumber("DC5"); !ok || n != 5 {
		t.Fatal("dc5")
	}
	for _, s := range []string{"dc0", "dc6", "dc", "dc12"} {
		if _, ok := dcNumber(s); ok {
			t.Fatal(s)
		}
	}
}

// fake socks5 server: accepts no-auth CONNECT and replies success.
func TestTcpingViaSocks5(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				b := make([]byte, 262)
				c.Read(b) // greeting
				c.Write([]byte{5, 0})
				c.Read(b) // request
				c.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 80})
				time.Sleep(50 * time.Millisecond)
			}(c)
		}
	}()
	port := ln.Addr().(*net.TCPAddr).Port
	t.Setenv("ALL_PROXY", "socks5://127.0.0.1:"+strconv.Itoa(port))
	r := tcpingProbe(context.Background(), "example.invalid", []int{443}, 2, 2*time.Second)
	if r == nil || r.loss != 0 || r.port != 443 {
		t.Fatalf("got %+v", r)
	}
}

func TestTcpingDirect(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	for _, k := range []string{"ALL_PROXY", "all_proxy", "HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
		t.Setenv(k, "")
	}
	port := ln.Addr().(*net.TCPAddr).Port
	r := tcpingProbe(context.Background(), "127.0.0.1", []int{1, port}, 3, time.Second)
	if r == nil || r.port != port || r.loss != 0 {
		t.Fatalf("got %+v", r)
	}
}

func TestPingHandleRoutesLocalTargets(t *testing.T) {
	ctx := &interfaces.CommandContext{Args: []string{"-bad"}}
	// no message: Edit fails, but routing must reach pingTarget's validation
	if err := NewPing().handle(ctx); err != interfaces.ErrNoMessage {
		t.Fatalf("err = %v", err)
	}
}

func TestFmtMs(t *testing.T) {
	if fmtMs(0) != "<1ms" || fmtMs(12) != "12ms" {
		t.Fatal(fmtMs(0), fmtMs(12))
	}
}

func TestPingRegistersPingdc(t *testing.T) {
	_, mgr := newHelpFixture(t)
	cmd, ok := mgr.Commands().Get("pingdc")
	if !ok || cmd.Plugin != "ping" || cmd.Handler == nil {
		t.Fatalf("pingdc missing: %+v", cmd)
	}
}

func TestDcLabelFollowsLanguage(t *testing.T) {
	zh := dcLabel(&interfaces.CommandContext{Lang: "zh-CN"}, 5)
	en := dcLabel(&interfaces.CommandContext{Lang: "en-US"}, 5)
	if zh != "DC5 新加坡" || en != "DC5 Singapore" {
		t.Fatalf("zh=%q en=%q", zh, en)
	}
}
