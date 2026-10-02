package bot

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// PluginInfo describes one plugin for the panels.
type PluginInfo struct {
	Name     string
	Desc     string
	DescEN   string
	Version  string
	Author   string
	Commands []string // with prefix, aliases included
	// Failed is the load error of an external plugin file that did not
	// load; such a plugin has no commands, settings or page.
	Failed string
}

// RepoEntry is one plugin in the external repository.
type RepoEntry struct {
	Name      string
	Desc      string
	DescEN    string
	Version   string
	Installed bool
}

// Catalog is what the bot needs to list and manage plugins.
type Catalog interface {
	Builtins() []PluginInfo
	// Externals lists loaded external plugins and files that failed.
	Externals() []PluginInfo
	Repo(ctx context.Context, fresh bool) ([]RepoEntry, error)
	Install(ctx context.Context, name string) error
	Remove(ctx context.Context, name string) error
	// Reload reloads a loaded plugin or retries a failed one.
	Reload(ctx context.Context, name string) error
	// Explain turns an action error into a reason (zh, en).
	Explain(err error) (string, string)
}

// SetCatalog wires plugin listing and management.
func (s *Service) SetCatalog(c Catalog) {
	s.mu.Lock()
	s.catalog = c
	s.mu.Unlock()
}

func (s *Service) cat() Catalog {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.catalog
}

// Grid sizes.
const (
	listCols    = 3
	listPerPage = 30
	repoPerPage = 15
)

// target is the chat message a tap came from.
type target struct {
	chatID int64
	msgID  int
}

func byName(list []PluginInfo) []PluginInfo {
	sort.SliceStable(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list
}

// builtins lists the built-in plugins. Without a catalog (tests) every
// plugin with a panel counts as built-in.
func (s *Service) builtins() []PluginInfo {
	if c := s.cat(); c != nil {
		return byName(c.Builtins())
	}
	var out []PluginInfo
	for _, n := range s.entries() {
		out = append(out, PluginInfo{Name: n})
	}
	return out
}

func (s *Service) externals() []PluginInfo {
	if c := s.cat(); c != nil {
		return byName(c.Externals())
	}
	return nil
}

func (s *Service) hasPanel(name string) bool {
	if _, ok := s.settings.Get(name); ok {
		return true
	}
	return s.page(name) != nil
}

// find looks a plugin up in both lists.
func (s *Service) find(name string) (p PluginInfo, builtin, ok bool) {
	for _, p := range s.builtins() {
		if p.Name == name {
			return p, true, true
		}
	}
	for _, p := range s.externals() {
		if p.Name == name {
			return p, false, true
		}
	}
	return PluginInfo{}, false, false
}

func (s *Service) desc(p PluginInfo) string { return s.pick(p.Desc, p.DescEN, "") }

func (s *Service) back(data string) plugin.Button {
	return plugin.Btn(s.tl("‹ 返回", "‹ Back"), data)
}

// grid lays buttons out cols per row.
func grid(btns []plugin.Button, cols int) [][]plugin.Button {
	var rows [][]plugin.Button
	for len(btns) > 0 {
		n := min(cols, len(btns))
		rows = append(rows, btns[:n:n])
		btns = btns[n:]
	}
	return rows
}

// clampPage keeps page inside the list and returns the page count.
func clampPage(page, total, per int) (int, int) {
	pages := max(1, (total+per-1)/per)
	return min(max(page, 0), pages-1), pages
}

// pager is the ‹ n/m › row, nil for a single page.
func (s *Service) pager(prefix string, page, pages int) []plugin.Button {
	if pages <= 1 {
		return nil
	}
	var row []plugin.Button
	if page > 0 {
		row = append(row, plugin.Btn("◀", fmt.Sprintf("%s%d", prefix, page-1)))
	}
	row = append(row, plugin.Btn(fmt.Sprintf("· %d / %d ·", page+1, pages), "n"))
	if page < pages-1 {
		row = append(row, plugin.Btn("▶", fmt.Sprintf("%s%d", prefix, page+1)))
	}
	return row
}

// ---------------------------------------------------------------- views

// withPanel keeps plugins that have settings or a page.
func (s *Service) withPanel(list []PluginInfo) []PluginInfo {
	var out []PluginInfo
	for _, p := range list {
		if p.Failed == "" && s.hasPanel(p.Name) {
			out = append(out, p)
		}
	}
	return out
}

// header is a screen title: icon, bold name and an optional code tag.
func header(icon, title, tag string) string {
	h := icon + " **" + title + "**"
	if tag != "" {
		h += "  " + plugin.Code(tag)
	}
	return h
}

func (s *Service) menuView() *View {
	b, x := s.withPanel(s.builtins()), s.externals()
	broken := 0
	for _, p := range x {
		if p.Failed != "" {
			broken++
		}
	}
	tag := ""
	if s.opt.Version != "" {
		tag = "v" + strings.TrimPrefix(s.opt.Version, "v")
	}
	lines := []string{
		fmt.Sprintf(s.tl("⚙️ 系统设置  **%d** 项", "⚙️ System settings  **%d**"), len(b)),
		fmt.Sprintf(s.tl("🔌 外置插件  **%d** 个可设置", "🔌 External plugins  **%d** with settings"), len(s.withPanel(x))),
	}
	rows := [][]plugin.Button{plugin.Row(
		plugin.Btn(s.tl("⚙️ 系统设置", "⚙️ System settings"), "l:b:0"),
		plugin.Btn(s.tl("🔌 外置插件", "🔌 External plugins"), "l:x:0"),
	)}
	if s.cat() != nil {
		l := fmt.Sprintf(s.tl("📦 已装插件  **%d** 个", "📦 Installed  **%d**"), len(x))
		if broken > 0 {
			l += fmt.Sprintf(s.tl("  ·  ⚠️ %d 个加载失败", "  ·  ⚠️ %d failed"), broken)
		}
		lines = append(lines, l)
		rows = append(rows, plugin.Row(plugin.Btn(s.tl("📦 插件管理", "📦 Plugin manager"), "g:0").Primary()))
	}
	return &View{Text: header("🗂", "PaperValet", tag) + "\n" + quote(lines), Buttons: rows}
}

// listView is the system settings ("b") or external plugin ("x") panel.
// Only plugins with settings or a page appear; buttons carry their names.
func (s *Service) listView(kind string, page int) *View {
	external := kind == "x"
	list := s.withPanel(s.builtins())
	icon, title := "⚙️", s.tl("系统设置", "System settings")
	if external {
		list = s.withPanel(s.externals())
		icon, title = "🔌", s.tl("外置插件", "External plugins")
	}
	page, pages := clampPage(page, len(list), listPerPage)
	var t strings.Builder
	t.WriteString(header(icon, title, ""))
	if len(list) == 0 {
		if external {
			t.WriteString("\n\n" + s.tl("还没有可设置的外置插件，去插件管理装几个吧", "No external plugin has settings yet; install some in the manager"))
		} else {
			t.WriteString("\n\n" + s.tl("没有系统设置", "No system settings"))
		}
	} else {
		t.WriteString("\n" + s.tl("点名字打开设置", "Tap a name to open its settings"))
	}
	var btns []plugin.Button
	var lines []string
	for _, p := range list[page*listPerPage : min(len(list), (page+1)*listPerPage)] {
		line := plugin.Code(p.Name)
		if d := s.desc(p); d != "" {
			line += "  " + plugin.Escape(d)
		}
		lines = append(lines, line)
		btns = append(btns, plugin.Btn(p.Name, "h:"+p.Name))
	}
	if len(lines) > 0 {
		t.WriteString("\n" + quote(lines))
	}
	v := &View{Text: t.String(), Buttons: grid(btns, listCols)}
	if row := s.pager("l:"+kind+":", page, pages); row != nil {
		v.Buttons = append(v.Buttons, row)
	}
	nav := plugin.Row(s.back("m"))
	if external && len(list) == 0 && s.cat() != nil {
		nav = append(nav, plugin.Btn(s.tl("📦 插件管理", "📦 Plugin manager"), "g:0").Primary())
	}
	v.Buttons = append(v.Buttons, nav)
	return v
}

// mgrEntry is one row of the manager: installed, failed or only in the
// repository.
type mgrEntry struct {
	PluginInfo
	installed bool
	inRepo    bool
	repoVer   string
}

// mark is the entry's status icon.
func (e mgrEntry) mark() string {
	switch {
	case e.Failed != "":
		return "⚠️"
	case e.installed:
		return "✅"
	}
	return "▫️"
}

// button is the entry's name button, colored by status.
func (e mgrEntry) button() plugin.Button {
	b := plugin.Btn(e.Name, "o:"+e.Name)
	switch {
	case e.Failed != "":
		return b.Danger()
	case e.installed:
		return b.Success()
	}
	return b
}

// mgrEntries merges installed plugins with the repository index. The
// error is the index fetch failure; installed plugins are listed anyway.
func (s *Service) mgrEntries(ctx context.Context, fresh bool) ([]mgrEntry, error) {
	by := map[string]*mgrEntry{}
	var out []mgrEntry
	for _, p := range s.externals() {
		by[p.Name] = &mgrEntry{PluginInfo: p, installed: true}
	}
	repo, err := s.cat().Repo(ctx, fresh)
	for _, r := range repo {
		if e, ok := by[r.Name]; ok {
			e.inRepo, e.repoVer = true, r.Version
			continue
		}
		by[r.Name] = &mgrEntry{PluginInfo: PluginInfo{Name: r.Name, Desc: r.Desc, DescEN: r.DescEN}, inRepo: true, repoVer: r.Version}
	}
	for _, e := range by {
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, err
}

// managerView is the plugin manager: installed plugins and the
// repository in one list, with install, reload and remove.
func (s *Service) managerView(ctx context.Context, page int, fresh bool) *View {
	if s.cat() == nil {
		return s.menuView()
	}
	list, repoErr := s.mgrEntries(ctx, fresh)
	installed, missing := 0, 0
	for _, e := range list {
		if e.installed {
			installed++
		} else {
			missing++
		}
	}
	page, pages := clampPage(page, len(list), listPerPage)
	var t strings.Builder
	t.WriteString(header("📦", s.tl("插件管理", "Plugin manager"), ""))
	t.WriteString("\n" + fmt.Sprintf(s.tl("已装 **%d**", "**%d** installed"), installed))
	if repoErr == nil {
		t.WriteString(fmt.Sprintf(s.tl("  ·  可装 **%d**", "  ·  **%d** available"), missing))
	}
	if repoErr != nil {
		t.WriteString("\n\n⚠️ " + s.tl("拉取插件仓库失败", "Could not fetch the repository") + "\n" + quote([]string{plugin.Escape(clip(repoErr.Error(), 120))}))
	}
	if len(list) == 0 && repoErr == nil {
		t.WriteString("\n\n" + s.tl("仓库是空的", "The repository is empty"))
	}
	var btns []plugin.Button
	var lines []string
	for _, e := range list[page*listPerPage : min(len(list), (page+1)*listPerPage)] {
		line := e.mark() + " " + plugin.Code(e.Name)
		if e.Failed != "" {
			line += "  " + s.tl("加载失败", "failed to load")
		} else if d := s.desc(e.PluginInfo); d != "" {
			line += "  " + plugin.Escape(d)
		}
		lines = append(lines, line)
		btns = append(btns, e.button())
	}
	if len(lines) > 0 {
		t.WriteString("\n" + quote(lines))
		t.WriteString("\n" + s.tl("绿色已安装  红色加载失败  点名字操作", "Green installed  red failed  tap a name"))
	}
	v := &View{Text: t.String(), Buttons: grid(btns, listCols)}
	if row := s.pager("g:", page, pages); row != nil {
		v.Buttons = append(v.Buttons, row)
	}
	var bulk []plugin.Button
	if missing > 0 && repoErr == nil {
		bulk = append(bulk, plugin.Btn(s.tl("📥 全部安装", "📥 Install all"), "a:ia").Success())
	}
	if installed > 0 {
		bulk = append(bulk,
			plugin.Btn(s.tl("🔄 全部重载", "🔄 Reload all"), "a:la").Primary(),
			plugin.Btn(s.tl("🗑 全部卸载", "🗑 Remove all"), "a:ra").Danger())
	}
	if len(bulk) > 0 {
		v.Buttons = append(v.Buttons, bulk)
	}
	v.Buttons = append(v.Buttons, plugin.Row(
		s.back("m"),
		plugin.Btn(s.tl("🔃 刷新仓库", "🔃 Refresh"), fmt.Sprintf("g:%d:f", page)),
	))
	return v
}

// manageView is one plugin in the manager: install when missing, reload
// and remove when installed, retry and delete when it failed to load.
func (s *Service) manageView(ctx context.Context, name string) (*View, string) {
	if s.cat() == nil {
		return s.menuView(), ""
	}
	list, _ := s.mgrEntries(ctx, false)
	var e mgrEntry
	found := false
	for _, x := range list {
		if x.Name == name {
			e, found = x, true
		}
	}
	if !found {
		return s.managerView(ctx, 0, false), s.tl("这个插件已经不在了", "That plugin is gone")
	}
	ver := e.Version
	if ver == "" {
		ver = e.repoVer
	}
	if ver != "" {
		ver = "v" + ver
	}
	var t strings.Builder
	t.WriteString(header("📦", plugin.Escape(e.Name), ver))
	if d := s.desc(e.PluginInfo); d != "" && e.Failed == "" {
		t.WriteString("\n" + plugin.Escape(d))
	}
	var facts []string
	switch {
	case e.Failed != "":
		facts = append(facts, s.tl("状态  ⚠️ 加载失败", "Status  ⚠️ failed to load"))
	case e.installed:
		facts = append(facts, s.tl("状态  ✅ 已安装", "Status  ✅ installed"))
	default:
		facts = append(facts, s.tl("状态  ▫️ 未安装", "Status  ▫️ not installed"))
	}
	if e.installed && e.repoVer != "" && e.Version != "" && e.repoVer != e.Version {
		facts = append(facts, s.tl("仓库版本  ", "Repository  ")+plugin.Code("v"+e.repoVer))
	}
	if e.Author != "" {
		facts = append(facts, s.tl("作者  ", "Author  ")+plugin.Escape(e.Author))
	}
	if e.installed && e.Failed == "" {
		if cmds := s.commandLine(e.PluginInfo); cmds != "" {
			facts = append(facts, cmds)
		}
	}
	t.WriteString("\n" + quote(facts))
	v := &View{}
	switch {
	case e.Failed != "":
		reason := s.tl(s.cat().Explain(errors.New(e.Failed)))
		t.WriteString("\n" + s.tl("原因  ", "Reason  ") + plugin.Escape(clip(reason, 300)))
		v.Buttons = append(v.Buttons, plugin.Row(
			plugin.Btn(s.tl("🔁 重试加载", "🔁 Retry"), "y:rl:"+name).Primary(),
			plugin.Btn(s.tl("🗑 删除", "🗑 Delete"), "a:rm:"+name).Danger(),
		))
	case e.installed:
		v.Buttons = append(v.Buttons, plugin.Row(
			plugin.Btn(s.tl("🔄 重载", "🔄 Reload"), "y:rl:"+name).Primary(),
			plugin.Btn(s.tl("🗑 卸载", "🗑 Remove"), "a:rm:"+name).Danger(),
		))
	default:
		v.Buttons = append(v.Buttons, plugin.Row(plugin.Btn(s.tl("📥 安装", "📥 Install"), "y:in:"+name).Success()))
	}
	v.Buttons = append(v.Buttons, plugin.Row(s.back("g:0")))
	v.Text = t.String()
	return v, ""
}

func (s *Service) commandLine(p PluginInfo) string {
	if len(p.Commands) == 0 {
		return ""
	}
	codes := make([]string, len(p.Commands))
	for i, c := range p.Commands {
		codes[i] = plugin.Code(c)
	}
	return s.tl("命令  ", "Commands  ") + strings.Join(codes, " ")
}

// infoView is a plugin's settings screen: what it is, its commands, its
// settings and its page. Plugins without either are not shown. The notice
// is set when the plugin is gone or has no settings.
func (s *Service) infoView(name string) (*View, string) {
	p, builtin, ok := s.find(name)
	if !s.hasPanel(name) || p.Failed != "" {
		if ok {
			return s.menuView(), s.tl("这个插件没有设置项", "That plugin has no settings")
		}
		return s.menuView(), s.tl("这个插件已经不在了", "That plugin is gone")
	}
	if !ok {
		p, builtin = PluginInfo{Name: name}, s.cat() == nil
	}
	listBack := "l:x:0"
	if builtin {
		listBack = "l:b:0"
	}
	tag := ""
	if p.Version != "" {
		tag = "v" + p.Version
	}
	var t strings.Builder
	t.WriteString(header("🔹", plugin.Escape(p.Name), tag))
	if d := s.desc(p); d != "" {
		t.WriteString("\n" + plugin.Escape(d))
	}
	if cmds := s.commandLine(p); cmds != "" {
		t.WriteString("\n" + cmds)
	}
	v := &View{}
	if st, ok := s.settings.Get(name); ok {
		sp := st.Spec()
		var lines []string
		for i := range sp.Settings {
			set := &sp.Settings[i]
			label := s.pick(set.Label, set.LabelEN, set.Key)
			lines = append(lines, plugin.Escape(label)+"  "+plugin.Code(s.display(st, set)))
			btn := plugin.Btn("✏️ "+label, "e:"+name+":"+set.Key)
			if set.Kind == plugin.SettingToggle {
				btn = plugin.Btn("⬜ "+label, "e:"+name+":"+set.Key)
				if st.Bool(set.Key) {
					btn = plugin.Btn("✅ "+label, "e:"+name+":"+set.Key).Success()
				}
			}
			v.Buttons = append(v.Buttons, plugin.Row(btn))
		}
		if len(lines) > 0 {
			t.WriteString("\n\n" + s.tl("**设置**", "**Settings**") + "\n" + quote(lines))
		}
	}
	if pg := s.page(name); pg != nil {
		v.Buttons = append(v.Buttons, plugin.Row(plugin.Btn("📋 "+s.pick(pg.Title, pg.TitleEN, name), "p:"+name).Primary()))
	}
	v.Buttons = append(v.Buttons, plugin.Row(s.back(listBack)))
	v.Text = t.String()
	return v, ""
}

// confirmView asks before a destructive or bulk operation.
func (s *Service) confirmView(op, name string) *View {
	var q, note, cancel string
	var yes plugin.Button
	data := "y:" + op
	if name != "" {
		data += ":" + name
	}
	switch op {
	case "rm":
		if name == "" {
			return s.menuView()
		}
		q = fmt.Sprintf(s.tl("卸载 %s？", "Remove %s?"), plugin.Code(name))
		note = s.tl("插件文件会被删除，设置会保留，重新安装后还在", "The file is deleted; its settings stay for a reinstall")
		yes = plugin.Btn(s.tl("🗑 卸载", "🗑 Remove"), data).Danger()
		cancel = "o:" + name
	case "ra":
		q = s.tl("卸载全部外置插件？", "Remove every external plugin?")
		note = s.tl("插件文件会被删除，设置会保留", "Files are deleted; settings stay")
		yes = plugin.Btn(s.tl("🗑 全部卸载", "🗑 Remove all"), data).Danger()
		cancel = "g:0"
	case "la":
		q = s.tl("重载全部外置插件？", "Reload every external plugin?")
		yes = plugin.Btn(s.tl("🔄 全部重载", "🔄 Reload all"), data).Primary()
		cancel = "g:0"
	case "ia":
		q = s.tl("安装仓库里全部未安装的插件？", "Install every plugin not installed yet?")
		yes = plugin.Btn(s.tl("📥 全部安装", "📥 Install all"), data).Success()
		cancel = "g:0"
	default:
		return s.menuView()
	}
	text := "❓ **" + q + "**"
	if note != "" {
		text += "\n" + quote([]string{note})
	}
	return &View{Text: text, Buttons: [][]plugin.Button{plugin.Row(
		plugin.Btn(s.tl("取消", "Cancel"), cancel),
		yes,
	)}}
}

func (s *Service) resultView(text, back string) *View {
	return &View{Text: text, Buttons: [][]plugin.Button{plugin.Row(s.back(back))}}
}

// ---------------------------------------------------------------- actions

// runOp performs a plugin operation, keeping a live progress panel while
// it runs. Only one runs at a time.
func (s *Service) runOp(ctx context.Context, at target, op, name string) (*View, string, error) {
	c := s.cat()
	if c == nil {
		return s.menuView(), "", nil
	}
	switch op {
	case "in", "rm", "rl":
		if name == "" {
			return nil, "", errors.New("bad button")
		}
	case "ia", "ra", "la":
	default:
		return nil, "", errors.New("bad button")
	}
	if !s.busy.CompareAndSwap(false, true) {
		return nil, "!" + s.tl("上一个操作还没完成，稍等一下", "Another operation is still running"), nil
	}
	defer s.busy.Store(false)

	reason := func(err error) string {
		zh, en := c.Explain(err)
		return s.tl(zh, en)
	}
	if op == "ia" || op == "ra" || op == "la" {
		return s.bulk(ctx, at, c, op, reason), "", nil
	}

	var verb, done, failed string
	act := c.Reload
	switch op {
	case "in":
		verb, done, failed, act = s.tl("正在安装 ", "Installing "), s.tl("已安装", "Installed"), s.tl("安装失败", "Install failed"), c.Install
	case "rm":
		verb, done, failed, act = s.tl("正在卸载 ", "Removing "), s.tl("已卸载", "Removed"), s.tl("卸载失败", "Remove failed"), c.Remove
	case "rl":
		verb, done, failed = s.tl("正在重载 ", "Reloading "), s.tl("已重载", "Reloaded"), s.tl("重载失败", "Reload failed")
	}
	pr := s.startProgress(ctx, at, verb+plugin.Escape(name), 0)
	err := act(ctx, name)
	pr.finish()
	took := pr.elapsed()
	if err != nil {
		v := &View{
			Text: "❌ **" + failed + "**  " + plugin.Code(name) + "\n" + quote([]string{plugin.Escape(reason(err))}),
			Buttons: [][]plugin.Button{plugin.Row(
				s.back("o:"+name),
				plugin.Btn(s.tl("🔁 重试", "🔁 Retry"), "y:"+op+":"+name).Primary(),
			)},
		}
		if op == "rm" {
			v.Buttons = [][]plugin.Button{plugin.Row(s.back("g:0"))}
		}
		return v, failed, nil
	}
	var v *View
	if op == "rm" {
		v = s.managerView(ctx, 0, false)
	} else {
		v, _ = s.manageView(ctx, name)
	}
	v.Text = "✅ " + done + " " + plugin.Code(name) + "  ·  " + took + "\n\n" + v.Text
	return v, done, nil
}

// bulk runs op over every target, updating the progress panel as each
// plugin starts and finishes, then shows the summary.
func (s *Service) bulk(ctx context.Context, at target, c Catalog, op string, reason func(error) string) *View {
	var names []string
	back, title, doneT := "g:0", "", ""
	act := c.Reload
	switch op {
	case "ia":
		title, doneT, act = s.tl("全部安装", "Install all"), s.tl("全部安装完成", "All installed"), c.Install
		pr := s.startProgress(ctx, at, s.tl("正在拉取插件清单", "Fetching the index"), 0)
		entries, err := c.Repo(ctx, true)
		pr.finish()
		if err != nil {
			return &View{
				Text: "❌ **" + s.tl("拉取插件清单失败", "Could not fetch the index") + "**\n" + quote([]string{plugin.Escape(clip(err.Error(), 200))}),
				Buttons: [][]plugin.Button{plugin.Row(
					s.back(back),
					plugin.Btn(s.tl("🔁 重试", "🔁 Retry"), "y:ia").Primary(),
				)},
			}
		}
		for _, e := range entries {
			if !e.Installed {
				names = append(names, e.Name)
			}
		}
	case "ra":
		title, doneT, act = s.tl("全部卸载", "Remove all"), s.tl("全部卸载完成", "All removed"), c.Remove
		for _, p := range s.externals() {
			names = append(names, p.Name)
		}
	case "la":
		title, doneT = s.tl("全部重载", "Reload all"), s.tl("全部重载完成", "All reloaded")
		for _, p := range s.externals() {
			if p.Failed == "" {
				names = append(names, p.Name)
			}
		}
	}
	if len(names) == 0 {
		return s.resultView("⚪ "+s.tl("没有要处理的插件", "Nothing to do"), back)
	}
	pr := s.startProgress(ctx, at, title, len(names))
	ok := 0
	for _, n := range names {
		if ctx.Err() != nil {
			break
		}
		pr.step(n)
		if err := act(ctx, n); err != nil {
			pr.result("❌ " + plugin.Code(n) + "  " + plugin.Escape(clip(reason(err), 80)))
			continue
		}
		ok++
		pr.result("✅ " + plugin.Code(n))
	}
	pr.finish()
	pr.mu.Lock()
	lines, finished := pr.lines, pr.done
	pr.mu.Unlock()

	head := "✅ **" + doneT + "**"
	switch {
	case ok == 0:
		head = "❌ **" + title + s.tl("失败", " failed") + "**"
	case ok < len(names):
		head = "⚠️ **" + title + s.tl("部分失败", ": some failed") + "**"
	}
	stats := fmt.Sprintf(s.tl("%s  成功 %d · 失败 %d · 用时 %s", "%s  %d ok · %d failed · %s"),
		bar(finished, len(names)), ok, finished-ok, pr.elapsed())
	if finished < len(names) {
		stats += fmt.Sprintf(s.tl(" · 中断 %d", " · %d skipped"), len(names)-finished)
	}
	return s.resultView(head+"\n"+stats+"\n"+quote(lines), back)
}
