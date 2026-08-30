package backend

import (
	"github.com/yann0917/dedao-gui/backend/app"
	"github.com/yann0917/dedao-gui/backend/services"
)

func (a *App) OfficialOrderList(page, limit int) (list *services.OfficialOrderList, err error) {
	list, err = app.OfficialOrderList(page, limit)
	return
}
