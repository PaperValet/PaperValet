package builtin

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/internal/plugin"
)

// RemindPlugin provides reminder functionality.
type RemindPlugin struct {
	reminders map[string]*reminder
}

type reminder struct {
	ChatID    int64
	UserID    int64
	Text      string
	TriggerAt time.Time
	Repeating bool
	Interval  time.Duration
}

func NewRemind() *RemindPlugin {
	return &RemindPlugin{reminders: make(map[string]*reminder)}
}

func (p *RemindPlugin) Name() string        { return "remind" }
func (p *RemindPlugin) Description() string { return "定时提醒" }

func (p *RemindPlugin) Init(_ context.Context, mgr *plugin.Manager) error {
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "remind",
		Aliases:     []string{"remindme"},
		Description: "设置提醒",
		Usage:       "remind <时间> <内容> | remind list | remind del <id>",
		Plugin:      p.Name(),
		Category:    "tools",
		Handler:     p.handleRemind,
	})
}

func (p *RemindPlugin) Start(ctx context.Context) error {
	go p.run(ctx)
	return nil
}

func (p *RemindPlugin) Stop(_ context.Context) error { return nil }

func (p *RemindPlugin) run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.checkReminders(ctx)
		}
	}
}

func (p *RemindPlugin) checkReminders(ctx context.Context) {
	now := time.Now()
	for id, r := range p.reminders {
		if now.After(r.TriggerAt) {
			// In a real implementation, we'd send via API
			delete(p.reminders, id)
		}
	}
}

func (p *RemindPlugin) handleRemind(ctx *interfaces.CommandContext) error {
	sub := strings.ToLower(ctx.GetArg(0))
	switch sub {
	case "", "help":
		return ctx.Edit("用法: remind <时间> <内容> | remind list | remind del <id>\n时间格式: 5m, 1h, 2024-12-31 23:59")
	case "list":
		if len(p.reminders) == 0 {
			return ctx.Edit("暂无提醒")
		}
		var b strings.Builder
		b.WriteString("⏰ 提醒列表:\n")
		for id, r := range p.reminders {
			b.WriteString(fmt.Sprintf("%s: %s (在 %v)\n", id, r.Text, r.TriggerAt.Format("15:04:05")))
		}
		return ctx.Edit(b.String())
	case "del", "delete":
		if ctx.ArgCount() < 2 {
			return ctx.Edit("用法: remind del <id>")
		}
		id := ctx.GetArg(1)
		if _, ok := p.reminders[id]; ok {
			delete(p.reminders, id)
			return ctx.Edit("已删除: " + id)
		}
		return ctx.Edit("未找到: " + id)
	default:
		duration := sub
		text := strings.Join(ctx.Args[1:], " ")
		if text == "" {
			return ctx.Edit("提醒内容不能为空")
		}
		d, err := time.ParseDuration(duration)
		if err != nil {
			return ctx.Edit("时间格式错误，如: 5m, 1h, 30s")
		}
		id := fmt.Sprintf("%d-%d", ctx.Message.UserID, time.Now().Unix())
		p.reminders[id] = &reminder{
			ChatID:    ctx.Message.ChatID,
			UserID:    ctx.Message.UserID,
			Text:      text,
			TriggerAt: time.Now().Add(d),
		}
		return ctx.Edit(fmt.Sprintf("⏰ 已设置提醒: %s 后提醒 \"%s\"", d, text))
	}
}

// NotePlugin provides simple notes.
type NotePlugin struct {
	notes map[int64]map[string]string // userID -> name -> content
}

func NewNote() *NotePlugin {
	return &NotePlugin{notes: make(map[int64]map[string]string)}
}

func (p *NotePlugin) Name() string        { return "note" }
func (p *NotePlugin) Description() string { return "笔记管理" }

func (p *NotePlugin) Init(_ context.Context, mgr *plugin.Manager) error {
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "note",
		Aliases:     []string{"n"},
		Description: "笔记管理",
		Usage:       "note <set|get|del|list> [name] [内容]",
		Plugin:      p.Name(),
		Category:    "tools",
		Handler:     p.handleNote,
	})
}

func (p *NotePlugin) Start(_ context.Context) error { return nil }
func (p *NotePlugin) Stop(_ context.Context) error  { return nil }

func (p *NotePlugin) handleNote(ctx *interfaces.CommandContext) error {
	if ctx.ArgCount() == 0 {
		return ctx.Edit("用法: note set <name> <内容> | note get <name> | note del <name> | note list")
	}
	sub := strings.ToLower(ctx.GetArg(0))
	userNotes := p.notes[ctx.Message.UserID]
	if userNotes == nil {
		userNotes = make(map[string]string)
		p.notes[ctx.Message.UserID] = userNotes
	}

	switch sub {
	case "set":
		if ctx.ArgCount() < 3 {
			return ctx.Edit("用法: note set <name> <内容>")
		}
		name := ctx.GetArg(1)
		content := strings.Join(ctx.Args[2:], " ")
		userNotes[name] = content
		return ctx.Edit(fmt.Sprintf("✅ 笔记 '%s' 已保存", name))
	case "get":
		if ctx.ArgCount() < 2 {
			return ctx.Edit("用法: note get <name>")
		}
		name := ctx.GetArg(1)
		if content, ok := userNotes[name]; ok {
			return ctx.Edit(fmt.Sprintf("📝 %s:\n%s", name, content))
		}
		return ctx.Edit("笔记不存在: " + name)
	case "del", "delete":
		if ctx.ArgCount() < 2 {
			return ctx.Edit("用法: note del <name>")
		}
		name := ctx.GetArg(1)
		if _, ok := userNotes[name]; ok {
			delete(userNotes, name)
			return ctx.Edit(fmt.Sprintf("🗑 笔记 '%s' 已删除", name))
		}
		return ctx.Edit("笔记不存在: " + name)
	case "list":
		if len(userNotes) == 0 {
			return ctx.Edit("暂无笔记")
		}
		var b strings.Builder
		b.WriteString("📋 笔记列表:\n")
		for name := range userNotes {
			b.WriteString("• " + name + "\n")
		}
		return ctx.Edit(b.String())
	default:
		return ctx.Edit("未知子命令: " + sub)
	}
}