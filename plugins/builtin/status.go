package builtin

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/internal/plugin/loader"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// StatusPlugin reports version, host and resource usage.
type StatusPlugin struct {
	version   string
	mgr       plugin.Manager
	loader    *loader.Loader
	startTime time.Time
}

func NewStatus(version string, l *loader.Loader) *StatusPlugin {
	return &StatusPlugin{version: version, loader: l, startTime: time.Now()}
}

func (p *StatusPlugin) Name() string        { return "status" }
func (p *StatusPlugin) Description() string { return "运行状态" }
func (p *StatusPlugin) DescEN() string      { return "Runtime status" }

func (p *StatusPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	p.mgr = mgr
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "status",
		Description: "查看运行状态",
		DescEN:      "Show runtime status",
		Usage: `status

<b>显示</b>
• 版本：PaperValet、Go、gotd
• 主机：主机名、系统、内核、架构
• 资源：CPU 核数与负载、进程内存、系统内存、磁盘
• 运行：本次运行时长、系统开机时长、外部插件数、扫描耗时

<b>机制</b>
• 系统信息读自 /proc 和 statfs，Linux 以外的系统只显示能拿到的项
• 进程内存是常驻内存 RSS，比 Go 堆大，是真实占用
• 外部插件数不含内建插件`,
		UsageEN: `status

<b>Shows</b>
• versions: PaperValet, Go, gotd
• host: hostname, OS, kernel, arch
• resources: CPU cores and load, process memory, system memory, disk
• runtime: uptime, system uptime, external plugins, scan time

<b>How it works</b>
• System data comes from /proc and statfs; non-Linux systems show what is available
• Process memory is resident RSS, larger than the Go heap and the real footprint
• The plugin count excludes built-ins`,
		Plugin:   p.Name(),
		Category: "core",
		Handler:  p.handleStatus,
	})
}

func (p *StatusPlugin) Start(_ context.Context) error { return nil }
func (p *StatusPlugin) Stop(_ context.Context) error  { return nil }

// humanDuration renders 3d 4h 5m style durations.
func humanDuration(d time.Duration, en bool) string {
	d = d.Round(time.Second)
	days := int(d.Hours()) / 24
	h := int(d.Hours()) % 24
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	var parts []string
	unit := func(n int, zh, e string) {
		if n > 0 {
			if en {
				parts = append(parts, fmt.Sprintf("%d%s", n, e))
			} else {
				parts = append(parts, fmt.Sprintf("%d%s", n, zh))
			}
		}
	}
	unit(days, "天", "d")
	unit(h, "小时", "h")
	unit(m, "分", "m")
	if days == 0 && h == 0 {
		unit(s, "秒", "s")
	}
	if len(parts) == 0 {
		return "0s"
	}
	return strings.Join(parts, " ")
}

// readKV parses "Key: value kB" style files like /proc/meminfo.
func readKV(path string) map[string]int64 {
	out := map[string]int64{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		fields := strings.Fields(v)
		if len(fields) == 0 {
			continue
		}
		n, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			continue
		}
		if len(fields) > 1 && fields[1] == "kB" {
			n *= 1024
		}
		out[k] = n
	}
	return out
}

func firstLine(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	line, _, _ := strings.Cut(string(data), "\n")
	return strings.TrimSpace(line)
}

// osName reads PRETTY_NAME from os-release.
func osName() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return runtime.GOOS
	}
	for _, l := range strings.Split(string(data), "\n") {
		if v, ok := strings.CutPrefix(l, "PRETTY_NAME="); ok {
			return strings.Trim(v, `"`)
		}
	}
	return runtime.GOOS
}

func kernel() string {
	if v := firstLine("/proc/sys/kernel/osrelease"); v != "" {
		return v
	}
	return "—"
}

func pct(used, total int64) string {
	if total <= 0 {
		return "—"
	}
	return fmt.Sprintf("%s / %s (%.0f%%)", formatBytes(used), formatBytes(total), float64(used)*100/float64(total))
}

func gotdVersion() string {
	if bi, ok := readBuildInfo(); ok {
		for _, d := range bi.Deps {
			if d.Path == "github.com/gotd/td" {
				return d.Version
			}
		}
	}
	return "—"
}

func (p *StatusPlugin) handleStatus(ctx *interfaces.CommandContext) error {
	start := time.Now()
	en := ctx.Lang == "en-US"
	host, _ := os.Hostname()

	proc := readKV("/proc/self/status")
	mem := readKV("/proc/meminfo")
	var heap runtime.MemStats
	runtime.ReadMemStats(&heap)

	external := 0
	if p.loader != nil {
		external = len(p.loader.GetLoaded())
	}

	c := newCard("📊", ctx.Tlocal("PaperValet 运行状态", "PaperValet status"))

	c.section(ctx.Tlocal("📦 版本", "📦 Versions"))
	c.field("PaperValet", p.version)
	c.field("Go", strings.TrimPrefix(runtime.Version(), "go"))
	c.field("gotd", gotdVersion())

	c.section(ctx.Tlocal("🏠 主机", "🏠 Host"))
	c.field(ctx.Tlocal("主机名", "Hostname"), host)
	c.field(ctx.Tlocal("系统", "OS"), osName())
	c.field(ctx.Tlocal("内核", "Kernel"), kernel())
	c.field(ctx.Tlocal("架构", "Arch"), runtime.GOOS+"/"+runtime.GOARCH)

	c.section(ctx.Tlocal("📈 资源", "📈 Resources"))
	load := firstLine("/proc/loadavg")
	if f := strings.Fields(load); len(f) >= 3 {
		load = strings.Join(f[:3], " ")
	} else {
		load = "—"
	}
	c.field("CPU", fmt.Sprintf("%d %s · %s %s", runtime.NumCPU(), ctx.Tlocal("核", "cores"), ctx.Tlocal("负载", "load"), load))
	if rss := proc["VmRSS"]; rss > 0 {
		c.field(ctx.Tlocal("进程内存", "Process"), fmt.Sprintf("%s · %s %s", formatBytes(rss), ctx.Tlocal("堆", "heap"), formatBytes(int64(heap.HeapAlloc))))
	} else {
		c.field(ctx.Tlocal("进程内存", "Process"), ctx.Tlocal("堆 ", "heap ")+formatBytes(int64(heap.HeapAlloc)))
	}
	if total := mem["MemTotal"]; total > 0 {
		c.field(ctx.Tlocal("系统内存", "Memory"), pct(total-mem["MemAvailable"], total))
		if st := mem["SwapTotal"]; st > 0 {
			c.field("Swap", pct(st-mem["SwapFree"], st))
		}
	}
	c.rawField(ctx.Tlocal("磁盘", "Disk"), diskUsage())

	c.section(ctx.Tlocal("⏱ 运行", "⏱ Runtime"))
	c.field(ctx.Tlocal("已运行", "Uptime"), humanDuration(time.Since(p.startTime), en))
	if up := firstLine("/proc/uptime"); up != "" {
		if secs, err := strconv.ParseFloat(strings.Fields(up)[0], 64); err == nil {
			c.field(ctx.Tlocal("系统开机", "System up"), humanDuration(time.Duration(secs)*time.Second, en))
		}
	}
	c.field(ctx.Tlocal("外部插件", "Plugins"), external)
	c.field(ctx.Tlocal("协程", "Goroutines"), runtime.NumGoroutine())
	c.field(ctx.Tlocal("扫描耗时", "Scan"), time.Since(start).Round(time.Millisecond))
	return ctx.Edit(c.String())
}
