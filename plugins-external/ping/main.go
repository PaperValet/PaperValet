package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// Telegram Data Centers (from TeleBox)
var dcs = map[int]string{
	1: "149.154.175.53",   // DC1 Miami
	2: "149.154.167.51",   // DC2 Amsterdam
	3: "149.154.175.100",  // DC3 Miami
	4: "149.154.167.91",   // DC4 Amsterdam
	5: "91.108.56.130",    // DC5 Singapore
}

type PingPlugin struct{}

func New() (plugin.Plugin, error) {
	return &PingPlugin{}, nil
}

var Metadata = &plugin.PluginMetadata{
	Name:        "ping",
	Description: "网络延迟测试工具",
	Version:     "1.0.0",
	Author:      "PaperValet",
	MinVersion:  "0.1.0",
}

func (p *PingPlugin) Name() string        { return "ping" }
func (p *PingPlugin) Description() string { return "网络延迟测试工具" }

func (p *PingPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&plugin.Command{
		Name:        "ping",
		Aliases:     []string{},
		Description: "网络延迟测试",
		Usage:       "ping [目标] — ping [8.8.8.8|google.com|dc1-dc5|all]",
		Plugin:      p.Name(),
		Category:    "tools",
		Handler:     p.handlePing,
	})
}

func (p *PingPlugin) handlePing(ctx *plugin.CommandContext) error {
	target := strings.ToLower(ctx.GetArg(0))

	if target == "" {
		// Basic Telegram latency test
		start := time.Now()
		_ = ctx.Edit("🏓 测试中...")
		apiLatency := time.Since(start)

		start = time.Now()
		_ = ctx.Edit("🏓 Pong!")
		msgLatency := time.Since(start)

		return ctx.Edit(fmt.Sprintf(`🏓 <b>Pong!</b>

📡 <b>API延迟:</b> <code>%dms</code>
✏️ <b>消息延迟:</b> <code>%dms</code>

⏰ <i>%s</i>`,
			apiLatency.Milliseconds(),
			msgLatency.Milliseconds(),
			time.Now().Format("2006-01-02 15:04:05"),
		))
	}

	if target == "help" || target == "h" {
		return ctx.Edit(`🏓 <b>Ping工具使用说明</b>

<b>基础用法:</b>
• <code>.ping</code> - Telegram API延迟测试
• <code>.ping all</code> - 所有数据中心延迟
• <code>.ping dc1</code> - 指定数据中心

<b>网络测试:</b>
• <code>.ping 8.8.8.8</code> - IP地址ping
• <code>.ping google.com</code> - 域名ping
• <code>.ping dc1-dc5</code> - Telegram数据中心

<b>支持的数据中心:</b>
• DC1/DC3: Miami
• DC2/DC4: Amsterdam
• DC5: Singapore`)
	}

	if target == "all" || target == "dc" {
		return p.pingAllDCs(ctx)
	}

	// Check if it's a DC
	if strings.HasPrefix(target, "dc") {
		if dcNum, err := strconv.Atoi(target[2:]); err == nil && dcNum >= 1 && dcNum <= 5 {
			return p.pingDC(ctx, dcNum)
		}
	}

	// Network target ping
	return p.pingTarget(ctx, target)
}

func (p *PingPlugin) pingAllDCs(ctx *plugin.CommandContext) error {
	_ = ctx.Edit("🔍 正在测试所有数据中心延迟...")

	var results []string
	for dc := 1; dc <= 5; dc++ {
		ip := dcs[dc]
		latency := p.tcpPing(ip, 443, 3*time.Second)

		location := ""
		switch dc {
		case 1, 3:
			location = "Miami"
		case 2, 4:
			location = "Amsterdam"
		case 5:
			location = "Singapore"
		}

		if latency >= 0 {
			results = append(results, fmt.Sprintf("🌐 <b>DC%d (%s):</b> <code>%dms</code>", dc, location, latency))
		} else {
			results = append(results, fmt.Sprintf("🌐 <b>DC%d (%s):</b> <code>超时</code>", dc, location))
		}
	}

	return ctx.Edit(fmt.Sprintf(`🌐 <b>Telegram数据中心延迟</b>

%s

⏰ <i>%s</i>`, strings.Join(results, "\n"), time.Now().Format("2006-01-02 15:04:05")))
}

func (p *PingPlugin) pingDC(ctx *plugin.CommandContext, dc int) error {
	ip := dcs[dc]
	location := ""
	switch dc {
	case 1, 3:
		location = "Miami"
	case 2, 4:
		location = "Amsterdam"
	case 5:
		location = "Singapore"
	}

	_ = ctx.Edit(fmt.Sprintf("🔍 正在测试 DC%d (%s)...", dc, location))
	latency := p.tcpPing(ip, 443, 5*time.Second)

	if latency >= 0 {
		return ctx.Edit(fmt.Sprintf("🌐 <b>DC%d (%s):</b> <code>%dms</code>", dc, location, latency))
	}
	return ctx.Edit(fmt.Sprintf("🌐 <b>DC%d (%s):</b> <code>超时</code>", dc, location))
}

func (p *PingPlugin) pingTarget(ctx *plugin.CommandContext, target string) error {
	_ = ctx.Edit(fmt.Sprintf("🔍 正在测试 <code>%s</code>...", target))

	// Parse target (IP or domain)
	parsed := p.parseTarget(target)
	testTarget := parsed.value

	var results []string

	// DNS lookup
	dnsStart := time.Now()
	ips, err := net.LookupIP(testTarget)
	dnsTime := time.Since(dnsStart)
	if err == nil && len(ips) > 0 {
		results = append(results, fmt.Sprintf("🔍 <b>DNS解析:</b> <code>%dms</code> → <code>%s</code>", dnsTime.Milliseconds(), ips[0].String()))
		testTarget = ips[0].String()
	}

	// TCP ping (port 80)
	tcp80 := p.tcpPing(testTarget, 80, 5*time.Second)
	if tcp80 >= 0 {
		results = append(results, fmt.Sprintf("🌐 <b>TCP连接 (80):</b> <code>%dms</code>", tcp80))
	}

	// TCP ping (port 443)
	tcp443 := p.tcpPing(testTarget, 443, 5*time.Second)
	if tcp443 >= 0 {
		results = append(results, fmt.Sprintf("🔒 <b>TCP连接 (443):</b> <code>%dms</code>", tcp443))
	}

	// HTTP ping
	httpTime := p.httpPing(testTarget, false)
	if httpTime >= 0 {
		results = append(results, fmt.Sprintf("📡 <b>HTTP Ping:</b> <code>%dms</code>", httpTime))
	}

	// HTTPS ping
	httpsTime := p.httpPing(testTarget, true)
	if httpsTime >= 0 {
		results = append(results, fmt.Sprintf("📡 <b>HTTPS Ping:</b> <code>%dms</code>", httpsTime))
	}

	// System ping (ICMP)
	if parsed.typeStr != "dc" {
		icmpTime := p.systemPing(testTarget)
		if icmpTime >= 0 {
			results = append(results, fmt.Sprintf("🏓 <b>ICMP Ping:</b> <code>%dms</code>", icmpTime))
		}
	}

	targetType := parsed.typeStr
	if parsed.typeStr == "ip" {
		targetType = "IP地址"
	} else if parsed.typeStr == "domain" {
		targetType = "域名"
	}

	display := ""
	if target == testTarget {
		display = fmt.Sprintf("<code>%s</code>\n\n", target)
	} else {
		display = fmt.Sprintf("<code>%s</code> → <code>%s</code>\n\n", target, testTarget)
	}

	if len(results) == 0 {
		results = append(results, "❌ 所有测试均失败，目标可能不可达")
	}

	return ctx.Edit(fmt.Sprintf(`🎯 <b>%s延迟测试</b>
%s%s
⏰ <i>%s</i>`, targetType, display, strings.Join(results, "\n"), time.Now().Format("2006-01-02 15:04:05")))
}

type parsedTarget struct {
	typeStr string
	value   string
}

func (p *PingPlugin) parseTarget(input string) parsedTarget {
	// Check for DC
	if strings.HasPrefix(strings.ToLower(input), "dc") {
		if dcNum, err := strconv.Atoi(input[2:]); err == nil && dcNum >= 1 && dcNum <= 5 {
			return parsedTarget{typeStr: "dc", value: dcs[dcNum]}
		}
	}

	// Check IP
	if net.ParseIP(input) != nil {
		return parsedTarget{typeStr: "ip", value: input}
	}

	return parsedTarget{typeStr: "domain", value: input}
}

func (p *PingPlugin) tcpPing(host string, port int, timeout time.Duration) int {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	start := time.Now()

	conn, err := net.DialTimeout("tcp", addr, timeout)
	elapsed := time.Since(start)

	if err != nil {
		return -1
	}
	conn.Close()
	return int(elapsed.Milliseconds())
}

func (p *PingPlugin) httpPing(host string, https bool, timeout ...time.Duration) int {
	t := 5 * time.Second
	if len(timeout) > 0 {
		t = timeout[0]
	}

	protocol := "http"
	port := 80
	if https {
		protocol = "https"
		port = 443
	}

	url := fmt.Sprintf("%s://%s", protocol, net.JoinHostPort(host, strconv.Itoa(port)))

	client := &http.Client{Timeout: t}
	start := time.Now()

	req, _ := http.NewRequest("HEAD", url, nil)
	req.Header.Set("User-Agent", "PaperValet-Ping/1.0")

	resp, err := client.Do(req)
	elapsed := time.Since(start)

	if err != nil {
		return -1
	}
	defer resp.Body.Close()
	return int(elapsed.Milliseconds())
}

func (p *PingPlugin) systemPing(host string) int {
	cmd := exec.Command("ping", "-c", "3", "-W", "5", host)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return -1
	}

	// Parse Linux ping output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "rtt min/avg/max/mdev") {
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.Contains(part, "/") {
					avgStr := strings.Split(part, "/")[1]
					if avg, err := strconv.ParseFloat(avgStr, 64); err == nil {
						return int(avg)
					}
				}
			}
		}
	}
	return -1
}