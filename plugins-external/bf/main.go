package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type BFPlugin struct{}

func New() (plugin.Plugin, error) {
	return &BFPlugin{}, nil
}

var Metadata = &plugin.PluginMetadata{
	Name:        "bf",
	Description: "Brainfuck解释器",
	Version:     "1.0.0",
	Author:      "PaperValet",
	MinVersion:  "0.1.0",
}

func (p *BFPlugin) Name() string        { return "bf" }
func (p *BFPlugin) Description() string { return "Brainfuck解释器" }

func (p *BFPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&plugin.Command{
		Name:        "bf",
		Description: "执行Brainfuck代码",
		Usage:       "bf <代码> [输入]",
		Plugin:      p.Name(),
		Category:    "fun",
		Handler:     p.handleBF,
	})
}

func (p *BFPlugin) handleBF(ctx *plugin.CommandContext) error {
	args := ctx.Args
	if len(args) == 0 {
		return ctx.Edit(`🧠 <b>Brainfuck解释器</b>

<b>用法:</b>
• <code>.bf +[>+<---]>.</code> - 执行代码
• <code>.bf +[>+<---]>. "Hello"</code> - 带输入

<b>指令:</b>
> < 移动指针
+ - 增减数值
. , 输出输入
[ ] 循环`)
	}

	code := args[0]
	input := ""
	if len(args) > 1 {
		input = strings.Join(args[1:], " ")
	}

	output := runBF(code, input)
	if len(output) > 4000 {
		output = output[:4000] + "..."
	}

	return ctx.Edit(fmt.Sprintf(`🧠 <b>Brainfuck 执行</b>

<b>代码:</b> <code>%s</code>
<b>输入:</b> <code>%s</code>
<b>输出:</b> <code>%s</code>`, code, input, output))
}

func runBF(code, input string) string {
	// Filter valid BF commands
	valid := map[rune]bool{'>': true, '<': true, '+': true, '-': true, '.': true, ',': true, '[': true, ']': true}
	var filtered []rune
	for _, c := range code {
		if valid[c] {
			filtered = append(filtered, c)
		}
	}
	code = string(filtered)

	// Build jump map
	jump := make(map[int]int)
	stack := []int{}
	for i, c := range code {
		if c == '[' {
			stack = append(stack, i)
		} else if c == ']' {
			if len(stack) == 0 {
				return "错误: 不匹配的 ]"
			}
			j := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			jump[i] = j
			jump[j] = i
		}
	}
	if len(stack) > 0 {
		return "错误: 不匹配的 ["
	}

	// Execute
	mem := make([]byte, 30000)
	ptr := 0
	ip := 0
	inputPtr := 0
	var output strings.Builder
	maxSteps := 1000000
	steps := 0

	for ip < len(code) && steps < maxSteps {
		steps++
		switch code[ip] {
		case '>':
			ptr++
			if ptr >= len(mem) {
				ptr = 0
			}
		case '<':
			ptr--
			if ptr < 0 {
				ptr = len(mem) - 1
			}
		case '+':
			mem[ptr]++
		case '-':
			mem[ptr]--
		case '.':
			output.WriteByte(mem[ptr])
		case ',':
			if inputPtr < len(input) {
				mem[ptr] = input[inputPtr]
				inputPtr++
			} else {
				mem[ptr] = 0
			}
		case '[':
			if mem[ptr] == 0 {
				ip = jump[ip]
			}
		case ']':
			if mem[ptr] != 0 {
				ip = jump[ip]
			}
		}
		ip++
	}

	if steps >= maxSteps {
		return output.String() + "\n⚠️ 执行步数超限"
	}

	return output.String()
}

func (p *BFPlugin) Start(ctx context.Context) error { return nil }
func (p *BFPlugin) Stop(ctx context.Context) error  { return nil }