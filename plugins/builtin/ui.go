package builtin

import (
	"fmt"
	"strings"
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
// Keep it flat: one emoji per header, values in <code>, no box drawing.

// card builds a message line by line.
type card struct{ b strings.Builder }

func newCard(icon, title string) *card {
	c := &card{}
	c.b.WriteString(icon + " <b>" + title + "</b>\n")
	return c
}

// section starts a titled block separated by a blank line.
func (c *card) section(title string) *card {
	c.b.WriteString("\n<b>" + title + "</b>\n")
	return c
}

// field writes "label  value" with value in code style.
func (c *card) field(label string, value any) *card {
	fmt.Fprintf(&c.b, "%s  <code>%s</code>\n", label, htmlEscape(fmt.Sprint(value)))
	return c
}

// rawField writes "label  value" with value already formatted as HTML.
func (c *card) rawField(label, html string) *card {
	fmt.Fprintf(&c.b, "%s  %s\n", label, html)
	return c
}

// line writes preformatted HTML.
func (c *card) line(html string) *card {
	c.b.WriteString(html + "\n")
	return c
}

// blank adds an empty line.
func (c *card) blank() *card {
	c.b.WriteString("\n")
	return c
}

// hint closes the card with a tip line.
func (c *card) hint(html string) *card {
	c.b.WriteString("\n💡 " + html + "\n")
	return c
}

func (c *card) String() string { return strings.TrimRight(c.b.String(), "\n") }

// cmdRef renders a command reference like <code>.apt i</code>.
func cmdRef(s string) string { return "<code>" + htmlEscape(s) + "</code>" }

// okLine / failLine / skipLine are the status rows used by batch results.
func okLine(name, note string) string   { return "✅ <b>" + htmlEscape(name) + "</b>  " + note }
func failLine(name, note string) string { return "❌ <b>" + htmlEscape(name) + "</b>  " + note }
func skipLine(name, note string) string { return "⚪ <b>" + htmlEscape(name) + "</b>  " + note }

// errText is the one-line error format.
func errText(msg string) string { return "❌ " + msg }
