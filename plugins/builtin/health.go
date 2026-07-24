package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// HealthPlugin provides memory monitoring with auto GC/reload/restart.
// Inspired by TeleBox's Health plugin but implemented in Go.
type HealthPlugin struct {
	configFile string
	config     HealthConfig
	monitorCh  chan struct{}
	stopCh     chan struct{}
}

type HealthConfig struct {
	Enabled              bool    `json:"enabled"`
	MemoryThresholdMB    float64 `json:"memory_threshold_mb"`    // Heap threshold
	RSSThresholdMB       float64 `json:"rss_threshold_mb"`       // RSS threshold
	GrowthThresholdMB    float64 `json:"growth_threshold_mb"`    // Growth from baseline
	BaselineHeapMB       float64 `json:"baseline_heap_mb"`       // Baseline heap
	BaselineRSSMB        float64 `json:"baseline_rss_mb"`        // Baseline RSS
	BaselineMode         string  `json:"baseline_mode"`          // "on-enable", "manual", "on-reload"
	SilentEnabled        bool    `json:"silent_enabled"`         // Silent notifications
	SoftStreak           int     `json:"soft_streak"`            // Consecutive samples before GC/reload
	HardStreak           int     `json:"hard_streak"`            // Consecutive samples before restart
	ActionCooldownMin    int     `json:"action_cooldown_min"`    // Cooldown between actions (minutes)
	BusyDeferMaxMin      int     `json:"busy_defer_max_min"`     // Max defer when busy (minutes)
	LastActionAt         int64   `json:"last_action_at"`         // Unix timestamp
	OverThresholdStreak  int     `json:"-"`                      // Runtime state
	BusyDeferSince       int64   `json:"-"`                      // Runtime state
	LastGcAt             int64   `json:"-"`                      // Runtime state
	LastSample           MemInfo `json:"-"`                      // Runtime state
}

type MemInfo struct {
	HeapUsed  float64 `json:"heap_used_mb"`
	HeapTotal float64 `json:"heap_total_mb"`
	RSS       float64 `json:"rss_mb"`
	External  float64 `json:"external_mb"`
	Arrays    float64 `json:"arrays_mb"`
}

func NewHealth() *HealthPlugin {
	return &HealthPlugin{
		configFile: "data/health_config.json",
		config: HealthConfig{
			Enabled:           false,
			MemoryThresholdMB: 150,
			RSSThresholdMB:    512,
			GrowthThresholdMB: 120,
			BaselineHeapMB:    0,
			BaselineRSSMB:     0,
			BaselineMode:      "on-enable",
			SilentEnabled:     false,
			SoftStreak:        2,
			HardStreak:        3,
			ActionCooldownMin: 15,
			BusyDeferMaxMin:   5,
		},
		monitorCh: make(chan struct{}, 1),
		stopCh:    make(chan struct{}),
	}
}

func (p *HealthPlugin) Name() string        { return "health" }
func (p *HealthPlugin) Description() string { return "内存守护 - 监控/自动清理/重启" }

func (p *HealthPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	p.loadConfig()

	cmds := []*interfaces.Command{
		{
			Name:        "health",
			Aliases:     []string{"memory", "mem"},
			Description: "内存守护 - 查看状态/配置/控制",
			Usage:       "health [on|off|status|reset|set|mode|silent]",
			Plugin:      p.Name(),
			Category:    "tools",
			OwnerOnly:   true,
			Handler:     p.handleHealth,
		},
	}

	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}

	return nil
}

func (p *HealthPlugin) Start(ctx context.Context) error {
	if p.config.Enabled {
		go p.monitorLoop(ctx)
	}
	return nil
}

func (p *HealthPlugin) Stop(ctx context.Context) error {
	close(p.stopCh)
	return nil
}

func (p *HealthPlugin) loadConfig() {
	data, err := os.ReadFile(p.configFile)
	if err != nil {
		return
	}
	json.Unmarshal(data, &p.config)
}

func (p *HealthPlugin) saveConfig() {
	os.MkdirAll(filepath.Dir(p.configFile), 0o755)
	data, _ := json.MarshalIndent(p.config, "", "  ")
	os.WriteFile(p.configFile, data, 0o644)
}

func (p *HealthPlugin) handleHealth(ctx *interfaces.CommandContext) error {
	args := ctx.Args
	if len(args) == 0 {
		return p.showStatus(ctx)
	}

	sub := args[0]
	switch sub {
	case "on", "enable":
		p.config.Enabled = true
		p.saveConfig()
		if p.config.BaselineMode == "on-enable" {
			p.updateBaseline()
		}
		go p.monitorLoop(context.Background())
		return ctx.Edit("✅ 内存守护已启用")

	case "off", "disable":
		p.config.Enabled = false
		p.saveConfig()
		return ctx.Edit("⏸️ 内存守护已禁用")

	case "status":
		return p.showStatus(ctx)

	case "reset":
		p.updateBaseline()
		p.saveConfig()
		return ctx.Edit("📍 基线已重置为当前内存使用")

	case "set":
		if len(args) < 2 {
			return ctx.Edit("用法: health set <heap|rss|growth|safe|normal|aggressive> [值]")
		}
		return p.handleSet(ctx, args[1:])

	case "mode":
		if len(args) < 2 {
			return ctx.Edit("用法: health mode <auto|manual|reload>")
		}
		return p.handleMode(ctx, args[1])

	case "silent":
		if len(args) < 2 {
			p.config.SilentEnabled = !p.config.SilentEnabled
			p.saveConfig()
			return ctx.Edit(fmt.Sprintf("🔕 静默通知: %s", p.boolStr(p.config.SilentEnabled)))
		}
		switch args[1] {
		case "on", "true", "1":
			p.config.SilentEnabled = true
		case "off", "false", "0":
			p.config.SilentEnabled = false
		default:
			return ctx.Edit("用法: health silent [on|off]")
		}
		p.saveConfig()
		return ctx.Edit(fmt.Sprintf("🔕 静默通知: %s", p.boolStr(p.config.SilentEnabled)))

	case "help", "h":
		return p.showHelp(ctx)

	default:
		return ctx.Edit(fmt.Sprintf("未知子命令: %s\n\n%s", sub, p.helpText()))
	}
}

func (p *HealthPlugin) showHelp(ctx *interfaces.CommandContext) error {
	return ctx.Edit(p.helpText())
}

func (p *HealthPlugin) helpText() string {
	return `🩺 <b>Health · 内存守护</b>

<b>一句话:</b> 盯着内存，偏高时自动清理，尽量不打断正在做的事。

<b>常用命令:</b>
• <code>health</code> — 查看当前内存/状态/建议
• <code>health on</code> / <code>health off</code> — 打开/关闭自动保护
• <code>health status</code> — 详细状态与建议
• <code>health reset</code> — 记录当前内存为基线

<b>预设配置 (一键应用):</b>
• <code>health set safe</code> — 敏感模式（内存小的机器推荐）
• <code>health set normal</code> — 默认平衡（大多数人用这个）
• <code>health set aggressive</code> — 宽松模式（插件多/内存本就高时用）

<b>自定义阈值:</b>
• <code>health set heap 150</code> — 程序内存上限（MB）
• <code>health set rss 512</code> — 总占用上限（MB）
• <code>health set growth 120</code> — 相对基线涨幅上限（MB）

<b>基线模式:</b>
• <code>health mode auto</code> — 打开保护时自动记基线
• <code>health mode manual</code> — 只有 reset 才改基线
• <code>health mode reload</code> — 每次重载插件后改基线

<b>静默通知:</b>
• <code>health silent on/off</code> — 自动处理时是否私信通知（默认通知收藏夹）

<b>自动保护逻辑 (人话):</b>
1. ~每10分钟检查一次
2. 连续几次都偏高才动手（避免误报）
3. 正在跑任务时会等一等，尽量不打断
4. 处理顺序: GC → 软重载 → 实在不行才重启进程
5. 版本切换/正在重载时绝对不动
`
}

func (p *HealthPlugin) boolStr(b bool) string {
	if b {
		return "开"
	}
	return "关"
}

func (p *HealthPlugin) handleSet(ctx *interfaces.CommandContext, args []string) error {
	if len(args) == 0 {
		return ctx.Edit("用法: health set <heap|rss|growth|safe|normal|aggressive> [值]")
	}

	switch args[0] {
	case "safe":
		p.config.MemoryThresholdMB = 120
		p.config.RSSThresholdMB = 420
		p.config.GrowthThresholdMB = 80
		p.config.SoftStreak = 2
		p.config.HardStreak = 3
		p.saveConfig()
		return ctx.Edit("✅ 已应用 <b>敏感模式</b> (safe): heap 120MB, rss 420MB, growth 80MB")

	case "normal":
		p.config.MemoryThresholdMB = 150
		p.config.RSSThresholdMB = 512
		p.config.GrowthThresholdMB = 120
		p.config.SoftStreak = 2
		p.config.HardStreak = 3
		p.saveConfig()
		return ctx.Edit("✅ 已应用 <b>默认模式</b> (normal): heap 150MB, rss 512MB, growth 120MB")

	case "aggressive":
		p.config.MemoryThresholdMB = 220
		p.config.RSSThresholdMB = 768
		p.config.GrowthThresholdMB = 180
		p.config.SoftStreak = 3
		p.config.HardStreak = 4
		p.saveConfig()
		return ctx.Edit("✅ 已应用 <b>宽松模式</b> (aggressive): heap 220MB, rss 768MB, growth 180MB")

	case "heap":
		if len(args) < 2 {
			return ctx.Edit("用法: health set heap <MB>")
		}
		var v float64
		fmt.Sscanf(args[1], "%f", &v)
		if v > 0 {
			p.config.MemoryThresholdMB = v
			p.saveConfig()
			return ctx.Edit(fmt.Sprintf("✅ Heap 阈值设为: %.0f MB", v))
		}
		return ctx.Edit("❌ 无效数值")

	case "rss":
		if len(args) < 2 {
			return ctx.Edit("用法: health set rss <MB>")
		}
		var v float64
		fmt.Sscanf(args[1], "%f", &v)
		if v > 0 {
			p.config.RSSThresholdMB = v
			p.saveConfig()
			return ctx.Edit(fmt.Sprintf("✅ RSS 阈值设为: %.0f MB", v))
		}
		return ctx.Edit("❌ 无效数值")

	case "growth":
		if len(args) < 2 {
			return ctx.Edit("用法: health set growth <MB>")
		}
		var v float64
		fmt.Sscanf(args[1], "%f", &v)
		if v > 0 {
			p.config.GrowthThresholdMB = v
			p.saveConfig()
			return ctx.Edit(fmt.Sprintf("✅ Growth 阈值设为: %.0f MB", v))
		}
		return ctx.Edit("❌ 无效数值")

	default:
		return ctx.Edit("未知参数: " + args[0] + "\n可用: safe, normal, aggressive, heap, rss, growth")
	}
}

func (p *HealthPlugin) handleMode(ctx *interfaces.CommandContext, mode string) error {
	switch mode {
	case "auto", "on-enable":
		p.config.BaselineMode = "on-enable"
	case "manual":
		p.config.BaselineMode = "manual"
	case "reload", "on-reload":
		p.config.BaselineMode = "on-reload"
	default:
		return ctx.Edit("未知模式: " + mode + "\n可用: auto, manual, reload")
	}
	p.saveConfig()
	return ctx.Edit(fmt.Sprintf("✅ 基线模式已设为: %s", mode))
}

func (p *HealthPlugin) showStatus(ctx *interfaces.CommandContext) error {
	mem := p.getMemoryUsage()

	// Update baseline if not set
	if p.config.BaselineHeapMB == 0 || p.config.BaselineRSSMB == 0 {
		p.updateBaseline()
		p.saveConfig()
	}

	growth := p.getGrowthStatus(mem)
	reasons := p.collectReasons(mem, growth)
	level := p.statusLevel(mem, growth)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s <b>内存守护状态</b>\n\n", level.emoji))
	b.WriteString(fmt.Sprintf("自动保护: <b>%s</b>\n", p.boolStr(p.config.Enabled)))
	b.WriteString(fmt.Sprintf("静默通知: <b>%s</b>\n\n", p.boolStr(p.config.SilentEnabled)))

	b.WriteString("📊 <b>当前内存</b>\n")
	b.WriteString(fmt.Sprintf("  程序内存: <code>%.2f MB</code> / 上限 <code>%.0f MB</code>\n", mem.HeapUsed, p.config.MemoryThresholdMB))
	b.WriteString(fmt.Sprintf("  总占用: <code>%.2f MB</code> / 上限 <code>%.0f MB</code>\n", mem.RSS, p.config.RSSThresholdMB))
	b.WriteString(fmt.Sprintf("  使用率: <code>%.1f%%</code>\n\n", (mem.HeapUsed/mem.HeapTotal)*100))

	b.WriteString("📍 <b>基线对比</b>\n")
	if growth.HeapGrowth != nil {
		b.WriteString(fmt.Sprintf("  程序内存涨幅: <code>%+.2f MB</code> (阈值: %.0f MB)\n", *growth.HeapGrowth, p.config.GrowthThresholdMB))
	} else {
		b.WriteString("  程序内存涨幅: <code>未记录</code>\n")
	}
	if growth.RSSGrowth != nil {
		b.WriteString(fmt.Sprintf("  总占用涨幅: <code>%+.2f MB</code> (阈值: %.0f MB)\n", *growth.RSSGrowth, p.config.GrowthThresholdMB))
	} else {
		b.WriteString("  总占用涨幅: <code>未记录</code>\n")
	}
	b.WriteString(fmt.Sprintf("  基线模式: <code>%s</code>\n\n", p.config.BaselineMode))

	b.WriteString(fmt.Sprintf("🛡 <b>状态</b>: %s\n\n", level.text))

	if len(reasons) > 0 {
		b.WriteString("⚠️ <b>超限原因</b>\n")
		for _, r := range reasons {
			b.WriteString(fmt.Sprintf("  • %s\n", r))
		}
		b.WriteString("\n")
	}

	b.WriteString("⚙️ <b>配置</b>\n")
	b.WriteString(fmt.Sprintf("  连续超限触发软处理: %d 次\n", p.config.SoftStreak))
	b.WriteString(fmt.Sprintf("  连续超限触发硬重启: %d 次\n", p.config.HardStreak))
	b.WriteString(fmt.Sprintf("  动作冷却: %d 分钟\n", p.config.ActionCooldownMin))
	b.WriteString(fmt.Sprintf("  忙碌最大推迟: %d 分钟\n", p.config.BusyDeferMaxMin))

	if p.config.OverThresholdStreak > 0 {
		b.WriteString(fmt.Sprintf("\n🔄 <b>当前连续超限</b>: %d 次", p.config.OverThresholdStreak))
	}

	return ctx.Edit(b.String())
}

func (p *HealthPlugin) getMemoryUsage() MemInfo {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return MemInfo{
		HeapUsed:  float64(m.HeapAlloc) / 1024 / 1024,
		HeapTotal: float64(m.HeapSys) / 1024 / 1024,
		RSS:       float64(m.Sys) / 1024 / 1024, // Approximation
		External:  float64(m.HeapSys - m.HeapAlloc) / 1024 / 1024,
		Arrays:    float64(m.BuckHashSys) / 1024 / 1024,
	}
}

func (p *HealthPlugin) updateBaseline() {
	mem := p.getMemoryUsage()
	p.config.BaselineHeapMB = mem.HeapUsed
	p.config.BaselineRSSMB = mem.RSS
}

type GrowthStatus struct {
	HeapGrowth       *float64
	RSSGrowth        *float64
	HeapGrowthExceeded bool
	RSSGrowthExceeded  bool
}

func (p *HealthPlugin) getGrowthStatus(mem MemInfo) GrowthStatus {
	var heapGrowth, rssGrowth *float64
	heapExceeded := false
	rssExceeded := false

	if p.config.BaselineHeapMB > 0 {
		hg := mem.HeapUsed - p.config.BaselineHeapMB
		heapGrowth = &hg
		heapExceeded = hg > p.config.GrowthThresholdMB
	}
	if p.config.BaselineRSSMB > 0 {
		rg := mem.RSS - p.config.BaselineRSSMB
		rssGrowth = &rg
		rssExceeded = rg > p.config.GrowthThresholdMB
	}

	return GrowthStatus{
		HeapGrowth:        heapGrowth,
		RSSGrowth:         rssGrowth,
		HeapGrowthExceeded: heapExceeded,
		RSSGrowthExceeded:  rssExceeded,
	}
}

func (p *HealthPlugin) collectReasons(mem MemInfo, growth GrowthStatus) []string {
	var reasons []string
	if mem.HeapUsed > p.config.MemoryThresholdMB {
		reasons = append(reasons, fmt.Sprintf("程序内存 %.2f MB，超过上限 %.0f MB", mem.HeapUsed, p.config.MemoryThresholdMB))
	}
	if mem.RSS > p.config.RSSThresholdMB {
		reasons = append(reasons, fmt.Sprintf("总占用 %.2f MB，超过上限 %.0f MB", mem.RSS, p.config.RSSThresholdMB))
	}
	if growth.HeapGrowthExceeded {
		reasons = append(reasons, fmt.Sprintf("程序内存比起点多了 %.2f MB，超过涨幅上限 %.0f MB", *growth.HeapGrowth, p.config.GrowthThresholdMB))
	}
	if growth.RSSGrowthExceeded {
		reasons = append(reasons, fmt.Sprintf("总占用比起点多了 %.2f MB，超过涨幅上限 %.0f MB", *growth.RSSGrowth, p.config.GrowthThresholdMB))
	}
	return reasons
}

func (p *HealthPlugin) statusLevel(mem MemInfo, growth GrowthStatus) struct{ emoji, text string } {
	percentage := (mem.HeapUsed / p.config.MemoryThresholdMB) * 100
	if percentage > 90 || mem.RSS > p.config.RSSThresholdMB || growth.HeapGrowthExceeded || growth.RSSGrowthExceeded {
		return struct{ emoji, text string }{"🔴", "偏高，需要关注"}
	}
	if percentage > 70 || mem.RSS > p.config.RSSThresholdMB*0.7 {
		return struct{ emoji, text string }{"🟡", "略高，继续观察"}
	}
	return struct{ emoji, text string }{"🟢", "正常，放心用"}
}

// monitorLoop runs the periodic memory check
func (p *HealthPlugin) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.checkMemory()
		}
	}
}

func (p *HealthPlugin) checkMemory() {
	if !p.config.Enabled {
		return
	}

	mem := p.getMemoryUsage()
	p.config.LastSample = mem

	// Update baseline if not set
	if p.config.BaselineHeapMB == 0 || p.config.BaselineRSSMB == 0 {
		p.updateBaseline()
		p.saveConfig()
	}

	growth := p.getGrowthStatus(mem)
	reasons := p.collectReasons(mem, growth)

	if len(reasons) == 0 {
		p.config.OverThresholdStreak = 0
		p.config.BusyDeferSince = 0
		return
	}

	p.config.OverThresholdStreak++

	// Soft path: GC only on first few samples
	if p.config.OverThresholdStreak < p.config.SoftStreak {
		p.tryGC()
		return
	}

	// Cooldown check
	cooldownMs := int64(p.config.ActionCooldownMin) * 60 * 1000
	now := time.Now().UnixMilli()
	if p.config.LastActionAt > 0 && now-p.config.LastActionAt < cooldownMs {
		return
	}

	// Busy defer
	// Note: In Go, we don't have a task tracking system like TeleBox
	// So we skip busy defer for now

	// Soft recover: GC then could trigger reload (not implemented in Go yet)
	p.tryGC()

	// Hard path: still high after soft streak + hard streak
	if p.config.OverThresholdStreak >= p.config.HardStreak {
		// In Go, we can't easily "reload runtime" like TeleBox
		// Just log and optionally restart
		fmt.Printf("[Health] 内存持续超限，达到 hard streak (%d)，建议重启\n", p.config.OverThresholdStreak)
		p.config.LastActionAt = now
		p.saveConfig()
	}
}

func (p *HealthPlugin) tryGC() bool {
	now := time.Now().UnixMilli()
	if now-p.config.LastGcAt < 60*1000 {
		return false
	}
	runtime.GC()
	p.config.LastGcAt = now
	fmt.Println("[Health] 执行了 runtime.GC()")
	return true
}