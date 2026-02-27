package app

import "github.com/yann0917/dedao-gui/backend/services"

func LiveTabList() (list *services.LiveTabList, err error) {
	list, err = getService().LiveTabList()
	return
}

func LiveList(liveType, page, limit int) (list *services.LiveList, err error) {
	list, err = getService().LiveList(liveType, page, limit)
	return
}

func LiveCheck(aliasID, inviteCode string) (detail *services.LiveCheck, err error) {
	detail, err = getService().LiveCheck(aliasID, inviteCode)
	return
}

func LiveBase(aliasID string) (detail *services.LiveBase, err error) {
	detail, err = getService().LiveBase(aliasID)
	return
}

func LiveRoomDetail(aliasID, liveUserUnionID, token string) (detail *services.LiveRoomDetail, err error) {
	detail, err = getService().LiveRoomDetail(aliasID, liveUserUnionID, token)
	return
}
