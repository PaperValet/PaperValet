package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type SendLogPlugin struct{}

func New() (plugin.Plugin, error) {
	return &SendLogPlugin{}, nil
}

var Metadata = &plugin.PluginMetadata{
	Name:        "sendlog",
	Description: "发送日志",
	Version:     "1.0.0",
	Author:      "PaperValet",
	MinVersion:  "0.1.0",
}

func (p *SendLogPlugin) Name() string        { return "sendlog" }
func (p *SendLogPlugin) Description() string { return "发送日志" }

func (p *SendLogPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&plugin.Command{
		Name:        "sendlog",
		Aliases:     []string{"slog", "log"},
		Description: "发送运行日志到当前聊天",
		Usage:       "sendlog [行数]",
		Plugin:      p.Name(),
		Category:    "tools",
		Handler:     p.handleSendLog,
	})
}

func (p *SendLogPlugin) handleSendLog(ctx *plugin.CommandContext) error {
	args := ctx.Args
	lines := 100
	if len(args) > 0 {
		if n, err := strconv.Atoi(args[0]); err == nil && n > 0 && n <= 1000 {
			lines = n
		}
	}

	// Locate the newest PaperValet log file.
	logFile := findLatestLog()
	if logFile == "" {
		return ctx.Edit("❌ 未找到日志文件（已检查 ./logs、~/.pm2/logs、/var/log/papervalet）")
	}

	data, err := os.ReadFile(logFile)
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 读取日志失败: %v", err))
	}

	text := string(data)
	if text == "" {
		return ctx.Edit(fmt.Sprintf("📋 日志文件为空: <code>%s</code>", logFile))
	}

	// Keep only the last N lines.
	allLines := strings.Split(text, "\n")
	if len(allLines) > lines {
		allLines = allLines[len(allLines)-lines:]
	}
	tail := strings.Join(allLines, "\n")

	// Telegram messages are capped at 4096 chars; truncate from the head.
	const maxLen = 3500
	if len(tail) > maxLen {
		tail = "…(截断)\n" + tail[len(tail)-maxLen:]
	}

	return ctx.Edit(fmt.Sprintf("📋 <b>日志尾部</b> (<code>%s</code>)\n\n<pre>%s</pre>", filepath.Base(logFile), tail))
}

func findLatestLog() string {
	var candidates []string
	for _, dir := range []string{"logs", filepath.Join(os.Getenv("HOME"), ".pm2", "logs"), "/var/log/papervalet"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := strings.ToLower(e.Name())
			if strings.Contains(name, "paper") && strings.HasSuffix(name, ".log") {
				candidates = append(candidates, filepath.Join(dir, e.Name()))
			}
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	// Newest first.
	sort.Slice(candidates, func(i, j int) bool {
		iInfo, iErr := os.Stat(candidates[i])
		jInfo, jErr := os.Stat(candidates[j])
		if iErr != nil || jErr != nil {
			return false
		}
		return iInfo.ModTime().After(jInfo.ModTime())
	})
	return candidates[0]
}

func (p *SendLogPlugin) Start(ctx context.Context) error { return nil }
func (p *SendLogPlugin) Stop(ctx context.Context) error  { return nil }
