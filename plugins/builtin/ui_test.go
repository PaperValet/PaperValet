package builtin

import (
	"strings"
	"testing"
	"time"
)

func TestCardLayout(t *testing.T) {
	c := newCard("📦", "Title")
	c.section("Part").field("A", 1).rawField("B", "<code>x</code>").line("plain")
	got := c.String()
	for _, want := range []string{"📦 <b>Title</b>", "<b>Part</b>", "A  <code>1</code>"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestHumanDuration(t *testing.T) {
	if got := humanDuration(90061*time.Second, true); got != "1d 1h 1m" {
		t.Fatal(got)
	}
}
