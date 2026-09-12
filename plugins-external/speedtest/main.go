package main

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type SpeedtestPlugin struct{}

func New() *SpeedtestPlugin { return &SpeedtestPlugin{} }

func (p *SpeedtestPlugin) Name() string        { return "speedtest" }
func (p *SpeedtestPlugin) Description() string { return "网络速度测试" }

func (p *SpeedtestPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	cmds := []*plugin.Command{
		{
			Name:        "speedtest",
			Aliases:     []string{"st", "网速", "speed"},
			Description: "测试网络速度 (需要 speedtest-cli 或 iperf3)",
			Usage:       "speedtest [simple|full]",
			Plugin:      p.Name(),
			Category:    "tools",
			OwnerOnly:   false,
			Handler:     p.handleSpeedtest,
		},
	}
	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *SpeedtestPlugin) Start(ctx context.Context) error { return nil }
func (p *SpeedtestPlugin) Stop(ctx context.Context) error  { return nil }

func (p *SpeedtestPlugin) handleSpeedtest(ctx *plugin.CommandContext) error {
	mode := "simple"
	if len(ctx.Args) > 0 {
		mode = ctx.Args[0]
	}

	// Locate a speedtest binary: prefer speedtest-cli, fall back to speedtest.
	bin := ""
	for _, candidate := range []string{"speedtest-cli", "speedtest"} {
		if p, err := exec.LookPath(candidate); err == nil {
			bin = p
			break
		}
	}
	if bin == "" {
		return ctx.Edit("❌ 未找到 speedtest-cli/speedtest，请先安装（如 <code>apt install speedtest-cli</code>）")
	}

	args := []string{}
	if mode == "full" {
		args = append(args, "--bytes")
	}

	out, err := exec.Command(bin, args...).CombinedOutput()
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 测速失败: %v\n<pre>%s</pre>", err, truncate(string(out), 1500)))
	}

	return ctx.Edit(fmt.Sprintf("🌐 <b>网络测速</b>\n\n模式: <code>%s</code>\n\n<pre>%s</pre>", mode, truncate(string(out), 3000)))
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n…(截断)"
}
