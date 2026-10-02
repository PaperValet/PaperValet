package bot

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// Progress refresh: state changes show within refreshTick, and the
// elapsed time ticks at least every refreshIdle.
const (
	refreshTick = time.Second
	refreshIdle = 3 * time.Second
	barCells    = 10
)

// progress keeps a panel message showing a running operation: a bar,
// the finished items, the current one and the elapsed time.
type progress struct {
	s     *Service
	ctx   context.Context
	at    target
	title string
	total int // 0 for a single item: no bar
	start time.Time

	mu    sync.Mutex
	done  int
	cur   string
	lines []string
	ver   int

	stop chan struct{}
	quit chan struct{}
}

// startProgress draws the first frame at once and keeps it fresh until
// finish. Without a panel message (tests) it only tracks state.
func (s *Service) startProgress(ctx context.Context, at target, title string, total int) *progress {
	p := &progress{s: s, ctx: ctx, at: at, title: title, total: total, start: time.Now()}
	if at.msgID == 0 {
		return p
	}
	p.draw(0)
	p.stop, p.quit = make(chan struct{}), make(chan struct{})
	go p.loop()
	return p
}

func (p *progress) loop() {
	defer close(p.quit)
	t := time.NewTicker(refreshTick)
	defer t.Stop()
	shown, last, frame := 0, time.Now(), 0
	for {
		select {
		case <-p.stop:
			return
		case <-p.ctx.Done():
			return
		case <-t.C:
		}
		p.mu.Lock()
		ver := p.ver
		p.mu.Unlock()
		if ver == shown && time.Since(last) < refreshIdle {
			continue
		}
		frame++
		p.draw(frame)
		shown, last = ver, time.Now()
	}
}

func (p *progress) draw(frame int) {
	if err := p.s.edit(p.ctx, p.at.chatID, p.at.msgID, &View{Text: p.text(frame)}); err != nil {
		p.s.log.Warn("progress edit", "error", err)
	}
}

// step marks name as the item being worked on.
func (p *progress) step(name string) {
	p.mu.Lock()
	p.cur = name
	p.ver++
	p.mu.Unlock()
}

// result records the current item's outcome line.
func (p *progress) result(line string) {
	p.mu.Lock()
	p.done++
	p.cur = ""
	p.lines = append(p.lines, line)
	p.ver++
	p.mu.Unlock()
}

// finish stops refreshing; the caller draws the final view.
func (p *progress) finish() {
	if p.stop == nil {
		return
	}
	close(p.stop)
	<-p.quit
}

func (p *progress) elapsed() string { return elapsed(time.Since(p.start)) }

// text renders a frame; the hourglass flips between frames.
func (p *progress) text(frame int) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	icon := "⏳"
	if frame%2 == 1 {
		icon = "⌛"
	}
	var b strings.Builder
	b.WriteString(icon + " **" + p.title + "**\n")
	if p.total > 0 {
		b.WriteString(fmt.Sprintf("%s  %d/%d  ·  %s", bar(p.done, p.total), p.done, p.total, p.elapsed()))
	} else {
		b.WriteString(p.s.tl("已用 ", "Elapsed ") + p.elapsed())
	}
	var q []string
	q = append(q, p.lines...)
	if p.cur != "" {
		q = append(q, "▸ "+plugin.Code(p.cur)+"  "+p.s.tl("进行中…", "working…"))
	}
	if len(q) > 0 {
		b.WriteString("\n" + quote(q))
	}
	return b.String()
}

// bar draws done/total as a ten-cell bar.
func bar(done, total int) string {
	if total <= 0 {
		return ""
	}
	n := min(barCells, done*barCells/total)
	return strings.Repeat("▰", n) + strings.Repeat("▱", barCells-n)
}

// elapsed formats a duration as 0.4s, 12s or 2m05s.
func elapsed(d time.Duration) string {
	switch {
	case d < 10*time.Second:
		return fmt.Sprintf("%.1fs", d.Seconds())
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
}

// quote renders lines as one block quote.
func quote(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return ">" + strings.Join(lines, "\n>")
}
