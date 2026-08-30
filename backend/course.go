package backend

import (
	"net/url"
	"strings"

	jsoniter "github.com/json-iterator/go"
	"github.com/yann0917/dedao-gui/backend/app"
	"github.com/yann0917/dedao-gui/backend/services"
)

func (a *App) CourseCategory() (list []services.CourseCategory, err error) {
	result, err := app.CourseType()
	if err != nil {
		return
	}
	list = result.Data.List
	return
}

func (a *App) GetNavbar() (data *services.NavbarData, err error) {
	data, err = app.GetNavbar()
	return
}

func (a *App) CourseList(category, order, filter string, page, limit int) (list *services.CourseList, err error) {
	list, err = app.CourseList(category, order, filter, page, limit)
	if err != nil {
		return
	}

	return
}

func (a *App) CourseGroupList(category, order, filter string, groupID, page, limit int) (list *services.CourseList, err error) {
	list, err = app.CourseGroupList(category, order, filter, groupID, page, limit)
	if err != nil {
		return
	}
	return
}

func (a *App) CourseInfo(enid string) (info *services.CourseInfo, err error) {
	info, err = app.CourseInfoByEnid(enid)
	return
}

func (a *App) OutsideDetail(enid string) (detail *services.OutsideDetail, err error) {
	detail, err = app.OutsideDetail(enid)
	if err != nil {
		return
	}
	return
}

func (a *App) ArticleList(enid, chapterID string, count, maxID int, reverse bool) (list *services.ArticleList, err error) {
	list, err = app.ArticleList(enid, chapterID, count, maxID, reverse)
	if err != nil {
		return
	}

	return
}

// ArticleDetail
// enid article enid  or odob audioAliasID, aType 1-course article, 2-odob article
func (a *App) ArticleDetail(aType int, aEnid string) (markdown string, err error) {
	detail, err := app.ArticleDetailByEnid(aType, aEnid)
	if err != nil {
		return
	}

	var content []services.Content
	err = jsoniter.UnmarshalFromString(detail.Content, &content)
	if err != nil {
		return
	}
	markdown = app.ContentsToMarkdown(content)
	return
}

func (a *App) GetArticleIntro(aType int, enid string) (intro *services.ArticleIntro, err error) {
	info, err := Instance.ArticleInfo(enid, aType)
	if err != nil {
		return
	}
	intro = &info.ArticleInfo
	return
}

func (a *App) GetVolcPlayAuthToken(mediaID, securityToken string) (info *services.MediaVolc, err error) {
	info, err = Instance.GetVolcPlayAuthToken(mediaID, securityToken)
	// fmt.Println(info)
	// fmt.Println(err)
	return
}

func (a *App) GetVolcPlayInfo(query string) (info *services.VodPlayInfoResp, err error) {
	info, err = Instance.GetVolcPlayInfo(query)
	if err != nil {
		return
	}
	return
}

func (a *App) GetMediaGateWebPlayInfo(mediaID, mediaAliasID, securityToken string) (info *services.MediaWeb, err error) {
	info, err = Instance.GetMediaGateWebPlayInfo(mediaID, mediaAliasID, securityToken)
	if err != nil {
		return
	}
	return
}

type VideoPlaybackResolve struct {
	PlayAuthToken string `json:"play_auth_token"`
	StreamURL     string `json:"stream_url"`
	Vid           string `json:"vid"`
	KeyToken      string `json:"key_token"`
}

func (a *App) ResolveVideoPlayback(mediaID, securityToken string) (info *VideoPlaybackResolve, err error) {
	info = &VideoPlaybackResolve{}
	var firstErr error

	// Prefer official web media-gate playback url first. This path is often
	// stable even when volc auth-token API has transient network failures.
	streamURL, webErr := tryPickMediaGateWebURL(mediaID, "", securityToken, 2)
	if streamURL != "" {
		info.StreamURL = streamURL
	} else if webErr != nil {
		firstErr = webErr
	}

	volc, volcErr := tryGetVolcPlayAuthToken(mediaID, securityToken, 2)
	if volcErr != nil {
		if firstErr == nil {
			firstErr = volcErr
		}
		if info.StreamURL != "" {
			return
		}
		err = firstErr
		return
	}
	if volc == nil {
		if info.StreamURL != "" {
			return
		}
		err = firstErr
		return
	}

	for _, track := range volc.Tracks {
		for _, format := range track.Formats {
			vid := strings.TrimSpace(format.VolcId)
			playAuth := strings.TrimSpace(format.VolcPlayAuthToken)
			keyToken := strings.TrimSpace(format.VolcKeyToken)
			if vid == "" || playAuth == "" {
				continue
			}

			if info.PlayAuthToken == "" {
				info.PlayAuthToken = playAuth
			}
			if info.Vid == "" {
				info.Vid = vid
			}
			if info.KeyToken == "" {
				info.KeyToken = keyToken
			}
		}
	}

	// Retry media-gate web url with alias if the first attempt did not return
	// a direct stream URL.
	if info.StreamURL == "" {
		streamURL, _ = tryPickMediaGateWebURL(mediaID, strings.TrimSpace(volc.MediaAliasId), securityToken, 2)
		if streamURL != "" {
			info.StreamURL = streamURL
			return
		}
	}

	for _, track := range volc.Tracks {
		for _, format := range track.Formats {
			vid := strings.TrimSpace(format.VolcId)
			playAuth := strings.TrimSpace(format.VolcPlayAuthToken)
			keyToken := strings.TrimSpace(format.VolcKeyToken)
			if vid == "" || playAuth == "" {
				continue
			}

			for _, query := range buildVolcPlayInfoQueries(vid, playAuth, keyToken) {
				playInfo, playErr := Instance.GetVolcPlayInfo(query)
				if playErr != nil || playInfo == nil {
					continue
				}
				if streamURL := pickVolcPlayableURL(playInfo); streamURL != "" {
					info.StreamURL = streamURL
					return
				}
			}
		}
	}

	if info.StreamURL == "" && info.PlayAuthToken == "" {
		err = firstErr
	}
	return
}

func tryGetVolcPlayAuthToken(mediaID, securityToken string, attempts int) (info *services.MediaVolc, err error) {
	if attempts < 1 {
		attempts = 1
	}
	for i := 0; i < attempts; i++ {
		info, err = Instance.GetVolcPlayAuthToken(mediaID, securityToken)
		if err == nil {
			return
		}
	}
	return
}

func tryPickMediaGateWebURL(mediaID, mediaAliasID, securityToken string, attempts int) (streamURL string, err error) {
	if attempts < 1 {
		attempts = 1
	}
	for i := 0; i < attempts; i++ {
		var info *services.MediaWeb
		info, err = Instance.GetMediaGateWebPlayInfo(mediaID, mediaAliasID, securityToken)
		if err != nil {
			continue
		}
		streamURL = pickMediaGateWebURL(info)
		if streamURL != "" {
			return
		}
	}
	return
}

func buildVolcPlayInfoQueries(vid, playAuth, keyToken string) []string {
	queries := make([]string, 0, 4)
	appendQuery := func(includeKey bool, extra bool) {
		values := url.Values{}
		values.Set("Vid", strings.TrimSpace(vid))
		values.Set("PlayAuthToken", strings.TrimSpace(playAuth))
		values.Set("Ssl", "1")
		if extra {
			values.Set("NeedHttps", "1")
			values.Set("NeedOriginal", "1")
		}
		if includeKey {
			kt := strings.TrimSpace(keyToken)
			if kt != "" {
				values.Set("KeyToken", kt)
			}
		}
		queries = append(queries, values.Encode())
	}

	appendQuery(false, true)
	appendQuery(true, true)
	appendQuery(false, false)
	appendQuery(true, false)
	return queries
}

func pickVolcPlayableURL(info *services.VodPlayInfoResp) string {
	if info == nil {
		return ""
	}
	main := strings.TrimSpace(info.Result.AdaptiveInfo.MainPlayUrl)
	backup := strings.TrimSpace(info.Result.AdaptiveInfo.BackupPlayUrl)
	if main != "" {
		return main
	}
	if backup != "" {
		return backup
	}
	for _, p := range info.Result.PlayInfoList {
		if u := strings.TrimSpace(p.MainPlayUrl); u != "" {
			return u
		}
		if u := strings.TrimSpace(p.BackupPlayUrl); u != "" {
			return u
		}
	}
	return ""
}

func pickMediaGateWebURL(info *services.MediaWeb) string {
	if info == nil {
		return ""
	}

	var bestNoDrm string
	var bestM3U8 string
	var bestMP4 string
	var bestAny string

	for _, track := range info.Tracks {
		for _, format := range track.Formats {
			url := strings.TrimSpace(format.URL)
			if url == "" {
				continue
			}
			lower := strings.ToLower(url)
			isDrm := format.DrmVersion > 0 || strings.Contains(lower, "/drm/") || strings.Contains(lower, "drm=")

			if !isDrm && strings.Contains(lower, ".mp4") {
				return url
			}
			if !isDrm && bestNoDrm == "" {
				bestNoDrm = url
			}
			if strings.Contains(lower, ".m3u8") && bestM3U8 == "" {
				bestM3U8 = url
			}
			if strings.Contains(lower, ".mp4") && bestMP4 == "" {
				bestMP4 = url
			}
			if bestAny == "" {
				bestAny = url
			}
		}
	}

	if bestNoDrm != "" {
		return bestNoDrm
	}
	if bestM3U8 != "" {
		return bestM3U8
	}
	if bestMP4 != "" {
		return bestMP4
	}
	return bestAny
}
