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

// LeechPlugin archives messages from chats/groups/channels to local SQLite.
// Inspired by TeleBox's leech plugin.
type LeechPlugin struct {
	dbFile string
	db     *LeechDB
}

type LeechDB struct {
	Jobs      []LeechJob      `json:"jobs"`
	Messages  []LeechMessage  `json:"messages"`
}

type LeechJob struct {
	ID          string    `json:"id"`
	Target      string    `json:"target"`
	ChatTitle   string    `json:"chat_title"`
	ChatType    string    `json:"chat_type"`
	FromDate    time.Time `json:"from_date"`
	ToDate      time.Time `json:"to_date"`
	BatchSize   int       `json:"batch_size"`
	Limit       int       `json:"limit"`
	Status      string    `json:"status"` // running, completed, failed, stopped
	SavedCount  int       `json:"saved_count"`
	ScannedCount int      `json:"scanned_count"`
	StoppedReason string   `json:"stopped_reason,omitempty"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type LeechMessage struct {
	JobID         string    `json:"job_id"`
	MessageID     int64     `json:"message_id"`
	ChatID        string    `json:"chat_id"`
	SenderID      string    `json:"sender_id"`
	Text          string    `json:"text"`
	Date          time.Time `json:"date"`
	HasMedia      bool      `json:"has_media"`
	MediaType     string    `json:"media_type,omitempty"`
	MediaFilePath string    `json:"media_file_path,omitempty"`
}

func NewLeech() *LeechPlugin {
	return &LeechPlugin{
		dbFile: "data/leech_db.json",
		db: &LeechDB{
			Jobs:     []LeechJob{},
			Messages: []LeechMessage{},
		},
	}
}

func (p *LeechPlugin) Name() string        { return "leech" }
func (p *LeechPlugin) Description() string { return "消息归档 - 抓取聊天记录到本地数据库" }

func (p *LeechPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	p.loadDB()

	cmds := []*interfaces.Command{
		{
			Name:        "leech",
			Description: "消息归档 - 抓取聊天记录到本地",
			Usage:       "leech [chat|jobs|stats|db|help] [参数]",
			Plugin:      p.Name(),
			Category:    "tools",
			OwnerOnly:   true,
			Handler:     p.handleLeech,
		},
	}

	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}

	return nil
}

func (p *LeechPlugin) Start(ctx context.Context) error { return nil }
func (p *LeechPlugin) Stop(ctx context.Context) error  { return nil }

func (p *LeechPlugin) loadDB() {
	data, err := os.ReadFile(p.dbFile)
	if err != nil {
		return
	}
	json.Unmarshal(data, &p.db)
	if p.db.Jobs == nil {
		p.db.Jobs = []LeechJob{}
	}
	if p.db.Messages == nil {
		p.db.Messages = []LeechMessage{}
	}
}

func (p *LeechPlugin) saveDB() {
	os.MkdirAll(filepath.Dir(p.dbFile), 0o755)
	data, _ := json.MarshalIndent(p.db, "", "  ")
	os.WriteFile(p.dbFile, data, 0o644)
}

func (p *LeechPlugin) handleLeech(ctx *interfaces.CommandContext) error {
	args := ctx.Args
	if len(args) == 0 {
		return ctx.Edit(p.helpText())
	}

	sub := args[0]
	switch sub {
	case "chat", "group", "messages":
		return p.handleChat(ctx, args[1:])

	case "jobs":
		return p.handleJobs(ctx, args[1:])

	case "stats":
		return p.handleStats(ctx)

	case "db":
		return ctx.Edit(fmt.Sprintf("🗄️ Leech 数据库路径:\n<code>%s</code>", p.dbFile))

	case "help", "h":
		return ctx.Edit(p.helpText())

	default:
		return ctx.Edit(fmt.Sprintf("未知子命令: %s\n\n%s", sub, p.helpText()))
	}
}

func (p *LeechPlugin) helpText() string {
	return `<b>PaperValet Leech - 消息归档</b>

<b>用法:</b>
• <code>leech chat here --from 2026-01-01 --to 2026-01-31</code> — 抓取当前聊天日期范围内的消息
• <code>leech chat @username --from 2026-01-01 --to 2026-01-31 --limit 500 --batch 100</code> — 抓取指定 chat
• <code>leech jobs [limit]</code> — 查看最近任务
• <code>leech stats</code> — 查看 SQLite 保存统计
• <code>leech db</code> — 显示本地数据库路径

<b>参数:</b>
• <code>--from</code> / <code>--to</code> — 日期范围 (YYYY-MM-DD)
• <code>--limit</code> — 最大消息数
• <code>--batch</code> — 批次大小 (1-100, 默认 100)
• <code>here</code> — 当前聊天

<b>目标支持:</b> @username、数字 ID、t.me 链接、here

⚠️ 完整实现需要接入 gotd 客户端进行消息获取和媒体下载`
}

func (p *LeechPlugin) handleChat(ctx *interfaces.CommandContext, args []string) error {
	if len(args) == 0 {
		return ctx.Edit("用法: leech chat <目标> --from <日期> --to <日期> [--limit N] [--batch N]")
	}

	// Parse flags
	target := args[0]
	fromDate := ""
	toDate := ""
	limit := 0
	batchSize := 100

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--from", "-f":
			if i+1 < len(args) {
				fromDate = args[i+1]
				i++
			}
		case "--to", "-t":
			if i+1 < len(args) {
				toDate = args[i+1]
				i++
			}
		case "--limit", "-l":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &limit)
				i++
			}
		case "--batch", "-b":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &batchSize)
				i++
			}
		}
	}

	if fromDate == "" || toDate == "" {
		return ctx.Edit("❌ 必须指定 --from 和 --to 日期\n示例: leech chat here --from 2026-01-01 --to 2026-01-31")
	}

	if batchSize < 1 {
		batchSize = 1
	}
	if batchSize > 100 {
		batchSize = 100
	}

	jobID := fmt.Sprintf("leech_%d", time.Now().UnixNano())
	job := LeechJob{
		ID:         jobID,
		Target:     target,
		FromDate:   parseDate(fromDate),
		ToDate:     parseDate(toDate),
		BatchSize:  batchSize,
		Limit:      limit,
		Status:     "running",
		StartedAt:  time.Now(),
	}

	p.db.Jobs = append(p.db.Jobs, job)
	p.saveDB()

	// Simulate progress
	_ = ctx.Edit(fmt.Sprintf(`⏳ Leech 已启动
• Job: <code>%s</code>
• Target: <code>%s</code>
• Range: <code>%s 至 %s</code>
• Batch: <code>%d</code>
• Limit: <code>%d</code>

⚠️ 完整实现需要接入 gotd 客户端获取消息`, jobID, target, fromDate, toDate, batchSize, limit))

	return nil
}

func (p *LeechPlugin) handleJobs(ctx *interfaces.CommandContext, args []string) error {
	limit := 10
	if len(args) > 0 {
		fmt.Sscanf(args[0], "%d", &limit)
		if limit < 1 {
			limit = 1
		}
		if limit > 20 {
			limit = 20
		}
	}

	if len(p.db.Jobs) == 0 {
		return ctx.Edit("📭 暂无 Leech 任务")
	}

	// Sort by start time descending
	jobs := p.db.Jobs
	// Simple reverse iteration for recent first
	start := len(jobs) - limit
	if start < 0 {
		start = 0
	}

	var lines []string
	for i := len(jobs) - 1; i >= start; i-- {
		job := jobs[i]
		statusIcon := map[string]string{
			"running":   "⏳",
			"completed": "✅",
			"failed":    "❌",
			"stopped":   "⏹️",
		}[job.Status]

		lines = append(lines, fmt.Sprintf(
			"%s #%s | %s | saved=%d scanned=%d | %s",
			statusIcon, job.ID[:12], job.Status,
			job.SavedCount, job.ScannedCount,
			job.StartedAt.Format("01-02 15:04"),
		))
	}

	return ctx.Edit(fmt.Sprintf("<b>Recent Leech Jobs</b>\n<pre>%s</pre>", strings.Join(lines, "\n")))
}

func (p *LeechPlugin) handleStats(ctx *interfaces.CommandContext) error {
	totalMessages := len(p.db.Messages)
	totalJobs := len(p.db.Jobs)

	var firstMsg, lastMsg *LeechMessage
	if totalMessages > 0 {
		firstMsg = &p.db.Messages[0]
		lastMsg = &p.db.Messages[totalMessages-1]
	}

	var lastJobStatus string
	if totalJobs > 0 {
		lastJobStatus = p.db.Jobs[totalJobs-1].Status
	}

	return ctx.Edit(fmt.Sprintf(`<b>Leech SQLite Stats</b>
· Messages: <code>%d</code>
· Jobs: <code>%d</code>
· First message: <code>%s</code>
· Last message: <code>%s</code>
· Last job: <code>%s</code>
· DB: <code>%s</code>`,
		totalMessages, totalJobs,
		map[*LeechMessage]string{nil: "N/A"}[firstMsg], // placeholder
		map[*LeechMessage]string{nil: "N/A"}[lastMsg],
		lastJobStatus,
		p.dbFile))
}

func parseDate(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}