# PaperValet Architecture Specification

## Overview
Production-grade Telegram Userbot built on **gotd/td** (Telegram MTProto client), learning from:
- **NexusValet** (Go, gotd): Hook system, plugin interfaces, peer resolution
- **PagerMaid-Modify** (Python, Pyrogram): Module system, alias manager, permissions
- **TeleBox** (TypeScript, gramJS): Plugin SDK, external plugins, command context helpers

---

## Architecture Principles

1. **Interface-based Dependency Injection** — No global state, all dependencies injected
2. **Public Plugin SDK** (`pkg/plugin`) — External plugins depend ONLY on this
3. **Built-in vs External Plugins** — Built-ins compile in; externals load as `.so` via Go plugins
4. **Type-safe Command Context** — Rich helpers: `Reply()`, `Edit()`, `Delete()`, `Typing()`, `GetArg()`
5. **Event Bus with Hooks** — `BeforeStart`, `AfterStart`, `BeforeStop`, `AfterStop` + custom events
6. **Permission System** — Owner-only, sudo, admin, rate limiting built-in
7. **Session Management** — Per-user-per-chat state persistence (SQLite)
8. **Peer Resolution** — AccessHashManager for reliable peer resolution

---

## Package Structure

```
github.com/TiaraBasori/PaperValet/
├── cmd/papervalet/           # Entry point
├── internal/
│   ├── app/                  # Application orchestrator (App, Bot)
│   ├── command/              # Command registry, parser, context
│   ├── config/               # Configuration (JSON, env)
│   ├── cron/                 # Cron scheduler
│   ├── eventbus/             # Event system + hooks
│   ├── peer/                 # Peer resolution + AccessHashManager
│   ├── plugin/
│   │   ├── loader/           # Go plugin (.so) loader
│   │   └── manager/          # Plugin lifecycle management
│   └── session/              # SQLite session storage
├── pkg/
│   ├── logger/               # Zap logger wrapper
│   └── plugin/               # PUBLIC SDK (external plugins import THIS)
│       ├── sdk.go            # Interfaces, types
│       ├── metadata.go       # PluginMetadata for .so plugins
│       └── helpers.go        # CommandContext helpers (Reply, Edit, etc.)
└── plugins/
    ├── builtin/              # Built-in plugins (compiled in)
    │   ├── core.go           # help, status, restart, shutdown, gc, version
    │   ├── tools.go          # ping, uptime, info, fwd, remind, note, calc, base64, hash, uuid, time
    │   ├── fun.go            # roll, coin, choose, 8ball, fact
    │   ├── cron.go           # cron add/list/remove/run/enable/disable
    │   ├── alias.go          # alias set/del/list
    │   ├── debug.go          # goroutines, heap, stack, profile
    │   ├── exec.go           # exec/shell (owner)
    │   ├── sudo.go           # sudo (owner)
    │   ├── log.go            # log show/clear/level/target (owner)
    │   ├── re.go             # repeat messages
    │   ├── bf.go             # brainfuck interpreter
    │   ├── prefix.go         # prefix get/set/list
    │   ├── help.go           # help, plugins (core UX)
    │   ├── status.go         # detailed system status
    │   └── ppm.go            # Plugin Package Manager (install/update/remove/list/enable/disable/reload/search/info)
    └── external/             # External plugins (built separately as .so)
        ├── qrcode/
        ├── leech/
        ├── ping/             # Advanced ping (DC, ICMP, HTTP)
        ├── alias/
        ├── re/
        ├── bf/
        ├── sendlog/
        └── ...
```

---

## Plugin SDK (`pkg/plugin/sdk.go`)

```go
// Package plugin provides the PUBLIC SDK for PaperValet plugins.
// External plugins build against this package ONLY.
package plugin

import (
    "context"
    "time"
    "github.com/gotd/td/tg"
)

// ============================================================
// Plugin Lifecycle
// ============================================================

// Plugin is the interface all plugins must implement.
type Plugin interface {
    Name() string
    Description() string
    Init(ctx context.Context, mgr Manager) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}

// PluginMetadata is exported by external .so plugins as `var Metadata *PluginMetadata`.
type PluginMetadata struct {
    Name        string
    Description string
    Version     string
    Author      string
    MinVersion  string
}

// ============================================================
// Manager Interface (exposed to plugins)
// ============================================================

// Manager is the plugin manager interface exposed to plugins.
type Manager interface {
    RegisterPlugin(p Plugin) error
    RegisterCommand(cmd *Command) error
    UnregisterCommand(name string)
    UnregisterPlugin(name string)
    Commands() RegistryProvider
    GetInfo(name string) (PluginInfo, bool)
    GetAllInfo() []PluginInfo
    Emit(ctx context.Context, eventType string, data any) error
    InitAll(ctx context.Context) error
    StartAll(ctx context.Context) error
    StopAll(ctx context.Context) error
}

// PluginInfo holds plugin metadata.
type PluginInfo struct {
    Name        string
    Description string
    Status      PluginStatus
}

type PluginStatus int
const (
    StatusInactive PluginStatus = iota
    StatusActive
    StatusError
)

// RegistryProvider provides command registry access.
type RegistryProvider interface {
    Get(name string) (*Command, bool)
    GetAll() map[string]*Command
    GetByPlugin(plugin string) map[string]*Command
    GetPrefix() string
}

// ============================================================
// Logging
// ============================================================

type Logger interface {
    Debug(msg string, keysAndValues ...any)
    Info(msg string, keysAndValues ...any)
    Warn(msg string, keysAndValues ...any)
    Error(msg string, keysAndValues ...any)
    Named(name string) Logger
    With(keysAndValues ...any) Logger
}

// ============================================================
// Event System
// ============================================================

type Emitter interface {
    Emit(ctx context.Context, eventType string, data any) error
}

// ============================================================
// Peer Resolution
// ============================================================

type PeerResolver interface {
    ResolveFromChatID(ctx context.Context, chatID int64) (tg.InputPeerClass, error)
    ResolveUserInChannel(ctx context.Context, channelPeer tg.InputChannelClass, userID int64) (tg.InputPeerClass, error)
    ResolveUserFromMessage(ctx context.Context, peer tg.InputPeerClass, msgID int, userID int64) (tg.InputPeerClass, error)
}

// ============================================================
// Message & Session
// ============================================================

type MessageEvent struct {
    Update    tg.UpdatesClass
    Message   *tg.Message
    Text      string
    UserID    int64
    ChatID    int64
    IsOut     bool
    IsReply   bool
    ReplyToID int
    Entities  []tg.MessageEntityClass
    Media     tg.MessageMediaClass
    Date      int
    PeerID    tg.PeerClass
    Raw       any
}

type Session struct {
    UserID    int64
    ChatID    int64
    State     string
    Data      map[string]any
    Timestamp int64
}

type SessionContext struct {
    Session *Session
    Context context.Context
    Data    map[string]any
}

func NewSessionContext(s *Session, ctx context.Context) *SessionContext {
    return &SessionContext{Session: s, Context: ctx, Data: make(map[string]any)}
}

func (s *SessionContext) Ctx() context.Context {
    if s != nil && s.Context != nil { return s.Context }
    return context.Background()
}
func (s *SessionContext) Get(key string) (any, bool) {
    if s == nil || s.Data == nil { return nil, false }
    v, ok := s.Data[key]; return v, ok
}
func (s *SessionContext) Set(key string, value any) {
    if s.Data == nil { s.Data = make(map[string]any) }
    s.Data[key] = value
}
func (s *SessionContext) Delete(key string) { delete(s.Data, key) }

// ============================================================
// Command System
// ============================================================

type Handler func(ctx *CommandContext) error
type Middleware func(next Handler) Handler

type Command struct {
    Name        string
    Aliases     []string
    Description string
    Usage       string
    Plugin      string
    Category    string
    OwnerOnly   bool
    SudoOnly    bool
    Hidden      bool
    RateLimit   int
    RateWindow  int
    Handler     Handler
}

type CommandContext struct {
    Command      string
    Args         []string
    RawArgs      string
    Message      *MessageEvent
    Session      *SessionContext
    API          *tg.Client
    PeerResolver PeerResolver
    Emitter      Emitter
    PluginName   string
    StartTime    time.Time
    Metadata     map[string]any
    Ctx          context.Context
    Logger       Logger
}

// Context() returns the request context.
func (c *CommandContext) Context() context.Context {
    if c.Ctx != nil { return c.Ctx }
    if c.Session != nil { return s.Ctx() }
    return context.Background()
}

// resolvePeer resolves the current chat peer.
func (c *CommandContext) resolvePeer() (tg.InputPeerClass, error) {
    if c.Message == nil || c.PeerResolver == nil { return nil, ErrNoMessage }
    return c.PeerResolver.ResolveFromChatID(c.Context(), c.Message.ChatID)
}

// Reply sends a reply to the triggering message.
func (c *CommandContext) Reply(text string) error {
    if c.Message == nil || c.API == nil || c.Message.Message == nil { return ErrNoMessage }
    peer, err := c.resolvePeer()
    if err != nil { return err }
    _, err = c.API.MessagesSendMessage(c.Context(), &tg.MessagesSendMessageRequest{
        Peer: peer, Message: text, RandomID: time.Now().UnixNano(),
        ReplyTo: &tg.InputReplyToMessage{ReplyToMsgID: c.Message.Message.ID},
    })
    return err
}

// Edit edits the triggering message.
func (c *CommandContext) Edit(text string) error {
    if c.Message == nil || c.API == nil || c.Message.Message == nil { return ErrNoMessage }
    peer, err := c.resolvePeer()
    if err != nil { return err }
    _, err = c.API.MessagesEditMessage(c.Context(), &tg.MessagesEditMessageRequest{
        Peer: peer, ID: c.Message.Message.ID, Message: text,
    })
    return err
}

// Delete deletes the triggering message.
func (c *CommandContext) Delete() error {
    if c.Message == nil || c.API == nil || c.Message.Message == nil { return ErrNoMessage }
    _, err := c.API.MessagesDeleteMessages(c.Context(), &tg.MessagesDeleteMessagesRequest{
        ID: []int{c.Message.Message.ID}, Revoke: true,
    })
    return err
}

// Typing sends a typing action.
func (c *CommandContext) Typing() error {
    if c.Message == nil || c.API == nil { return ErrNoMessage }
    peer, err := c.resolvePeer()
    if err != nil { return err }
    _, err = c.API.MessagesSetTyping(c.Context(), &tg.MessagesSetTypingRequest{
        Peer: peer, Action: &tg.SendMessageTypingAction{},
    })
    return err
}

// GetArg returns the argument at index, or empty string.
func (c *CommandContext) GetArg(index int) string {
    if index < 0 || index >= len(c.Args) { return "" }
    return c.Args[index]
}

// GetArgs returns the raw arguments string.
func (c *CommandContext) GetArgs() string { return c.RawArgs }

// ArgCount returns the number of arguments.
func (c *CommandContext) ArgCount() int { return len(c.Args) }

// HasArg checks if an argument exists.
func (c *CommandContext) HasArg(arg string) bool {
    for _, a := range c.Args { if a == arg { return true } }
    return false
}

var ErrNoMessage = &CommandError{Code: "NO_MESSAGE", Message: "no message in context"}

type CommandError struct {
    Code    string
    Message string
    Err     error
}
func (e *CommandError) Error() string {
    if e.Err != nil { return e.Message + ": " + e.Err.Error() }
    return e.Message
}
func (e *CommandError) Unwrap() error { return e.Err }
```

---

## Built-in Plugins (15 total)

| Plugin | Commands | Category | Notes |
|--------|----------|----------|-------|
| **core** | help, status, restart, shutdown, gc, version | core | Essential system commands |
| **ppm** | ppm install/remove/update/list/enable/disable/reload/search/info/repo | admin | Plugin Package Manager |
| **tools** | ping, uptime, info, fwd, remind, note, calc, base64, hash, uuid, time, rand, qr | tools | Daily utilities |
| **fun** | roll, coin, choose, 8ball, fact, ascii, emoji | fun | Entertainment |
| **cron** | cron add/list/remove/run/enable/disable | tools | Scheduled tasks |
| **alias** | alias set/del/list | tools | Command aliases (persisted) |
| **debug** | goroutines, heap, stack, profile | debug | Profiling (owner) |
| **exec** | exec/shell | admin | Shell commands (owner) |
| **sudo** | sudo | admin | Root commands (owner) |
| **log** | log show/clear/level/target/on/off/max | admin | Log capture (owner) |
| **re** | re [count] [repeat] | tools | Repeat messages |
| **bf** | bf <code> [input] | fun | Brainfuck interpreter |
| **prefix** | prefix get/set/list | core | Command prefix |
| **help** | help [cmd\|plugin], plugins | core | Help system |
| **status** | status, sysinfo, memory | core | Detailed status |

---

## External Plugin System

- **Build**: `go build -buildmode=plugin -o plugins/<name>.so ./plugins-external/<name>`
- **Load**: `PluginLoader.LoadAll(ctx)` scans `plugins/` directory
- **Metadata**: External plugin exports `var Metadata *plugin.PluginMetadata`
- **Dependencies**: External plugins import ONLY `github.com/TiaraBasori/PaperValet/pkg/plugin`

---

## Event Bus & Hooks

```go
// Event types
const (
    EventStart       = "start"
    EventStop        = "stop"
    EventMessage     = "message"
    EventCommand     = "command"
    EventCommandError = "command_error"
    EventPluginLoad  = "plugin_load"
    EventPluginUnload = "plugin_unload"
)

// Hook points (like NexusValet)
const (
    HookBeforeStart = "before_start"
    HookAfterStart  = "after_start"
    HookBeforeStop  = "before_stop"
    HookAfterStop   = "after_stop"
)
```

---

## Configuration (`config.json`)

```json
{
  "telegram": {
    "api_id": 123456,
    "api_hash": "abcdef...",
    "database": "data/telegram.db",
    "session_file": "data/session.json"
  },
  "bot": {
    "command_prefix": ".",
    "owner_id": 123456789,
    "sudoers": [987654321],
    "plugins_dir": "plugins"
  },
  "logger": {
    "level": "info",
    "format": "console"
  },
  "ppm": {
    "repo_url": "https://github.com/TiaraBasori/PaperValet-Plugins",
    "auto_update": false
  }
}
```

---

## Key Design Decisions

### 1. No Global State
All dependencies injected via `App` constructor. Testable, parallelizable.

### 2. Public SDK Separation
`pkg/plugin` is the ONLY import for external plugins. Internal packages use type aliases in `internal/interfaces`.

### 3. Go Plugins for External Isolation
True dynamic loading via `.so` files. Each plugin runs in same process but isolated namespace.

### 4. Command Context Helpers
`Reply()`, `Edit()`, `Delete()`, `Typing()`, `GetArg()` reduce boilerplate (inspired by TeleBox).

### 5. Hook System (NexusValet)
`BeforeStart`, `AfterStart`, `BeforeStop`, `AfterStop` for cross-cutting concerns.

### 6. AccessHashManager (NexusValet)
Reliable peer resolution with persistent caching to SQLite.

### 7. Session/State (PagerMaid + NexusValet)
Per-user-per-chat SQLite-backed sessions for multi-step flows.

### 8. Alias Manager (PagerMaid)
Persistent command aliases stored in JSON, hot-reloadable.

### 9. Plugin Package Manager (PPM)
Built-in plugin to manage external plugins from GitHub releases.

### 10. Rate Limiting & Permissions
Built into command registry: `OwnerOnly`, `SudoOnly`, `RateLimit`, `RateWindow`.

---

## Build & Deploy

```bash
# Build main binary
go build -o papervalet ./cmd/papervalet

# Build external plugins
go build -buildmode=plugin -o plugins/qrcode.so ./plugins-external/qrcode
go build -buildmode=plugin -o plugins/leech.so ./plugins-external/leech
# ...

# Run
./papervalet -config config.json
```

---

## CI/CD

- **Main CI**: `go vet`, `go test`, `go build`, `go build -race`
- **Plugin Release**: Trigger on `plugins/**` tags or `workflow_dispatch`
- **Docker**: Multi-stage build with `golang:1.25-alpine`