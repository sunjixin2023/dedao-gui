package app

import "github.com/yann0917/dedao-gui/backend/services"

func OfficialOrderList(page, limit int) (list *services.OfficialOrderList, err error) {
	list, err = getService().OfficialOrderList(page, limit)
	return
}
