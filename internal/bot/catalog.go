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
	return plugin.Btn(s.tl("« 返回", "« Back"), data)
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
		row = append(row, plugin.Btn("‹", fmt.Sprintf("%s%d", prefix, page-1)))
	}
	row = append(row, plugin.Btn(fmt.Sprintf("%d/%d", page+1, pages), "n"))
	if page < pages-1 {
		row = append(row, plugin.Btn("›", fmt.Sprintf("%s%d", prefix, page+1)))
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

func (s *Service) menuView() *View {
	b, x := s.withPanel(s.builtins()), s.externals()
	broken := 0
	for _, p := range x {
		if p.Failed != "" {
			broken++
		}
	}
	var t strings.Builder
	t.WriteString("🗂 **PaperValet**\n\n")
	t.WriteString(fmt.Sprintf(s.tl("⚙️ 系统设置  %d 项", "⚙️ System settings  %d"), len(b)) + "\n")
	t.WriteString(fmt.Sprintf(s.tl("🔌 外置插件  %d 个有设置", "🔌 External plugins  %d with settings"), len(s.withPanel(x))))
	rows := [][]plugin.Button{plugin.Row(
		plugin.Btn(s.tl("⚙️ 系统设置", "⚙️ System settings"), "l:b:0"),
		plugin.Btn(s.tl("🔌 外置插件", "🔌 External plugins"), "l:x:0"),
	)}
	if s.cat() != nil {
		t.WriteString("\n" + fmt.Sprintf(s.tl("📦 插件管理  已装 %d 个", "📦 Plugin manager  %d installed"), len(x)))
		if broken > 0 {
			t.WriteString(fmt.Sprintf(s.tl("  ·  ⚠️ %d 个加载失败", "  ·  ⚠️ %d failed"), broken))
		}
		rows = append(rows, plugin.Row(plugin.Btn(s.tl("📦 插件管理", "📦 Plugin manager"), "g:0")))
	}
	return &View{Text: t.String(), Buttons: rows}
}

// listView is the system settings ("b") or external plugin ("x") panel.
// Only plugins with settings or a page appear; buttons carry their names.
func (s *Service) listView(kind string, page int) *View {
	external := kind == "x"
	list := s.withPanel(s.builtins())
	title := s.tl("⚙️ **系统设置**", "⚙️ **System settings**")
	if external {
		list = s.withPanel(s.externals())
		title = s.tl("🔌 **外置插件**", "🔌 **External plugins**")
	}
	page, pages := clampPage(page, len(list), listPerPage)
	var t strings.Builder
	t.WriteString(title + "\n")
	if len(list) == 0 {
		if external {
			t.WriteString("\n" + s.tl("没有带设置项的外置插件", "No external plugin has settings"))
		} else {
			t.WriteString("\n" + s.tl("没有系统设置", "No system settings"))
		}
	}
	var btns []plugin.Button
	for _, p := range list[page*listPerPage : min(len(list), (page+1)*listPerPage)] {
		line := "\n• " + plugin.Code(p.Name)
		if d := s.desc(p); d != "" {
			line += "  " + plugin.Escape(d)
		}
		t.WriteString(line)
		btns = append(btns, plugin.Btn(p.Name, "h:"+p.Name))
	}
	v := &View{Text: t.String(), Buttons: grid(btns, listCols)}
	if row := s.pager("l:"+kind+":", page, pages); row != nil {
		v.Buttons = append(v.Buttons, row)
	}
	if external && s.cat() != nil {
		v.Buttons = append(v.Buttons, plugin.Row(plugin.Btn(s.tl("📦 插件管理", "📦 Plugin manager"), "g:0")))
	}
	v.Buttons = append(v.Buttons, plugin.Row(s.back("m")))
	return v
}

// managerView is the apt panel: every installed external plugin, broken
// files included, plus the repository and bulk actions.
func (s *Service) managerView(page int) *View {
	if s.cat() == nil {
		return s.menuView()
	}
	list := s.externals()
	page, pages := clampPage(page, len(list), listPerPage)
	var t strings.Builder
	t.WriteString(s.tl("📦 **插件管理**", "📦 **Plugin manager**") + fmt.Sprintf(s.tl("  已装 %d 个\n", "  %d installed\n"), len(list)))
	if len(list) == 0 {
		t.WriteString("\n" + s.tl("还没有安装外置插件，去仓库看看", "No external plugins yet, browse the repository"))
	}
	var btns []plugin.Button
	for _, p := range list[page*listPerPage : min(len(list), (page+1)*listPerPage)] {
		line := "\n✅ " + plugin.Code(p.Name)
		if p.Failed != "" {
			line = "\n⚠️ " + plugin.Code(p.Name) + "  " + s.tl("加载失败", "failed to load")
		} else {
			if p.Version != "" {
				line += " " + plugin.Code("v"+p.Version)
			}
			if d := s.desc(p); d != "" {
				line += "  " + plugin.Escape(d)
			}
		}
		t.WriteString(line)
		btns = append(btns, plugin.Btn(p.Name, "o:"+p.Name))
	}
	v := &View{Text: t.String(), Buttons: grid(btns, listCols)}
	if row := s.pager("g:", page, pages); row != nil {
		v.Buttons = append(v.Buttons, row)
	}
	v.Buttons = append(v.Buttons, plugin.Row(plugin.Btn(s.tl("📥 插件仓库", "📥 Repository"), "s:0")))
	if len(list) > 0 {
		v.Buttons = append(v.Buttons, plugin.Row(
			plugin.Btn(s.tl("🔄 全部重载", "🔄 Reload all"), "a:la"),
			plugin.Btn(s.tl("🗑 全部卸载", "🗑 Remove all"), "a:ra"),
		))
	}
	v.Buttons = append(v.Buttons, plugin.Row(s.back("m")))
	return v
}

// manageView is one installed external plugin in the apt panel: version,
// commands, reload and remove (retry and delete when it failed to load).
func (s *Service) manageView(name string) (*View, string) {
	var p PluginInfo
	found := false
	for _, x := range s.externals() {
		if x.Name == name {
			p, found = x, true
		}
	}
	if !found {
		return s.managerView(0), s.tl("这个插件已经不在了", "That plugin is gone")
	}
	var t strings.Builder
	t.WriteString("📦 **" + plugin.Escape(p.Name) + "**")
	if p.Version != "" {
		t.WriteString("  " + plugin.Code("v"+p.Version))
	}
	t.WriteString("\n")
	if d := s.desc(p); d != "" {
		t.WriteString(plugin.Escape(d) + "\n")
	}
	if p.Author != "" {
		t.WriteString(s.tl("作者 ", "Author ") + plugin.Escape(p.Author) + "\n")
	}
	if p.Failed != "" {
		reason := s.tl(s.cat().Explain(errors.New(p.Failed)))
		t.WriteString("\n⚠️ " + s.tl("加载失败：", "Failed to load: ") + plugin.Escape(clip(reason, 300)))
		return &View{Text: t.String(), Buttons: [][]plugin.Button{
			plugin.Row(
				plugin.Btn(s.tl("🔁 重试", "🔁 Retry"), "y:rl:"+name),
				plugin.Btn(s.tl("🗑 删除", "🗑 Delete"), "a:rm:"+name),
			),
			plugin.Row(s.back("g:0")),
		}}, ""
	}
	if cmds := s.commandLine(p); cmds != "" {
		t.WriteString("\n" + cmds + "\n")
	}
	v := &View{}
	if s.hasPanel(name) {
		v.Buttons = append(v.Buttons, plugin.Row(plugin.Btn(s.tl("⚙️ 设置", "⚙️ Settings"), "h:"+name)))
	}
	v.Buttons = append(v.Buttons,
		plugin.Row(
			plugin.Btn(s.tl("🔄 重载", "🔄 Reload"), "y:rl:"+name),
			plugin.Btn(s.tl("🗑 卸载", "🗑 Remove"), "a:rm:"+name),
		),
		plugin.Row(s.back("g:0")),
	)
	v.Text = strings.TrimRight(t.String(), "\n")
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
// settings and its page. Plugins without either are not shown here:
// external ones go to their apt screen, others back to the menu. The
// notice is set when the plugin is gone.
func (s *Service) infoView(name string) (*View, string) {
	p, builtin, ok := s.find(name)
	if !s.hasPanel(name) || p.Failed != "" {
		if ok && !builtin && s.cat() != nil {
			return s.manageView(name)
		}
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
	var t strings.Builder
	t.WriteString("🔹 **" + plugin.Escape(p.Name) + "**")
	if p.Version != "" {
		t.WriteString("  " + plugin.Code("v"+p.Version))
	}
	t.WriteString("\n")
	if d := s.desc(p); d != "" {
		t.WriteString(plugin.Escape(d) + "\n")
	}
	if cmds := s.commandLine(p); cmds != "" {
		t.WriteString("\n" + cmds + "\n")
	}
	v := &View{}
	if st, ok := s.settings.Get(name); ok {
		sp := st.Spec()
		t.WriteString("\n" + s.tl("**设置**", "**Settings**"))
		for i := range sp.Settings {
			set := &sp.Settings[i]
			label := s.pick(set.Label, set.LabelEN, set.Key)
			t.WriteString("\n" + plugin.Escape(label) + "  " + plugin.Code(s.display(st, set)))
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
	}
	if pg := s.page(name); pg != nil {
		v.Buttons = append(v.Buttons, plugin.Row(plugin.Btn("📋 "+s.pick(pg.Title, pg.TitleEN, name), "p:"+name)))
	}
	if !builtin && ok && s.cat() != nil {
		v.Buttons = append(v.Buttons, plugin.Row(plugin.Btn(s.tl("📦 管理", "📦 Manage"), "o:"+name)))
	}
	v.Buttons = append(v.Buttons, plugin.Row(s.back(listBack)))
	v.Text = strings.TrimRight(t.String(), "\n")
	return v, ""
}

// repoView lists the repository; installed entries open their screen,
// the rest open the install screen.
func (s *Service) repoView(ctx context.Context, page int, fresh bool) *View {
	c := s.cat()
	if c == nil {
		return s.menuView()
	}
	head := "📦 **" + s.tl("插件仓库", "Repository") + "**"
	entries, err := c.Repo(ctx, fresh)
	if err != nil {
		return &View{
			Text: head + "\n\n❌ " + s.tl("拉取插件清单失败：", "Could not fetch the index: ") + plugin.Escape(err.Error()),
			Buttons: [][]plugin.Button{
				plugin.Row(plugin.Btn(s.tl("🔁 重试", "🔁 Retry"), "s:0:f")),
				plugin.Row(s.back("g:0")),
			},
		}
	}
	installed := 0
	for _, e := range entries {
		if e.Installed {
			installed++
		}
	}
	page, pages := clampPage(page, len(entries), repoPerPage)
	var t strings.Builder
	t.WriteString(head + "  " + fmt.Sprintf(s.tl("%d 个 · 已装 %d", "%d · %d installed"), len(entries), installed) + "\n")
	if len(entries) == 0 {
		t.WriteString("\n" + s.tl("仓库是空的", "The repository is empty"))
	}
	var btns []plugin.Button
	for _, e := range entries[page*repoPerPage : min(len(entries), (page+1)*repoPerPage)] {
		mark, data := "▫️", "i:"+e.Name
		if e.Installed {
			mark, data = "✅", "o:"+e.Name
		}
		line := "\n" + mark + " " + plugin.Code(e.Name)
		if d := s.pick(e.Desc, e.DescEN, ""); d != "" {
			line += "  " + plugin.Escape(d)
		}
		t.WriteString(line)
		btns = append(btns, plugin.Btn(e.Name, data))
	}
	if len(entries) > 0 {
		t.WriteString("\n\n" + s.tl("✅ 已安装  ▫️ 点名字安装", "✅ installed  ▫️ tap a name to install"))
	}
	v := &View{Text: t.String(), Buttons: grid(btns, listCols)}
	if row := s.pager("s:", page, pages); row != nil {
		v.Buttons = append(v.Buttons, row)
	}
	last := []plugin.Button{plugin.Btn(s.tl("🔄 刷新", "🔄 Refresh"), fmt.Sprintf("s:%d:f", page))}
	if installed < len(entries) {
		last = append(last, plugin.Btn(s.tl("📥 全部安装", "📥 Install all"), "a:ia"))
	}
	v.Buttons = append(v.Buttons, last, plugin.Row(s.back("g:0")))
	return v
}

// repoEntryView is the install screen of one repository plugin.
func (s *Service) repoEntryView(ctx context.Context, name string) (*View, string) {
	c := s.cat()
	if c == nil {
		return s.menuView(), ""
	}
	entries, err := c.Repo(ctx, false)
	if err != nil {
		return s.repoView(ctx, 0, false), ""
	}
	for _, e := range entries {
		if e.Name != name {
			continue
		}
		if e.Installed {
			return s.manageView(name)
		}
		var t strings.Builder
		t.WriteString("📦 **" + plugin.Escape(e.Name) + "**")
		if e.Version != "" {
			t.WriteString("  " + plugin.Code("v"+e.Version))
		}
		if d := s.pick(e.Desc, e.DescEN, ""); d != "" {
			t.WriteString("\n" + plugin.Escape(d))
		}
		t.WriteString("\n\n" + s.tl("还没安装", "Not installed"))
		return &View{Text: t.String(), Buttons: [][]plugin.Button{
			plugin.Row(plugin.Btn(s.tl("📥 安装", "📥 Install"), "y:in:"+name)),
			plugin.Row(s.back("s:0")),
		}}, ""
	}
	return s.repoView(ctx, 0, false), s.tl("仓库里没有这个插件了", "No longer in the repository")
}

// confirmView asks before a destructive or bulk operation.
func (s *Service) confirmView(op, name string) *View {
	var q, cancel string
	switch op {
	case "rm":
		if name == "" {
			return s.managerView(0)
		}
		q = fmt.Sprintf(s.tl("卸载 %s？插件文件会被删除，设置会保留，重新安装后还在。", "Remove %s? The file is deleted; its settings stay for a reinstall."), plugin.Code(name))
		cancel = "o:" + name
	case "ra":
		q = s.tl("卸载全部外置插件？设置会保留。", "Remove every external plugin? Settings stay.")
		cancel = "g:0"
	case "la":
		q = s.tl("重载全部外置插件？", "Reload every external plugin?")
		cancel = "g:0"
	case "ia":
		q = s.tl("安装仓库里全部未安装的插件？", "Install every plugin not installed yet?")
		cancel = "s:0"
	default:
		return s.menuView()
	}
	yes := "y:" + op
	if name != "" {
		yes += ":" + name
	}
	return &View{Text: "❓ " + q, Buttons: [][]plugin.Button{plugin.Row(
		plugin.Btn(s.tl("✅ 确定", "✅ Confirm"), yes),
		plugin.Btn(s.tl("取消", "Cancel"), cancel),
	)}}
}

func (s *Service) resultView(text, back string) *View {
	return &View{Text: text, Buttons: [][]plugin.Button{plugin.Row(s.back(back))}}
}

// ---------------------------------------------------------------- actions

// runOp performs a plugin operation, showing progress on the panel first.
// Only one runs at a time.
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

	progress := func(text string) {
		if at.msgID != 0 {
			_ = s.edit(ctx, at.chatID, at.msgID, &View{Text: "⏳ " + text})
		}
	}
	reason := func(err error) string {
		zh, en := c.Explain(err)
		return s.tl(zh, en)
	}
	fail := func(err error) string {
		return fmt.Sprintf("❌ %s  %s", plugin.Code(name), plugin.Escape(reason(err)))
	}
	switch op {
	case "in":
		progress(s.tl("正在安装 ", "Installing ") + plugin.Code(name))
		if err := c.Install(ctx, name); err != nil {
			return s.resultView(fail(err), "s:0"), "", nil
		}
		v, _ := s.manageView(name)
		return v, s.tl("已安装", "Installed"), nil
	case "rm":
		progress(s.tl("正在卸载 ", "Removing ") + plugin.Code(name))
		if err := c.Remove(ctx, name); err != nil {
			return s.resultView(fail(err), "g:0"), "", nil
		}
		return s.managerView(0), s.tl("已卸载", "Removed"), nil
	case "rl":
		progress(s.tl("正在重载 ", "Reloading ") + plugin.Code(name))
		if err := c.Reload(ctx, name); err != nil {
			v, gone := s.manageView(name)
			if gone != "" {
				return s.resultView(fail(err), "g:0"), "", nil
			}
			return v, "!" + reason(err), nil
		}
		v, _ := s.manageView(name)
		return v, s.tl("已重载", "Reloaded"), nil
	}
	return s.bulk(ctx, c, op, progress, reason), "", nil
}

func (s *Service) bulk(ctx context.Context, c Catalog, op string, progress func(string), reason func(error) string) *View {
	var names []string
	back, verb, done := "g:0", "", ""
	act := c.Reload
	switch op {
	case "ia":
		back, verb, done, act = "s:0", s.tl("安装中", "Installing"), s.tl("已安装", "installed"), c.Install
		entries, err := c.Repo(ctx, true)
		if err != nil {
			return s.resultView("❌ "+s.tl("拉取插件清单失败：", "Could not fetch the index: ")+plugin.Escape(err.Error()), back)
		}
		for _, e := range entries {
			if !e.Installed {
				names = append(names, e.Name)
			}
		}
	case "ra":
		verb, done, act = s.tl("卸载中", "Removing"), s.tl("已卸载", "removed"), c.Remove
		for _, p := range s.externals() {
			names = append(names, p.Name)
		}
	case "la":
		verb, done = s.tl("重载中", "Reloading"), s.tl("已重载", "reloaded")
		for _, p := range s.externals() {
			if p.Failed == "" {
				names = append(names, p.Name)
			}
		}
	}
	if len(names) == 0 {
		return s.resultView("⚪ "+s.tl("没有要处理的插件", "Nothing to do"), back)
	}
	var lines []string
	ok := 0
	for i, n := range names {
		progress(fmt.Sprintf("%s %d/%d  %s", verb, i+1, len(names), plugin.Code(n)))
		if err := act(ctx, n); err != nil {
			lines = append(lines, "❌ "+plugin.Code(n)+"  "+plugin.Escape(reason(err)))
			continue
		}
		ok++
		lines = append(lines, "✅ "+plugin.Code(n)+"  "+done)
	}
	head := fmt.Sprintf("**%d/%d**\n\n", ok, len(names))
	return s.resultView(head+strings.Join(lines, "\n"), back)
}
