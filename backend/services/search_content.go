package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	highlightTagRe = regexp.MustCompile(`</?hl>`)
	htmlTagRe      = regexp.MustCompile(`<[^>]+>`)
	spaceRe        = regexp.MustCompile(`\s+`)
)

type searchV2Response struct {
	List      []map[string]interface{} `json:"list"`
	Total     int                      `json:"total"`
	IsMore    int                      `json:"is_more"`
	Page      int                      `json:"page"`
	Size      int                      `json:"size"`
	RequestID string                   `json:"request_id"`
	Type      int                      `json:"type"`
}

func (s *Service) reqSearchV2(path string, payload map[string]interface{}) (io.ReadCloser, error) {
	resp, err := s.client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(payload).
		Post(path)
	return handleHTTPResponse(resp, err)
}

func (s *Service) searchV2(path string, payload map[string]interface{}) (resp *searchV2Response, err error) {
	body, err := s.reqSearchV2(path, payload)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	if err = handleJSONParse(body, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *Service) SearchMoreContent(keyword string, page, limit int) (list *CourseList, err error) {
	key := strings.TrimSpace(keyword)
	if key == "" {
		return &CourseList{List: []Course{}}, nil
	}
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}

	requestID := fmt.Sprintf("dedao-gui-%d", time.Now().UnixNano())
	result := &CourseList{List: make([]Course, 0, limit*4)}
	seen := make(map[string]struct{})
	var firstErr error

	addCourses := func(items []Course) {
		for _, item := range items {
			key := strings.ToLower(strings.TrimSpace(item.Enid))
			if key == "" {
				key = strings.ToLower(strings.TrimSpace(item.DdURL))
			}
			if key == "" {
				key = fmt.Sprintf("%d|%d|%s", item.Type, item.ID, strings.ToLower(strings.TrimSpace(item.Title)))
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			result.List = append(result.List, item)
		}
	}

	run := func(path string, fallbackType int, payload map[string]interface{}) {
		resp, e := s.searchV2(path, payload)
		if e != nil {
			if firstErr == nil {
				firstErr = e
			}
			return
		}
		addCourses(convertSearchItems(resp.List, fallbackType))
	}

	basePayload := map[string]interface{}{
		"content":    key,
		"hl_num":     2,
		"page":       page,
		"request_id": requestID,
		"size":       limit,
	}

	classPayload := cloneMap(basePayload)
	classPayload["type"] = 66
	run("/api/search/v2/pc/searchclass", 66, classPayload)

	audioPayload := cloneMap(basePayload)
	audioPayload["type"] = 13
	run("/api/search/v2/pc/searchaudio", 13, audioPayload)

	articlePayload := cloneMap(basePayload)
	articlePayload["type"] = 65
	run("/api/search/v2/pc/searchallarticle", 65, articlePayload)

	run("/api/search/v2/pc/searchtopic", 6301, cloneMap(basePayload))

	result.Total = len(result.List)
	result.IsMore = 0

	if len(result.List) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return result, nil
}

func cloneMap(src map[string]interface{}) map[string]interface{} {
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func convertSearchItems(items []map[string]interface{}, fallbackType int) []Course {
	out := make([]Course, 0, len(items))
	for _, item := range items {
		title := cleanSnippet(toString(item["title"]))
		if title == "" {
			continue
		}

		content := cleanSnippet(toString(item["content"]))
		detail := nestedMap(item, "detail")
		extra := nestedMap(item, "extra")
		typeVal := toInt(item["type"])
		if typeVal <= 0 {
			typeVal = fallbackType
		}

		ddURL := normalizeDDURL(toString(item["url"]))
		icon := normalizeDDURL(firstNonEmpty(
			toString(item["image"]),
			toString(extra["image"]),
			toString(extra["img"]),
			toString(detail["index_img"]),
			toString(detail["IndexImg"]),
			toString(detail["logo"]),
			toString(detail["Logo"]),
		))
		intro := firstNonEmpty(
			content,
			cleanSnippet(toString(detail["intro"])),
			cleanSnippet(toString(detail["Intro"])),
			cleanSnippet(toString(detail["summary"])),
			cleanSnippet(toString(detail["Summary"])),
		)
		author := firstNonEmpty(
			cleanSnippet(toString(item["author"])),
			cleanSnippet(toString(extra["author"])),
			cleanSnippet(nestedString(detail, "lecturers_info", "name")),
			cleanSnippet(nestedString(detail, "author_list", "0")),
		)
		enid := firstNonEmpty(
			toString(extra["enid"]),
			toString(detail["enid"]),
			queryParam(ddURL, "enid"),
			queryParam(ddURL, "id"),
			queryParam(ddURL, "topic_id_hazy"),
		)

		out = append(out, Course{
			Enid:   strings.TrimSpace(enid),
			ID:     toInt(item["id"]),
			Type:   typeVal,
			Title:  title,
			Intro:  intro,
			Author: author,
			Icon:   icon,
			DdURL:  ddURL,
		})
	}
	return out
}

func nestedMap(root map[string]interface{}, key string) map[string]interface{} {
	if root == nil {
		return nil
	}
	value, ok := root[key]
	if !ok {
		return nil
	}
	result, ok := value.(map[string]interface{})
	if !ok {
		return nil
	}
	return result
}

func nestedString(root map[string]interface{}, keys ...string) string {
	if root == nil || len(keys) == 0 {
		return ""
	}
	var current interface{} = root
	for _, key := range keys {
		switch typed := current.(type) {
		case map[string]interface{}:
			current = typed[key]
		case []interface{}:
			idx, err := strconv.Atoi(key)
			if err != nil || idx < 0 || idx >= len(typed) {
				return ""
			}
			current = typed[idx]
		default:
			return ""
		}
	}
	return toString(current)
}

func toString(v interface{}) string {
	switch value := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(value)
	case float64:
		if value == float64(int64(value)) {
			return strconv.FormatInt(int64(value), 10)
		}
		return strconv.FormatFloat(value, 'f', -1, 64)
	case int:
		return strconv.Itoa(value)
	case int64:
		return strconv.FormatInt(value, 10)
	case json.Number:
		return value.String()
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func toInt(v interface{}) int {
	switch value := v.(type) {
	case nil:
		return 0
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case string:
		num, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return 0
		}
		return num
	case json.Number:
		num, err := value.Int64()
		if err != nil {
			return 0
		}
		return int(num)
	default:
		return 0
	}
}

func cleanSnippet(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return ""
	}
	text = highlightTagRe.ReplaceAllString(text, "")
	text = htmlTagRe.ReplaceAllString(text, "")
	text = spaceRe.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func normalizeDDURL(raw string) string {
	urlText := strings.TrimSpace(raw)
	if urlText == "" {
		return ""
	}
	if strings.HasPrefix(urlText, "http://") || strings.HasPrefix(urlText, "https://") {
		return urlText
	}
	if strings.HasPrefix(urlText, "//") {
		return "https:" + urlText
	}
	if strings.HasPrefix(urlText, "/") {
		return "https://www.dedao.cn" + urlText
	}
	return "https://www.dedao.cn/" + strings.TrimPrefix(urlText, "/")
}

func queryParam(rawURL, key string) string {
	urlText := strings.TrimSpace(rawURL)
	if urlText == "" || strings.TrimSpace(key) == "" {
		return ""
	}
	parsed, err := url.Parse(urlText)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.Query().Get(key))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
