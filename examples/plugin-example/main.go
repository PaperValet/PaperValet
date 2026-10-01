// Command example is a minimal PaperValet external plugin.
//
//	go work init . /path/to/PaperValet
//	go build -trimpath -buildmode=plugin -o example.so .
package main

import (
	"context"
	"os"
	"path/filepath"
	"strconv"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// Metadata is read by the loader and by `apt info`.
var Metadata = &plugin.PluginMetadata{
	Name:        "example",
	Description: "示例插件",
	DescEN:      "Example plugin",
	Version:     "1.0.0",
	Author:      "PaperValet",
	MinVersion:  "0.1.0",
}

type Example struct {
	host plugin.Host
}

// New is the entry point the loader looks up.
func New() *Example { return &Example{} }

func (p *Example) Name() string        { return Metadata.Name }
func (p *Example) Description() string { return Metadata.Description }
func (p *Example) DescEN() string      { return Metadata.DescEN }

func (p *Example) Init(_ context.Context, mgr plugin.Manager) error {
	p.host = mgr.Host()
	return mgr.RegisterCommand(&plugin.Command{
		Name:        "hello",
		Description: "打招呼，并数一数被叫了几次",
		DescEN:      "Say hello and count how often it was called",
		Usage:       "hello [名字]",
		UsageEN:     "hello [name]",
		Plugin:      p.Name(),
		Category:    "tools",
		Handler:     p.hello,
	})
}

func (p *Example) Start(context.Context) error { return nil }
func (p *Example) Stop(context.Context) error  { return nil }

func (p *Example) hello(ctx *plugin.CommandContext) error {
	name := ctx.Tlocal("世界", "World")
	if ctx.ArgCount() > 0 {
		name = ctx.GetArgs()
	}
	n := p.bump()
	return ctx.Edit("👋 " + ctx.Tlocal("你好，", "Hello, ") + plugin.Bold(name) +
		"\n" + ctx.Tlocal("第 ", "Call #") + plugin.Code(n) + ctx.Tlocal(" 次", ""))
}

// bump keeps a counter in data/example/count.
func (p *Example) bump() int {
	dir, err := p.host.DataDir(p.Name())
	if err != nil {
		return 0
	}
	path := filepath.Join(dir, "count")
	raw, _ := os.ReadFile(path)
	n, _ := strconv.Atoi(string(raw))
	n++
	_ = os.WriteFile(path, []byte(strconv.Itoa(n)), 0o600)
	return n
}
