package backend

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/yann0917/dedao-gui/backend/app"
	"github.com/yann0917/dedao-gui/backend/services"
)

func (a *App) SvgHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/svg/") {
			parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/svg/"), "/")
			if len(parts) >= 3 {
				enid := parts[0]
				chapterID := parts[1]
				pageIndex, err := strconv.Atoi(parts[2])
				if err == nil {
					pages, err := app.EbookChapterPages(enid, chapterID)
					if err == nil && pageIndex >= 0 && pageIndex < len(pages) {
						w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
						w.Header().Set("Cache-Control", "public, max-age=86400")

						// Sanitize SVG intrinsic sizes to prevent WebKit OOM
						svgStr := pages[pageIndex]
						reWidth := regexp.MustCompile(`(?i)<svg([^>]*)\bwidth="[^"]*"`)
						reHeight := regexp.MustCompile(`(?i)<svg([^>]*)\bheight="[^"]*"`)
						svgStr = reWidth.ReplaceAllString(svgStr, `<svg${1} width="100%"`)
						svgStr = reHeight.ReplaceAllString(svgStr, `<svg${1} height="auto"`)

						w.Write([]byte(svgStr))
						return
					}
				}
			}
		}
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Not found: %s", r.URL.Path)
	})
}

func (a *App) EbookInfo(enid string) (info *services.EbookDetail, err error) {
	return app.EbookDetail(enid)
}

func (a *App) EbookReadInfo(enid string) (info *services.EbookInfo, err error) {
	return app.EbookReadInfo(enid)
}

func (a *App) EbookChapterPages(enid, chapterID string) (pages []string, err error) {
	return app.EbookChapterPages(enid, chapterID)
}

func (a *App) EbookChapterPageCount(enid, chapterID string) (count int, err error) {
	pages, err := app.EbookChapterPages(enid, chapterID)
	if err != nil {
		return 0, err
	}
	return len(pages), nil
}

func (a *App) EbookChapterHtml(enid, chapterID string) (htmlContent string, err error) {
	return app.EbookChapterHtml(enid, chapterID)
}

func (a *App) EbookCommentList(enid string, page, limit int) (list *services.EbookCommentList, err error) {
	list, err = app.EbookCommentList(enid, "like_count", page, limit)
	return
}

func (a *App) EbookShelfAdd(enids []string) (resp *services.EbookShelfAddResp, err error) {
	resp, err = app.EbookShelfAdd(enids)
	return
}

func (a *App) EbookShelfRemove(enids []string) (resp *services.EbookShelfAddResp, err error) {
	resp, err = app.EbookShelfRemove(enids)
	return
}

// EbookSyncedNotes 获取官方电子书笔记列表
func (a *App) EbookSyncedNotes(enid string) (list *services.EbookNoteListResp, err error) {
	list, err = app.EbookSyncedNotes(enid)
	return
}

// EbookSyncSave 创建/更新官方电子书笔记
func (a *App) EbookSyncSave(enid, chapterID, noteLine, note, noteIDHazy string) (resp *services.EbookNoteSaveResp, err error) {
	resp, err = app.EbookSyncSave(enid, chapterID, noteLine, note, noteIDHazy)
	return
}

// EbookSyncDelete 删除官方电子书笔记
func (a *App) EbookSyncDelete(noteIDHazy string) (resp *services.NoteDestroyResp, err error) {
	resp, err = app.EbookSyncDelete(noteIDHazy)
	return
}
