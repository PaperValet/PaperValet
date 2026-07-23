package interfaces

import (
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// Type aliases so internal code uses pkg/plugin types uniformly.
type (
	Plugin           = plugin.Plugin
	PluginInfo       = plugin.PluginInfo
	PluginStatus     = plugin.PluginStatus
	Manager          = plugin.Manager
	RegistryProvider = plugin.RegistryProvider
	Logger           = plugin.Logger
	Emitter          = plugin.Emitter
	PeerResolver     = plugin.PeerResolver
	MessageEvent     = plugin.MessageEvent
	Session          = plugin.Session
	SessionContext   = plugin.SessionContext
	Handler          = plugin.Handler
	Middleware       = plugin.Middleware
	Command          = plugin.Command
	CommandContext   = plugin.CommandContext
	CommandError     = plugin.CommandError
	Error            = plugin.CommandError
)

var NewSessionContext = plugin.NewSessionContext
var ErrNoMessage = plugin.ErrNoMessage