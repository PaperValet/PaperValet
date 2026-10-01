package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// SudoPlugin implements permission delegation: the owner can let other
// Telegram users run the userbot's commands.
type SudoPlugin struct {
	mu          sync.RWMutex
	enabled     bool
	users       map[int64]bool
	file        string
	mgrCommands plugin.RegistryProvider
}

func NewSudo() *SudoPlugin {
	return &SudoPlugin{
		users: make(map[int64]bool),
		file:  "data/sudo.json",
	}
}

func (p *SudoPlugin) Name() string        { return "sudo" }
func (p *SudoPlugin) Description() string { return "授权他人使用命令" }
func (p *SudoPlugin) DescEN() string      { return "Delegate command access to other users" }

func (p *SudoPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	p.mgrCommands = mgr.Commands()
	p.load()
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "sudo",
		Description: "授权他人使用命令",
		DescEN:      "Delegate command access",
		Usage: "sudo add|remove [用户ID] · sudo list · sudo on|off\n" +
			"\n" +
			"**示例**\n" +
			"• 回复某人的消息发 `sudo add`  授权他\n" +
			"• `sudo add 123456`  按 ID 授权\n" +
			"• 回复某人发 `sudo remove`  取消授权\n" +
			"• `sudo list`  查看名单\n" +
			"• `sudo off` / `sudo on`  整体关闭 / 开启\n" +
			"\n" +
			"**机制**\n" +
			"• 名单里的人发的命令和你自己发的一样会被执行，包括仅主人可用的命令\n" +
			"• 添加第一个人时会自动打开总开关\n" +
			"• 总开关关闭时名单保留，但所有人都不能用\n" +
			"• 名单保存在 data/sudo.json，重启不丢",
		UsageEN: "sudo add|remove [user ID] · sudo list · sudo on|off\n" +
			"\n" +
			"**Examples**\n" +
			"• Reply to someone with `sudo add`  grant access\n" +
			"• `sudo add 123456`  grant by ID\n" +
			"• Reply with `sudo remove`  revoke\n" +
			"• `sudo list`  show the list\n" +
			"• `sudo off` / `sudo on`  master switch\n" +
			"\n" +
			"**How it works**\n" +
			"• Commands from listed users run exactly like yours, owner-only ones included\n" +
			"• Adding someone turns the master switch on\n" +
			"• With the switch off the list is kept but nobody can use it\n" +
			"• Stored in data/sudo.json, survives restarts",
		Plugin:    p.Name(),
		Category:  "admin",
		OwnerOnly: true,
		Handler:   p.handleSudo,
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
	prefix := p.mgrCommandsPrefix()
	c := newCard("🔐", "Sudo")
	c.blank().rawField(ctx.Tlocal("状态", "Status"), state)
	c.field(ctx.Tlocal("已授权", "Granted"), len(p.users))
	c.hint(ctx.Tlocal(
		"回复某人发 "+cmdRef(prefix+"sudo add")+" 授权，"+cmdRef(prefix+"sudo list")+" 看名单",
		"Reply with "+cmdRef(prefix+"sudo add")+" to grant; "+cmdRef(prefix+"sudo list")+" shows the list"))
	return ctx.Edit(c.String())
}

func (p *SudoPlugin) mgrCommandsPrefix() string {
	return p.mgrCommands.GetPrefix()
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
		return ctx.Edit(errText(esc(err.Error())))
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
	note := ctx.Tlocal("已授权，他发的命令会被执行", "granted; their commands now run")
	if !enabled {
		note += ctx.Tlocal("（总开关已顺手打开）", " (master switch turned on)")
	}
	return ctx.Edit(okLine(displayName(u), note))
}

func (p *SudoPlugin) removeUser(ctx *interfaces.CommandContext) error {
	u, err := p.targetUser(ctx)
	if err != nil {
		return ctx.Edit("❌ " + esc(err.Error()))
	}
	p.mu.Lock()
	if !p.users[u.ID] {
		p.mu.Unlock()
		return ctx.Edit(skipLine(displayName(u), ctx.Tlocal("本来就不在名单里", "not on the list")))
	}
	delete(p.users, u.ID)
	p.mu.Unlock()
	p.save()
	return ctx.Edit("🗑 **" + esc(displayName(u)) + "**  " + ctx.Tlocal("已移除授权", "revoked"))
}

func (p *SudoPlugin) listUsers(ctx *interfaces.CommandContext) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	prefix := p.mgrCommandsPrefix()
	if len(p.users) == 0 {
		c := newCard("🔐", "Sudo")
		c.blank().line(ctx.Tlocal("名单是空的", "The list is empty"))
		c.hint(ctx.Tlocal("回复某人发 "+cmdRef(prefix+"sudo add")+" 添加", "Reply with "+cmdRef(prefix+"sudo add")+" to add"))
		return ctx.Edit(c.String())
	}
	ids := make([]int64, 0, len(p.users))
	for id := range p.users {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	c := newCard("🔐", ctx.Tlocal(fmt.Sprintf("Sudo · %d 位", len(ids)), fmt.Sprintf("Sudo · %d users", len(ids)))).blank()
	for _, id := range ids {
		c.rawField("•", plugin.Code(id)+" · "+plugin.Mention(ctx.Tlocal("发消息", "message"), id))
	}
	c.hint(ctx.Tlocal("移除：回复其消息发 "+cmdRef(prefix+"sudo remove"), "Revoke: reply with "+cmdRef(prefix+"sudo remove")))
	return ctx.Edit(c.String())
}

func esc(s string) string {
	return plugin.Escape(s)
}
