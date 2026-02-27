package backend

import (
	"github.com/yann0917/dedao-gui/backend/app"
	"github.com/yann0917/dedao-gui/backend/services"
)

// NotesTimeline 知识城邦首页时间线列表
func (a *App) NotesTimeline(maxID string) (list *services.NotesTimeline, err error) {
	list, err = app.TopicNotesTimeline(maxID)
	return
}

// TopicAll 推荐话题列表
func (a *App) TopicAll(page, limit int) (list *services.TopicAll, err error) {
	list, err = app.TopicAll(page, limit)
	return
}

// TopicNoteDetail 话题笔记详情
func (a *App) TopicNoteDetail(id string) (list *services.TopicDetail, err error) {
	list, err = app.TopicDetail(id)
	return
}

// TopicNotesList 话题笔记列表
// isElected true 精选，false 最新
func (a *App) TopicNotesList(id string, isElected bool, page, limit int) (list *services.NotesList, err error) {
	list, err = app.TopicNotesList(id, isElected, page, limit)
	return
}

// KnowledgeCreateNote 发布知识城邦笔记（官方接口）
func (a *App) KnowledgeCreateNote(noteContent, topicIDHazy string) (resp *services.CreateNoteResp, err error) {
	resp, err = app.TopicCreateNote(noteContent, topicIDHazy)
	return
}
