package request

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/go-resty/resty/v2"
)

var (
	// UserAgent UserAgent
	UserAgent               = "Mozilla/5.0 (Macintosh; Intel Mac OS X 11_1_0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/87.0.4280.88 Safari/537.36"
	errContentLengthMissing = errors.New("Content-Length is not present")
)

// HTTPClient http client
type HTTPClient struct {
	resty.Client
}

type GetOptions struct {
	Header         http.Header
	ExpectedStatus []int
}

type StatusError struct {
	Code int
	URL  string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("unexpected HTTP status %d for %s", e.Code, e.URL)
}

func (e *StatusError) AuthenticationRequired() bool {
	return e.Code == http.StatusUnauthorized || e.Code == http.StatusForbidden
}

func (e *StatusError) VerificationRequired() bool {
	return e.Code == 496
}

// NewClient new HTTPClient
func NewClient(baseURL string) *resty.Client {
	c := resty.New().SetBaseURL(baseURL)
	// c = c.SetBaseURL(baseURL)
	return c
}

// HTTPGet http get request
func HTTPGet(url string) (body []byte, err error) {
	r, err := resty.New().R().Get(url)
	if err != nil {
		return
	}

	body = r.Body()
	return
}

func newRequestWithHeader(ctx context.Context, method, rawURL string, header http.Header) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header = header.Clone()
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", UserAgent)
	}
	return req, nil
}

func contentLengthFromHeader(header http.Header) (int64, error) {
	s := header.Get("Content-Length")
	if s == "" {
		return 0, errContentLengthMissing
	}
	size, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}
	if size < 0 {
		return 0, fmt.Errorf("invalid Content-Length %q", s)
	}
	return size, nil
}

func expectedStatuses(codes []int) []int {
	if len(codes) == 0 {
		return []int{http.StatusOK}
	}
	return codes
}

func getMetadataHeaders(ctx context.Context, rawURL string, header http.Header) (http.Header, error) {
	body, response, err := GetWithOptions(ctx, rawURL, GetOptions{Header: header})
	if err != nil {
		return nil, err
	}
	defer body.Close()
	return response.Header.Clone(), nil
}

func shouldFallbackFromHead(err error) bool {
	var statusErr *StatusError
	if !errors.As(err, &statusErr) {
		return false
	}
	return statusErr.Code == http.StatusMethodNotAllowed || statusErr.Code == http.StatusNotImplemented
}

func GetWithOptions(ctx context.Context, rawURL string, opts GetOptions) (io.ReadCloser, *http.Response, error) {
	req, err := newRequestWithHeader(ctx, http.MethodGet, rawURL, opts.Header)
	if err != nil {
		return nil, nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, err
	}

	for _, code := range expectedStatuses(opts.ExpectedStatus) {
		if resp.StatusCode == code {
			return resp.Body, resp, nil
		}
	}

	_ = resp.Body.Close()
	return nil, resp, &StatusError{Code: resp.StatusCode, URL: rawURL}
}

// Get http get request
func Get(url string) (io.ReadCloser, error) {
	body, _, err := GetWithOptions(context.Background(), url, GetOptions{
		ExpectedStatus: []int{http.StatusOK},
	})
	return body, err
}

func Head(ctx context.Context, rawURL string, header http.Header) (http.Header, error) {
	req, err := newRequestWithHeader(ctx, http.MethodHead, rawURL, header)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &StatusError{Code: resp.StatusCode, URL: rawURL}
	}

	return resp.Header.Clone(), nil
}

// Headers return the HTTP Headers of the url
func Headers(url string) (http.Header, error) {
	return getMetadataHeaders(context.Background(), url, nil)
}

func SizeWithHeader(ctx context.Context, url string, header http.Header) (int64, error) {
	h, err := Head(ctx, url, header)
	if err != nil {
		return 0, err
	}
	return contentLengthFromHeader(h)
}

// Size get size of the url
func Size(url string) (int, error) {
	header, err := Head(context.Background(), url, nil)
	switch {
	case err == nil:
		size, sizeErr := contentLengthFromHeader(header)
		if sizeErr == nil {
			return checkedSize(size)
		}
		if !errors.Is(sizeErr, errContentLengthMissing) {
			return 0, sizeErr
		}
	case shouldFallbackFromHead(err):
	default:
		return 0, err
	}

	header, err = getMetadataHeaders(context.Background(), url, nil)
	if err != nil {
		return 0, err
	}
	size, err := contentLengthFromHeader(header)
	if err != nil {
		return 0, err
	}
	return checkedSize(size)
}

func checkedSize(size int64) (int, error) {
	maxInt := int64(^uint(0) >> 1)
	if size > maxInt {
		return 0, fmt.Errorf("Content-Length %d exceeds int range", size)
	}
	return int(size), nil
}
