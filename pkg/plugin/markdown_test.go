package plugin

import (
	"testing"

	"github.com/gotd/td/tg"
)

func TestMarkdownHelpersStayLiteral(t *testing.T) {
	cases := map[string]string{
		Escape(`a_b*c~d|e[f]g>h\i` + "`j"): `a_b*c~d|e[f]g>h\i` + "`j",
		Code("x`y"):                        "x`y",
		Code("`edge`"):                     "`edge`",
		Bold("**not nested**"):             "**not nested**",
		Link("[t]", "https://e.com/a b"):   "[t]",
	}
	for in, want := range cases {
		got, _ := ParseMarkdown(in, nil)
		if got != want {
			t.Errorf("%q rendered %q, want %q", in, got, want)
		}
	}
}

func TestParseMarkdownEntities(t *testing.T) {
	plain, ents := ParseMarkdown("**b** "+Code("c")+"\n"+Pre("x\ny"), nil)
	if plain != "b c\n\nx\ny" {
		t.Fatalf("plain %q", plain)
	}
	if len(ents) != 3 {
		t.Fatalf("entities %v", ents)
	}
	if _, ok := ents[0].(*tg.MessageEntityBold); !ok {
		t.Errorf("first entity %T", ents[0])
	}
}

func TestUnresolvableMentionDegrades(t *testing.T) {
	fail := func(int64) (tg.InputUserClass, error) { return nil, errTest }
	plain, _ := ParseMarkdown("hi "+Mention("bob", 42)+" **x**", fail)
	if plain != "hi bob x" {
		t.Fatalf("got %q", plain)
	}
}

type testErr struct{}

func (testErr) Error() string { return "nope" }

var errTest = testErr{}
