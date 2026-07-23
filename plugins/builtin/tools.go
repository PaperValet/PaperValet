package builtin

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/gotd/td/tg"
	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// PingPlugin provides ping/pong command.
type PingPlugin struct{}

func NewPing() *PingPlugin { return &PingPlugin{} }

func (p *PingPlugin) Name() string        { return "ping" }
func (p *PingPlugin) Description() string { return "Ping/pong latency check" }

func (p *PingPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "ping",
		Description: "检查延迟",
		Plugin:      p.Name(),
		Category:    "tools",
		Handler:     p.handlePing,
	})
}

func (p *PingPlugin) Start(_ context.Context) error { return nil }
func (p *PingPlugin) Stop(_ context.Context) error  { return nil }

func (p *PingPlugin) handlePing(ctx *interfaces.CommandContext) error {
	start := time.Now()
	msg := "Pong! 🏓"
	if err := ctx.Edit(msg); err != nil {
		return err
	}
	latency := time.Since(start)
	return ctx.Edit(fmt.Sprintf("%s\n延迟: %v", msg, latency))
}

// UptimePlugin shows bot uptime.
type UptimePlugin struct {
	startTime time.Time
}

func NewUptime() *UptimePlugin { return &UptimePlugin{startTime: time.Now()} }

func (p *UptimePlugin) Name() string        { return "uptime" }
func (p *UptimePlugin) Description() string { return "显示运行时间" }

func (p *UptimePlugin) Init(_ context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "uptime",
		Aliases:     []string{"up"},
		Description: "显示运行时间",
		Plugin:      p.Name(),
		Category:    "tools",
		Handler:     p.handleUptime,
	})
}

func (p *UptimePlugin) Start(_ context.Context) error { return nil }
func (p *UptimePlugin) Stop(_ context.Context) error  { return nil }

func (p *UptimePlugin) handleUptime(ctx *interfaces.CommandContext) error {
	uptime := time.Since(p.startTime).Truncate(time.Second)
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return ctx.Edit(fmt.Sprintf(
		"⏱ 运行时间: %s\n🧠 内存: %.1f MB\n🔀 Goroutines: %d",
		uptime, float64(mem.Alloc)/1024/1024, runtime.NumGoroutine(),
	))
}

// InfoPlugin shows user/chat info.
type InfoPlugin struct{}

func NewInfo() *InfoPlugin { return &InfoPlugin{} }

func (p *InfoPlugin) Name() string        { return "info" }
func (p *InfoPlugin) Description() string { return "显示用户/群组信息" }

func (p *InfoPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "info",
		Aliases:     []string{"id", "whois"},
		Description: "显示用户/群组 ID 信息",
		Usage:       "info [@用户名|回复消息]",
		Plugin:      p.Name(),
		Category:    "tools",
		Handler:     p.handleInfo,
	})
}

func (p *InfoPlugin) Start(_ context.Context) error { return nil }
func (p *InfoPlugin) Stop(_ context.Context) error  { return nil }

func (p *InfoPlugin) handleInfo(ctx *interfaces.CommandContext) error {
	msg := ctx.Message
	var targetID int64 = msg.UserID

	if ctx.ArgCount() > 0 {
		arg := ctx.GetArg(0)
		if len(arg) > 0 && arg[0] == '@' {
			return ctx.Edit("用户名解析暂未实现，请回复消息或使用 .info")
		}
	} else if msg.IsReply && msg.Message != nil {
		if msg.Message.FromID != nil {
			if u, ok := msg.Message.FromID.(*tg.PeerUser); ok {
				targetID = u.UserID
			}
		}
	}

	return ctx.Edit(fmt.Sprintf(
		"👤 用户 ID: %d\n💬 群组 ID: %d\n📨 消息 ID: %d",
		targetID, msg.ChatID, msg.Message.ID,
	))
}

// ForwardPlugin forwards messages.
type ForwardPlugin struct{}

func NewForward() *ForwardPlugin { return &ForwardPlugin{} }

func (p *ForwardPlugin) Name() string        { return "forward" }
func (p *ForwardPlugin) Description() string { return "转发消息" }

func (p *ForwardPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "fwd",
		Aliases:     []string{"forward"},
		Description: "转发回复的消息到目标",
		Usage:       "fwd @目标用户名",
		Plugin:      p.Name(),
		Category:    "tools",
		Handler:     p.handleForward,
	})
}

func (p *ForwardPlugin) Start(_ context.Context) error { return nil }
func (p *ForwardPlugin) Stop(_ context.Context) error  { return nil }

func (p *ForwardPlugin) handleForward(ctx *interfaces.CommandContext) error {
	if !ctx.Message.IsReply {
		return ctx.Edit("请回复一条消息")
	}
	if ctx.ArgCount() == 0 {
		return ctx.Edit("用法: fwd @目标用户名")
	}
	target := ctx.GetArg(0)
	return ctx.Edit(fmt.Sprintf("转发功能待实现: %s", target))
}