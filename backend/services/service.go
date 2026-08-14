package services

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/yann0917/dedao-gui/backend/utils"
)

var (
	dedaoCommURL = &url.URL{
		Scheme: "https",
		Host:   "dedao.cn",
	}
	baseURL   = "https://www.dedao.cn"
	UserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/106.0.0.0 Safari/537.36"
)

// Response dedao success response
type Response struct {
	H respH `json:"h"`
	C respC `json:"c"`
}

type respH struct {
	C   int    `json:"c"`
	E   string `json:"e"`
	S   int    `json:"s"`
	T   int    `json:"t"`
	Apm string `json:"apm"`
}

// respC response content
type respC []byte

func (r *respC) UnmarshalJSON(data []byte) error {
	*r = data

	return nil
}

func (r respC) String() string {
	return string(r)
}

// Service dedao service
type Service struct {
	client *resty.Client
}

// CookieOptions dedao cookie options
type CookieOptions struct {
	GAT           string `json:"gat"`
	ISID          string `json:"isid"`
	Iget          string `json:"iget"`
	Token         string `json:"token"`
	CsrfToken     string `json:"csrfToken"`
	GuardDeviceID string `json:"_guard_device_id"`
	SID           string `json:"_sid"`
	AcwTc         string `json:"acw_tc"`
	AliyungfTc    string `json:"aliyungf_tc"`
}

type cookieFieldSpec struct {
	fieldName string
	wireName  string
	domain    string
}

// NewService new service
func NewService(co *CookieOptions) *Service {
	if co == nil {
		co = &CookieOptions{}
	}

	var cookies []*http.Cookie
	for _, spec := range cookieFieldSpecs() {
		if value := cookieFieldValue(co, spec); value != "" {
			cookies = append(cookies, &http.Cookie{
				Name:   spec.wireName,
				Value:  value,
				Domain: spec.domain,
			})
		}
	}

	client := resty.New()
	client.SetDebug(false)
	client.SetBaseURL(baseURL).
		SetCookies(cookies).
		SetHeaderVerbatim("User-Agent", UserAgent).
		SetHeaderVerbatim("Xi-DT", "web")

	if co.CsrfToken != "" {
		client.SetHeaderVerbatim("Xi-Csrf-Token", co.CsrfToken)
	}
	return &Service{client: client}
}

func (r *Response) isSuccess() bool {
	return r.H.C == 0
}

func handleHTTPResponse(resp *resty.Response, err error) (io.ReadCloser, error) {
	if err != nil {
		return nil, err
	}

	if resp.StatusCode() == http.StatusNotFound {
		return nil, errors.New("404 NotFound")
	}
	if resp.StatusCode() == http.StatusBadRequest {
		return nil, errors.New("400 BadRequest")
	}
	if resp.StatusCode() == http.StatusUnauthorized {
		return nil, errors.New("401 Unauthorized")
	}
	if resp.StatusCode() == 496 {
		return nil, errors.New("496 NoCertificate")
	}

	data := resp.Body()
	reader := bytes.NewReader(data)
	result := io.NopCloser(reader)
	return result, nil
}

func handleJSONParse(reader io.Reader, v interface{}) error {
	result := new(Response)

	err := utils.UnmarshalReader(reader, &result)
	if err != nil {
		fmt.Printf("err1: %s \n", err.Error())
		return err
	}
	// fmt.Printf("result.C:=%#v", result.C)
	if !result.isSuccess() {
		// 未登录或者登录凭证无效
		err = errors.New("服务异常，请稍后重试。errMsg:" + result.H.E)
		return err
	}
	err = utils.UnmarshalJSON(result.C, v)
	if err != nil {
		fmt.Printf("err2: %s", err.Error())
		return err
	}

	return nil
}

// ParseCookies parse cookie string to cookie options
func ParseCookies(cookie string, v interface{}) (err error) {
	if cookie == "" {
		return errors.New("cookie is empty")
	}
	parsedExact, parsedInsensitive := parseCookiePairs(cookie)

	switch target := v.(type) {
	case *CookieOptions:
		if target == nil {
			return errors.New("v must be *CookieOptions or *map[string]string")
		}

		value := reflect.ValueOf(target).Elem()
		for _, spec := range cookieFieldSpecs() {
			if parsedValue, ok := parsedInsensitive[strings.ToLower(spec.wireName)]; ok {
				field := value.FieldByName(spec.fieldName)
				if field.IsValid() && field.CanSet() && field.Kind() == reflect.String {
					field.SetString(parsedValue)
				}
			}
		}
		return nil
	case *map[string]string:
		if target == nil {
			return errors.New("v must be *CookieOptions or *map[string]string")
		}
		result := make(map[string]string, len(parsedExact))
		for key, value := range parsedExact {
			result[key] = value
		}
		*target = result
		return nil
	default:
		return errors.New("v must be *CookieOptions or *map[string]string")
	}
}

func parseCookiePairs(cookie string) (map[string]string, map[string]string) {
	list := strings.Split(cookie, ";")
	parsedExact := make(map[string]string, len(list))
	parsedInsensitive := make(map[string]string, len(list))
	lastSeenKeyByLower := make(map[string]string, len(list))
	for _, item := range list {
		name, value, found := strings.Cut(strings.TrimSpace(item), "=")
		if !found || name == "" || value == "" {
			continue
		}

		lowerName := strings.ToLower(name)
		if previousKey, ok := lastSeenKeyByLower[lowerName]; ok && previousKey != name {
			delete(parsedExact, previousKey)
		}
		lastSeenKeyByLower[lowerName] = name
		parsedExact[name] = value
		parsedInsensitive[lowerName] = value
	}
	return parsedExact, parsedInsensitive
}

// CookieHeaderFromSetCookies converts Set-Cookie headers to a Cookie header.
func CookieHeaderFromSetCookies(values []string) (string, error) {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		response := &http.Response{Header: make(http.Header)}
		response.Header.Add("Set-Cookie", value)
		cookies := response.Cookies()
		if len(cookies) != 1 || cookies[0] == nil || strings.TrimSpace(cookies[0].Name) == "" {
			return "", errors.New("invalid Set-Cookie header")
		}
	}

	pairs := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		response := &http.Response{Header: make(http.Header)}
		response.Header.Add("Set-Cookie", value)
		cookie := response.Cookies()[0]
		spec, ok := cookieFieldSpecByName(cookie.Name)
		if !ok {
			continue
		}
		pairs = append(pairs, spec.wireName+"="+cookie.Value)
	}
	return strings.Join(pairs, "; "), nil
}

// MergeCookieHeaders applies updates over existing cookies and serializes known fields in stable order.
func MergeCookieHeaders(existing, update string) string {
	merged := make(map[string]string)
	mergeCookieHeaderInto(merged, existing)
	mergeCookieHeaderInto(merged, update)

	pairs := make([]string, 0, len(cookieOptionNames()))
	for _, name := range cookieOptionNames() {
		if value := merged[strings.ToLower(name)]; value != "" {
			pairs = append(pairs, name+"="+value)
		}
	}
	return strings.Join(pairs, "; ")
}

// CookieMap returns the configured cookies as name/value pairs.
func CookieMap(options *CookieOptions) map[string]string {
	result := make(map[string]string)
	if options == nil {
		return result
	}
	for _, spec := range cookieFieldSpecs() {
		if value := cookieFieldValue(options, spec); value != "" {
			result[spec.wireName] = value
		}
	}
	return result
}

func mergeCookieHeaderInto(dest map[string]string, header string) {
	if strings.TrimSpace(header) == "" {
		return
	}

	var parsed map[string]string
	if err := ParseCookies(header, &parsed); err != nil {
		return
	}
	for key, value := range parsed {
		spec, ok := cookieFieldSpecByName(key)
		if !ok {
			continue
		}
		dest[strings.ToLower(spec.wireName)] = value
	}
}

func cookieOptionNames() []string {
	names := make([]string, 0, len(cookieFieldSpecs()))
	for _, spec := range cookieFieldSpecs() {
		names = append(names, spec.wireName)
	}
	return names
}

func cookieFieldSpecs() []cookieFieldSpec {
	return []cookieFieldSpec{
		{fieldName: "GAT", wireName: "GAT", domain: "." + dedaoCommURL.Host},
		{fieldName: "ISID", wireName: "ISID", domain: "." + dedaoCommURL.Host},
		{fieldName: "Iget", wireName: "iget", domain: "www." + dedaoCommURL.Host},
		{fieldName: "Token", wireName: "token", domain: "www." + dedaoCommURL.Host},
		{fieldName: "CsrfToken", wireName: "csrfToken", domain: "www." + dedaoCommURL.Host},
		{fieldName: "GuardDeviceID", wireName: "_guard_device_id", domain: "www." + dedaoCommURL.Host},
		{fieldName: "SID", wireName: "_sid", domain: "www." + dedaoCommURL.Host},
		{fieldName: "AcwTc", wireName: "acw_tc", domain: "www." + dedaoCommURL.Host},
		{fieldName: "AliyungfTc", wireName: "aliyungf_tc", domain: "www." + dedaoCommURL.Host},
	}
}

func cookieFieldSpecByName(name string) (cookieFieldSpec, bool) {
	needle := strings.ToLower(strings.TrimSpace(name))
	for _, spec := range cookieFieldSpecs() {
		if strings.ToLower(spec.wireName) == needle {
			return spec, true
		}
	}
	return cookieFieldSpec{}, false
}

func cookieFieldValue(options *CookieOptions, spec cookieFieldSpec) string {
	if options == nil {
		return ""
	}
	value := reflect.ValueOf(options).Elem()
	field := value.FieldByName(spec.fieldName)
	if !field.IsValid() || field.Kind() != reflect.String {
		return ""
	}
	return field.String()
}
