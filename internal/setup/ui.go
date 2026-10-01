// Package setup implements the interactive `initialize` subcommand: language,
// API credentials, Telegram login and optional system service registration.
package setup

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"

	"github.com/TiaraBasori/PaperValet/internal/i18n"
)

// ErrAborted is returned when stdin closes mid-prompt.
var ErrAborted = errors.New("aborted")

const (
	ansiReset = "\033[0m"
	ansiBold  = "\033[1m"
	ansiDim   = "\033[2m"
	ansiRed   = "\033[31m"
	ansiGreen = "\033[32m"
	ansiYel   = "\033[33m"
	ansiCyan  = "\033[36m"
)

// UI is a small line-oriented terminal prompter. All text goes through the
// setup catalog in the language picked at the start.
type UI struct {
	in    *bufio.Reader
	out   io.Writer
	fd    int
	tty   bool
	color bool
	cat   *i18n.Catalog
	Lang  i18n.Lang
}

// NewUI builds a prompter on stdin/stdout.
func NewUI() *UI {
	fd := int(os.Stdin.Fd())
	tty := term.IsTerminal(fd)
	color := term.IsTerminal(int(os.Stdout.Fd())) && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"
	return &UI{
		in:    bufio.NewReader(os.Stdin),
		out:   os.Stdout,
		fd:    fd,
		tty:   tty,
		color: color,
		cat:   i18n.SetupCatalog(),
		Lang:  i18n.EnUS,
	}
}

// T translates a setup key into the chosen language.
func (u *UI) T(key string, args ...any) string { return u.cat.T(u.Lang, key, args...) }

func (u *UI) paint(code, s string) string {
	if !u.color {
		return s
	}
	return code + s + ansiReset
}

func (u *UI) line(s string) { fmt.Fprintln(u.out, "  "+s) }

// Blank prints an empty line.
func (u *UI) Blank() { fmt.Fprintln(u.out) }

// Title prints the banner.
func (u *UI) Title(s string) {
	u.Blank()
	u.line(u.paint(ansiBold+ansiCyan, s))
	u.line(u.paint(ansiDim, strings.Repeat("─", max(len([]rune(s)), 16))))
}

// Step prints a numbered section header.
func (u *UI) Step(n, total int, key string) {
	u.Blank()
	u.line(u.paint(ansiBold, u.T("setup.step", n, total, u.T(key))))
}

// Hint prints dimmed helper text.
func (u *UI) Hint(s string) { u.line(u.paint(ansiDim, s)) }

// OK prints a success line.
func (u *UI) OK(s string) { u.line(u.paint(ansiGreen, "✓ ") + s) }

// Warn prints a warning line.
func (u *UI) Warn(s string) { u.line(u.paint(ansiYel, "! ") + s) }

// Err prints an error line.
func (u *UI) Err(s string) { u.line(u.paint(ansiRed, "✗ ") + s) }

// Cmd highlights a shell command inside a sentence.
func (u *UI) Cmd(s string) string { return u.paint(ansiCyan, s) }

func (u *UI) prompt(label, def string) {
	p := u.paint(ansiCyan, "› ") + label
	if def != "" {
		p += " " + u.paint(ansiDim, "["+def+"]")
	}
	fmt.Fprint(u.out, "  "+p+": ")
}

func (u *UI) readLine() (string, error) {
	s, err := u.in.ReadString('\n')
	if err != nil && (s == "" || !errors.Is(err, io.EOF)) {
		if errors.Is(err, io.EOF) {
			fmt.Fprintln(u.out)
			return "", ErrAborted
		}
		return "", err
	}
	return strings.TrimSpace(s), nil
}

// Ask reads a line; empty input returns def. valid may be nil.
func (u *UI) Ask(label, def string, valid func(string) bool, invalidKey string) (string, error) {
	return u.ask(label, def, def, valid, invalidKey)
}

// AskMasked is Ask but shows a masked form of def in the prompt.
func (u *UI) AskMasked(label, def string, valid func(string) bool, invalidKey string) (string, error) {
	return u.ask(label, mask(def), def, valid, invalidKey)
}

func (u *UI) ask(label, shown, def string, valid func(string) bool, invalidKey string) (string, error) {
	for {
		u.prompt(label, shown)
		s, err := u.readLine()
		if err != nil {
			return "", err
		}
		if s == "" {
			s = def
		}
		if s != "" && (valid == nil || valid(s)) {
			return s, nil
		}
		if invalidKey != "" {
			u.Err(u.T(invalidKey))
		}
	}
}

// Secret reads a line without echo when stdin is a terminal.
func (u *UI) Secret(label string) (string, error) {
	for {
		u.prompt(label, "")
		var s string
		if u.tty {
			b, err := term.ReadPassword(u.fd)
			fmt.Fprintln(u.out)
			if err != nil {
				return "", err
			}
			s = string(b)
		} else {
			var err error
			if s, err = u.readLine(); err != nil {
				return "", err
			}
		}
		if s != "" {
			return s, nil
		}
	}
}

// Confirm asks a yes/no question.
func (u *UI) Confirm(label string, def bool) (bool, error) {
	hint := u.T("setup.no_yes")
	if def {
		hint = u.T("setup.yes_no")
	}
	for {
		fmt.Fprint(u.out, "  "+u.paint(ansiCyan, "› ")+label+" "+u.paint(ansiDim, hint)+" ")
		s, err := u.readLine()
		if err != nil {
			return false, err
		}
		switch strings.ToLower(s) {
		case "":
			return def, nil
		case "y", "yes", "是", "好":
			return true, nil
		case "n", "no", "否", "不":
			return false, nil
		}
	}
}

// Choose shows a numbered list and returns the picked index. invalid is shown
// on bad input; it is passed in because the language may not be chosen yet.
func (u *UI) Choose(options []string, def int, invalid string) (int, error) {
	for i, o := range options {
		u.line("  " + u.paint(ansiBold, strconv.Itoa(i+1)) + ") " + o)
	}
	for {
		u.prompt("", strconv.Itoa(def+1))
		s, err := u.readLine()
		if err != nil {
			return 0, err
		}
		if s == "" {
			return def, nil
		}
		if n, err := strconv.Atoi(s); err == nil && n >= 1 && n <= len(options) {
			return n - 1, nil
		}
		u.Err(invalid)
	}
}

// mask keeps only the edges of a secret for display.
func mask(s string) string {
	r := []rune(s)
	if len(r) <= 8 {
		return strings.Repeat("•", len(r))
	}
	return string(r[:4]) + "…" + string(r[len(r)-4:])
}
