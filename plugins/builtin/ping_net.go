package builtin

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Network probes for ping: TCP (direct or through ALL_PROXY/HTTPS_PROXY/
// HTTP_PROXY), HTTP(S) HEAD and the system ping binary for ICMP.

// Telegram data centers.
var dcs = map[int]string{
	1: "149.154.175.53",  // DC1 Miami
	2: "149.154.167.51",  // DC2 Amsterdam
	3: "149.154.175.100", // DC3 Miami
	4: "149.154.167.91",  // DC4 Amsterdam
	5: "91.108.56.130",   // DC5 Singapore
}

var dcLocations = map[int]string{1: "Miami", 2: "Amsterdam", 3: "Miami", 4: "Amsterdam", 5: "Singapore"}

func dcNumber(s string) (int, bool) {
	s = strings.ToLower(s)
	if len(s) != 3 || !strings.HasPrefix(s, "dc") {
		return 0, false
	}
	n, err := strconv.Atoi(s[2:])
	if err != nil || n < 1 || n > 5 {
		return 0, false
	}
	return n, true
}

// dcLatency probes one DC: TCP 443/80 (2 samples), then a single ICMP ping.
func dcLatency(ctx context.Context, dc int) (int, bool) {
	ip := dcs[dc]
	if r := tcpingProbe(ctx, ip, []int{443, 80}, 2, 3*time.Second); r != nil {
		return r.avg, true
	}
	if ms, ok := icmpOnce(ctx, ip); ok {
		return ms, true
	}
	return 0, false
}

func fmtMs(ms int) string {
	if ms <= 0 {
		return "<1ms"
	}
	return fmt.Sprintf("%dms", ms)
}

func pickIP(ips []net.IPAddr) string {
	for _, ip := range ips {
		if ip.IP.To4() != nil {
			return ip.IP.String()
		}
	}
	return ips[0].IP.String()
}

type parsedTarget struct {
	typ   string // ip | domain | dc | invalid
	value string
}

var hostRe = regexp.MustCompile(`^[A-Za-z0-9._:\-]+$`)

func parseTarget(input string) parsedTarget {
	input = strings.TrimSpace(input)
	if n, ok := dcNumber(input); ok {
		return parsedTarget{typ: "dc", value: dcs[n]}
	}
	// Accept URLs like https://example.com/path for convenience.
	if strings.Contains(input, "://") {
		if u, err := url.Parse(input); err == nil && u.Hostname() != "" {
			input = u.Hostname()
		}
	}
	input = strings.TrimSuffix(strings.TrimPrefix(input, "["), "]")
	if ip := net.ParseIP(input); ip != nil {
		return parsedTarget{typ: "ip", value: ip.String()}
	}
	if input == "" || strings.HasPrefix(input, "-") || !hostRe.MatchString(input) || strings.Contains(input, ":") {
		return parsedTarget{typ: "invalid", value: input}
	}
	return parsedTarget{typ: "domain", value: input}
}

// ---------------------------------------------------------------- TCP

type tcpProbeResult struct {
	avg, best, port, loss int
}

// tcpingProbe tries ports in order for each sample; the first that connects
// counts. Returns nil when every sample failed.
func tcpingProbe(ctx context.Context, host string, ports []int, samples int, timeout time.Duration) *tcpProbeResult {
	type hit struct{ ms, port int }
	var ok []hit
	for i := 0; i < samples; i++ {
		if ctx.Err() != nil {
			break
		}
		for _, port := range ports {
			if ms := tcpPing(ctx, host, port, timeout); ms >= 0 {
				ok = append(ok, hit{ms, port})
				break
			}
		}
	}
	if len(ok) == 0 {
		return nil
	}
	sum, best := 0, ok[0].ms
	count := map[int]int{}
	for _, h := range ok {
		sum += h.ms
		if h.ms < best {
			best = h.ms
		}
		count[h.port]++
	}
	port, maxC := ok[0].port, 0
	keys := make([]int, 0, len(count))
	for k := range count {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	for _, k := range keys {
		if count[k] > maxC {
			maxC, port = count[k], k
		}
	}
	avg := int(float64(sum)/float64(len(ok)) + 0.5)
	loss := int(float64(samples-len(ok))/float64(samples)*100 + 0.5)
	return &tcpProbeResult{avg: avg, best: best, port: port, loss: loss}
}

// tcpPing measures a TCP connect, through the configured proxy if any.
func tcpPing(ctx context.Context, host string, port int, timeout time.Duration) int {
	c, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	var (
		conn net.Conn
		err  error
	)
	if px := resolveProxy(); px != nil {
		conn, err = px.dial(c, host, port)
	} else {
		var d net.Dialer
		conn, err = d.DialContext(c, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	}
	elapsed := time.Since(start)
	if err != nil {
		return -1
	}
	conn.Close()
	return int(elapsed.Milliseconds())
}

// ---------------------------------------------------------------- HTTP

// httpPing times the first response to HEAD / (redirects not followed).
// Certificates are not verified: only latency matters and IP targets would
// otherwise always fail on HTTPS.
func httpPing(ctx context.Context, host string, https bool) int {
	scheme := "http"
	if https {
		scheme = "https"
	}
	u := scheme + "://" + hostForURL(host) + "/"
	c, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(c, http.MethodHead, u, nil)
	if err != nil {
		return -1
	}
	req.Header.Set("User-Agent", "PaperValet-Ping/1.1")
	tr := &http.Transport{
		Proxy:             nil,
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // latency probe only
		DisableKeepAlives: true,
	}
	defer tr.CloseIdleConnections()
	client := &http.Client{
		Transport:     tr,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		return -1
	}
	resp.Body.Close()
	return int(elapsed.Milliseconds())
}

func hostForURL(host string) string {
	if ip := net.ParseIP(host); ip != nil && ip.To4() == nil {
		return "[" + host + "]"
	}
	return host
}

// ---------------------------------------------------------------- ICMP

type icmpResult struct {
	avg, loss int
}

var (
	avgRe  = regexp.MustCompile(`(?:min/avg/max[^=]*=\s*[0-9.]+/)([0-9.]+)`)
	lossRe = regexp.MustCompile(`([0-9.]+)% packet loss`)
	timeRe = regexp.MustCompile(`time[=<]([0-9.]+)`)
)

func parsePingOutput(out string) icmpResult {
	r := icmpResult{avg: -1, loss: 100}
	if m := avgRe.FindStringSubmatch(out); m != nil {
		if f, err := strconv.ParseFloat(m[1], 64); err == nil {
			r.avg = int(f + 0.5)
		}
	}
	if m := lossRe.FindStringSubmatch(out); m != nil {
		if f, err := strconv.ParseFloat(m[1], 64); err == nil {
			r.loss = int(f + 0.5)
		}
	}
	return r
}

func validPingTarget(t string) bool {
	return t != "" && !strings.HasPrefix(t, "-") && hostRe.MatchString(t)
}

// systemPing runs the system ping binary (no shell). Error means ping is
// missing / unusable; unreachable hosts return loss=100.
func systemPing(ctx context.Context, target string, count int) (icmpResult, error) {
	if !validPingTarget(target) {
		return icmpResult{avg: -1, loss: 100}, errors.New("invalid target")
	}
	if count < 1 || count > 10 {
		count = 3
	}
	bin, err := exec.LookPath("ping")
	if err != nil {
		return icmpResult{avg: -1, loss: 100}, err
	}
	c, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(c, bin, "-c", strconv.Itoa(count), "-W", "3", "-i", "0.5", target).CombinedOutput()
	r := parsePingOutput(string(out))
	if err != nil && r.avg < 0 && !strings.Contains(string(out), "packet loss") {
		// e.g. permission denied / unknown option: retry without -i.
		out, err = exec.CommandContext(c, bin, "-c", strconv.Itoa(count), "-W", "3", target).CombinedOutput()
		r = parsePingOutput(string(out))
		if err != nil && r.avg < 0 && !strings.Contains(string(out), "packet loss") {
			return r, fmt.Errorf("ping: %v", err)
		}
	}
	return r, nil
}

func icmpOnce(ctx context.Context, target string) (int, bool) {
	if !validPingTarget(target) {
		return 0, false
	}
	c, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(c, "ping", "-c", "1", "-W", "3", target).Output()
	if err != nil {
		return 0, false
	}
	if m := timeRe.FindStringSubmatch(string(out)); m != nil {
		if f, err := strconv.ParseFloat(m[1], 64); err == nil {
			return int(f + 0.5), true
		}
	}
	return 0, false
}

// ---------------------------------------------------------------- proxy

type pingProxy struct {
	kind     string // socks5 | socks4 | http
	host     string
	port     int
	user     string
	password string
}

func (p *pingProxy) display() string {
	return p.kind + "://" + net.JoinHostPort(p.host, strconv.Itoa(p.port))
}

// resolveProxy reads ALL_PROXY / HTTPS_PROXY / HTTP_PROXY (either case),
// mirroring the reference's environment fallback.
func resolveProxy() *pingProxy {
	for _, k := range []string{"ALL_PROXY", "all_proxy", "HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return parseProxyURL(v)
		}
	}
	return nil
}

func parseProxyURL(raw string) *pingProxy {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return nil
	}
	proto := strings.ToLower(u.Scheme)
	px := &pingProxy{host: u.Hostname()}
	switch proto {
	case "socks5", "socks5h", "socks":
		px.kind = "socks5"
	case "socks4", "socks4a":
		px.kind = "socks4"
	case "http", "https":
		px.kind = "http"
	default:
		return nil
	}
	if ps := u.Port(); ps != "" {
		n, err := strconv.Atoi(ps)
		if err != nil || n <= 0 || n > 65535 {
			return nil
		}
		px.port = n
	} else if px.kind == "http" {
		px.port = 8080
	} else {
		px.port = 1080
	}
	if u.User != nil {
		px.user = u.User.Username()
		px.password, _ = u.User.Password()
	}
	return px
}

func (p *pingProxy) dial(ctx context.Context, host string, port int) (net.Conn, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(p.host, strconv.Itoa(p.port)))
	if err != nil {
		return nil, err
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	switch p.kind {
	case "socks5":
		err = socks5Handshake(conn, p, host, port)
	case "socks4":
		err = socks4Handshake(conn, p, host, port)
	default:
		err = httpConnectHandshake(conn, p, host, port)
	}
	if err != nil {
		conn.Close()
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

func socks5Handshake(conn net.Conn, p *pingProxy, host string, port int) error {
	if p.user != "" {
		if _, err := conn.Write([]byte{5, 2, 0, 2}); err != nil {
			return err
		}
	} else if _, err := conn.Write([]byte{5, 1, 0}); err != nil {
		return err
	}
	buf := make([]byte, 2)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return err
	}
	if buf[0] != 5 {
		return errors.New("bad socks5 version")
	}
	switch buf[1] {
	case 0:
	case 2:
		if len(p.user) > 255 || len(p.password) > 255 {
			return errors.New("socks5 credentials too long")
		}
		auth := []byte{1, byte(len(p.user))}
		auth = append(auth, p.user...)
		auth = append(auth, byte(len(p.password)))
		auth = append(auth, p.password...)
		if _, err := conn.Write(auth); err != nil {
			return err
		}
		if _, err := io.ReadFull(conn, buf); err != nil {
			return err
		}
		if buf[1] != 0 {
			return errors.New("socks5 auth failed")
		}
	default:
		return fmt.Errorf("socks5 auth method %d", buf[1])
	}

	req := []byte{5, 1, 0}
	if ip := net.ParseIP(host); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			req = append(req, 1)
			req = append(req, v4...)
		} else {
			req = append(req, 4)
			req = append(req, ip.To16()...)
		}
	} else {
		if len(host) > 255 {
			return errors.New("host too long")
		}
		req = append(req, 3, byte(len(host)))
		req = append(req, host...)
	}
	req = binary.BigEndian.AppendUint16(req, uint16(port))
	if _, err := conn.Write(req); err != nil {
		return err
	}
	head := make([]byte, 4)
	if _, err := io.ReadFull(conn, head); err != nil {
		return err
	}
	if head[0] != 5 {
		return errors.New("bad socks5 reply")
	}
	if head[1] != 0 {
		return fmt.Errorf("socks5 connect status %d", head[1])
	}
	var skip int
	switch head[3] {
	case 1:
		skip = 4 + 2
	case 4:
		skip = 16 + 2
	case 3:
		l := make([]byte, 1)
		if _, err := io.ReadFull(conn, l); err != nil {
			return err
		}
		skip = int(l[0]) + 2
	default:
		return fmt.Errorf("socks5 atyp %d", head[3])
	}
	_, err := io.ReadFull(conn, make([]byte, skip))
	return err
}

func socks4Handshake(conn net.Conn, p *pingProxy, host string, port int) error {
	ip := net.ParseIP(host).To4()
	if ip == nil {
		return errors.New("socks4 needs IPv4")
	}
	req := []byte{4, 1}
	req = binary.BigEndian.AppendUint16(req, uint16(port))
	req = append(req, ip...)
	req = append(req, p.user...)
	req = append(req, 0)
	if _, err := conn.Write(req); err != nil {
		return err
	}
	resp := make([]byte, 8)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return err
	}
	if resp[1] != 0x5a {
		return fmt.Errorf("socks4 status %d", resp[1])
	}
	return nil
}

func httpConnectHandshake(conn net.Conn, p *pingProxy, host string, port int) error {
	target := net.JoinHostPort(host, strconv.Itoa(port))
	req := "CONNECT " + target + " HTTP/1.1\r\nHost: " + target + "\r\n"
	if p.user != "" {
		req += "Proxy-Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte(p.user+":"+p.password)) + "\r\n"
	}
	req += "Proxy-Connection: keep-alive\r\n\r\n"
	if _, err := io.WriteString(conn, req); err != nil {
		return err
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodConnect})
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http proxy CONNECT %d", resp.StatusCode)
	}
	return nil
}
