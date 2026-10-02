package app

import (
	"context"
	"sort"

	"github.com/TiaraBasori/PaperValet/internal/bot"
	"github.com/TiaraBasori/PaperValet/internal/command"
	"github.com/TiaraBasori/PaperValet/internal/plugin/loader"
	pkgplugin "github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// catalog feeds the bot's plugin panels from the plugin manager, the
// command registry and the external loader.
type catalog struct {
	mgr      pkgplugin.Manager
	commands *command.Registry
	loader   *loader.Loader
}

var _ bot.Catalog = (*catalog)(nil)

// commandsOf lists a plugin's commands with prefix, aliases after each.
func (c *catalog) commandsOf(name string) []string {
	prefix := c.commands.GetPrefix()
	cmds := c.commands.GetByPlugin(name)
	names := make([]string, 0, len(cmds))
	for n, cmd := range cmds {
		if !cmd.Hidden {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	var out []string
	for _, n := range names {
		out = append(out, prefix+n)
		for _, a := range cmds[n].Aliases {
			out = append(out, prefix+a)
		}
	}
	return out
}

func (c *catalog) Builtins() []bot.PluginInfo {
	var out []bot.PluginInfo
	for _, info := range c.mgr.GetAllInfo() {
		if c.loader.IsLoaded(info.Name) {
			continue
		}
		out = append(out, bot.PluginInfo{
			Name:     info.Name,
			Desc:     info.Description,
			DescEN:   info.DescEN,
			Commands: c.commandsOf(info.Name),
		})
	}
	return out
}

func (c *catalog) Externals() []bot.PluginInfo {
	var out []bot.PluginInfo
	for name, lp := range c.loader.GetLoaded() {
		p := bot.PluginInfo{Name: name, Commands: c.commandsOf(lp.Plugin.Name())}
		if m := lp.Metadata; m != nil {
			p.Desc, p.DescEN, p.Version, p.Author = m.Description, m.DescEN, m.Version, m.Author
		} else if info, ok := c.mgr.GetInfo(lp.Plugin.Name()); ok {
			p.Desc, p.DescEN = info.Description, info.DescEN
		}
		out = append(out, p)
	}
	for name, reason := range c.loader.Failed() {
		out = append(out, bot.PluginInfo{Name: name, Failed: reason})
	}
	return out
}

type errString string

func (e errString) Error() string { return string(e) }

func (c *catalog) Repo(ctx context.Context, fresh bool) ([]bot.RepoEntry, error) {
	idx, err := c.loader.Index(ctx, fresh)
	if err != nil {
		return nil, err
	}
	out := make([]bot.RepoEntry, 0, len(idx))
	for _, e := range idx {
		out = append(out, bot.RepoEntry{
			Name:      e.Name,
			Desc:      e.Description,
			DescEN:    e.DescEN,
			Version:   e.Version,
			Installed: c.loader.IsLoaded(e.Name),
		})
	}
	return out, nil
}

// errBuiltin refuses to shadow a built-in plugin with an external one.
var errBuiltin = errString("built-in")

func (c *catalog) Install(ctx context.Context, name string) error {
	if _, ok := c.mgr.GetInfo(name); ok && !c.loader.IsLoaded(name) {
		return errBuiltin
	}
	return c.loader.InstallAndLoad(ctx, name)
}

func (c *catalog) Remove(ctx context.Context, name string) error {
	if _, ok := c.mgr.GetInfo(name); ok && !c.loader.IsLoaded(name) {
		return errBuiltin
	}
	return c.loader.Remove(ctx, name)
}

func (c *catalog) Reload(ctx context.Context, name string) error {
	if c.loader.IsLoaded(name) {
		return c.loader.Reload(ctx, name)
	}
	return c.loader.LoadByName(ctx, name)
}

func (c *catalog) Explain(err error) (string, string) {
	if err == errBuiltin {
		return "这是系统插件，不能装卸", "a system plugin, cannot be installed or removed"
	}
	return loader.Explain(err)
}
