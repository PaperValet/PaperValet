package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

const restartMarker = "data/restart.json"

// RestartPlugin re-executes the bot in place.
type RestartPlugin struct {
	// BeforeRestart lets the app flush state (session, plugins) before the
	// process image is replaced.
	BeforeRestart func()
}

func NewRestart() *RestartPlugin { return &RestartPlugin{} }

func (p *RestartPlugin) Name() string        { return "restart" }
func (p *RestartPlugin) Description() string { return "重启机器人" }
func (p *RestartPlugin) DescEN() string      { return "Restart the bot" }

func (p *RestartPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "restart",
		Description: "重启机器人",
		DescEN:      "Restart the bot",
		Usage: `restart

**机制**
• 先停止所有插件、写盘、刷新日志
• 在同一进程号上原地重新加载程序，systemd、Docker、tmux 都不会把它当成退出
• 回来后把这条命令消息改成「重启完成」和用时
• 已加载的外部插件会随启动自动重新加载`,
		UsageEN: `restart

**How it works**
• Stops all plugins, flushes state and logs
• Re-executes in place under the same PID, so systemd, Docker and tmux keep supervising it
• After boot the command message turns into "Restarted" with the elapsed time
• Installed external plugins are loaded again on startup`,
		Plugin:    p.Name(),
		Category:  "admin",
		OwnerOnly: true,
		Handler:   p.handleRestart,
	})
}

func (p *RestartPlugin) Start(_ context.Context) error { return nil }
func (p *RestartPlugin) Stop(_ context.Context) error  { return nil }

type restartState struct {
	ChatID int64  `json:"chat_id"`
	MsgID  int    `json:"msg_id"`
	At     int64  `json:"at"`
	Lang   string `json:"lang"`
}

// Restart is exported so update can reuse the same restart flow.
func (p *RestartPlugin) Restart(ctx *interfaces.CommandContext) error { return p.handleRestart(ctx) }

func (p *RestartPlugin) handleRestart(ctx *interfaces.CommandContext) error {
	exe, err := os.Executable()
	if err != nil {
		return ctx.Edit("❌ " + esc(err.Error()))
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
		if err := reexec(exe); err != nil {
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
