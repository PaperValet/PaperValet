package main

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

type CalcPlugin struct{}

func New() *CalcPlugin { return &CalcPlugin{} }

func (p *CalcPlugin) Name() string        { return "calc" }
func (p *CalcPlugin) Description() string { return "计算器" }

func (p *CalcPlugin) Init(ctx context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&plugin.Command{
		Name:        "calc",
		Aliases:     []string{"calculate", "计算"},
		Description: "计算数学表达式",
		Usage:       "calc <表达式>",
		Plugin:      p.Name(),
		Category:    "tools",
		Handler:     p.handleCalc,
	})
}

func (p *CalcPlugin) Start(ctx context.Context) error { return nil }
func (p *CalcPlugin) Stop(ctx context.Context) error  { return nil }

func (p *CalcPlugin) handleCalc(ctx *plugin.CommandContext) error {
	if len(ctx.Args) == 0 {
		return ctx.Edit("🧮 用法：<code>calc <表达式></code>\n支持 + - * / % ^ ( ) sqrt sin cos tan asin acos atan log ln abs floor ceil round，常量 pi e")
	}
	expr := strings.Join(ctx.Args, " ")

	ps := &parser{s: expr}
	v, err := ps.parseExpr()
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 表达式错误: %v", err))
	}
	ps.skipSpace()
	if ps.pos < len(ps.s) {
		return ctx.Edit(fmt.Sprintf("❌ 表达式在 %q 处无法解析", ps.s[ps.pos:]))
	}

	result := strconv.FormatFloat(v, 'g', 12, 64)
	return ctx.Edit(fmt.Sprintf("🧮 <code>%s</code> = <b>%s</b>", expr, result))
}

// parser is a small recursive-descent math expression evaluator.
type parser struct {
	s   string
	pos int
}

func (p *parser) skipSpace() {
	for p.pos < len(p.s) && unicode.IsSpace(rune(p.s[p.pos])) {
		p.pos++
	}
}

func (p *parser) peek() byte {
	p.skipSpace()
	if p.pos >= len(p.s) {
		return 0
	}
	return p.s[p.pos]
}

func (p *parser) parseExpr() (float64, error) {
	v, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		switch p.peek() {
		case '+':
			p.pos++
			r, err := p.parseTerm()
			if err != nil {
				return 0, err
			}
			v += r
		case '-':
			p.pos++
			r, err := p.parseTerm()
			if err != nil {
				return 0, err
			}
			v -= r
		default:
			return v, nil
		}
	}
}

func (p *parser) parseTerm() (float64, error) {
	v, err := p.parseFactor()
	if err != nil {
		return 0, err
	}
	for {
		switch p.peek() {
		case '*':
			p.pos++
			r, err := p.parseFactor()
			if err != nil {
				return 0, err
			}
			v *= r
		case '/':
			p.pos++
			r, err := p.parseFactor()
			if err != nil {
				return 0, err
			}
			if r == 0 {
				return 0, fmt.Errorf("除数为 0")
			}
			v /= r
		case '%':
			p.pos++
			r, err := p.parseFactor()
			if err != nil {
				return 0, err
			}
			if r == 0 {
				return 0, fmt.Errorf("取模除数为 0")
			}
			v = math.Mod(v, r)
		default:
			return v, nil
		}
	}
}

func (p *parser) parseFactor() (float64, error) {
	base, err := p.parseUnary()
	if err != nil {
		return 0, err
	}
	if p.peek() == '^' {
		p.pos++
		exp, err := p.parseFactor() // right-associative
		if err != nil {
			return 0, err
		}
		return math.Pow(base, exp), nil
	}
	return base, nil
}

func (p *parser) parseUnary() (float64, error) {
	switch p.peek() {
	case '-':
		p.pos++
		v, err := p.parseUnary()
		return -v, err
	case '+':
		p.pos++
		return p.parseUnary()
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (float64, error) {
	p.skipSpace()
	if p.pos >= len(p.s) {
		return 0, fmt.Errorf("表达式意外结束")
	}

	c := p.s[p.pos]
	switch {
	case c == '(':
		p.pos++
		v, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		if p.peek() != ')' {
			return 0, fmt.Errorf("缺少右括号")
		}
		p.pos++
		return v, nil

	case c >= '0' && c <= '9' || c == '.':
		start := p.pos
		for p.pos < len(p.s) {
			ch := p.s[p.pos]
			if (ch >= '0' && ch <= '9') || ch == '.' || ch == 'e' || ch == 'E' ||
				((ch == '+' || ch == '-') && p.pos > start && (p.s[p.pos-1] == 'e' || p.s[p.pos-1] == 'E')) {
				p.pos++
			} else {
				break
			}
		}
		v, err := strconv.ParseFloat(p.s[start:p.pos], 64)
		if err != nil {
			return 0, fmt.Errorf("无效数字 %q", p.s[start:p.pos])
		}
		return v, nil

	case unicode.IsLetter(rune(c)):
		start := p.pos
		for p.pos < len(p.s) && unicode.IsLetter(rune(p.s[p.pos])) {
			p.pos++
		}
		name := p.s[start:p.pos]
		switch strings.ToLower(name) {
		case "pi", "π":
			return math.Pi, nil
		case "e":
			return math.E, nil
		}
		if p.peek() != '(' {
			return 0, fmt.Errorf("未知标识符 %q", name)
		}
		p.pos++ // consume '('
		arg, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		if p.peek() != ')' {
			return 0, fmt.Errorf("函数 %s 缺少右括号", name)
		}
		p.pos++
		return applyFunc(strings.ToLower(name), arg)
	}
	return 0, fmt.Errorf("无法解析 %q", string(c))
}

func applyFunc(name string, x float64) (float64, error) {
	switch name {
	case "sqrt":
		if x < 0 {
			return 0, fmt.Errorf("sqrt 参数不能为负")
		}
		return math.Sqrt(x), nil
	case "sin":
		return math.Sin(x), nil
	case "cos":
		return math.Cos(x), nil
	case "tan":
		return math.Tan(x), nil
	case "asin":
		return math.Asin(x), nil
	case "acos":
		return math.Acos(x), nil
	case "atan":
		return math.Atan(x), nil
	case "log":
		if x <= 0 {
			return 0, fmt.Errorf("log 参数必须为正")
		}
		return math.Log10(x), nil
	case "ln":
		if x <= 0 {
			return 0, fmt.Errorf("ln 参数必须为正")
		}
		return math.Log(x), nil
	case "abs":
		return math.Abs(x), nil
	case "floor":
		return math.Floor(x), nil
	case "ceil":
		return math.Ceil(x), nil
	case "round":
		return math.Round(x), nil
	}
	return 0, fmt.Errorf("未知函数 %q", name)
}
