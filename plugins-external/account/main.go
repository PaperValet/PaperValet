package main

import (
	"context"
	"fmt"

	"github.com/gotd/td/tg"

	"github.com/TiaraBasori/PaperValet/pkg/plugin"
)

// AccountPlugin manages the userbot's own account profile.
type AccountPlugin struct{}

func New() (plugin.Plugin, error) {
	return &AccountPlugin{}, nil
}

var Metadata = &plugin.PluginMetadata{
	Name:        "account",
	Description: "账号资料管理（用户名/昵称/简介/头像）",
	Version:     "1.0.0",
	Author:      "PaperValet",
	MinVersion:  "0.1.0",
}

func (p *AccountPlugin) Name() string { return "account" }
func (p *AccountPlugin) Description() string {
	return "账号资料管理（用户名/昵称/简介/头像）"
}

func (p *AccountPlugin) Init(_ context.Context, mgr plugin.Manager) error {
	cmds := []*plugin.Command{
		{Name: "username", Description: "设置/清空用户名", Usage: "username [新用户名]", Plugin: p.Name(), Category: "account", OwnerOnly: true, Handler: p.handleUsername},
		{Name: "name", Description: "设置显示昵称", Usage: "name <名> [姓]", Plugin: p.Name(), Category: "account", OwnerOnly: true, Handler: p.handleName},
		{Name: "bio", Description: "设置个人简介", Usage: "bio <内容>", Plugin: p.Name(), Category: "account", OwnerOnly: true, Handler: p.handleBio},
		{Name: "rmpfp", Description: "删除头像", Usage: "rmpfp [数量|all]", Plugin: p.Name(), Category: "account", OwnerOnly: true, Handler: p.handleRmPfp},
	}
	for _, cmd := range cmds {
		if err := mgr.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *AccountPlugin) Start(_ context.Context) error { return nil }
func (p *AccountPlugin) Stop(_ context.Context) error  { return nil }

func (p *AccountPlugin) handleUsername(ctx *plugin.CommandContext) error {
	newName := ""
	if ctx.ArgCount() > 0 {
		newName = ctx.GetArg(0)
	}
	_, err := ctx.API.AccountUpdateUsername(ctx.Context(), newName)
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 设置失败: %v", err))
	}
	if newName == "" {
		return ctx.Edit("✅ 用户名已清空")
	}
	return ctx.Edit(fmt.Sprintf("✅ 用户名已设为 @%s", newName))
}

func (p *AccountPlugin) handleName(ctx *plugin.CommandContext) error {
	if ctx.ArgCount() == 0 {
		return ctx.Edit("用法: name <名> [姓]")
	}
	first := ctx.GetArg(0)
	last := ""
	if ctx.ArgCount() > 1 {
		last = ctx.GetArg(1)
	}
	_, err := ctx.API.AccountUpdateProfile(ctx.Context(), &tg.AccountUpdateProfileRequest{
		FirstName: first,
		LastName:  last,
	})
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 设置失败: %v", err))
	}
	if last != "" {
		return ctx.Edit(fmt.Sprintf("✅ 昵称已设为 %s %s", first, last))
	}
	return ctx.Edit(fmt.Sprintf("✅ 昵称已设为 %s", first))
}

func (p *AccountPlugin) handleBio(ctx *plugin.CommandContext) error {
	bio := ctx.RawArgs
	_, err := ctx.API.AccountUpdateProfile(ctx.Context(), &tg.AccountUpdateProfileRequest{About: bio})
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 设置失败: %v", err))
	}
	if bio == "" {
		return ctx.Edit("✅ 简介已清空")
	}
	return ctx.Edit(fmt.Sprintf("✅ 简介已设为: %s", bio))
}

func (p *AccountPlugin) handleRmPfp(ctx *plugin.CommandContext) error {
	limit := 1
	if ctx.ArgCount() > 0 {
		if ctx.GetArg(0) == "all" {
			limit = 0 // 0 = no limit in photos.getUserPhotos
		} else if n, err := parseInt(ctx.GetArg(0)); err == nil && n > 0 {
			limit = n
		}
	}

	photos, err := ctx.API.PhotosGetUserPhotos(ctx.Context(), &tg.PhotosGetUserPhotosRequest{
		UserID: &tg.InputUserSelf{},
		Limit:  limit,
	})
	if err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 获取头像失败: %v", err))
	}

	var inputPhotos []tg.InputPhotoClass
	for _, ph := range photos.GetPhotos() {
		if photo, ok := ph.(*tg.Photo); ok {
			inputPhotos = append(inputPhotos, &tg.InputPhoto{
				ID:            photo.ID,
				AccessHash:    photo.AccessHash,
				FileReference: photo.FileReference,
			})
		}
	}
	if len(inputPhotos) == 0 {
		return ctx.Edit("没有头像可删除")
	}
	if _, err := ctx.API.PhotosDeletePhotos(ctx.Context(), inputPhotos); err != nil {
		return ctx.Edit(fmt.Sprintf("❌ 删除失败: %v", err))
	}
	return ctx.Edit(fmt.Sprintf("✅ 已删除 %d 张头像", len(inputPhotos)))
}

func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
