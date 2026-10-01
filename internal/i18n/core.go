package i18n

// Core translations shared by the framework and built-in plugins.
// Keys are grouped by feature area; plugins add their own keys.

func CoreCatalog() *Catalog {
	c := New(ZhCN)

	// ---- core ----
	c.AddMany(ZhCN, map[string]string{
		"core.version":       "PaperValet **{0}**\nGo: {1}\n构建: {2}",
		"core.ping":          "🏓 Pong!\n📡 **延迟:** {0}",
		"core.ping_start":    "🏓 Pong!",
		"core.unknown_cmd":   "❌ 未知命令: {0}\n使用 {1}help 查看帮助",
		"core.no_permission": "⛔ 你没有权限执行此命令",
		"core.arg_error":     "❌ 参数错误\n用法: {0}",
		"core.not_reply":     "❌ 请回复一条消息",
		"core.executing":     "⏳ 执行中...",
		"core.done":          "✅ 完成",
		"core.failed":        "❌ 失败: {0}",
		"core.usage":         "用法: {0}",
	})
	c.AddMany(EnUS, map[string]string{
		"core.version":       "PaperValet **{0}**\nGo: {1}\nBuild: {2}",
		"core.ping":          "🏓 Pong!\n📡 **Latency:** {0}",
		"core.ping_start":    "🏓 Pong!",
		"core.unknown_cmd":   "❌ Unknown command: {0}\nUse {1}help to list commands",
		"core.no_permission": "⛔ You don't have permission to run this command",
		"core.arg_error":     "❌ Invalid arguments\nUsage: {0}",
		"core.not_reply":     "❌ Reply to a message first",
		"core.executing":     "⏳ Executing...",
		"core.done":          "✅ Done",
		"core.failed":        "❌ Failed: {0}",
		"core.usage":         "Usage: {0}",
	})

	// ---- help ----
	c.AddMany(ZhCN, map[string]string{
		"help.title":         "📚 **PaperValet 帮助**",
		"help.cat_core":      "🔧 核心",
		"help.cat_admin":     "👑 管理员",
		"help.cat_tools":     "🛠 工具",
		"help.cat_fun":       "🎮 娱乐",
		"help.cat_debug":     "🐛 调试",
		"help.cat_other":     "📦 其他",
		"help.detail_hint":   "使用 {0}help <命令> 查看详情",
		"help.plugins_hint":  "使用 {0}apt list 查看插件列表",
		"help.not_found":     "未找到命令或插件: {0}",
		"help.usage":         "**用法:** `{0}`",
		"help.aliases":       "**别名:** `{0}`",
		"help.owner_only":    "⚠️ **仅拥有者可用**",
		"help.rate_limit":    "⏱ **频率限制:** {0}次/{1}s",
		"help.status":        "状态: {0}",
		"help.status_active": "✅ 活跃",
		"help.status_error":  "❌ 错误",
		"help.status_idle":   "⏸️ 未激活",
		"help.commands":      "**命令:**",
		"help.no_commands":   "无命令",
	})
	c.AddMany(EnUS, map[string]string{
		"help.title":         "📚 **PaperValet Help**",
		"help.cat_core":      "🔧 Core",
		"help.cat_admin":     "👑 Admin",
		"help.cat_tools":     "🛠 Tools",
		"help.cat_fun":       "🎮 Fun",
		"help.cat_debug":     "🐛 Debug",
		"help.cat_other":     "📦 Other",
		"help.detail_hint":   "Use {0}help <command> for details",
		"help.plugins_hint":  "Use {0}apt list to list plugins",
		"help.not_found":     "Command or plugin not found: {0}",
		"help.usage":         "**Usage:** `{0}`",
		"help.aliases":       "**Aliases:** `{0}`",
		"help.owner_only":    "⚠️ **Owner only**",
		"help.rate_limit":    "⏱ **Rate limit:** {0}/per {1}s",
		"help.status":        "Status: {0}",
		"help.status_active": "✅ Active",
		"help.status_error":  "❌ Error",
		"help.status_idle":   "⏸️ Inactive",
		"help.commands":      "**Commands:**",
		"help.no_commands":   "No commands",
	})

	// ---- admin ----
	c.AddMany(ZhCN, map[string]string{
		"admin.restarting": "🔄 正在重启...",
	})
	c.AddMany(EnUS, map[string]string{
		"admin.restarting": "🔄 Restarting...",
	})

	// ---- i18n ----
	c.AddMany(ZhCN, map[string]string{
		"i18n.current":   "🌐 当前语言: `{0}`",
		"i18n.available": "可用语言: {0}",
		"i18n.set":       "✅ 语言已切换为: `{0}`",
		"i18n.invalid":   "❌ 不支持的语言: {0}\n可用: {1}",
	})
	c.AddMany(EnUS, map[string]string{
		"i18n.current":   "🌐 Current language: `{0}`",
		"i18n.available": "Available languages: {0}",
		"i18n.set":       "✅ Language switched to: `{0}`",
		"i18n.invalid":   "❌ Unsupported language: {0}\nAvailable: {1}",
	})

	return c
}
