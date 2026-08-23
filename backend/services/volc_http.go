package services

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxMediaProxyBytes = 32 << 20
const maxPlaybackProbeBytes = 256 << 10

type MediaProxyResult struct {
	Status        int    `json:"status"`
	ContentType   string `json:"contentType"`
	ContentRange  string `json:"contentRange"`
	ContentLength int    `json:"contentLength"`
	BodyB64       string `json:"bodyB64"`
}

type VolcVodProxySummary struct {
	JSON            bool `json:"json"`
	HasResult       bool `json:"has_result"`
	HasPlayAuthList bool `json:"has_play_auth_list"`
	PlayAuthCount   int  `json:"play_auth_count"`
	HasError        bool `json:"has_error"`
	BodyBytes       int  `json:"body_bytes"`
}

func SummarizeVolcVodProxyBody(body string) VolcVodProxySummary {
	summary := VolcVodProxySummary{BodyBytes: len(body)}
	var parsed struct {
		ResponseMetadata struct {
			Error json.RawMessage `json:"Error"`
		} `json:"ResponseMetadata"`
		Result json.RawMessage `json:"Result"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return summary
	}
	summary.JSON = true
	if len(parsed.Result) > 0 && string(parsed.Result) != "null" {
		summary.HasResult = true
		var result struct {
			PlayAuthInfoList []json.RawMessage `json:"PlayAuthInfoList"`
		}
		if json.Unmarshal(parsed.Result, &result) == nil {
			summary.PlayAuthCount = len(result.PlayAuthInfoList)
			summary.HasPlayAuthList = summary.PlayAuthCount > 0
		}
	}
	if len(parsed.ResponseMetadata.Error) > 0 && string(parsed.ResponseMetadata.Error) != "null" {
		summary.HasError = true
	}
	return summary
}

func volcProxyProbePath() string {
	return filepath.Join(os.TempDir(), "dedao-playback-probe.log")
}

func AppendPlaybackProbe(line string) {
	appendVolcProxyProbe(line)
}

func shouldAppendPlaybackProbe(line string) bool {
	if strings.Contains(line, `"kind":"media_proxy"`) && strings.Contains(line, `"ok":true`) {
		return false
	}
	return true
}

func shouldRotatePlaybackProbe(size int64) bool {
	return size > maxPlaybackProbeBytes
}

func appendVolcProxyProbe(line string) {
	line = strings.TrimSpace(line)
	if line == "" || !shouldAppendPlaybackProbe(line) {
		return
	}
	path := volcProxyProbePath()
	if info, err := os.Stat(path); err == nil && shouldRotatePlaybackProbe(info.Size()) {
		_ = os.Truncate(path, 0)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(line + "\n")
}

func volcQueryAction(query string) string {
	values, err := url.ParseQuery(query)
	if err != nil {
		return ""
	}
	return values.Get("Action")
}

func volcSignedGET(endpoint, query string, cookies []*http.Cookie) (io.ReadCloser, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("火山点播请求参数为空")
	}
	if strings.ContainsAny(query, "\r\n") {
		return nil, errors.New("火山点播请求参数包含非法换行")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("解析火山点播地址: %w", err)
	}
	parsed.RawQuery = query
	req, err := http.NewRequest(http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	req.URL.RawQuery = query
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Xi-DT", "web")
	for _, cookie := range cookies {
		if cookie != nil {
			req.AddCookie(cookie)
		}
	}
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, errors.New("404 NotFound")
	}
	if resp.StatusCode == http.StatusBadRequest {
		resp.Body.Close()
		return nil, errors.New("400 BadRequest")
	}
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		return nil, errors.New("401 Unauthorized")
	}
	if resp.StatusCode == 496 {
		resp.Body.Close()
		return nil, errors.New("496 NoCertificate")
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		resp.Body.Close()
		return nil, fmt.Errorf("火山点播 HTTP %d", resp.StatusCode)
	}
	return resp.Body, nil
}

var allowedMediaHostSuffixes = []string{
	"umiwi.com",
	"volces.com",
	"volccdn.com",
	"byteimg.com",
	"bytedance.com",
	"bytecdn.com",
}

func allowedMediaHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if h, _, ok := strings.Cut(host, ":"); ok {
		host = h
	}
	if host == "" {
		return false
	}
	for _, suffix := range allowedMediaHostSuffixes {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
}

func parseAllowedMediaURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("解析媒体地址: %w", err)
	}
	if parsed.Scheme == "http" {
		parsed.Scheme = "https"
	}
	if parsed.Scheme != "https" {
		return nil, errors.New("媒体地址必须是 HTTPS")
	}
	if parsed.Host == "" || !allowedMediaHost(parsed.Host) {
		return nil, errors.New("媒体域名不在允许列表中")
	}
	return parsed, nil
}

func mediaGET(target *url.URL, rangeHeader string) (*MediaProxyResult, error) {
	req, err := http.NewRequest(http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	if strings.TrimSpace(rangeHeader) != "" {
		req.Header.Set("Range", rangeHeader)
	}
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Scheme != "https" || !allowedMediaHost(req.URL.Host) {
				return errors.New("拒绝跳转到未允许的媒体域名")
			}
			if len(via) >= 5 {
				return errors.New("媒体重定向过多")
			}
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, maxMediaProxyBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if len(data) > maxMediaProxyBytes {
		return nil, errors.New("媒体分片过大")
	}
	return &MediaProxyResult{
		Status:        resp.StatusCode,
		ContentType:   resp.Header.Get("Content-Type"),
		ContentRange:  resp.Header.Get("Content-Range"),
		ContentLength: len(data),
		BodyB64:       base64.StdEncoding.EncodeToString(data),
	}, nil
}

func (s *Service) ProxyMediaGet(rawURL, rangeHeader string) (*MediaProxyResult, error) {
	parsed, err := parseAllowedMediaURL(rawURL)
	if err != nil {
		appendVolcProxyProbe(fmt.Sprintf(`{"kind":"media_proxy","ok":false,"reason":"denied"}`))
		return nil, err
	}
	result, err := mediaGET(parsed, rangeHeader)
	if err != nil {
		appendVolcProxyProbe(fmt.Sprintf(`{"kind":"media_proxy","ok":false,"host":%q}`, parsed.Host))
		return nil, err
	}
	appendVolcProxyProbe(fmt.Sprintf(
		`{"kind":"media_proxy","ok":true,"host":%q,"status":%d,"bytes":%d,"has_range":%t}`,
		parsed.Host,
		result.Status,
		len(result.BodyB64)*3/4,
		strings.TrimSpace(rangeHeader) != "",
	))
	return result, nil
}

func (s *Service) ProxyVolcVodGet(query string) (string, error) {
	body, err := s.reqVolcQuery(query)
	if err != nil {
		appendVolcProxyProbe(fmt.Sprintf(`{"kind":"volc_proxy","action":%q,"ok":false,"error":true,"query_bytes":%d}`, volcQueryAction(query), len(query)))
		return "", err
	}
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil {
		appendVolcProxyProbe(fmt.Sprintf(`{"kind":"volc_proxy","action":%q,"ok":false,"error":true,"query_bytes":%d}`, volcQueryAction(query), len(query)))
		return "", err
	}
	text := string(data)
	summary := SummarizeVolcVodProxyBody(text)
	appendVolcProxyProbe(fmt.Sprintf(
		`{"kind":"volc_proxy","action":%q,"ok":true,"json":%t,"has_result":%t,"has_play_auth_list":%t,"play_auth_count":%d,"has_error":%t,"body_bytes":%d}`,
		volcQueryAction(query),
		summary.JSON,
		summary.HasResult,
		summary.HasPlayAuthList,
		summary.PlayAuthCount,
		summary.HasError,
		summary.BodyBytes,
	))
	return text, nil
}
