package plugin

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gotd/td/telegram/message/entity"
	"github.com/gotd/td/telegram/message/markdown"
	"github.com/gotd/td/tg"
)

// Message text passed to Reply and Edit is Telegram-style Markdown:
//
//	**bold**  _italic_  ~~strike~~  ||spoiler||  `code`
//	```pre```  [text](url)  > quote
//
// Wrap anything user-controlled with Escape, Code or Pre so stray
// markdown characters stay literal.

var mdEscaper = strings.NewReplacer(
	`\`, `\\`, "`", "\\`", `*`, `\*`, `_`, `\_`, `~`, `\~`,
	`|`, `\|`, `[`, `\[`, `]`, `\]`, `>`, `\>`,
)

// Escape makes s render literally inside Markdown text.
func Escape(s string) string { return mdEscaper.Replace(s) }

// Code renders v as inline code. Backticks inside are kept verbatim.
func Code(v any) string {
	s := strings.ReplaceAll(fmt.Sprint(v), "\n", " ")
	if s == "" {
		return ""
	}
	fence := strings.Repeat("`", longestRun(s, '`')+1)
	if strings.HasPrefix(s, "`") || strings.HasSuffix(s, "`") ||
		(strings.HasPrefix(s, " ") && strings.HasSuffix(s, " ") && strings.TrimSpace(s) != "") {
		s = " " + s + " "
	}
	return fence + s + fence
}

// Pre renders s as a code block on its own lines.
func Pre(s string) string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return ""
	}
	n := longestRun(s, '`') + 1
	if n < 3 {
		n = 3
	}
	fence := strings.Repeat("`", n)
	return fence + "\n" + s + "\n" + fence
}

// Bold renders plain text s in bold.
func Bold(s string) string {
	if s == "" {
		return ""
	}
	return "**" + Escape(s) + "**"
}

// Italic renders plain text s in italic.
func Italic(s string) string {
	if s == "" {
		return ""
	}
	return "_" + Escape(s) + "_"
}

// Link renders plain text as a link to url.
func Link(text, url string) string {
	r := strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`, " ", "%20")
	return "[" + Escape(text) + "](" + r.Replace(url) + ")"
}

// Mention links text to a user by id.
func Mention(text string, id int64) string {
	return Link(text, fmt.Sprintf("tg://user?id=%d", id))
}

func longestRun(s string, c byte) int {
	best, cur := 0, 0
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			cur++
			if cur > best {
				best = cur
			}
		} else {
			cur = 0
		}
	}
	return best
}

var mentionLink = regexp.MustCompile(`\[((?:\\.|[^\]\\])*)\]\(tg://user\?id=\d+\)`)

// ParseMarkdown converts Markdown into plain text plus entities. Mentions of
// users that cannot be resolved degrade to plain text; any other failure
// falls back to the raw text.
func ParseMarkdown(text string, resolve func(int64) (tg.InputUserClass, error)) (string, []tg.MessageEntityClass) {
	parse := func(s string) (string, []tg.MessageEntityClass, error) {
		var b entity.Builder
		if err := markdown.Markdown(strings.NewReader(s), &b, markdown.Options{UserResolver: resolve}); err != nil {
			return "", nil, err
		}
		plain, ents := b.Complete()
		return plain, ents, nil
	}
	if plain, ents, err := parse(text); err == nil {
		return plain, ents
	}
	if stripped := mentionLink.ReplaceAllString(text, "$1"); stripped != text {
		if plain, ents, err := parse(stripped); err == nil {
			return plain, ents
		}
	}
	return text, nil
}
