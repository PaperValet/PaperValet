package builtin

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// InfoPlugin shows a detailed profile card for a user plus the current
// chat and message IDs, modeled on TeleBox's ids plugin.
type InfoPlugin struct{}

func NewInfo() *InfoPlugin { return &InfoPlugin{} }

func (p *InfoPlugin) Name() string        { return "info" }
func (p *InfoPlugin) Description() string { return "查看用户和聊天信息" }
func (p *InfoPlugin) DescEN() string      { return "Look up user and chat info" }

func (p *InfoPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "info",
		Description: "查看用户和聊天信息",
		DescEN:      "Look up user and chat info",
		Usage: "info [@用户名|用户ID]，或回复消息\n" +
			"\n" +
			"**查谁**\n" +
			"• 不带参数  查自己\n" +
			"• 回复一条消息  查发送者\n" +
			"• `info @username` 或 `info 123456`  查指定用户\n" +
			"\n" +
			"**显示内容**\n" +
			"• 名字、全部用户名（含收藏用户名）、用户 ID\n" +
			"• 注册时间（按 ID 估算，误差约两个月）\n" +
			"• 入群时间（仅超级群/频道）\n" +
			"• 头像所在 DC、共同群数量\n" +
			"• 机器人、已验证、Premium、诈骗、虚假等标签\n" +
			"• 个人简介\n" +
			"• 资料、聊天、打开消息三种跳转链接及可复制的链接文本\n" +
			"• 当前会话 ID、消息 ID 和 t.me/c 跳转链接；回复时还有被回复消息和转发来源",
		UsageEN: "info [@username|user ID], or reply to a message\n" +
			"\n" +
			"**Target**\n" +
			"• no argument  yourself\n" +
			"• reply to a message  its sender\n" +
			"• `info @username` or `info 123456`  a specific user\n" +
			"\n" +
			"**Shows**\n" +
			"• name, every username (collectible ones too), user ID\n" +
			"• registration date (estimated from the ID, about ±2 months)\n" +
			"• join date (supergroups/channels only)\n" +
			"• profile photo DC, number of common chats\n" +
			"• bot, verified, Premium, scam and fake badges\n" +
			"• bio\n" +
			"• profile, chat and open-message links plus copyable link text\n" +
			"• current chat ID, message ID and t.me/c links; on reply also the replied message and forward origin",
		Plugin:   p.Name(),
		Category: "tools",
		Handler:  p.handleInfo,
	})
}

func (p *InfoPlugin) Start(_ context.Context) error { return nil }
func (p *InfoPlugin) Stop(_ context.Context) error  { return nil }

// regPoints are (user id, unix time) samples used to estimate when an
// account was registered. Same calibration as TeleBox ids.
var regPoints = [][2]float64{
	{0, 1376438400}, {50000000, 1400000000}, {150000000, 1451606400},
	{350000000, 1483228800}, {500000000, 1514764800}, {900000000, 1559347200},
	{1100000000, 1585699200}, {1450000000, 1609459200}, {2150000000, 1640995200},
	{5100000000, 1654041600}, {5600000000, 1672531200}, {6800000000, 1704067200},
	{7800000000, 1735689600}, {8500000000, 1767225600},
}

// estimateRegDate interpolates the registration month from the user id.
func estimateRegDate(id int64) time.Time {
	x := float64(id)
	lo, hi := regPoints[0], regPoints[len(regPoints)-1]
	for i := 0; i < len(regPoints)-1; i++ {
		if x >= regPoints[i][0] && x <= regPoints[i+1][0] {
			lo, hi = regPoints[i], regPoints[i+1]
			break
		}
	}
	if x > hi[0] {
		lo, hi = regPoints[len(regPoints)-2], regPoints[len(regPoints)-1]
	}
	ts := lo[1] + (x-lo[0])*(hi[1]-lo[1])/(hi[0]-lo[0])
	return time.Unix(int64(ts), 0)
}

// chatLink returns the t.me/c base link for groups and channels.
func chatLink(chatID int64) string {
	s := fmt.Sprintf("%d", chatID)
	switch {
	case strings.HasPrefix(s, "-100"):
		return "https://t.me/c/" + s[4:]
	case strings.HasPrefix(s, "-"):
		return "https://t.me/c/" + s[1:]
	}
	return ""
}

// usernamesOf returns the main username plus active collectible ones.
func usernamesOf(u *tg.User) []string {
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	add(u.Username)
	for _, un := range u.Usernames {
		if un.Active {
			add(un.Username)
		}
	}
	return out
}

func displayName(u *tg.User) string {
	name := strings.TrimSpace(strings.Join([]string{u.FirstName, u.LastName}, " "))
	if name == "" {
		return fmt.Sprintf("#%d", u.ID)
	}
	if u.Username != "" {
		return fmt.Sprintf("%s (@%s)", name, u.Username)
	}
	return name
}

func code(v any) string { return plugin.Code(fmt.Sprint(v)) }

// profile collects everything shown on the card.
type profile struct {
	user    *tg.User
	id      int64
	bio     string
	common  int
	dc      string
	joined  time.Time
	hasFull bool
}

// resolveTarget picks the user to describe: argument, replied sender, or self.
func (p *InfoPlugin) resolveTarget(ctx *interfaces.CommandContext) (*tg.User, int64, error) {
	arg := ctx.GetArg(0)
	switch {
	case strings.HasPrefix(arg, "@") && len(arg) > 1:
		res, err := ctx.API.ContactsResolveUsername(ctx.Context(), &tg.ContactsResolveUsernameRequest{Username: arg[1:]})
		if err != nil {
			return nil, 0, err
		}
		for _, u := range res.Users {
			if user, ok := u.(*tg.User); ok {
				return user, user.ID, nil
			}
		}
		return nil, 0, fmt.Errorf("%s", ctx.Tlocal("这个用户名不是用户", "that username is not a user"))
	case arg != "":
		var id int64
		if _, err := fmt.Sscanf(arg, "%d", &id); err != nil || id <= 0 {
			return nil, 0, fmt.Errorf("%s", ctx.Tlocal("参数要写 @用户名 或数字 ID", "use @username or a numeric ID"))
		}
		return nil, id, nil
	case ctx.Message.IsReply:
		if msg, res, err := fetchMessage(ctx, ctx.Message.ReplyToID); err == nil {
			if pu, ok := msg.FromID.(*tg.PeerUser); ok {
				return findUserInChats(res, pu.UserID), pu.UserID, nil
			}
		}
	}
	return nil, ctx.SelfID, nil
}

func (p *InfoPlugin) inputUser(ctx *interfaces.CommandContext, user *tg.User, id int64) tg.InputUserClass {
	if user != nil && user.AccessHash != 0 {
		return &tg.InputUser{UserID: user.ID, AccessHash: user.AccessHash}
	}
	if id == ctx.SelfID {
		return &tg.InputUserSelf{}
	}
	if peer, err := ctx.PeerResolver.ResolveFromChatID(ctx.Context(), id); err == nil {
		if pu, ok := peer.(*tg.InputPeerUser); ok {
			return &tg.InputUser{UserID: pu.UserID, AccessHash: pu.AccessHash}
		}
	}
	return &tg.InputUser{UserID: id}
}

func (p *InfoPlugin) load(ctx *interfaces.CommandContext, user *tg.User, id int64) *profile {
	pr := &profile{user: user, id: id}
	full, err := ctx.API.UsersGetFullUser(ctx.Context(), p.inputUser(ctx, user, id))
	if err == nil {
		pr.hasFull = true
		pr.bio = full.FullUser.About
		pr.common = full.FullUser.CommonChatsCount
		for _, u := range full.Users {
			if fu, ok := u.(*tg.User); ok && fu.ID == id {
				pr.user = fu
			}
		}
	}
	if pr.user == nil {
		pr.user = &tg.User{ID: id}
	}
	switch ph := pr.user.Photo.(type) {
	case *tg.UserProfilePhoto:
		pr.dc = fmt.Sprintf("DC%d", ph.DCID)
	case *tg.UserProfilePhotoEmpty:
		pr.dc = ctx.Tlocal("无头像", "no photo")
	default:
		pr.dc = ctx.Tlocal("未知", "unknown")
	}

	// Join date only exists for supergroups and channels.
	if peer, err := ctx.ResolvePeer(); err == nil {
		if ch, ok := peer.(*tg.InputPeerChannel); ok {
			part, err := ctx.API.ChannelsGetParticipant(ctx.Context(), &tg.ChannelsGetParticipantRequest{
				Channel:     &tg.InputChannel{ChannelID: ch.ChannelID, AccessHash: ch.AccessHash},
				Participant: inputPeerOfUser(p.inputUser(ctx, pr.user, id)),
			})
			if err == nil {
				if d := participantDate(part.Participant); d > 0 {
					pr.joined = time.Unix(int64(d), 0)
				}
			}
		}
	}
	return pr
}

func inputPeerOfUser(u tg.InputUserClass) tg.InputPeerClass {
	switch v := u.(type) {
	case *tg.InputUser:
		return &tg.InputPeerUser{UserID: v.UserID, AccessHash: v.AccessHash}
	case *tg.InputUserSelf:
		return &tg.InputPeerSelf{}
	}
	return &tg.InputPeerEmpty{}
}

func participantDate(p tg.ChannelParticipantClass) int {
	switch v := p.(type) {
	case *tg.ChannelParticipant:
		return v.Date
	case *tg.ChannelParticipantSelf:
		return v.Date
	case *tg.ChannelParticipantAdmin:
		return v.Date
	case *tg.ChannelParticipantBanned:
		return v.Date
	case *tg.ChannelParticipantCreator:
		return 0
	}
	return 0
}

func (p *InfoPlugin) handleInfo(ctx *interfaces.CommandContext) error {
	msg := ctx.Message
	if msg == nil || msg.Message == nil {
		return plugin.ErrNoMessage
	}
	_ = ctx.Edit("🔍 …")
	user, id, err := p.resolveTarget(ctx)
	if err != nil {
		return ctx.Edit("❌ " + esc(err.Error()))
	}
	pr := p.load(ctx, user, id)
	return ctx.Edit(p.render(ctx, pr))
}

func (p *InfoPlugin) render(ctx *interfaces.CommandContext, pr *profile) string {
	u := pr.user
	name := strings.TrimSpace(u.FirstName + " " + u.LastName)
	unames := usernamesOf(u)
	if name == "" {
		if len(unames) > 0 {
			name = "@" + unames[0]
		} else {
			name = fmt.Sprintf("%s %d", ctx.Tlocal("用户", "User"), pr.id)
		}
	}
	unameText := ctx.Tlocal("无", "none")
	if len(unames) > 0 {
		at := make([]string, len(unames))
		for i, n := range unames {
			at[i] = "@" + n
		}
		unameText = strings.Join(at, ctx.Tlocal("、", ", "))
	}

	var tags []string
	if u.Bot {
		tags = append(tags, ctx.Tlocal("🤖 机器人", "🤖 Bot"))
	}
	if u.Verified {
		tags = append(tags, ctx.Tlocal("✅ 已验证", "✅ Verified"))
	}
	if u.Premium {
		tags = append(tags, "⭐ Premium")
	}
	if u.Scam {
		tags = append(tags, ctx.Tlocal("⚠️ 诈骗", "⚠️ Scam"))
	}
	if u.Fake {
		tags = append(tags, ctx.Tlocal("❌ 虚假", "❌ Fake"))
	}
	if u.Deleted {
		tags = append(tags, ctx.Tlocal("🗑 已注销", "🗑 Deleted"))
	}

	reg := estimateRegDate(pr.id)
	regText := ctx.Tlocal(fmt.Sprintf("%d年%d月（±2月）", reg.Year(), int(reg.Month())),
		fmt.Sprintf("%s (±2 months)", reg.Format("Jan 2006")))

	var b strings.Builder
	fmt.Fprintf(&b, "👤 **%s**\n\n", esc(name))
	fmt.Fprintf(&b, "**%s**\n", ctx.Tlocal("基本信息", "Basics"))
	fmt.Fprintf(&b, "• %s：%s\n", ctx.Tlocal("用户名", "Username"), code(unameText))
	fmt.Fprintf(&b, "• %s：%s\n", ctx.Tlocal("用户 ID", "User ID"), code(pr.id))
	fmt.Fprintf(&b, "• %s：%s\n", ctx.Tlocal("注册时间", "Registered"), code(regText))
	if !pr.joined.IsZero() {
		fmt.Fprintf(&b, "• %s：%s\n", ctx.Tlocal("入群时间", "Joined"), code(pr.joined.Format("2006-01-02 15:04")))
	}
	fmt.Fprintf(&b, "• DC：%s\n", code(pr.dc))
	if pr.hasFull {
		fmt.Fprintf(&b, "• %s：%s\n", ctx.Tlocal("共同群", "Common chats"), code(pr.common))
	}
	if len(tags) > 0 {
		fmt.Fprintf(&b, "• %s：%s\n", ctx.Tlocal("标签", "Badges"), strings.Join(tags, " "))
	}

	bio := pr.bio
	if bio == "" {
		bio = ctx.Tlocal("无简介", "no bio")
	}
	if r := []rune(bio); len(r) > 200 {
		bio = string(r[:200]) + "…"
	}
	fmt.Fprintf(&b, "\n**%s**\n%s\n", ctx.Tlocal("简介", "Bio"), code(bio))

	link1 := fmt.Sprintf("tg://user?id=%d", pr.id)
	link2 := fmt.Sprintf("https://t.me/@id%d", pr.id)
	if len(unames) > 0 {
		link2 = "https://t.me/" + unames[0]
	}
	link3 := fmt.Sprintf("tg://openmessage?user_id=%d", pr.id)
	fmt.Fprintf(&b, "\n**%s**\n", ctx.Tlocal("跳转", "Links"))
	fmt.Fprintf(&b, "• %s · %s · %s\n",
		plugin.Link(ctx.Tlocal("资料", "Profile"), link1), plugin.Link(ctx.Tlocal("聊天", "Chat"), link2), plugin.Link(ctx.Tlocal("打开消息", "Open"), link3))
	fmt.Fprintf(&b, "• %s\n• %s\n• %s\n", code(link1), code(link2), code(link3))

	// Chat and message context.
	msg := ctx.Message
	jump := ctx.Tlocal("跳转", "open")
	kind := ctx.Tlocal("私聊", "private")
	if msg.ChatID < -1000000000000 {
		kind = ctx.Tlocal("超级群/频道", "supergroup/channel")
	} else if msg.ChatID < 0 {
		kind = ctx.Tlocal("群组", "group")
	}
	base := chatLink(msg.ChatID)
	withLink := func(v int64, url string) string {
		if url == "" {
			return code(v)
		}
		return fmt.Sprintf("%s (%s)", code(v), plugin.Link(jump, url))
	}
	msgURL := ""
	if base != "" {
		msgURL = fmt.Sprintf("%s/%d", base, msg.Message.ID)
	}
	fmt.Fprintf(&b, "\n**%s**\n", ctx.Tlocal("会话", "Chat"))
	fmt.Fprintf(&b, "• %s：%s · %s\n", ctx.Tlocal("会话 ID", "Chat ID"), withLink(msg.ChatID, base), kind)
	fmt.Fprintf(&b, "• %s：%s\n", ctx.Tlocal("消息 ID", "Message ID"), withLink(int64(msg.Message.ID), msgURL))

	if msg.IsReply && msg.ReplyToID > 0 {
		replyURL := ""
		if base != "" {
			replyURL = fmt.Sprintf("%s/%d", base, msg.ReplyToID)
		}
		fmt.Fprintf(&b, "• %s：%s\n", ctx.Tlocal("被回复消息", "Replied message"), withLink(int64(msg.ReplyToID), replyURL))
		if rm, res, err := fetchMessage(ctx, msg.ReplyToID); err == nil {
			if pc, ok := rm.FromID.(*tg.PeerChannel); ok {
				fmt.Fprintf(&b, "• %s：%s\n", ctx.Tlocal("以频道身份发送", "Sent as channel"), code(-1000000000000-pc.ChannelID))
			}
			if fwd, ok := rm.GetFwdFrom(); ok {
				switch from := fwd.FromID.(type) {
				case *tg.PeerUser:
					label := fmt.Sprint(from.UserID)
					if fu := findUserInChats(res, from.UserID); fu != nil {
						label = displayName(fu)
					}
					fmt.Fprintf(&b, "• %s：%s %s\n",
						ctx.Tlocal("转发自", "Forwarded from"), plugin.Mention(label, from.UserID), code(from.UserID))
				case *tg.PeerChannel:
					fmt.Fprintf(&b, "• %s：%s\n", ctx.Tlocal("转发自频道", "Forwarded from channel"), code(-1000000000000-from.ChannelID))
				default:
					if fwd.FromName != "" {
						fmt.Fprintf(&b, "• %s：%s\n", ctx.Tlocal("转发自", "Forwarded from"), esc(fwd.FromName))
					}
				}
			}
		}
	}
	return b.String()
}
