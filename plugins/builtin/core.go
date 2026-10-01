package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

const restartMarker = "data/restart.json"

// CorePlugin provides ping and restart.
// The old version command merged into status.
type CorePlugin struct {
	version string
	// BeforeRestart lets the app flush state (session, plugins) before the
	// process image is replaced.
	BeforeRestart func()
}

func NewCore(version string) *CorePlugin {
	return &CorePlugin{version: version}
}

func (p *CorePlugin) Name() string        { return "core" }
func (p *CorePlugin) Description() string { return "ping / restart 基础命令" }
func (p *CorePlugin) DescEN() string      { return "ping / restart basics" }

func (p *CorePlugin) Init(_ context.Context, mgr plugin.Manager) error {
	cmds := []*interfaces.Command{
		{
			Name:        "ping",
			Description: "测量响应延迟",
			DescEN:      "Measure response latency",
			Usage:       "ping",
			Plugin:      p.Name(),
			Category:    "core",
			Handler:     p.handlePing,
		},
		{
			Name:        "restart",
			Description: "重启 PaperValet 进程，完成后在原消息报到",
			DescEN:      "Restart PaperValet; the message updates when it is back",
			Usage:       "restart",
			Plugin:      p.Name(),
			Category:    "admin",
			OwnerOnly:   true,
			Handler:     p.handleRestart,
		},
	}
	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *CorePlugin) Start(_ context.Context) error { return nil }
func (p *CorePlugin) Stop(_ context.Context) error  { return nil }

func (p *CorePlugin) handlePing(ctx *interfaces.CommandContext) error {
	start := time.Now()
	if err := ctx.Edit("🏓"); err != nil {
		return err
	}
	return ctx.Edit(fmt.Sprintf("🏓 Pong · %s", time.Since(start).Round(time.Millisecond)))
}

type restartState struct {
	ChatID int64  `json:"chat_id"`
	MsgID  int    `json:"msg_id"`
	At     int64  `json:"at"`
	Lang   string `json:"lang"`
}

// Restart is exported so update can reuse the same restart flow.
func (p *CorePlugin) Restart(ctx *interfaces.CommandContext) error { return p.handleRestart(ctx) }

func (p *CorePlugin) handleRestart(ctx *interfaces.CommandContext) error {
	exe, err := os.Executable()
	if err != nil {
		return ctx.Edit("❌ " + err.Error())
	}
	_ = ctx.Edit(ctx.Tlocal("🔄 正在重启…", "🔄 Restarting…"))

	st := restartState{ChatID: ctx.Message.ChatID, MsgID: ctx.Message.Message.ID, At: time.Now().UnixMilli(), Lang: ctx.Lang}
	if data, err := json.Marshal(st); err == nil {
		_ = os.MkdirAll(filepath.Dir(restartMarker), 0o700)
		_ = os.WriteFile(restartMarker, data, 0o600)
	}

	go func() {
		time.Sleep(300 * time.Millisecond)
		if p.BeforeRestart != nil {
			p.BeforeRestart()
		}
		// Replace the process image in place: same PID, so systemd,
		// Docker and tmux all keep supervising it. Exit(0) alone left
		// Restart=on-failure units dead.
		if err := syscall.Exec(exe, os.Args, os.Environ()); err != nil {
			os.Exit(1) // non-zero so on-failure supervisors restart us
		}
	}()
	return nil
}

// FinishRestart edits the restart message after the process came back.
// Called by the app once the client is authorized.
func FinishRestart(ctx context.Context, api *tg.Client, resolve func(context.Context, int64) (tg.InputPeerClass, error)) {
	data, err := os.ReadFile(restartMarker)
	if err != nil {
		return
	}
	_ = os.Remove(restartMarker)
	var st restartState
	if json.Unmarshal(data, &st) != nil || st.MsgID == 0 {
		return
	}
	peer, err := resolve(ctx, st.ChatID)
	if err != nil {
		return
	}
	took := time.Since(time.UnixMilli(st.At)).Round(100 * time.Millisecond)
	text := fmt.Sprintf("✅ 重启完成 · %s", took)
	if st.Lang == "en-US" {
		text = fmt.Sprintf("✅ Restarted · %s", took)
	}
	_, _ = api.MessagesEditMessage(ctx, &tg.MessagesEditMessageRequest{Peer: peer, ID: st.MsgID, Message: text})
}
