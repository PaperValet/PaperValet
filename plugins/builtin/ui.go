package builtin

import (
	"fmt"
	"strings"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// Shared output styling so every built-in reads the same way:
//
//	📦 Title
//
//	Section
//	  label  value
//
//	💡 hint
//
// Keep it flat: one emoji per header, values as inline code, no box drawing.
// Text is Telegram Markdown; wrap user-controlled text with esc or plugin.Code.

// card builds a message line by line.
type card struct{ b strings.Builder }

func newCard(icon, title string) *card {
	c := &card{}
	c.b.WriteString(icon + " **" + title + "**\n")
	return c
}

// section starts a titled block separated by a blank line.
func (c *card) section(title string) *card {
	c.b.WriteString("\n**" + title + "**\n")
	return c
}

// field writes "label  value" with value in code style.
func (c *card) field(label string, value any) *card {
	fmt.Fprintf(&c.b, "%s  %s\n", label, plugin.Code(value))
	return c
}

// rawField writes "label  value" with value already formatted as Markdown.
func (c *card) rawField(label, md string) *card {
	fmt.Fprintf(&c.b, "%s  %s\n", label, md)
	return c
}

// line writes preformatted Markdown.
func (c *card) line(md string) *card {
	c.b.WriteString(md + "\n")
	return c
}

// blank adds an empty line.
func (c *card) blank() *card {
	c.b.WriteString("\n")
	return c
}

// hint closes the card with a tip line.
func (c *card) hint(md string) *card {
	c.b.WriteString("\n💡 " + md + "\n")
	return c
}

func (c *card) String() string { return strings.TrimRight(c.b.String(), "\n") }

// cmdRef renders a command reference like `.apt i`.
func cmdRef(s string) string { return plugin.Code(s) }

// okLine / failLine / skipLine are the status rows used by batch results.
func okLine(name, note string) string   { return "✅ **" + esc(name) + "**  " + note }
func failLine(name, note string) string { return "❌ **" + esc(name) + "**  " + note }
func skipLine(name, note string) string { return "⚪ **" + esc(name) + "**  " + note }

// errText is the one-line error format.
func errText(msg string) string { return "❌ " + msg }
