package builtin

import (
	"strings"
	"testing"
	"time"
)

func TestCardLayout(t *testing.T) {
	c := newCard("📦", "Title")
	c.section("Part").field("A", 1).rawField("B", "`x`").line("plain")
	got := c.String()
	for _, want := range []string{"📦 **Title**", "**Part**", "A  `1`"} {
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
