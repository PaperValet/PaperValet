package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// SudoPlugin implements PagerMaid-style permission delegation.
// Other Telegram users can be granted permission to use the userbot's commands.
type SudoPlugin struct {
	mu      sync.RWMutex
	enabled bool
	users   map[int64]bool
	file    string
}

func NewSudo() *SudoPlugin {
	return &SudoPlugin{
		users: make(map[int64]bool),
		file:  "data/sudo.json",
	}
}

func (p *SudoPlugin) Name() string        { return "sudo" }
func (p *SudoPlugin) Description() string { return "权限委派 — 授权其他用户使用命令" }

func (p *SudoPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	p.load()
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "sudo",
		Description: "权限委派管理",
		Usage:       "sudo on|off|add|remove|list",
		Plugin:      p.Name(),
		Category:    "admin",
		OwnerOnly:   true,
		Handler:     p.handleSudo,
	})
}

func (p *SudoPlugin) Start(_ context.Context) error { return nil }
func (p *SudoPlugin) Stop(_ context.Context) error  { return nil }

// IsSudoUser reports whether the given user is permitted.
func (p *SudoPlugin) IsSudoUser(userID int64) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.enabled && p.users[userID]
}

func (p *SudoPlugin) load() {
	data, err := os.ReadFile(p.file)
	if err != nil {
		return
	}
	var st struct {
		Enabled bool    `json:"enabled"`
		Users   []int64 `json:"users"`
	}
	if json.Unmarshal(data, &st) == nil {
		p.enabled = st.Enabled
		for _, id := range st.Users {
			p.users[id] = true
		}
	}
}

func (p *SudoPlugin) save() {
	os.MkdirAll(filepath.Dir(p.file), 0o755)
	users := make([]int64, 0, len(p.users))
	for id := range p.users {
		users = append(users, id)
	}
	data, _ := json.MarshalIndent(map[string]any{"enabled": p.enabled, "users": users}, "", "  ")
	os.WriteFile(p.file, data, 0o600)
}

func (p *SudoPlugin) handleSudo(ctx *interfaces.CommandContext) error {
	if ctx.ArgCount() == 0 {
		return p.showStatus(ctx)
	}

	switch strings.ToLower(ctx.GetArg(0)) {
	case "on", "enable":
		p.mu.Lock()
		p.enabled = true
		p.mu.Unlock()
		p.save()
		return ctx.Edit("✅ Sudo 已启用")
	case "off", "disable":
		p.mu.Lock()
		p.enabled = false
		p.mu.Unlock()
		p.save()
		return ctx.Edit("⏸️ Sudo 已禁用")
	case "add":
		return p.addUser(ctx)
	case "remove", "del", "rm":
		return p.removeUser(ctx)
	case "list", "ls":
		return p.listUsers(ctx)
	default:
		return ctx.Edit("用法: sudo on|off|add <用户ID>|remove <用户ID>|list")
	}
}

func (p *SudoPlugin) showStatus(ctx *interfaces.CommandContext) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	state := "⏸️ 禁用"
	if p.enabled {
		state = "✅ 启用"
	}
	return ctx.Edit(fmt.Sprintf("🔐 <b>Sudo 权限委派</b>\n\n状态: %s\n已授权用户: %d 个", state, len(p.users)))
}

func (p *SudoPlugin) targetUser(ctx *interfaces.CommandContext) (int64, error) {
	// Prefer the replied-to message sender.
	if ctx.Message != nil && ctx.Message.IsReply {
		// Resolve the sender of the replied message via the API.
		peer, err := ctx.ResolvePeer()
		if err == nil {
			msgs, err := ctx.API.MessagesGetMessages(ctx.Context(),
				[]tg.InputMessageClass{&tg.InputMessageID{ID: ctx.Message.ReplyToID}},
			)
			if err == nil {
				if ml, ok := msgs.(*tg.MessagesMessages); ok {
					for _, m := range ml.Messages {
						if msg, ok := m.(*tg.Message); ok && msg.ID == ctx.Message.ReplyToID {
							if pu, ok := msg.FromID.(*tg.PeerUser); ok {
								return pu.UserID, nil
							}
						}
					}
				}
			}
			_ = peer
		}
	}
	if ctx.ArgCount() >= 2 {
		var id int64
		if _, err := fmt.Sscanf(ctx.GetArg(1), "%d", &id); err == nil && id != 0 {
			return id, nil
		}
	}
	return 0, fmt.Errorf("无法确定目标用户（回复消息或提供用户ID）")
}

func (p *SudoPlugin) addUser(ctx *interfaces.CommandContext) error {
	id, err := p.targetUser(ctx)
	if err != nil {
		return ctx.Edit("❌ " + err.Error())
	}
	p.mu.Lock()
	p.users[id] = true
	p.mu.Unlock()
	p.save()
	return ctx.Edit(fmt.Sprintf("✅ 已授权用户 <code>%d</code>", id))
}

func (p *SudoPlugin) removeUser(ctx *interfaces.CommandContext) error {
	id, err := p.targetUser(ctx)
	if err != nil {
		return ctx.Edit("❌ " + err.Error())
	}
	p.mu.Lock()
	if !p.users[id] {
		p.mu.Unlock()
		return ctx.Edit(fmt.Sprintf("⚠️ 用户 <code>%d</code> 未被授权", id))
	}
	delete(p.users, id)
	p.mu.Unlock()
	p.save()
	return ctx.Edit(fmt.Sprintf("🗑 已移除授权 <code>%d</code>", id))
}

func (p *SudoPlugin) listUsers(ctx *interfaces.CommandContext) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if len(p.users) == 0 {
		return ctx.Edit("🔐 暂无授权用户")
	}
	var b strings.Builder
	b.WriteString("🔐 <b>Sudo 授权用户</b>\n\n")
	for id := range p.users {
		b.WriteString(fmt.Sprintf("• <code>%d</code>\n", id))
	}
	return ctx.Edit(b.String())
}
