# PaperValet External Plugin SDK

This document describes how to create external plugins for PaperValet as shared libraries (.so files).

## Plugin Structure

Each plugin is a Go package with a `New()` function that returns a `plugin.Plugin` interface.

### Required Interface

```go
package main

import (
    "context"
    "github.com/PaperValet/PaperValet/internal/plugin"
)

// Plugin is the interface all plugins must implement
type Plugin interface {
    Name() string
    Description() string
    Init(ctx context.Context, mgr *plugin.Manager) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}
```

### Plugin Metadata (Optional)

```go
// Metadata variable (optional, for plugin loader)
var Metadata = &loader.PluginMetadata{
    Name:        "my-plugin",
    Description: "My custom plugin",
    Version:     "1.0.0",
    Author:      "Your Name",
    MinVersion:  "0.1.0",  // Minimum PaperValet version
}
```

## Complete Example

### myplugin/main.go

```go
package main

import (
    "context"
    "fmt"
    "strings"

    "github.com/PaperValet/PaperValet/internal/interfaces"
    "github.com/PaperValet/PaperValet/internal/plugin"
)

// MyPlugin implements plugin.Plugin
type MyPlugin struct {
    mgr *plugin.Manager
}

func (p *MyPlugin) Name() string        { return "myplugin" }
func (p *MyPlugin) Description() string { return "My custom plugin example" }

// Init registers commands
func (p *MyPlugin) Init(ctx context.Context, mgr *plugin.Manager) error {
    p.mgr = mgr
    
    return mgr.RegisterCommand(&interfaces.Command{
        Name:        "hello",
        Aliases:     []string{"hi"},
        Description: "Say hello",
        Usage:       "hello [name]",
        Plugin:      p.Name(),
        Category:    "tools",
        Handler:     p.handleHello,
    })
}

func (p *MyPlugin) Start(ctx context.Context) error { return nil }
func (p *MyPlugin) Stop(ctx context.Context) error  { return nil }

func (p *MyPlugin) handleHello(ctx *interfaces.CommandContext) error {
    name := "World"
    if ctx.ArgCount() > 0 {
        name = ctx.GetArg(0)
    }
    return ctx.Edit(fmt.Sprintf("Hello, %s! 👋", name))
}

// New is the entry point for the plugin loader
func New() interface{} {
    return &MyPlugin{}
}

// Metadata for the plugin loader
var Metadata = &loader.PluginMetadata{
    Name:        "myplugin",
    Description: "My custom plugin example",
    Version:     "1.0.0",
    Author:      "Your Name",
    MinVersion:  "0.1.0",
}
```

### Building the Plugin

```bash
# Build as a plugin (not executable)
go build -buildmode=plugin -o myplugin.so ./myplugin

# Copy to plugins directory
cp myplugin.so /path/to/papervalet/plugins/
```

## Building with PaperValet

### Method 1: Using the example template

```bash
# Clone the example
git clone https://github.com/PaperValet/plugin-template myplugin
cd myplugin

# Edit main.go with your plugin logic

# Build
go build -buildmode=plugin -o myplugin.so .

# Install
mkdir -p ~/.config/papervalet/plugins
cp myplugin.so ~/.config/papervalet/plugins/
```

### Method 2: Go module

```go
// go.mod
module github.com/yourname/myplugin

go 1.25

require github.com/PaperValet/PaperValet v0.1.0
```

```bash
go mod tidy
go build -buildmode=plugin -o myplugin.so .
```

## Plugin Lifecycle

1. **Load** - PaperValet loads `.so` file via `plugin.Open()`
2. **New()** - Calls `New()` function, expects `plugin.Plugin` return
3. **Init()** - Called with plugin manager for command registration
4. **Start()** - Called after all plugins initialized
5. **Stop()** - Called on shutdown

## Command Context Methods

```go
ctx.Message    // *interfaces.MessageEvent - the triggering message
ctx.Args       // []string - parsed arguments
ctx.GetArg(i)  // string - get argument by index
ctx.ArgCount() // int - number of arguments
ctx.Edit(text) // error - edit the command message (userbot UX)
ctx.Reply(text) // error - reply to message
ctx.Delete()   // error - delete command message
ctx.API        // *tg.Client - Telegram API client
ctx.PeerResolver // interfaces.PeerResolver - resolve peers
ctx.Emitter    // interfaces.Emitter - emit events
ctx.Session    // *interfaces.SessionContext - per-chat session
ctx.Logger     // interfaces.Logger - logging
```

## Session Usage

```go
func (p *MyPlugin) handleCmd(ctx *interfaces.CommandContext) error {
    session := ctx.Session
    if session == nil {
        return ctx.Edit("No session available")
    }
    
    // Get/set session data
    count, _ := session.Get("counter")
    if count == nil {
        count = 0
    }
    count = count.(int) + 1
    session.Set("counter", count)
    
    return ctx.Edit(fmt.Sprintf("Counter: %d", count))
}
```

## Event Emission

```go
func (p *MyPlugin) handleCmd(ctx *interfaces.CommandContext) error {
    ctx.Emitter.Emit(ctx.Context(), "myplugin.custom_event", map[string]any{
        "user_id": ctx.Message.UserID,
        "data":    "custom data",
    })
    return nil
}
```

## Version Compatibility

- Set `MinVersion` in Metadata to require minimum PaperValet version
- Plugin loader validates version before loading
- Use semantic versioning (e.g., "0.1.0", "1.0.0")

## Best Practices

1. **Handle errors gracefully** - Return errors from handlers, don't panic
2. **Use context** - Respect `ctx.Context()` for cancellation
3. **Don't block** - Use goroutines for long operations
4. **Clean up** - Implement proper `Stop()` for resources
5. **Logging** - Use `ctx.Logger` for structured logging
6. **Dependencies** - Only depend on PaperValet interfaces, not internal packages

## Publishing Plugins

1. Create a GitHub repo for your plugin
2. Add GitHub Actions for building `.so` on tag push
3. Publish releases with `.so` artifacts
4. Users download and place in `plugins/` directory

## Example Release Workflow

```yaml
# .github/workflows/release.yml
name: Release
on:
  push:
    tags: ['v*']
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - run: go build -buildmode=plugin -o myplugin.so .
      - uses: softprops/action-gh-release@v2
        with:
          files: myplugin.so
```