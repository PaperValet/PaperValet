package plugin

import (
	"context"
	"errors"
)

// ============================================================
// Settings
// ============================================================
//
// Plugin options are declared, not commanded. A plugin describes its
// settings once; the companion bot renders them as a button panel and the
// owner edits them there. Commands stay for actions only.

// SettingKind selects how the bot panel edits a setting.
type SettingKind int

const (
	// SettingToggle is a bool flipped with one tap.
	SettingToggle SettingKind = iota
	// SettingChoice is a string picked from Choices.
	SettingChoice
	// SettingText is a string the owner types.
	SettingText
	// SettingNumber is an int the owner types, kept within Min..Max.
	SettingNumber
)

// Choice is one option of a SettingChoice.
type Choice struct {
	Value   string
	Label   string
	LabelEN string
}

// Setting declares one option.
type Setting struct {
	// Key is the stable id, lowercase letters, digits and _ only.
	Key     string
	Label   string
	LabelEN string
	// Hint is one line shown on the edit screen.
	Hint   string
	HintEN string
	Kind   SettingKind
	// Default must match Kind: bool, string or int.
	Default any
	Choices []Choice
	// Min and Max bound a SettingNumber; both 0 means unbounded.
	Min, Max int
	// Validate checks typed text (SettingText) and may normalize it.
	// Return Invalid(zh, en) so the owner reads the reason in their language.
	Validate func(string) (string, error)
	// Secret masks the value in the panel.
	Secret bool
}

// InvalidError is a validation error with text in both languages.
type InvalidError struct{ Zh, En string }

func (e *InvalidError) Error() string { return e.En }

// Text returns the message in lang ("zh-CN" or "en-US").
func (e *InvalidError) Text(lang string) string {
	if lang == "en-US" || e.Zh == "" {
		return e.En
	}
	return e.Zh
}

// Invalid builds a bilingual validation error for Setting.Validate.
func Invalid(zh, en string) error { return &InvalidError{Zh: zh, En: en} }

// SettingsSpec is a plugin's settings panel.
type SettingsSpec struct {
	Plugin   string
	Title    string // defaults to the plugin name
	TitleEN  string
	Settings []Setting
	// OnChange runs after a value changes, from the panel or from Set.
	OnChange func(key string)
}

// Settings reads and writes a plugin's options, persisted in
// data/<plugin>/settings.json. Reads of unknown keys return zero values.
type Settings interface {
	Bool(key string) bool
	String(key string) string
	Int(key string) int
	// Set stores a value; its type must match the setting's Kind.
	Set(key string, value any) error
}

// ============================================================
// Bot
// ============================================================

// Button is one inline button. Set Data for a callback or URL for a link.
type Button struct {
	Text string
	// Data comes back to Page.Handle as BotContext.Data, at most 32 bytes.
	Data string
	URL  string
}

// Btn builds a callback button.
func Btn(text, data string) Button { return Button{Text: text, Data: data} }

// LinkBtn builds a URL button.
func LinkBtn(text, url string) Button { return Button{Text: text, URL: url} }

// Row groups buttons into one keyboard row.
func Row(b ...Button) []Button { return b }

// View is one bot screen: Markdown text plus button rows.
type View struct {
	Text    string
	Buttons [][]Button
}

// Page is a custom screen a plugin adds to the bot menu, for things that
// are more than a setting: lists with per-item buttons, dashboards, wizards.
// Buttons on messages the plugin posts with Notify, Send or Edit also come
// back to Handle.
type Page struct {
	Title   string
	TitleEN string
	// Command optionally opens the page with /<Command> in the bot chat.
	Command string
	// Handle renders the page. Data is "" when the page opens, otherwise
	// the Data of the pressed button (or the Ask key with Input set).
	// The bot appends a back button; do not add one.
	Handle func(ctx *BotContext) (*View, error)
}

// BotContext is passed to Page.Handle.
type BotContext struct {
	Ctx  context.Context
	Lang string
	Data string
	// Input is the text the owner typed after Ask.
	Input string

	asked string
	toast string
	alert bool
}

// Context returns the request context.
func (c *BotContext) Context() context.Context {
	if c.Ctx != nil {
		return c.Ctx
	}
	return context.Background()
}

// Tlocal picks between a Chinese and an English string.
func (c *BotContext) Tlocal(zh, en string) string {
	if c.Lang == "en-US" {
		return en
	}
	return zh
}

// Ask waits for the owner to type text. The next message comes back to
// Handle with Data set to key and Input set to the text. The returned
// View should say what to type.
func (c *BotContext) Ask(key string) { c.asked = key }

// Toast shows a short notice on the pressed button.
func (c *BotContext) Toast(text string) { c.toast, c.alert = text, false }

// Alert shows a notice the owner must dismiss.
func (c *BotContext) Alert(text string) { c.toast, c.alert = text, true }

// Asked returns the key passed to Ask; read by the host after Handle.
func (c *BotContext) Asked() string { return c.asked }

// Notice returns the toast text and whether it is an alert; read by the
// host after Handle.
func (c *BotContext) Notice() (string, bool) { return c.toast, c.alert }

// Bot is the companion bot as seen by one plugin (Host().Bot(name)). It
// only talks to the owner, renders settings panels and plugin pages, and
// posts messages on the plugin's behalf. Buttons with Data on anything the
// plugin sends are routed back to its Page.
type Bot interface {
	// Ready reports whether the bot is logged in.
	Ready() bool
	// Username is the bot's @username without @, "" before login.
	Username() string
	// SetPage adds or replaces the plugin's page; nil removes it. Pages
	// and settings are removed automatically when the plugin unloads.
	SetPage(p *Page) error
	// Notify sends a view to the owner's bot chat.
	Notify(ctx context.Context, v *View) (int, error)
	// Send posts a view to a chat the bot can write to.
	Send(ctx context.Context, chatID int64, v *View) (int, error)
	// Edit replaces a message the bot sent.
	Edit(ctx context.Context, chatID int64, msgID int, v *View) error
}

// ErrBotNotReady is returned by Bot calls before the bot has logged in.
var ErrBotNotReady = errors.New("bot not ready")
