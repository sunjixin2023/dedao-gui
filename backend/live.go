package backend

import (
	"github.com/yann0917/dedao-gui/backend/app"
	"github.com/yann0917/dedao-gui/backend/services"
)

func (a *App) LiveTabList() (list *services.LiveTabList, err error) {
	list, err = app.LiveTabList()
	return
}

func (a *App) LiveList(liveType, page, limit int) (list *services.LiveList, err error) {
	list, err = app.LiveList(liveType, page, limit)
	return
}

func (a *App) LiveCheck(aliasID, inviteCode string) (detail *services.LiveCheck, err error) {
	detail, err = app.LiveCheck(aliasID, inviteCode)
	return
}

func (a *App) LiveBase(aliasID string) (detail *services.LiveBase, err error) {
	detail, err = app.LiveBase(aliasID)
	return
}

func (a *App) LiveRoomDetail(aliasID, liveUserUnionID, token string) (detail *services.LiveRoomDetail, err error) {
	detail, err = app.LiveRoomDetail(aliasID, liveUserUnionID, token)
	return
}
