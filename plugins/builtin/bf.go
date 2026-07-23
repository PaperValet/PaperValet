package builtin

import (
	"context"
	"fmt"
	"strings"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// BFPlugin provides a Brainfuck interpreter.
type BFPlugin struct{}

func NewBF() *BFPlugin { return &BFPlugin{} }

func (p *BFPlugin) Name() string        { return "bf" }
func (p *BFPlugin) Description() string { return "Brainfuck 解释器" }

func (p *BFPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "bf",
		Description: "执行 Brainfuck 代码",
		Usage:       "bf <代码> [输入]",
		Plugin:      p.Name(),
		Category:    "fun",
		Handler:     p.handleBF,
	})
}

func (p *BFPlugin) Start(_ context.Context) error { return nil }
func (p *BFPlugin) Stop(_ context.Context) error  { return nil }

func (p *BFPlugin) handleBF(ctx *interfaces.CommandContext) error {
	args := ctx.Args
	if len(args) == 0 {
		return ctx.Edit(`🧠 <b>Brainfuck 解释器</b>

<b>用法:</b>
• <code>.bf +[>+<---]>.</code> - 执行代码
• <code>.bf +[>+<---]>. "Hello"</code> - 带输入

<b>指令:</b>
> < 移动指针
+ - 增减数值
. , 输出输入
[ ] 循环

<b>示例:</b>
• <code>.bf ++++++++++[>+++++++>++++++++++>+++>+<<<<-]>++.>+.+++++++..+++.>++.<<+++++++++++++++.>.+++.------.--------.>+.>.</code> (Hello World! - Hello World!)`)
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
<b>输出:</b> <code>%s</code>`, escapeHTML(code), escapeHTML(input), escapeHTML(output)))
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

	// Precompute bracket matching
	bracketMap := make(map[int]int)
	var stack []int
	for i, c := range filtered {
		if c == '[' {
			stack = append(stack, i)
		} else if c == ']' {
			if len(stack) > 0 {
				open := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				bracketMap[open] = i
				bracketMap[i] = open
			}
		}
	}

	// Execute
	tape := make([]byte, 30000)
	ptr := 0
	inputPtr := 0
	var output strings.Builder

	for ip := 0; ip < len(filtered); ip++ {
		cmd := filtered[ip]
		switch cmd {
		case '>':
			ptr++
			if ptr >= len(tape) {
				ptr = 0
			}
		case '<':
			ptr--
			if ptr < 0 {
				ptr = len(tape) - 1
			}
		case '+':
			tape[ptr]++
		case '-':
			tape[ptr]--
		case '.':
			output.WriteByte(tape[ptr])
		case ',':
			if inputPtr < len(input) {
				tape[ptr] = input[inputPtr]
				inputPtr++
			} else {
				tape[ptr] = 0
			}
		case '[':
			if tape[ptr] == 0 {
				if jump, ok := bracketMap[ip]; ok {
					ip = jump
				}
			}
		case ']':
			if tape[ptr] != 0 {
				if jump, ok := bracketMap[ip]; ok {
					ip = jump
				}
			}
		}
	}

	return output.String()
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&")
	s = strings.ReplaceAll(s, "<", "<")
	s = strings.ReplaceAll(s, ">", ">")
	return s
}