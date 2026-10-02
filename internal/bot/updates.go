package bot

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// pendingTTL drops a typed-answer prompt the owner walked away from.
const pendingTTL = 15 * time.Minute

// opTimeout bounds one tap or message; bulk installs need minutes.
const opTimeout = 10 * time.Minute

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
			text := m.Message
			if m.Media != nil {
				// Captions are not answers; media gets the "send text" hint.
				text = ""
			}
			go s.onMessage(pu.UserID, m.ID, text)
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
	return context.WithTimeout(context.Background(), opTimeout)
}

// isOwner rejects everyone before the owner is known.
func (s *Service) isOwner(id int64) bool {
	o := s.owner()
	return o != 0 && id == o
}

func (s *Service) onMessage(from int64, msgID int, text string) {
	if !s.isOwner(from) {
		return
	}
	ctx, cancel := opCtx()
	defer cancel()
	text = strings.TrimSpace(text)
	if cmd, ok := strings.CutPrefix(text, "/"); ok && cmd != "" {
		s.onCommand(ctx, cmd)
		return
	}

	p := s.currentPending()
	if p != nil && time.Since(p.at) > pendingTTL {
		s.setPending(nil)
		p = nil
	}
	if p == nil {
		s.sendView(ctx, s.menuView())
		return
	}
	s.deleteMsg(ctx, msgID)
	if text == "" {
		s.showPanel(ctx, p.msgID, &View{
			Text:    "⚠️ " + s.tl("请发送文字，或发 /cancel 取消", "Send text, or /cancel"),
			Buttons: [][]plugin.Button{plugin.Row(s.back(p.backData()))},
		})
		return
	}
	if p.page {
		s.setPending(nil)
		v, notice, err := s.pageView(ctx, p.plugin, p.key, text, p.msgID)
		if err != nil {
			v = s.errView(err)
		}
		if v == nil && notice != "" {
			v = &View{Text: "ℹ️ " + plugin.Escape(strings.TrimPrefix(notice, "!")),
				Buttons: [][]plugin.Button{plugin.Row(s.back(p.backData()))}}
		}
		if v != nil {
			s.showPanel(ctx, p.msgID, v)
		}
		return
	}
	st, ok := s.settings.Get(p.plugin)
	if !ok {
		s.setPending(nil)
		v, _ := s.infoView(p.plugin)
		s.showPanel(ctx, p.msgID, v)
		return
	}
	set, ok := st.Setting(p.key)
	if !ok {
		s.setPending(nil)
		v, _ := s.infoView(p.plugin)
		s.showPanel(ctx, p.msgID, v)
		return
	}
	lang := s.lang()
	var v *View
	if err := st.SetText(p.key, text); err != nil {
		v = s.editView(p.plugin, st, set, s.localErr(err))
	} else {
		s.setPending(nil)
		v, _ = s.infoView(p.plugin)
	}
	s.afterChange(lang)
	s.showPanel(ctx, p.msgID, v)
}

func (s *Service) onCommand(ctx context.Context, cmd string) {
	cmd, _, _ = strings.Cut(cmd, " ")
	cmd, at, _ := strings.Cut(cmd, "@")
	if at != "" && !strings.EqualFold(at, s.Username()) {
		return
	}
	cmd = strings.ToLower(cmd)
	had := s.currentPending() != nil
	s.setPending(nil)
	switch cmd {
	case "start", "menu":
		s.sendView(ctx, s.menuView())
		return
	case "cancel":
		text := s.tl("没有在等你输入", "Nothing to cancel")
		if had {
			text = s.tl("已取消", "Cancelled")
		}
		s.sendView(ctx, &View{Text: text, Buttons: [][]plugin.Button{plugin.Row(plugin.Btn(s.tl("« 菜单", "« Menu"), "m"))}})
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
	if target == "" {
		s.sendView(ctx, s.menuView())
		return
	}
	v, notice, err := s.pageView(ctx, target, "", "", 0)
	if err != nil {
		v = s.errView(err)
	}
	if v == nil {
		if notice == "" {
			return
		}
		v = &View{Text: "ℹ️ " + plugin.Escape(strings.TrimPrefix(notice, "!"))}
	}
	id, err := s.send(ctx, s.owner(), v)
	if err != nil {
		s.log.Warn("send", "error", err)
		return
	}
	// A page that asked for input from a command updates the new message.
	s.mu.Lock()
	if s.pending != nil && s.pending.msgID == 0 {
		s.pending.msgID = id
	}
	s.mu.Unlock()
}

// showPanel replaces the panel message, or sends a new one when it is
// gone or unknown.
func (s *Service) showPanel(ctx context.Context, msgID int, v *View) {
	if msgID != 0 && s.edit(ctx, s.owner(), msgID, v) == nil {
		return
	}
	s.sendView(ctx, v)
}

// afterChange refreshes the command menu when the owner's language moved.
func (s *Service) afterChange(before string) {
	if s.lang() != before {
		s.resyncCommands()
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

func (s *Service) errView(err error) *View {
	return &View{Text: "❌ " + plugin.Escape(s.localErr(err)), Buttons: [][]plugin.Button{plugin.Row(plugin.Btn(s.tl("« 菜单", "« Menu"), "m"))}}
}

func (s *Service) sendView(ctx context.Context, v *View) {
	if _, err := s.send(ctx, s.owner(), v); err != nil {
		s.log.Warn("send", "error", err)
	}
}

func (s *Service) answer(ctx context.Context, q *tg.UpdateBotCallbackQuery, notice string) {
	req := &tg.MessagesSetBotCallbackAnswerRequest{QueryID: q.QueryID}
	if notice != "" {
		alert := strings.HasPrefix(notice, "!")
		req.SetMessage(clip(strings.TrimPrefix(notice, "!"), 190))
		if alert {
			req.SetAlert(true)
		}
	}
	_, _ = s.api.MessagesSetBotCallbackAnswer(ctx, req)
}

// answerDeadline is how long a tap waits for its view before it is
// answered empty.
const answerDeadline = 4 * time.Second

func (s *Service) onCallback(q *tg.UpdateBotCallbackQuery) {
	ctx, cancel := opCtx()
	defer cancel()
	userID, msgID, data := q.UserID, q.MsgID, string(q.Data)
	if !s.isOwner(userID) {
		s.answer(ctx, q, "!"+s.tl("这个机器人只为主人服务", "This bot only serves its owner"))
		return
	}
	type result struct {
		v      *View
		notice string
		err    error
	}
	done := make(chan result, 1)
	go func() {
		v, n, err := s.dispatch(ctx, data, target{chatID: userID, msgID: msgID})
		done <- result{v, n, err}
	}()
	// Telegram wants the tap answered within seconds. Slow taps get an
	// empty answer first and their view when ready.
	var r result
	late := false
	select {
	case r = <-done:
	case <-time.After(answerDeadline):
		s.answer(ctx, q, "")
		r = <-done
		late = true
	}
	if r.err != nil {
		if !late {
			s.answer(ctx, q, "!"+s.localErr(r.err))
		}
		r.v = s.errView(r.err)
	} else if !late {
		s.answer(ctx, q, r.notice)
	} else if strings.HasPrefix(r.notice, "!") && r.v != nil {
		// The alert can no longer ride on the tap; put it on the panel.
		r.v = &View{Text: "⚠️ " + plugin.Escape(r.notice[1:]) + "\n\n" + r.v.Text, Buttons: r.v.Buttons}
	}
	if r.v != nil {
		if err := s.edit(ctx, userID, msgID, r.v); err != nil {
			s.log.Warn("edit panel", "error", err)
			s.sendView(ctx, r.v)
		}
	}
}

// dispatch turns callback data into the next view. A nil view leaves the
// message as it is.
func (s *Service) dispatch(ctx context.Context, data string, at target) (*View, string, error) {
	kind, rest, _ := strings.Cut(data, ":")
	if kind != "n" {
		s.setPending(nil)
	}
	switch kind {
	case "n":
		return nil, "", nil
	case "m":
		return s.menuView(), "", nil
	case "l":
		k, pg, _ := strings.Cut(rest, ":")
		n, _ := strconv.Atoi(pg)
		if k != "b" && k != "x" {
			k = "b"
		}
		return s.listView(k, n), "", nil
	case "h":
		v, notice := s.infoView(rest)
		return v, notice, nil
	case "p":
		name, pd, _ := strings.Cut(rest, ":")
		return s.pageView(ctx, name, pd, "", at.msgID)
	case "g":
		pg, f, _ := strings.Cut(rest, ":")
		n, _ := strconv.Atoi(pg)
		return s.managerView(ctx, n, f == "f"), "", nil
	case "o":
		v, notice := s.manageView(ctx, rest)
		return v, notice, nil
	case "a":
		op, name, _ := strings.Cut(rest, ":")
		return s.confirmView(op, name), "", nil
	case "y":
		op, name, _ := strings.Cut(rest, ":")
		return s.runOp(ctx, at, op, name)
	case "e", "c", "r":
		return s.settingTap(ctx, kind, rest, at)
	}
	return s.menuView(), s.tl("按钮已过期", "That button is outdated"), nil
}

// settingTap handles e/c/r taps on a settings panel.
func (s *Service) settingTap(_ context.Context, kind, rest string, at target) (*View, string, error) {
	parts := strings.Split(rest, ":")
	if len(parts) < 2 {
		return s.menuView(), s.tl("按钮已过期", "That button is outdated"), nil
	}
	name, key := parts[0], parts[1]
	st, ok := s.settings.Get(name)
	if !ok {
		v, notice := s.infoView(name)
		return v, notice, nil
	}
	set, ok := st.Setting(key)
	if !ok {
		v, _ := s.infoView(name)
		return v, s.tl("这个设置项已经没有了", "That setting is gone"), nil
	}
	lang := s.lang()
	defer s.afterChange(lang)
	info := func(notice string) (*View, string, error) {
		v, _ := s.infoView(name)
		return v, notice, nil
	}
	switch kind {
	case "e":
		switch set.Kind {
		case plugin.SettingToggle:
			if err := st.Set(key, !st.Bool(key)); err != nil {
				return nil, "!" + s.localErr(err), nil
			}
			return info("")
		case plugin.SettingText, plugin.SettingNumber:
			s.setPending(&pending{plugin: name, key: key, msgID: at.msgID, at: time.Now()})
		}
		return s.editView(name, st, set, ""), "", nil
	case "c":
		if len(parts) < 3 {
			return s.editView(name, st, set, ""), "", nil
		}
		i, err := strconv.Atoi(parts[2])
		if err != nil || i < 0 || i >= len(set.Choices) {
			// Choices changed since the panel was drawn.
			return s.editView(name, st, set, ""), s.tl("选项已更新，请重选", "Choices changed, pick again"), nil
		}
		if err := st.Set(key, set.Choices[i].Value); err != nil {
			return nil, "!" + s.localErr(err), nil
		}
		return info(s.tl("已保存", "Saved"))
	case "r":
		if err := st.Reset(key); err != nil {
			return nil, "!" + s.localErr(err), nil
		}
		return info(s.tl("已恢复默认", "Reset"))
	}
	return info("")
}
