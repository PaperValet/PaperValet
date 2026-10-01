package builtin

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// PingPlugin measures latency to Telegram, its data centers and any host.
type PingPlugin struct{}

func NewPing() *PingPlugin { return &PingPlugin{} }

func (p *PingPlugin) Name() string        { return "ping" }
func (p *PingPlugin) Description() string { return "延迟测试" }
func (p *PingPlugin) DescEN() string      { return "Latency test" }

func (p *PingPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "ping",
		Description: "测延迟：Telegram、数据中心、IP 或域名",
		DescEN:      "Latency to Telegram, its data centers, an IP or a domain",
		Usage: "ping [all|dc1-dc5|IP|域名]\n" +
			"\n" +
			"**用法**\n" +
			"• `ping`  API 往返和编辑消息的耗时\n" +
			"• `ping all`  五个数据中心\n" +
			"• `ping dc2`  单个数据中心\n" +
			"• `ping 1.1.1.1` / `ping example.com`  DNS、TCP、HTTP(S)、ICMP\n" +
			"\n" +
			"**机制**\n" +
			"• 先测 TCP（443/80/22/53），不通再用 ICMP，最后 HTTP\n" +
			"• 设了 ALL\\_PROXY / HTTPS\\_PROXY / HTTP\\_PROXY 时 TCP 走代理\n" +
			"• ICMP 用系统的 ping 命令，没有就跳过",
		UsageEN: "ping [all|dc1-dc5|IP|domain]\n" +
			"\n" +
			"**Usage**\n" +
			"• `ping`  API round trip and message edit time\n" +
			"• `ping all`  all five data centers\n" +
			"• `ping dc2`  one data center\n" +
			"• `ping 1.1.1.1` / `ping example.com`  DNS, TCP, HTTP(S), ICMP\n" +
			"\n" +
			"**How it works**\n" +
			"• TCP first (443/80/22/53), then ICMP, then HTTP\n" +
			"• TCP goes through ALL\\_PROXY / HTTPS\\_PROXY / HTTP\\_PROXY when set\n" +
			"• ICMP uses the system ping binary and is skipped if missing",
		Plugin:   p.Name(),
		Category: "core",
		Handler:  p.handle,
	})
}

func (p *PingPlugin) Start(_ context.Context) error { return nil }
func (p *PingPlugin) Stop(_ context.Context) error  { return nil }

func (p *PingPlugin) handle(ctx *interfaces.CommandContext) error {
	target := strings.ToLower(strings.TrimSpace(ctx.GetArg(0)))
	switch target {
	case "":
		return p.pingTelegram(ctx)
	case "all", "dc":
		return p.pingAllDCs(ctx)
	}
	if n, ok := dcNumber(target); ok {
		return p.pingDC(ctx, n)
	}
	return p.pingTarget(ctx, target)
}

// pingTelegram times a raw API call and one edit of the command message.
func (p *PingPlugin) pingTelegram(ctx *interfaces.CommandContext) error {
	api := ctx.Tlocal("失败", "failed")
	if ctx.API != nil {
		c, cancel := context.WithTimeout(ctx.Context(), 15*time.Second)
		start := time.Now()
		_, err := ctx.API.UsersGetUsers(c, []tg.InputUserClass{&tg.InputUserSelf{}})
		cancel()
		if err == nil {
			api = fmtMs(int(time.Since(start).Milliseconds()))
		}
	}
	start := time.Now()
	if err := ctx.Edit("🏓"); err != nil {
		return err
	}
	edit := fmtMs(int(time.Since(start).Milliseconds()))
	c := newCard("🏓", "Pong").blank()
	c.field("API", api)
	c.field(ctx.Tlocal("编辑", "Edit"), edit)
	return ctx.Edit(c.String())
}

func dcLabel(dc int) string { return fmt.Sprintf("DC%d %s", dc, dcLocations[dc]) }

func dcValue(ctx *interfaces.CommandContext, ms int, ok bool) string {
	if !ok {
		return ctx.Tlocal("超时", "timeout")
	}
	return fmtMs(ms)
}

func (p *PingPlugin) pingAllDCs(ctx *interfaces.CommandContext) error {
	_ = ctx.Edit("🌐 …")
	type res struct {
		ms int
		ok bool
	}
	var results [6]res
	var wg sync.WaitGroup
	for dc := 1; dc <= 5; dc++ {
		wg.Add(1)
		go func(dc int) {
			defer wg.Done()
			ms, ok := dcLatency(ctx.Context(), dc)
			results[dc] = res{ms, ok}
		}(dc)
	}
	wg.Wait()
	c := newCard("🌐", ctx.Tlocal("数据中心延迟", "Data center latency")).blank()
	for dc := 1; dc <= 5; dc++ {
		c.field(dcLabel(dc), dcValue(ctx, results[dc].ms, results[dc].ok))
	}
	proxyHint(ctx, c)
	return ctx.Edit(c.String())
}

func (p *PingPlugin) pingDC(ctx *interfaces.CommandContext, dc int) error {
	_ = ctx.Edit("🌐 …")
	ms, ok := dcLatency(ctx.Context(), dc)
	c := newCard("🌐", dcLabel(dc)).blank()
	c.field(ctx.Tlocal("地址", "Address"), dcs[dc])
	c.field(ctx.Tlocal("延迟", "Latency"), dcValue(ctx, ms, ok))
	proxyHint(ctx, c)
	return ctx.Edit(c.String())
}

func proxyHint(ctx *interfaces.CommandContext, c *card) {
	if px := resolveProxy(); px != nil {
		c.hint(ctx.Tlocal("TCP 经代理 ", "TCP via proxy ") + plugin.Code(px.display()))
	}
}

func (p *PingPlugin) pingTarget(ctx *interfaces.CommandContext, target string) error {
	parsed := parseTarget(target)
	if parsed.typ == "invalid" {
		return ctx.Edit(errText(ctx.Tlocal("无效的目标 ", "Invalid target ") + plugin.Code(target)))
	}
	_ = ctx.Edit("🎯 " + plugin.Code(target) + " …")

	base := ctx.Context()
	host := parsed.value // hostname kept for HTTP(S) Host/SNI
	addr := host         // resolved address for TCP / ICMP
	c := newCard("🎯", plugin.Code(target)).blank()

	if parsed.typ == "domain" {
		dc, cancel := context.WithTimeout(base, 5*time.Second)
		start := time.Now()
		ips, err := net.DefaultResolver.LookupIPAddr(dc, host)
		cancel()
		if err != nil || len(ips) == 0 {
			c.field("DNS", ctx.Tlocal("失败", "failed"))
		} else {
			addr = pickIP(ips)
			c.rawField("DNS", plugin.Code(fmtMs(int(time.Since(start).Milliseconds())))+" → "+plugin.Code(addr))
		}
	}

	var (
		wg      sync.WaitGroup
		probe   *tcpProbeResult
		icmp    icmpResult
		icmpErr error
		httpMs  = -1
		httpsMs = -1
	)
	wg.Add(4)
	go func() { defer wg.Done(); icmp, icmpErr = systemPing(base, addr, 3) }()
	go func() { defer wg.Done(); probe = tcpingProbe(base, addr, []int{443, 80, 22, 53}, 3, 3*time.Second) }()
	go func() { defer wg.Done(); httpMs = httpPing(base, host, false) }()
	go func() { defer wg.Done(); httpsMs = httpPing(base, host, true) }()
	wg.Wait()
	icmpOK := icmpErr == nil && icmp.avg >= 0 && icmp.loss < 100

	loss := ctx.Tlocal("丢包", "loss")
	if probe != nil {
		via := ""
		if resolveProxy() != nil {
			via = ctx.Tlocal(" · 经代理", " · proxy")
		}
		c.rawField("TCP", plugin.Code(fmtMs(probe.avg))+esc(fmt.Sprintf("  :%d · %s %d%%%s", probe.port, loss, probe.loss, via)))
	} else {
		c.field("TCP", ctx.Tlocal("不通", "closed"))
	}
	switch {
	case icmpOK:
		c.rawField("ICMP", plugin.Code(fmtMs(icmp.avg))+esc(fmt.Sprintf("  %s %d%%", loss, icmp.loss)))
	case icmpErr != nil:
		c.field("ICMP", ctx.Tlocal("不可用", "n/a"))
	default:
		c.field("ICMP", ctx.Tlocal("超时", "timeout"))
	}
	c.field("HTTP", httpValue(ctx, httpMs))
	c.field("HTTPS", httpValue(ctx, httpsMs))

	if probe == nil && !icmpOK && httpMs < 0 && httpsMs < 0 {
		c.hint(ctx.Tlocal("全部失败，目标可能不可达", "Every probe failed, the target may be unreachable"))
	}
	return ctx.Edit(c.String())
}

func httpValue(ctx *interfaces.CommandContext, ms int) string {
	if ms < 0 {
		return ctx.Tlocal("失败", "failed")
	}
	return fmtMs(ms)
}
