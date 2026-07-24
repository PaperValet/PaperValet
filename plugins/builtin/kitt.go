package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// KittPlugin provides advanced trigger system (match -> execute).
// Inspired by TeleBox's Kitt plugin but implemented in Go.
type KittPlugin struct {
	tasksFile string
	tasks     []KittTask
}

type KittTask struct {
	ID      string `json:"id"`
	Remark  string `json:"remark,omitempty"`
	Match   string `json:"match"`
	Action  string `json:"action"`
	Enabled bool   `json:"enabled"`
}

func NewKitt() *KittPlugin {
	return &KittPlugin{
		tasksFile: "data/kitt_tasks.json",
		tasks:     []KittTask{},
	}
}

func (p *KittPlugin) Name() string        { return "kitt" }
func (p *KittPlugin) Description() string { return "高级触发器 (匹配 -> 执行)" }

func (p *KittPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	p.loadTasks()

	cmds := []*interfaces.Command{
		{
			Name:        "kitt",
			Aliases:     []string{"trigger"},
			Description: "KITT 触发器管理 - 匹配 -> 执行",
			Usage:       "kitt [add|ls|del|enable|disable|help]",
			Plugin:      p.Name(),
			Category:    "tools",
			OwnerOnly:   true,
			Handler:     p.handleKitt,
		},
	}

	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}

	return nil
}

func (p *KittPlugin) Start(ctx context.Context) error { return nil }
func (p *KittPlugin) Stop(ctx context.Context) error  { return nil }

func (p *KittPlugin) loadTasks() {
	data, err := os.ReadFile(p.tasksFile)
	if err != nil {
		return
	}
	json.Unmarshal(data, &p.tasks)
}

func (p *KittPlugin) saveTasks() {
	os.MkdirAll(filepath.Dir(p.tasksFile), 0o755)
	data, _ := json.MarshalIndent(p.tasks, "", "  ")
	os.WriteFile(p.tasksFile, data, 0o644)
}

func (p *KittPlugin) handleKitt(ctx *interfaces.CommandContext) error {
	args := ctx.Args
	if len(args) == 0 {
		return p.showHelp(ctx)
	}

	sub := args[0]
	switch sub {
	case "help", "h":
		return p.showHelp(ctx)
	case "add", "create":
		return p.addTask(ctx, args[1:])
	case "ls", "list":
		return p.listTasks(ctx, args[1:])
	case "del", "rm", "delete", "remove":
		if len(args) < 2 {
			return ctx.Edit("用法: kitt del <ID>")
		}
		return p.deleteTask(ctx, args[1])
	case "enable", "on":
		if len(args) < 2 {
			return ctx.Edit("用法: kitt enable <ID>")
		}
		return p.setTaskEnabled(ctx, args[1], true)
	case "disable", "off":
		if len(args) < 2 {
			return ctx.Edit("用法: kitt disable <ID>")
		}
		return p.setTaskEnabled(ctx, args[1], false)
	default:
		return ctx.Edit(fmt.Sprintf("未知子命令: %s\n\n%s", sub, p.helpText()))
	}
}

func (p *KittPlugin) showHelp(ctx *interfaces.CommandContext) error {
	return ctx.Edit(p.helpText())
}

func (p *KittPlugin) helpText() string {
	return `🤖 <b>KITT - 高级触发器</b>

<b>用法:</b>
• <code>kitt add <备注> <匹配条件> <执行动作></code> — 添加触发器
• <code>kitt ls</code> — 列出所有触发器
• <code>kitt del <ID></code> — 删除触发器
• <code>kitt enable/disable <ID></code> — 启用/禁用触发器

<b>匹配条件语法 (Go 表达式):</b>
• <code>msg.Text == "hello"</code> — 精确匹配
• <code>strings.HasPrefix(msg.Text, "!")</code> — 前缀匹配
• <code>msg.UserID == 123456</code> — 用户匹配
• <code>msg.ChatID == -1001234567890</code> — 群组匹配
• <code>msg.IsOut</code> — 自己发送的消息
• <code>msg.IsReply</code> — 回复消息
• <code>strings.Contains(msg.Text, "keyword")</code> — 包含关键词

<b>执行动作:</b>
• <code>reply("回复内容")</code> — 回复消息
• <code>edit("新内容")</code> — 编辑触发消息
• <code>delete()</code> — 删除触发消息
• <code>run("command args")</code> — 执行其他命令

<b>示例:</b>
• <code>kitt add "疯狂星期四" "strings.Contains(msg.Text, \"V我50\") && time.Now().Weekday() == time.Thursday" "reply(\"V我50!\")"</code>
• <code>kitt add "自动回复" "msg.Text == \"ping\"" "reply(\"pong!\")"</code>
• <code>kitt add "防撤回" "msg.IsOut && strings.HasPrefix(msg.Text, \"撤回\")" "run(\"log show 10\")"</code>

<b>可用变量:</b>
msg.Text, msg.UserID, msg.ChatID, msg.IsOut, msg.IsReply, msg.MessageID, msg.Date
time (time.Time), strings, fmt
`
}

func (p *KittPlugin) addTask(ctx *interfaces.CommandContext, args []string) error {
	if len(args) < 3 {
		return ctx.Edit("用法: kitt add <备注> <匹配条件> <执行动作>")
	}

	remark := args[0]
	match := args[1]
	action := strings.Join(args[2:], " ")

	// Generate ID
	id := fmt.Sprintf("%d", time.Now().UnixNano())

	task := KittTask{
		ID:      id,
		Remark:  remark,
		Match:   match,
		Action:  action,
		Enabled: true,
	}

	p.tasks = append(p.tasks, task)
	p.saveTasks()

	return ctx.Edit(fmt.Sprintf("✅ 触发器已添加: <b>%s</b> (ID: <code>%s</code>)", remark, id))
}

func (p *KittPlugin) listTasks(ctx *interfaces.CommandContext, args []string) error {
	verbose := len(args) > 0 && (args[0] == "-v" || args[0] == "--verbose")

	if len(p.tasks) == 0 {
		return ctx.Edit("📭 暂无触发器\n\n使用 <code>kitt add</code> 创建")
	}

	var b strings.Builder
	b.WriteString("🤖 <b>KITT 触发器列表</b>\n\n")

	enabledCount := 0
	for _, task := range p.tasks {
		if task.Enabled {
			enabledCount++
		}
	}
	b.WriteString(fmt.Sprintf("启用: %d / 总计: %d\n\n", enabledCount, len(p.tasks)))

	for _, task := range p.tasks {
		status := "⏸️"
		if task.Enabled {
			status = "✅"
		}

		remark := task.Remark
		if remark == "" {
			remark = "无备注"
		}

		b.WriteString(fmt.Sprintf("%s <b>%s</b> (ID: <code>%s</code>)\n", status, remark, task.ID))

		if verbose {
			b.WriteString(fmt.Sprintf("  匹配: <code>%s</code>\n", htmlEscape(task.Match)))
			b.WriteString(fmt.Sprintf("  执行: <code>%s</code>\n", htmlEscape(task.Action)))
		} else {
			matchPreview := task.Match
			if len(matchPreview) > 60 {
				matchPreview = matchPreview[:60] + "..."
			}
			actionPreview := task.Action
			if len(actionPreview) > 60 {
				actionPreview = actionPreview[:60] + "..."
			}
			b.WriteString(fmt.Sprintf("  匹配: <code>%s</code>\n", htmlEscape(matchPreview)))
			b.WriteString(fmt.Sprintf("  执行: <code>%s</code>\n", htmlEscape(actionPreview)))
		}
		b.WriteString("\n")
	}

	if !verbose {
		b.WriteString("💡 使用 <code>kitt ls -v</code> 查看完整代码")
	}

	return ctx.Edit(b.String())
}

func (p *KittPlugin) deleteTask(ctx *interfaces.CommandContext, id string) error {
	for i, task := range p.tasks {
		if task.ID == id {
			p.tasks = append(p.tasks[:i], p.tasks[i+1:]...)
			p.saveTasks()
			return ctx.Edit(fmt.Sprintf("🗑 触发器已删除: <code>%s</code>", id))
		}
	}
	return ctx.Edit(fmt.Sprintf("❌ 未找到触发器: <code>%s</code>", id))
}

func (p *KittPlugin) setTaskEnabled(ctx *interfaces.CommandContext, id string, enabled bool) error {
	for i, task := range p.tasks {
		if task.ID == id {
			p.tasks[i].Enabled = enabled
			p.saveTasks()
			action := "启用"
			if !enabled {
				action = "禁用"
			}
			return ctx.Edit(fmt.Sprintf("✅ 触发器已%s: <code>%s</code>", action, id))
		}
	}
	return ctx.Edit(fmt.Sprintf("❌ 未找到触发器: <code>%s</code>", id))
}

// Helper functions for condition evaluation
func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&#34;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}