package builtin

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// DCPlugin tells which data center a user, group or channel lives on. The
// DC of a profile photo is the only public hint, so targets without a
// photo cannot be located.
type DCPlugin struct{}

func NewDC() *DCPlugin { return &DCPlugin{} }

func (p *DCPlugin) Name() string        { return "dc" }
func (p *DCPlugin) Description() string { return "查看用户或群组所在数据中心" }
func (p *DCPlugin) DescEN() string      { return "Show a user's or chat's data center" }

func (p *DCPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	return mgr.RegisterCommand(&interfaces.Command{
		Name:        "dc",
		Description: "查看用户或群组所在数据中心",
		DescEN:      "Show a user's or chat's data center",
		Usage: "dc [@用户名|ID]，或回复消息\n" +
			"\n" +
			"• 不带参数  当前聊天\n" +
			"• 回复一条消息  发送者\n" +
			"• `dc @username` 或 `dc 123456`  指定用户、群组或频道\n" +
			"\n" +
			"数据中心从头像推断，没有头像的对象查不到。",
		UsageEN: "dc [@username|ID], or reply to a message\n" +
			"\n" +
			"• no argument  the current chat\n" +
			"• reply to a message  its sender\n" +
			"• `dc @username` or `dc 123456`  a user, group or channel\n" +
			"\n" +
			"The DC comes from the profile photo; targets without one cannot be located.",
		Plugin:   p.Name(),
		Category: "tools",
		Handler:  p.handle,
	})
}

func (p *DCPlugin) Start(context.Context) error { return nil }
func (p *DCPlugin) Stop(context.Context) error  { return nil }

// dcTarget is what the command resolved to.
type dcTarget struct {
	name string
	dc   int // 0 when the target has no photo
	kind string
}

func (p *DCPlugin) handle(ctx *interfaces.CommandContext) error {
	if ctx.ArgCount() > 1 {
		return ctx.Edit(errText(ctx.Tlocal("最多指定一个目标", "Give at most one target")))
	}
	t, err := p.resolve(ctx)
	if err != nil {
		return ctx.Edit(errText(esc(err.Error())))
	}
	return ctx.Edit(dcCard(ctx, t))
}

func dcCard(ctx *interfaces.CommandContext, t *dcTarget) string {
	c := newCard("🌐", ctx.Tlocal("数据中心", "Data center"))
	c.blank()
	c.rawField(t.kind, plugin.Bold(t.name))
	if t.dc == 0 {
		c.rawField("DC", esc(ctx.Tlocal("没有头像，查不到", "no profile photo, unknown")))
	} else {
		c.field("DC", fmt.Sprintf("DC%d · %s", t.dc, dcLocation(t.dc)))
	}
	return c.String()
}

// dcLocation is where each production DC sits.
func dcLocation(dc int) string {
	switch dc {
	case 1, 3:
		return "Miami"
	case 2, 4:
		return "Amsterdam"
	case 5:
		return "Singapore"
	}
	return "?"
}

func (p *DCPlugin) resolve(ctx *interfaces.CommandContext) (*dcTarget, error) {
	arg := strings.TrimSpace(ctx.GetArg(0))
	switch {
	case arg != "":
		return p.byArg(ctx, arg)
	case ctx.Message.IsReply:
		msg, res, err := fetchMessage(ctx, ctx.Message.ReplyToID)
		if err != nil {
			return nil, fmt.Errorf("%s", ctx.Tlocal("拿不到被回复的消息", "cannot fetch the replied message"))
		}
		switch from := msg.FromID.(type) {
		case *tg.PeerUser:
			return p.user(ctx, findUserInChats(res, from.UserID), from.UserID)
		case *tg.PeerChannel:
			return p.chat(ctx, plugin.ChannelChatID(from.ChannelID))
		}
		if pu, ok := msg.PeerID.(*tg.PeerUser); ok {
			return p.user(ctx, findUserInChats(res, pu.UserID), pu.UserID)
		}
		return p.chat(ctx, ctx.Message.ChatID)
	case ctx.Message.ChatID > 0:
		return p.user(ctx, nil, ctx.Message.ChatID)
	}
	return p.chat(ctx, ctx.Message.ChatID)
}

func (p *DCPlugin) byArg(ctx *interfaces.CommandContext, arg string) (*dcTarget, error) {
	if id, err := strconv.ParseInt(arg, 10, 64); err == nil {
		if id > 0 {
			return p.user(ctx, nil, id)
		}
		return p.chat(ctx, id)
	}
	name := strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(arg, "https://t.me/"), "t.me/"), "@")
	if name == "" {
		return nil, fmt.Errorf("%s", ctx.Tlocal("参数要写 @用户名 或数字 ID", "use @username or a numeric ID"))
	}
	res, err := ctx.API.ContactsResolveUsername(ctx.Context(), &tg.ContactsResolveUsernameRequest{Username: name})
	if err != nil {
		return nil, fmt.Errorf("%s", ctx.Tlocal("找不到 @"+name, "@"+name+" not found"))
	}
	for _, u := range res.Users {
		if user, ok := u.(*tg.User); ok {
			return userTarget(ctx, user), nil
		}
	}
	for _, c := range res.Chats {
		if t := chatTarget(ctx, c); t != nil {
			return t, nil
		}
	}
	return nil, fmt.Errorf("%s", ctx.Tlocal("找不到 @"+name, "@"+name+" not found"))
}

func (p *DCPlugin) user(ctx *interfaces.CommandContext, user *tg.User, id int64) (*dcTarget, error) {
	in := tg.InputUserClass(&tg.InputUser{UserID: id})
	switch {
	case user != nil && user.AccessHash != 0:
		in = user.AsInput()
	case id == ctx.SelfID:
		in = &tg.InputUserSelf{}
	default:
		if peer, err := ctx.PeerResolver.ResolveFromChatID(ctx.Context(), id); err == nil {
			if pu, ok := peer.(*tg.InputPeerUser); ok {
				in = &tg.InputUser{UserID: pu.UserID, AccessHash: pu.AccessHash}
			}
		}
	}
	users, err := ctx.API.UsersGetUsers(ctx.Context(), []tg.InputUserClass{in})
	if err == nil {
		for _, u := range users {
			if fu, ok := u.(*tg.User); ok && fu.ID == id {
				return userTarget(ctx, fu), nil
			}
		}
	}
	if user != nil {
		return userTarget(ctx, user), nil
	}
	return nil, fmt.Errorf("%s", ctx.Tlocal("找不到这个用户，先让他出现在你的聊天里", "user not found; they must appear in one of your chats first"))
}

func (p *DCPlugin) chat(ctx *interfaces.CommandContext, chatID int64) (*dcTarget, error) {
	peer, err := ctx.PeerResolver.ResolveFromChatID(ctx.Context(), chatID)
	if err != nil {
		return nil, err
	}
	var chats []tg.ChatClass
	switch v := peer.(type) {
	case *tg.InputPeerChannel:
		res, err := ctx.API.ChannelsGetChannels(ctx.Context(), []tg.InputChannelClass{&tg.InputChannel{ChannelID: v.ChannelID, AccessHash: v.AccessHash}})
		if err != nil {
			return nil, err
		}
		chats = res.GetChats()
	case *tg.InputPeerChat:
		res, err := ctx.API.MessagesGetChats(ctx.Context(), []int64{v.ChatID})
		if err != nil {
			return nil, err
		}
		chats = res.GetChats()
	case *tg.InputPeerUser:
		return p.user(ctx, nil, v.UserID)
	case *tg.InputPeerSelf:
		return p.user(ctx, nil, ctx.SelfID)
	}
	for _, c := range chats {
		if t := chatTarget(ctx, c); t != nil {
			return t, nil
		}
	}
	return nil, fmt.Errorf("%s", ctx.Tlocal("找不到这个聊天", "chat not found"))
}

func userTarget(ctx *interfaces.CommandContext, u *tg.User) *dcTarget {
	t := &dcTarget{name: strings.TrimSpace(u.FirstName + " " + u.LastName), kind: ctx.Tlocal("用户", "User")}
	if u.Bot {
		t.kind = ctx.Tlocal("机器人", "Bot")
	}
	if t.name == "" {
		t.name = fmt.Sprint(u.ID)
	}
	if ph, ok := u.Photo.(*tg.UserProfilePhoto); ok {
		t.dc = ph.DCID
	}
	return t
}

func chatTarget(ctx *interfaces.CommandContext, c tg.ChatClass) *dcTarget {
	var (
		title string
		photo tg.ChatPhotoClass
		kind  = ctx.Tlocal("群组", "Group")
	)
	switch v := c.(type) {
	case *tg.Chat:
		title, photo = v.Title, v.Photo
	case *tg.Channel:
		title, photo = v.Title, v.Photo
		if v.Broadcast {
			kind = ctx.Tlocal("频道", "Channel")
		}
	default:
		return nil
	}
	t := &dcTarget{name: title, kind: kind}
	if ph, ok := photo.(*tg.ChatPhoto); ok {
		t.dc = ph.DCID
	}
	return t
}
