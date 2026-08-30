package backend

import "github.com/yann0917/dedao-gui/backend/services"

// SearchMoreContent 补充检索得到官方内容（课程/听书/文稿/话题），用于补齐“全部内容”视图。
func (a *App) SearchMoreContent(keyword string, page, limit int) (list *services.CourseList, err error) {
	list, err = Instance.SearchMoreContent(keyword, page, limit)
	return
}
