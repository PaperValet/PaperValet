package bot

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/internal/settings"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// Callback data layout (Telegram allows 64 bytes):
//
//	m                       main menu
//	h:<plugin>              plugin screen (settings, page entry)
//	e:<plugin>:<key>        edit a setting (toggle flips it)
//	c:<plugin>:<key>:<i>    pick choice i
//	r:<plugin>:<key>        reset to default
//	p:<plugin>              open the plugin page
//	p:<plugin>:<data>       page button
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
	if _, ok := s.settings.Get(name); ok && s.page(name) != nil {
		return plugin.Btn(s.tl("« 返回", "« Back"), "h:"+name)
	}
	return plugin.Btn(s.tl("« 返回", "« Back"), "m")
}

func (s *Service) title(name string) string {
	if st, ok := s.settings.Get(name); ok {
		sp := st.Spec()
		return s.pick(sp.Title, sp.TitleEN, name)
	}
	if p := s.page(name); p != nil {
		return s.pick(p.Title, p.TitleEN, name)
	}
	return name
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
	s.sortNames(out)
	return out
}

// sortNames puts pinned panels (built-ins) first in their order, then the
// rest alphabetically.
func (s *Service) sortNames(n []string) {
	s.mu.RLock()
	rank := make(map[string]int, len(s.pinned))
	for i, p := range s.pinned {
		rank[p] = i + 1
	}
	s.mu.RUnlock()
	sort.SliceStable(n, func(i, j int) bool {
		ri, rj := rank[n[i]], rank[n[j]]
		switch {
		case ri != 0 && rj != 0:
			return ri < rj
		case ri != 0 || rj != 0:
			return ri != 0
		}
		return n[i] < n[j]
	})
}

func (s *Service) menuView() *View {
	v := &View{Text: "🗂 **PaperValet**\n\n" + s.tl("选一个插件调整设置", "Pick a plugin to adjust")}
	names := s.entries()
	if len(names) == 0 {
		v.Text += "\n\n" + s.tl("还没有插件提供设置", "No plugin offers settings yet")
		return v
	}
	var row []plugin.Button
	for _, n := range names {
		row = append(row, plugin.Btn(s.title(n), "h:"+n))
		if len(row) == 2 {
			v.Buttons = append(v.Buttons, row)
			row = nil
		}
	}
	if len(row) > 0 {
		v.Buttons = append(v.Buttons, row)
	}
	return v
}

// pluginView renders a plugin's settings panel, or its page when it has
// no settings.
func (s *Service) pluginView(ctx context.Context, name string) (*View, string, error) {
	st, ok := s.settings.Get(name)
	if !ok {
		if s.page(name) != nil {
			return s.pageView(ctx, name, "", "", 0)
		}
		return s.menuView(), "", nil
	}
	sp := st.Spec()
	var b strings.Builder
	b.WriteString("⚙️ **" + plugin.Escape(s.title(name)) + "**\n")
	for i := range sp.Settings {
		set := &sp.Settings[i]
		b.WriteString("\n" + plugin.Escape(s.pick(set.Label, set.LabelEN, set.Key)) + "  " + plugin.Code(s.display(st, set)))
	}
	v := &View{Text: b.String()}
	for i := range sp.Settings {
		set := &sp.Settings[i]
		label := s.pick(set.Label, set.LabelEN, set.Key)
		if set.Kind == plugin.SettingToggle {
			mark := "⬜ "
			if st.Bool(set.Key) {
				mark = "✅ "
			}
			label = mark + label
		} else {
			label = "✏️ " + label
		}
		v.Buttons = append(v.Buttons, plugin.Row(plugin.Btn(label, "e:"+name+":"+set.Key)))
	}
	if p := s.page(name); p != nil {
		v.Buttons = append(v.Buttons, plugin.Row(plugin.Btn("📋 "+s.pick(p.Title, p.TitleEN, name), "p:"+name)))
	}
	v.Buttons = append(v.Buttons, plugin.Row(plugin.Btn(s.tl("« 返回", "« Back"), "m")))
	return v, "", nil
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
	return v
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
		return s.menuView(), "", nil
	}
	bc := &plugin.BotContext{Ctx: ctx, Lang: s.lang(), Data: data, Input: input}
	v, err := safeHandle(p, bc)
	if err != nil {
		return nil, "", err
	}
	if asked := bc.Asked(); asked != "" {
		s.setPending(&pending{plugin: name, key: asked, page: true, msgID: msgID})
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

// ---------------------------------------------------------------- updates

func (s *Service) handle(ctx context.Context, u tg.UpdatesClass) error {
	var list []tg.UpdateClass
	switch v := u.(type) {
	case *tg.Updates:
		s.harvest(v.Users)
		list = v.Updates
	case *tg.UpdatesCombined:
		s.harvest(v.Users)
		list = v.Updates
	case *tg.UpdateShort:
		list = []tg.UpdateClass{v.Update}
	case *tg.UpdateShortMessage:
		if !v.Out {
			go s.onMessage(v.UserID, v.ID, v.Message)
		}
		return nil
	}
	for _, up := range list {
		switch x := up.(type) {
		case *tg.UpdateNewMessage:
			m, ok := x.Message.(*tg.Message)
			if !ok || m.Out {
				continue
			}
			pu, ok := m.PeerID.(*tg.PeerUser)
			if !ok {
				continue
			}
			go s.onMessage(pu.UserID, m.ID, m.Message)
		case *tg.UpdateBotCallbackQuery:
			go s.onCallback(x)
		}
	}
	return nil
}

func (s *Service) harvest(users []tg.UserClass) {
	for _, u := range users {
		if x, ok := u.(*tg.User); ok && !x.Min && x.AccessHash != 0 {
			s.peers.RegisterPeer(x.ID, x.AccessHash, "user")
		}
	}
}

func opCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 2*time.Minute)
}

func (s *Service) onMessage(from int64, msgID int, text string) {
	if from != s.owner() || s.owner() == 0 {
		return
	}
	ctx, cancel := opCtx()
	defer cancel()
	text = strings.TrimSpace(text)
	if cmd, ok := strings.CutPrefix(text, "/"); ok {
		cmd, _, _ = strings.Cut(cmd, " ")
		cmd, _, _ = strings.Cut(cmd, "@")
		s.setPending(nil)
		switch cmd {
		case "start", "menu":
			s.sendView(ctx, s.menuView())
			return
		}
		s.mu.RLock()
		var target string
		for n, p := range s.pages {
			if p.Command == cmd {
				target = n
			}
		}
		s.mu.RUnlock()
		if target != "" {
			v, _, err := s.pageView(ctx, target, "", "", 0)
			s.sendResult(ctx, v, err)
			return
		}
		s.sendView(ctx, s.menuView())
		return
	}

	p := s.currentPending()
	if p == nil {
		s.sendView(ctx, s.menuView())
		return
	}
	s.deleteMsg(ctx, msgID)
	if p.page {
		s.setPending(nil)
		v, _, err := s.pageView(ctx, p.plugin, p.key, text, p.msgID)
		if p.msgID != 0 && err == nil && v != nil {
			if s.edit(ctx, s.owner(), p.msgID, v) == nil {
				return
			}
		}
		s.sendResult(ctx, v, err)
		return
	}
	st, ok := s.settings.Get(p.plugin)
	if !ok {
		s.setPending(nil)
		return
	}
	set, ok := st.Setting(p.key)
	if !ok {
		s.setPending(nil)
		return
	}
	var v *View
	if err := st.SetText(p.key, text); err != nil {
		v = s.editView(p.plugin, st, set, s.localErr(err))
	} else {
		s.setPending(nil)
		v, _, _ = s.pluginView(ctx, p.plugin)
	}
	if p.msgID == 0 || s.edit(ctx, s.owner(), p.msgID, v) != nil {
		s.sendView(ctx, v)
	}
}

// localErr shows a validation error in the owner's language.
func (s *Service) localErr(err error) string {
	var inv *plugin.InvalidError
	if errors.As(err, &inv) {
		return inv.Text(s.lang())
	}
	return err.Error()
}

func (s *Service) sendView(ctx context.Context, v *View) {
	if _, err := s.send(ctx, s.owner(), v); err != nil {
		s.log.Warn("send", "error", err)
	}
}

func (s *Service) sendResult(ctx context.Context, v *View, err error) {
	if err != nil {
		v = &View{Text: "❌ " + plugin.Escape(err.Error()), Buttons: [][]plugin.Button{plugin.Row(plugin.Btn(s.tl("« 菜单", "« Menu"), "m"))}}
	}
	if v != nil {
		s.sendView(ctx, v)
	}
}

func (s *Service) answer(ctx context.Context, q *tg.UpdateBotCallbackQuery, notice string) {
	req := &tg.MessagesSetBotCallbackAnswerRequest{QueryID: q.QueryID}
	if notice != "" {
		alert := strings.HasPrefix(notice, "!")
		req.SetMessage(strings.TrimPrefix(notice, "!"))
		if alert {
			req.SetAlert(true)
		}
	}
	_, _ = s.api.MessagesSetBotCallbackAnswer(ctx, req)
}

func (s *Service) onCallback(q *tg.UpdateBotCallbackQuery) {
	ctx, cancel := opCtx()
	defer cancel()
	if q.UserID != s.owner() || s.owner() == 0 {
		s.answer(ctx, q, "!"+s.tl("这个机器人只为主人服务", "This bot only serves its owner"))
		return
	}
	v, notice, err := s.dispatch(ctx, string(q.Data), q.MsgID)
	if err != nil {
		s.answer(ctx, q, "!"+s.localErr(err))
		return
	}
	s.answer(ctx, q, notice)
	if v != nil {
		if err := s.edit(ctx, q.UserID, q.MsgID, v); err != nil {
			s.log.Warn("edit panel", "error", err)
		}
	}
}

// dispatch turns callback data into the next view. A nil view leaves the
// message as it is.
func (s *Service) dispatch(ctx context.Context, data string, msgID int) (*View, string, error) {
	kind, rest, _ := strings.Cut(data, ":")
	if kind != "e" {
		s.setPending(nil)
	}
	switch kind {
	case "m":
		return s.menuView(), "", nil
	case "h":
		return s.pluginView(ctx, rest)
	case "p":
		name, pd, _ := strings.Cut(rest, ":")
		return s.pageView(ctx, name, pd, "", msgID)
	}

	parts := strings.Split(rest, ":")
	if len(parts) < 2 {
		return nil, "", errors.New("bad button")
	}
	name, key := parts[0], parts[1]
	st, ok := s.settings.Get(name)
	if !ok {
		return s.menuView(), "", nil
	}
	set, ok := st.Setting(key)
	if !ok {
		v, n, err := s.pluginView(ctx, name)
		return v, n, err
	}
	switch kind {
	case "e":
		s.setPending(nil)
		switch set.Kind {
		case plugin.SettingToggle:
			if err := st.Set(key, !st.Bool(key)); err != nil {
				return nil, "", err
			}
			return s.pluginViewNoErr(ctx, name), "", nil
		case plugin.SettingText, plugin.SettingNumber:
			s.setPending(&pending{plugin: name, key: key, msgID: msgID})
		}
		return s.editView(name, st, set, ""), "", nil
	case "c":
		if len(parts) < 3 {
			return nil, "", errors.New("bad button")
		}
		i, err := strconv.Atoi(parts[2])
		if err != nil || i < 0 || i >= len(set.Choices) {
			return nil, "", errors.New("bad choice")
		}
		if err := st.Set(key, set.Choices[i].Value); err != nil {
			return nil, "", err
		}
		return s.pluginViewNoErr(ctx, name), s.tl("已保存", "Saved"), nil
	case "r":
		if err := st.Reset(key); err != nil {
			return nil, "", err
		}
		return s.pluginViewNoErr(ctx, name), s.tl("已恢复默认", "Reset"), nil
	}
	return nil, "", errors.New("bad button")
}

func (s *Service) pluginViewNoErr(ctx context.Context, name string) *View {
	v, _, err := s.pluginView(ctx, name)
	if err != nil || v == nil {
		return s.menuView()
	}
	return v
}
