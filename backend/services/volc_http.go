package services

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

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

func (s *Service) ProxyVolcVodGet(query string) (string, error) {
	body, err := s.reqVolcQuery(query)
	if err != nil {
		return "", err
	}
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
