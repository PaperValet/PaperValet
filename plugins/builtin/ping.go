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
	cmds := []*interfaces.Command{
		{
			Name:        "ping",
			Description: "测延迟",
			DescEN:      "Measure latency",
			Usage: "ping [IP|域名|dc1-dc5]\n" +
				"\n" +
				"**用法**\n" +
				"• `ping`  API 往返和编辑消息的耗时\n" +
				"• `ping 1.1.1.1` / `ping example.com`  测指定目标\n" +
				"• `ping dc2`  测单个数据中心\n" +
				"• 全部数据中心用 `pingdc`\n" +
				"\n" +
				"**机制**\n" +
				"• 域名先解析 DNS\n" +
				"• 延迟先测 TCP（443/80/22/53），不通再用 ICMP，最后 HTTP\n" +
				"• 另外测一次 HTTPS 请求耗时\n" +
				"• 设了 ALL\\_PROXY / HTTPS\\_PROXY / HTTP\\_PROXY 时 TCP 走代理",
			UsageEN: "ping [IP|domain|dc1-dc5]\n" +
				"\n" +
				"**Usage**\n" +
				"• `ping`  API round trip and message edit time\n" +
				"• `ping 1.1.1.1` / `ping example.com`  a specific target\n" +
				"• `ping dc2`  one data center\n" +
				"• all data centers: `pingdc`\n" +
				"\n" +
				"**How it works**\n" +
				"• Domains are resolved first\n" +
				"• Latency tries TCP (443/80/22/53), then ICMP, then HTTP\n" +
				"• Also times one HTTPS request\n" +
				"• TCP goes through ALL\\_PROXY / HTTPS\\_PROXY / HTTP\\_PROXY when set",
			Plugin:   p.Name(),
			Category: "core",
			Handler:  p.handle,
		},
		{
			Name:        "pingdc",
			Description: "测各数据中心延迟",
			DescEN:      "Data center latency",
			Usage: "pingdc\n" +
				"\n" +
				"**机制**\n" +
				"• 五个数据中心同时测，每个先测 TCP 443/80，不通再用 ICMP\n" +
				"• 测的是本机到数据中心，和你的客户端网络无关",
			UsageEN: "pingdc\n" +
				"\n" +
				"**How it works**\n" +
				"• Probes all five in parallel: TCP 443/80 first, ICMP as fallback\n" +
				"• Measures this server to each data center, not your client",
			Plugin:   p.Name(),
			Category: "core",
			Handler:  p.pingAllDCs,
		},
	}
	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *PingPlugin) Start(_ context.Context) error { return nil }
func (p *PingPlugin) Stop(_ context.Context) error  { return nil }

func (p *PingPlugin) handle(ctx *interfaces.CommandContext) error {
	target := strings.ToLower(strings.TrimSpace(ctx.GetArg(0)))
	if target == "" {
		return p.pingTelegram(ctx)
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

// dcPlaces names each data center's city.
var dcPlaces = map[int][2]string{
	1: {"迈阿密", "Miami"},
	2: {"阿姆斯特丹", "Amsterdam"},
	3: {"迈阿密", "Miami"},
	4: {"阿姆斯特丹", "Amsterdam"},
	5: {"新加坡", "Singapore"},
}

func dcLabel(ctx *interfaces.CommandContext, dc int) string {
	return fmt.Sprintf("DC%d %s", dc, ctx.Tlocal(dcPlaces[dc][0], dcPlaces[dc][1]))
}

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
		c.field(dcLabel(ctx, dc), dcValue(ctx, results[dc].ms, results[dc].ok))
	}
	proxyHint(ctx, c)
	return ctx.Edit(c.String())
}

func proxyHint(ctx *interfaces.CommandContext, c *card) {
	if px := resolveProxy(); px != nil {
		c.hint(ctx.Tlocal("TCP 经代理 ", "TCP via proxy ") + plugin.Code(px.display()))
	}
}

// pingTarget follows TeleBox: DNS for domains, one headline latency
// (TCP, else ICMP, else HTTP), then an HTTPS request time.
func (p *PingPlugin) pingTarget(ctx *interfaces.CommandContext, target string) error {
	parsed := parseTarget(target)
	if parsed.typ == "invalid" {
		return ctx.Edit(errText(ctx.Tlocal("无效的目标 ", "Invalid target ") + plugin.Code(target)))
	}
	_ = ctx.Edit("🔍 " + plugin.Code(target) + " …")

	base := ctx.Context()
	host := parsed.value // hostname kept for HTTP(S) Host/SNI
	addr := host         // resolved address for TCP / ICMP

	kind := map[string][2]string{
		"ip":     {"IP 延迟", "IP latency"},
		"domain": {"域名延迟", "Domain latency"},
		"dc":     {"数据中心延迟", "Data center latency"},
	}[parsed.typ]
	c := newCard("🎯", ctx.Tlocal(kind[0], kind[1]))
	if host == target {
		c.line(plugin.Code(target))
	} else {
		c.line(plugin.Code(target) + " → " + plugin.Code(host))
	}
	c.blank()

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
	go func() { defer wg.Done(); probe = tcpingProbe(base, addr, []int{443, 80, 22, 53}, 3, 3*time.Second) }()
	go func() { defer wg.Done(); icmp, icmpErr = systemPing(base, addr, 3) }()
	go func() { defer wg.Done(); httpMs = httpPing(base, host, false) }()
	go func() { defer wg.Done(); httpsMs = httpPing(base, host, true) }()
	wg.Wait()

	latency := ctx.Tlocal("延迟", "Latency")
	loss := ctx.Tlocal("丢包", "loss")
	switch {
	case probe != nil:
		note := fmt.Sprintf("  TCP %d · %s %d%%", probe.port, loss, probe.loss)
		if resolveProxy() != nil {
			note += ctx.Tlocal(" · 经代理", " · via proxy")
		}
		c.rawField(latency, plugin.Code(fmtMs(probe.avg))+esc(note))
	case icmpErr == nil && icmp.avg >= 0 && icmp.loss < 100:
		c.rawField(latency, plugin.Code(fmtMs(icmp.avg))+esc(fmt.Sprintf("  ICMP · %s %d%%", loss, icmp.loss)))
	case httpMs >= 0:
		c.rawField(latency, plugin.Code(fmtMs(httpMs))+"  HTTP")
	default:
		c.field(latency, ctx.Tlocal("不可达", "unreachable"))
	}
	if httpsMs >= 0 {
		c.field("HTTPS", fmtMs(httpsMs))
	}
	return ctx.Edit(c.String())
}
