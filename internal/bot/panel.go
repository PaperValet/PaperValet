package bot

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/TiaraBasori/PaperValet/internal/settings"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// Callback data layout (Telegram allows 64 bytes):
//
//	m                       main menu
//	n                       no-op (page counter)
//	l:<b|x>:<page>          system settings / external plugin panel
//	h:<plugin>              plugin settings screen (settings, page entry)
//	g:<page>[:f]            plugin manager: install, reload, remove; f refetches
//	o:<plugin>              one plugin in the manager
//	e:<plugin>:<key>        edit a setting (toggle flips it)
//	c:<plugin>:<key>:<i>    pick choice i
//	r:<plugin>:<key>        reset to default
//	p:<plugin>              open the plugin page
//	p:<plugin>:<data>       page button
//	a:<op>[:<plugin>]       ask to confirm op (rm, ra, la, ia)
//	y:<op>[:<plugin>]       run op (in, rm, rl, ra, la, ia)
const maxPageData = 32

// route rewrites a plugin view's callback buttons into the p:<plugin>:
// namespace and optionally appends a back row.
func (s *Service) route(name string, v *View, back bool) *View {
	out := &View{Text: v.Text}
	for _, row := range v.Buttons {
		var r []plugin.Button
		for _, b := range row {
			if b.URL == "" {
				if len(b.Data) > maxPageData {
					s.log.Warn("button data too long, dropped", "plugin", name, "data", b.Data)
					continue
				}
				b.Data = "p:" + name + ":" + b.Data
			}
			r = append(r, b)
		}
		if len(r) > 0 {
			out.Buttons = append(out.Buttons, r)
		}
	}
	if back {
		out.Buttons = append(out.Buttons, plugin.Row(s.backBtn(name)))
	}
	return out
}

func (s *Service) backBtn(name string) plugin.Button {
	return s.back("h:" + name)
}

// entries lists plugins with settings or a page, sorted.
func (s *Service) entries() []string {
	seen := map[string]bool{}
	var out []string
	for _, n := range s.settings.Names() {
		seen[n] = true
		out = append(out, n)
	}
	s.mu.RLock()
	for n := range s.pages {
		if !seen[n] {
			out = append(out, n)
		}
	}
	s.mu.RUnlock()
	sort.Strings(out)
	return out
}

// display formats a value for the panel.
func (s *Service) display(st *settings.Store, set *plugin.Setting) string {
	switch set.Kind {
	case plugin.SettingToggle:
		if st.Bool(set.Key) {
			return s.tl("开", "on")
		}
		return s.tl("关", "off")
	case plugin.SettingChoice:
		cur := st.String(set.Key)
		for _, c := range set.Choices {
			if c.Value == cur {
				return s.pick(c.Label, c.LabelEN, c.Value)
			}
		}
		return cur
	case plugin.SettingNumber:
		return strconv.Itoa(st.Int(set.Key))
	}
	v := st.String(set.Key)
	if v == "" {
		return s.tl("未设置", "not set")
	}
	if set.Secret {
		return "••••••"
	}
	return clip(v, 60)
}

// clip shortens s to n runes with an ellipsis.
func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// editView is the screen for changing one setting.
func (s *Service) editView(name string, st *settings.Store, set *plugin.Setting, problem string) *View {
	var b strings.Builder
	b.WriteString("✏️ **" + plugin.Escape(s.pick(set.Label, set.LabelEN, set.Key)) + "**\n\n")
	b.WriteString(s.tl("当前 ", "Current ") + plugin.Code(s.display(st, set)) + "\n")
	if h := s.pick(set.Hint, set.HintEN, ""); h != "" {
		b.WriteString("\n" + plugin.Escape(h) + "\n")
	}
	v := &View{}
	switch set.Kind {
	case plugin.SettingChoice:
		cur := st.String(set.Key)
		var row []plugin.Button
		for i, c := range set.Choices {
			label := s.pick(c.Label, c.LabelEN, c.Value)
			if c.Value == cur {
				label = "● " + label
			}
			row = append(row, plugin.Btn(label, fmt.Sprintf("c:%s:%s:%d", name, set.Key, i)))
			if len(row) == 2 {
				v.Buttons = append(v.Buttons, row)
				row = nil
			}
		}
		if len(row) > 0 {
			v.Buttons = append(v.Buttons, row)
		}
	case plugin.SettingNumber:
		if set.Min != 0 || set.Max != 0 {
			b.WriteString("\n" + s.tl("发送 ", "Send a number from ") + fmt.Sprintf("%d–%d", set.Min, set.Max) + s.tl(" 之间的数字", ""))
		} else {
			b.WriteString("\n" + s.tl("发送一个数字", "Send a number"))
		}
	case plugin.SettingText:
		b.WriteString("\n" + s.tl("直接发送新的值", "Send the new value"))
	}
	if problem != "" {
		b.WriteString("\n\n❌ " + plugin.Escape(problem))
	}
	v.Text = b.String()
	var last []plugin.Button
	if !st.IsDefault(set.Key) {
		last = append(last, plugin.Btn(s.tl("↺ 恢复默认", "↺ Default"), "r:"+name+":"+set.Key))
	}
	last = append(last, plugin.Btn(s.tl("« 返回", "« Back"), "h:"+name))
	v.Buttons = append(v.Buttons, last)
	return v
}

// pageView runs a plugin page and returns the routed view.
func (s *Service) pageView(ctx context.Context, name, data, input string, msgID int) (*View, string, error) {
	p := s.page(name)
	if p == nil {
		v, notice := s.infoView(name)
		return v, notice, nil
	}
	bc := &plugin.BotContext{Ctx: ctx, Lang: s.lang(), Data: data, Input: input}
	v, err := safeHandle(p, bc)
	if err != nil {
		return nil, "", err
	}
	if asked := bc.Asked(); asked != "" {
		s.setPending(&pending{plugin: name, key: asked, page: true, msgID: msgID, at: time.Now()})
	}
	notice, alert := bc.Notice()
	if v == nil {
		return nil, encodeNotice(notice, alert), nil
	}
	return s.route(name, v, true), encodeNotice(notice, alert), nil
}

func safeHandle(p *plugin.Page, bc *plugin.BotContext) (v *View, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("page panic: %v", r)
		}
	}()
	return p.Handle(bc)
}

// Notices travel as "!" + text for alerts, plain text for toasts.
func encodeNotice(text string, alert bool) string {
	if text != "" && alert {
		return "!" + text
	}
	return text
}

func (s *Service) setPending(p *pending) {
	s.mu.Lock()
	s.pending = p
	s.mu.Unlock()
}

func (s *Service) currentPending() *pending {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pending
}
