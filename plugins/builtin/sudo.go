package builtin

import (
	"context"
	"encoding/json"
	"errors"
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
// Telegram users run the userbot's commands. Granting needs a person from
// the chat, so add/remove stay commands; the master switch and the list
// live in the bot panel.
type SudoPlugin struct {
	mu          sync.RWMutex
	users       map[int64]bool
	file        string
	mgrCommands plugin.RegistryProvider
	set         plugin.Settings
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
	set, err := mgr.Host().Settings(&plugin.SettingsSpec{
		Plugin:  p.Name(),
		Title:   "🔐 Sudo",
		TitleEN: "🔐 Sudo",
		Settings: []plugin.Setting{{
			Key: "enabled", Label: "允许授权用户", LabelEN: "Delegation",
			Hint:   "关闭时名单保留，但除了你谁都不能用命令",
			HintEN: "Off keeps the list but only you can run commands",
			Kind:   plugin.SettingToggle, Default: true,
		}},
	})
	if err != nil {
		return err
	}
	p.set = set
	if err := mgr.Host().Bot(p.Name()).SetPage(&plugin.Page{
		Title: "授权名单", TitleEN: "Granted users",
		Handle: p.page,
	}); err != nil && !errors.Is(err, plugin.ErrBotNotReady) {
		return err
	}
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "sudo",
		Description: "授权他人使用命令",
		DescEN:      "Delegate command access",
		Usage: "sudo add|remove [用户ID]\n" +
			"\n" +
			"**示例**\n" +
			"• 回复某人的消息发 `sudo add`  授权他\n" +
			"• `sudo add 123456`  按 ID 授权\n" +
			"• 回复某人发 `sudo remove`  取消授权\n" +
			"• `sudo`  查看状态\n" +
			"\n" +
			"**机制**\n" +
			"• 名单里的人发的命令和你自己发的一样会被执行，包括仅主人可用的命令\n" +
			"• 总开关和名单在机器人面板里管理\n" +
			"• 名单保存在 data/sudo.json，重启不丢",
		UsageEN: "sudo add|remove [user ID]\n" +
			"\n" +
			"**Examples**\n" +
			"• Reply to someone with `sudo add`  grant access\n" +
			"• `sudo add 123456`  grant by ID\n" +
			"• Reply with `sudo remove`  revoke\n" +
			"• `sudo`  show status\n" +
			"\n" +
			"**How it works**\n" +
			"• Commands from listed users run exactly like yours, owner-only ones included\n" +
			"• The master switch and the list live in the bot panel\n" +
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
	if p.set != nil && !p.set.Bool("enabled") {
		return false
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.users[userID]
}

func (p *SudoPlugin) load() {
	data, err := os.ReadFile(p.file)
	if err != nil {
		return
	}
	var st struct {
		Users []int64 `json:"users"`
	}
	if json.Unmarshal(data, &st) == nil {
		for _, id := range st.Users {
			p.users[id] = true
		}
	}
}

func (p *SudoPlugin) save() {
	_ = os.MkdirAll(filepath.Dir(p.file), 0o700)
	data, _ := json.MarshalIndent(map[string]any{"users": p.sortedUsers()}, "", "  ")
	_ = os.WriteFile(p.file, data, 0o600)
}

func (p *SudoPlugin) sortedUsers() []int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	ids := make([]int64, 0, len(p.users))
	for id := range p.users {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// page lists granted users with a revoke button each.
func (p *SudoPlugin) page(ctx *plugin.BotContext) (*plugin.View, error) {
	if id, ok := strings.CutPrefix(ctx.Data, "rm:"); ok {
		var uid int64
		if _, err := fmt.Sscanf(id, "%d", &uid); err == nil {
			p.mu.Lock()
			delete(p.users, uid)
			p.mu.Unlock()
			p.save()
			ctx.Toast(ctx.Tlocal("已移除", "Revoked"))
		}
	}
	ids := p.sortedUsers()
	v := &plugin.View{Text: "🔐 **" + ctx.Tlocal("授权名单", "Granted users") + "**\n\n"}
	if len(ids) == 0 {
		v.Text += ctx.Tlocal("名单是空的。在聊天里回复某人发 `sudo add` 添加", "The list is empty. Reply to someone with `sudo add` in a chat")
		return v, nil
	}
	for _, id := range ids {
		v.Text += "• " + plugin.Code(id) + "\n"
		v.Buttons = append(v.Buttons, plugin.Row(plugin.Btn("🗑 "+fmt.Sprint(id), fmt.Sprintf("rm:%d", id))))
	}
	return v, nil
}

func (p *SudoPlugin) handleSudo(ctx *interfaces.CommandContext) error {
	switch strings.ToLower(ctx.GetArg(0)) {
	case "add":
		return p.addUser(ctx)
	case "remove":
		return p.removeUser(ctx)
	}
	return p.showStatus(ctx)
}

func (p *SudoPlugin) showStatus(ctx *interfaces.CommandContext) error {
	state := ctx.Tlocal("⏸️ 关闭", "⏸️ off")
	if p.set.Bool("enabled") {
		state = ctx.Tlocal("✅ 开启", "✅ on")
	}
	prefix := p.mgrCommandsPrefix()
	c := newCard("🔐", "Sudo")
	c.blank().rawField(ctx.Tlocal("状态", "Status"), state)
	c.field(ctx.Tlocal("已授权", "Granted"), len(p.sortedUsers()))
	c.hint(ctx.Tlocal(
		"回复某人发 "+cmdRef(prefix+"sudo add")+" 授权，开关和名单在机器人面板里",
		"Reply with "+cmdRef(prefix+"sudo add")+" to grant; switch and list are in the bot panel"))
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
	p.mu.Unlock()
	p.save()
	note := ctx.Tlocal("已授权，他发的命令会被执行", "granted; their commands now run")
	if !p.set.Bool("enabled") {
		_ = p.set.Set("enabled", true)
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

func esc(s string) string {
	return plugin.Escape(s)
}
