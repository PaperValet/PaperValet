// Package bot runs the companion Telegram bot. It talks only to the owner,
// renders plugin settings as button panels and hosts plugin pages.
package bot

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/internal/i18n"
	"github.com/TiaraBasori/PaperValet/internal/peer"
	"github.com/TiaraBasori/PaperValet/internal/settings"
	"github.com/TiaraBasori/PaperValet/pkg/logger"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// Options configures the bot.
type Options struct {
	APIID       int
	APIHash     string
	Token       string
	SessionFile string
	PeersFile   string
	Device      telegram.DeviceConfig
	// Version is shown in the menu header.
	Version string
}

// Service is the companion bot.
type Service struct {
	opt      Options
	client   *telegram.Client
	api      *tg.Client
	peers    *peer.AccessHashManager
	settings *settings.Registry
	i18n     *i18n.Manager
	log      plugin.Logger

	mu       sync.RWMutex
	ready    bool
	username string
	ownerID  int64
	pages    map[string]*plugin.Page
	catalog  Catalog
	pending  *pending
	readyCh  chan struct{}
	busy     atomic.Bool // a plugin install/remove/reload is running
}

// pending is a typed answer the bot is waiting for.
type pending struct {
	plugin string
	key    string // setting key, or the page's Ask key
	page   bool
	msgID  int // panel message to update afterwards
	at     time.Time
}

// backData is where the prompt's back button leads.
func (p *pending) backData() string {
	if p.page {
		return "p:" + p.plugin
	}
	return "h:" + p.plugin
}

// New builds the service. Without a token Run returns at once and the bot
// stays unavailable, but settings still work for plugins.
func New(opt Options, reg *settings.Registry, i18nMgr *i18n.Manager) *Service {
	s := &Service{
		opt:      opt,
		settings: reg,
		i18n:     i18nMgr,
		log:      logger.NamedLogger("bot"),
		pages:    map[string]*plugin.Page{},
		readyCh:  make(chan struct{}),
	}
	if opt.Token == "" {
		return s
	}
	s.client = NewClient(opt, telegram.UpdateHandlerFunc(s.handle))
	s.api = s.client.API()
	s.peers = peer.NewAccessHashManager(s.api, peer.NewStore(opt.PeersFile))
	return s
}

// NewClient builds the gotd client for the bot account.
func NewClient(opt Options, h telegram.UpdateHandler) *telegram.Client {
	return telegram.NewClient(opt.APIID, opt.APIHash, telegram.Options{
		SessionStorage: &telegram.FileSessionStorage{Path: opt.SessionFile},
		UpdateHandler:  h,
		Device:         opt.Device,
		RetryInterval:  time.Second,
		MaxRetries:     -1,
		DialTimeout:    15 * time.Second,
	})
}

// TokenBotID is the numeric bot id at the start of a token, 0 if malformed.
func TokenBotID(token string) int64 {
	id, _, ok := strings.Cut(token, ":")
	if !ok {
		return 0
	}
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

// Login authorizes client as the bot behind token. A session that belongs
// to another bot is logged out first.
func Login(ctx context.Context, client *telegram.Client, token string) (*tg.User, error) {
	status, err := client.Auth().Status(ctx)
	if err != nil {
		return nil, err
	}
	if status.Authorized && status.User != nil && status.User.ID != TokenBotID(token) {
		_, _ = client.API().AuthLogOut(ctx)
		status.Authorized = false
	}
	if !status.Authorized {
		if _, err := client.Auth().Bot(ctx, token); err != nil {
			return nil, err
		}
	}
	return client.Self(ctx)
}

// SetOwner sets the only user the bot answers.
func (s *Service) SetOwner(id int64) {
	s.mu.Lock()
	s.ownerID = id
	s.mu.Unlock()
}

func (s *Service) owner() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ownerID
}

// Enabled reports whether a token is configured.
func (s *Service) Enabled() bool { return s.client != nil }

// Ready reports whether the bot is logged in.
func (s *Service) Ready() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ready
}

// Username is the bot's username, "" before login.
func (s *Service) Username() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.username
}

// WaitReady blocks until the bot is online or ctx ends.
func (s *Service) WaitReady(ctx context.Context) bool {
	if !s.Enabled() {
		return false
	}
	select {
	case <-s.readyCh:
		return true
	case <-ctx.Done():
		return false
	}
}

// KnowsOwner reports whether the bot can already message the owner. Bots
// cannot open a chat; the owner has to message it once.
func (s *Service) KnowsOwner() bool {
	if s.peers == nil {
		return false
	}
	p, _ := s.peers.GetInputPeer(context.Background(), s.owner())
	u, ok := p.(*tg.InputPeerUser)
	return ok && u.AccessHash != 0
}

// Run connects the bot and blocks until ctx ends. Connection problems are
// logged, never fatal: the userbot keeps working without its bot.
func (s *Service) Run(ctx context.Context) error {
	if !s.Enabled() {
		return nil
	}
	err := s.client.Run(ctx, func(ctx context.Context) error {
		self, err := Login(ctx, s.client, s.opt.Token)
		if err != nil {
			return fmt.Errorf("bot login: %w", err)
		}
		// Ask for updates; a bot that never calls getState gets none.
		if _, err := s.api.UpdatesGetState(ctx); err != nil {
			s.log.Warn("get state", "error", err)
		}
		s.mu.Lock()
		s.ready = true
		s.username = self.Username
		close(s.readyCh)
		s.mu.Unlock()
		s.log.Info("bot online", "username", self.Username)
		s.syncCommands(ctx)
		<-ctx.Done()
		return ctx.Err()
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		s.log.Error("bot stopped", "error", err)
	}
	return err
}

// RemovePlugin drops a plugin's page and settings (on unload).
func (s *Service) RemovePlugin(name string) {
	s.settings.Unregister(name)
	s.mu.Lock()
	_, had := s.pages[name]
	delete(s.pages, name)
	if s.pending != nil && s.pending.plugin == name {
		s.pending = nil
	}
	s.mu.Unlock()
	if had {
		s.resyncCommands()
	}
}

// Settings registers a plugin's settings panel.
func (s *Service) Settings(spec *plugin.SettingsSpec) (plugin.Settings, error) {
	st, err := s.settings.Register(spec)
	if err != nil {
		return nil, err
	}
	return st, nil
}

// For returns the bot as seen by one plugin.
func (s *Service) For(name string) plugin.Bot { return scoped{s: s, plugin: name} }

func (s *Service) setPage(name string, p *plugin.Page) error {
	if p != nil {
		if p.Handle == nil {
			return errors.New("page without Handle")
		}
		if p.Command != "" && !cmdRe(p.Command) {
			return fmt.Errorf("invalid page command %q", p.Command)
		}
	}
	s.mu.Lock()
	if p == nil {
		delete(s.pages, name)
	} else {
		s.pages[name] = p
	}
	s.mu.Unlock()
	s.resyncCommands()
	return nil
}

func cmdRe(c string) bool {
	if len(c) == 0 || len(c) > 32 {
		return false
	}
	for _, r := range c {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_') {
			return false
		}
	}
	return c != "start" && c != "menu" && c != "cancel"
}

func (s *Service) page(name string) *plugin.Page {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pages[name]
}

func (s *Service) lang() string {
	if s.i18n == nil {
		return string(i18n.ZhCN)
	}
	return string(s.i18n.UserLang(s.owner()))
}

func (s *Service) tl(zh, en string) string {
	if s.lang() == string(i18n.EnUS) {
		return en
	}
	return zh
}

// resyncCommands refreshes the bot command list when online.
func (s *Service) resyncCommands() {
	if !s.Ready() {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		s.syncCommands(ctx)
	}()
}

func (s *Service) syncCommands(ctx context.Context) {
	cmds := []tg.BotCommand{
		{Command: "menu", Description: s.tl("插件面板", "Plugin panels")},
		{Command: "cancel", Description: s.tl("取消输入", "Cancel typing")},
	}
	s.mu.RLock()
	names := make([]string, 0, len(s.pages))
	for n, p := range s.pages {
		if p.Command != "" {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for _, n := range names {
		p := s.pages[n]
		cmds = append(cmds, tg.BotCommand{Command: p.Command, Description: s.pick(p.Title, p.TitleEN, n)})
	}
	s.mu.RUnlock()
	if _, err := s.api.BotsSetBotCommands(ctx, &tg.BotsSetBotCommandsRequest{
		Scope:    &tg.BotCommandScopeDefault{},
		Commands: cmds,
	}); err != nil {
		s.log.Warn("set commands", "error", err)
	}
}

// pick returns the localized text, falling back to def.
func (s *Service) pick(zh, en, def string) string {
	if s.lang() == string(i18n.EnUS) && en != "" {
		return en
	}
	if zh != "" {
		return zh
	}
	if en != "" {
		return en
	}
	return def
}

// ---------------------------------------------------------------- sending

func (s *Service) inputPeer(ctx context.Context, chatID int64) (tg.InputPeerClass, error) {
	if !s.Ready() {
		return nil, plugin.ErrBotNotReady
	}
	return s.peers.GetInputPeer(ctx, chatID)
}

func noUsers(int64) (tg.InputUserClass, error) { return nil, errors.New("no user resolution") }

func markup(rows [][]plugin.Button) tg.ReplyMarkupClass {
	if len(rows) == 0 {
		return nil
	}
	m := &tg.ReplyInlineMarkup{}
	for _, row := range rows {
		var kr tg.KeyboardButtonRow
		for _, b := range row {
			st, styled := style(b.Style)
			if b.URL != "" {
				k := &tg.KeyboardButtonURL{Text: b.Text, URL: b.URL}
				if styled {
					k.SetStyle(st)
				}
				kr.Buttons = append(kr.Buttons, k)
			} else {
				k := &tg.KeyboardButtonCallback{Text: b.Text, Data: []byte(b.Data)}
				if styled {
					k.SetStyle(st)
				}
				kr.Buttons = append(kr.Buttons, k)
			}
		}
		if len(kr.Buttons) > 0 {
			m.Rows = append(m.Rows, kr)
		}
	}
	return m
}

// style maps a button color to Telegram's keyboard button style.
func style(s plugin.ButtonStyle) (tg.KeyboardButtonStyle, bool) {
	switch s {
	case plugin.StylePrimary:
		return tg.KeyboardButtonStyle{BgPrimary: true}, true
	case plugin.StyleSuccess:
		return tg.KeyboardButtonStyle{BgSuccess: true}, true
	case plugin.StyleDanger:
		return tg.KeyboardButtonStyle{BgDanger: true}, true
	}
	return tg.KeyboardButtonStyle{}, false
}

func (s *Service) send(ctx context.Context, chatID int64, v *View) (int, error) {
	p, err := s.inputPeer(ctx, chatID)
	if err != nil {
		return 0, err
	}
	plain, ents := plugin.ParseMarkdown(v.Text, noUsers)
	req := &tg.MessagesSendMessageRequest{Peer: p, Message: plain, RandomID: randomID(), NoWebpage: true}
	if len(ents) > 0 {
		req.SetEntities(ents)
	}
	if m := markup(v.Buttons); m != nil {
		req.SetReplyMarkup(m)
	}
	upd, err := s.api.MessagesSendMessage(ctx, req)
	if err != nil {
		return 0, err
	}
	return sentID(upd), nil
}

func (s *Service) edit(ctx context.Context, chatID int64, msgID int, v *View) error {
	p, err := s.inputPeer(ctx, chatID)
	if err != nil {
		return err
	}
	plain, ents := plugin.ParseMarkdown(v.Text, noUsers)
	req := &tg.MessagesEditMessageRequest{Peer: p, ID: msgID, Message: plain, NoWebpage: true}
	req.SetEntities(ents)
	// Without reply_markup the edit drops the keyboard; an empty inline
	// markup is rejected with REPLY_MARKUP_INVALID.
	if m := markup(v.Buttons); m != nil {
		req.SetReplyMarkup(m)
	}
	_, err = s.api.MessagesEditMessage(ctx, req)
	if err != nil && strings.Contains(err.Error(), "MESSAGE_NOT_MODIFIED") {
		return nil
	}
	return err
}

func (s *Service) deleteMsg(ctx context.Context, ids ...int) {
	if !s.Ready() {
		return
	}
	_, _ = s.api.MessagesDeleteMessages(ctx, &tg.MessagesDeleteMessagesRequest{ID: ids, Revoke: true})
}

func sentID(u tg.UpdatesClass) int {
	switch v := u.(type) {
	case *tg.UpdateShortSentMessage:
		return v.ID
	case *tg.Updates:
		for _, up := range v.Updates {
			if x, ok := up.(*tg.UpdateMessageID); ok {
				return x.ID
			}
			if x, ok := up.(*tg.UpdateNewMessage); ok {
				if m, ok := x.Message.(*tg.Message); ok {
					return m.ID
				}
			}
		}
	}
	return 0
}

func randomID() int64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UnixNano()
	}
	return int64(binary.LittleEndian.Uint64(b[:]))
}

// ---------------------------------------------------------------- scoped

// View aliases plugin.View inside this package.
type View = plugin.View

type scoped struct {
	s      *Service
	plugin string
}

func (b scoped) Ready() bool                  { return b.s.Ready() }
func (b scoped) Username() string             { return b.s.Username() }
func (b scoped) SetPage(p *plugin.Page) error { return b.s.setPage(b.plugin, p) }

func (b scoped) Notify(ctx context.Context, v *plugin.View) (int, error) {
	return b.Send(ctx, b.s.owner(), v)
}

func (b scoped) Send(ctx context.Context, chatID int64, v *plugin.View) (int, error) {
	if v == nil {
		return 0, errors.New("nil view")
	}
	return b.s.send(ctx, chatID, b.s.route(b.plugin, v, false))
}

func (b scoped) Edit(ctx context.Context, chatID int64, msgID int, v *plugin.View) error {
	if v == nil {
		return errors.New("nil view")
	}
	return b.s.edit(ctx, chatID, msgID, b.s.route(b.plugin, v, false))
}

// Disabled reports whether the env asks to skip the bot (tests, CI).
func Disabled() bool { return os.Getenv("PAPERVALET_NO_BOT") == "1" }
