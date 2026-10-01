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

// SudoPlugin implements permission delegation: the owner can let other
// Telegram users run the userbot's commands.
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
func (p *SudoPlugin) Description() string { return "授权其他人使用本机器人的命令" }
func (p *SudoPlugin) DescEN() string      { return "Delegate command access to other users" }

func (p *SudoPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	p.load()
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "sudo",
		Description: "把机器人命令授权给别的用户（回复他或给 ID）",
		DescEN:      "Grant command access to another user (reply or by ID)",
		Usage:       "sudo add [ID] | remove [ID] | list | on | off",
		UsageEN:     "sudo add [ID] | remove [ID] | list | on | off",
		Plugin:      p.Name(),
		Category:    "admin",
		OwnerOnly:   true,
		Handler:     p.handleSudo,
	})
}

func (p *SudoPlugin) Start(_ context.Context) error { return nil }
func (p *SudoPlugin) Stop(_ context.Context) error  { return nil }

// IsSudoUser reports whether the given user is permitted.
// Delegation only works while the switch is on.
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
	p.mu.RLock()
	enabled := p.enabled
	users := make([]int64, 0, len(p.users))
	for id := range p.users {
		users = append(users, id)
	}
	p.mu.RUnlock()
	_ = os.MkdirAll(filepath.Dir(p.file), 0o700)
	data, _ := json.MarshalIndent(map[string]any{"enabled": enabled, "users": users}, "", "  ")
	_ = os.WriteFile(p.file, data, 0o600)
}

func (p *SudoPlugin) handleSudo(ctx *interfaces.CommandContext) error {
	if ctx.ArgCount() == 0 {
		return p.showStatus(ctx)
	}
	switch strings.ToLower(ctx.GetArg(0)) {
	case "on":
		p.mu.Lock()
		p.enabled = true
		p.mu.Unlock()
		p.save()
		return ctx.Edit(ctx.Tlocal("✅ Sudo 已开启，名单里的用户可以使用命令", "✅ Sudo on: listed users can run commands"))
	case "off":
		p.mu.Lock()
		p.enabled = false
		p.mu.Unlock()
		p.save()
		return ctx.Edit(ctx.Tlocal("⏸️ Sudo 已关闭，只有你自己能用命令", "⏸️ Sudo off: only you can run commands"))
	case "add", "allow", "grant":
		return p.addUser(ctx)
	case "remove", "del", "rm", "revoke":
		return p.removeUser(ctx)
	case "list", "ls":
		return p.listUsers(ctx)
	default:
		return p.showStatus(ctx)
	}
}

func (p *SudoPlugin) showStatus(ctx *interfaces.CommandContext) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	state := ctx.Tlocal("⏸️ 关闭", "⏸️ off")
	if p.enabled {
		state = ctx.Tlocal("✅ 开启", "✅ on")
	}
	hint := ctx.Tlocal(
		"回复某人的消息发 <code>sudo add</code> 就能授权他；<code>sudo list</code> 看名单，<code>sudo remove</code>（同样可回复）移除",
		"Reply to someone with <code>sudo add</code> to grant access; <code>sudo list</code> shows the list, <code>sudo remove</code> (reply works too) revokes",
	)
	return ctx.Edit(fmt.Sprintf("🔐 <b>Sudo</b>\n\n%s · %d %s\n\n%s",
		state, len(p.users), ctx.Tlocal("位用户", "users"), hint))
}

// targetUser resolves the delegation target: replied sender first, then a
// numeric arg. Returns the user entity too so we can show a real name.
func (p *SudoPlugin) targetUser(ctx *interfaces.CommandContext) (*tg.User, error) {
	if ctx.Message != nil && ctx.Message.IsReply && ctx.API != nil {
		if msg, res, err := fetchMessage(ctx, ctx.Message.ReplyToID); err == nil {
			if pu, ok := msg.FromID.(*tg.PeerUser); ok {
				if u := findUserInChats(res, pu.UserID); u != nil {
					return u, nil
				}
				return &tg.User{ID: pu.UserID}, nil
			}
		}
	}
	if ctx.ArgCount() >= 2 {
		var id int64
		if _, err := fmt.Sscanf(ctx.GetArg(1), "%d", &id); err == nil && id != 0 {
			return &tg.User{ID: id}, nil
		}
	}
	return nil, fmt.Errorf("%s", ctx.Tlocal(
		"不知道要操作谁：回复那个人的消息再发命令，或在命令后面带上用户 ID",
		"no target: reply to the user's message or pass a user ID",
	))
}

// findUserInChats scans the users attached to a messages response.
func findUserInChats(msgs tg.MessagesMessagesClass, userID int64) *tg.User {
	for _, u := range usersOf(msgs) {
		if user, ok := u.(*tg.User); ok && user.ID == userID {
			return user
		}
	}
	return nil
}

func (p *SudoPlugin) addUser(ctx *interfaces.CommandContext) error {
	u, err := p.targetUser(ctx)
	if err != nil {
		return ctx.Edit("❌ " + err.Error())
	}
	p.mu.Lock()
	p.users[u.ID] = true
	enabled := p.enabled
	p.mu.Unlock()
	if !enabled {
		p.mu.Lock()
		p.enabled = true
		p.mu.Unlock()
	}
	p.save()
	extra := ""
	if !enabled {
		extra = ctx.Tlocal("\n（sudo 原先是关闭的，已顺手打开）", "\n(sudo was off; turned it on for you)")
	}
	return ctx.Edit(fmt.Sprintf("✅ %s %s%s",
		ctx.Tlocal("已授权", "granted"), htmlEscape(displayName(u)), extra))
}

func (p *SudoPlugin) removeUser(ctx *interfaces.CommandContext) error {
	u, err := p.targetUser(ctx)
	if err != nil {
		return ctx.Edit("❌ " + err.Error())
	}
	p.mu.Lock()
	if !p.users[u.ID] {
		p.mu.Unlock()
		return ctx.Edit(fmt.Sprintf("⚠️ %s %s", htmlEscape(displayName(u)), ctx.Tlocal("本来就不在名单里", "is not on the list")))
	}
	delete(p.users, u.ID)
	p.mu.Unlock()
	p.save()
	return ctx.Edit(fmt.Sprintf("🗑 %s %s", htmlEscape(displayName(u)), ctx.Tlocal("已移除授权", "revoked")))
}

func (p *SudoPlugin) listUsers(ctx *interfaces.CommandContext) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if len(p.users) == 0 {
		return ctx.Edit(ctx.Tlocal("名单是空的。回复某人的消息发 <code>sudo add</code> 添加", "The list is empty. Reply to someone with <code>sudo add</code>"))
	}
	var b strings.Builder
	b.WriteString("🔐 <b>Sudo</b>\n")
	for id := range p.users {
		fmt.Fprintf(&b, "• <code>%d</code>\n", id)
	}
	b.WriteString("\n" + ctx.Tlocal("移除: 回复其消息发 <code>sudo remove</code>", "Revoke: reply to their message with <code>sudo remove</code>"))
	return ctx.Edit(b.String())
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}
