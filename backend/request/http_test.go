package request

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type trackingReadCloser struct {
	reader io.Reader
	closed bool
}

func (t *trackingReadCloser) Read(p []byte) (int, error) {
	return t.reader.Read(p)
}

func (t *trackingReadCloser) Close() error {
	t.closed = true
	return nil
}

func setDefaultClient(t *testing.T, client *http.Client) {
	t.Helper()
	oldClient := http.DefaultClient
	http.DefaultClient = client
	t.Cleanup(func() {
		http.DefaultClient = oldClient
	})
}

func TestGetWithOptionsSendsRangeHeader(t *testing.T) {
	errCh := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Header.Get("Range") != "bytes=3-5":
			errCh <- errors.New("unexpected Range header: " + r.Header.Get("Range"))
			http.Error(w, "bad range", http.StatusBadRequest)
			return
		case r.Header.Get("User-Agent") != UserAgent:
			errCh <- errors.New("unexpected User-Agent header: " + r.Header.Get("User-Agent"))
			http.Error(w, "bad user agent", http.StatusBadRequest)
			return
		case r.Header.Get("X-Test") != "range":
			errCh <- errors.New("unexpected X-Test header: " + r.Header.Get("X-Test"))
			http.Error(w, "bad header", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Range", "bytes 3-5/10")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("345"))
		errCh <- nil
	}))
	defer server.Close()

	header := make(http.Header)
	header.Set("Range", "bytes=3-5")
	header.Set("X-Test", "range")
	body, response, err := GetWithOptions(context.Background(), server.URL, GetOptions{
		Header:         header,
		ExpectedStatus: []int{http.StatusPartialContent},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()

	if got := header.Get("User-Agent"); got != "" {
		t.Fatalf("caller header mutated User-Agent = %q, want empty", got)
	}
	if got := header.Get("Range"); got != "bytes=3-5" {
		t.Fatalf("caller header mutated Range = %q, want %q", got, "bytes=3-5")
	}
	if response.StatusCode != http.StatusPartialContent {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusPartialContent)
	}
	if got, err := io.ReadAll(body); err != nil {
		t.Fatalf("read body: %v", err)
	} else if string(got) != "345" {
		t.Fatalf("body = %q, want %q", got, "345")
	}
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
}

func TestGetWithOptionsDefaultsExpectedStatusToOK(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts GetOptions
	}{
		{name: "nil"},
		{name: "empty", opts: GetOptions{ExpectedStatus: []int{}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bodyReader := &trackingReadCloser{reader: strings.NewReader("ok")}
			setDefaultClient(t, &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if got := req.Header.Get("User-Agent"); got != UserAgent {
						return nil, errors.New("unexpected User-Agent header: " + got)
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       bodyReader,
						Request:    req,
					}, nil
				}),
			})

			body, response, err := GetWithOptions(context.Background(), "https://example.test/default-ok", tc.opts)
			if err != nil {
				t.Fatal(err)
			}
			defer body.Close()

			if response.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
			}
			if got, err := io.ReadAll(body); err != nil {
				t.Fatalf("read body: %v", err)
			} else if string(got) != "ok" {
				t.Fatalf("body = %q, want %q", got, "ok")
			}
			if bodyReader.closed {
				t.Fatal("successful body should remain open until caller closes it")
			}
		})
	}
}

func TestGetWithOptionsRejectsUnexpectedStatus(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		status                 int
		wantAuthentication     bool
		wantVerificationNeeded bool
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, wantAuthentication: true},
		{name: "forbidden", status: http.StatusForbidden, wantAuthentication: true},
		{name: "verification", status: 496, wantVerificationNeeded: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte("blocked"))
			}))
			defer server.Close()

			body, response, err := GetWithOptions(context.Background(), server.URL, GetOptions{
				ExpectedStatus: []int{http.StatusOK},
			})
			if body != nil {
				t.Fatal("expected nil body")
			}
			if response == nil {
				t.Fatal("expected response")
			}

			var statusErr *StatusError
			if !errors.As(err, &statusErr) {
				t.Fatalf("error = %T, want *StatusError", err)
			}
			if statusErr.Code != tc.status {
				t.Fatalf("status error code = %d, want %d", statusErr.Code, tc.status)
			}
			if statusErr.URL != server.URL {
				t.Fatalf("status error URL = %q, want %q", statusErr.URL, server.URL)
			}
			if got := statusErr.Error(); got != "unexpected HTTP status "+strconv.Itoa(tc.status)+" for "+server.URL {
				t.Fatalf("error string = %q", got)
			}
			if statusErr.AuthenticationRequired() != tc.wantAuthentication {
				t.Fatalf("AuthenticationRequired = %v, want %v", statusErr.AuthenticationRequired(), tc.wantAuthentication)
			}
			if statusErr.VerificationRequired() != tc.wantVerificationNeeded {
				t.Fatalf("VerificationRequired = %v, want %v", statusErr.VerificationRequired(), tc.wantVerificationNeeded)
			}
			if _, readErr := io.ReadAll(response.Body); readErr == nil {
				t.Fatal("expected closed response body after unexpected status")
			}
		})
	}
}

func TestGetWithOptionsHonoursCancellation(t *testing.T) {
	started := make(chan struct{}, 1)
	handlerErr := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		<-r.Context().Done()
		handlerErr <- r.Context().Err()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type result struct {
		body     io.ReadCloser
		response *http.Response
		err      error
	}
	resultCh := make(chan result, 1)
	go func() {
		body, response, err := GetWithOptions(ctx, server.URL, GetOptions{
			ExpectedStatus: []int{http.StatusOK},
		})
		resultCh <- result{body: body, response: response, err: err}
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not observe request start")
	}

	cancel()

	select {
	case res := <-resultCh:
		if res.body != nil {
			t.Fatal("expected nil body")
		}
		if res.response != nil {
			t.Fatal("expected nil response")
		}
		if !errors.Is(res.err, context.Canceled) {
			t.Fatalf("error = %v, want %v", res.err, context.Canceled)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("GetWithOptions did not return after cancellation")
	}

	select {
	case err := <-handlerErr:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("handler context error = %v, want %v", err, context.Canceled)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not observe cancellation")
	}
}

func TestHeadRejectsUnexpectedStatus(t *testing.T) {
	oldClient := http.DefaultClient
	defer func() {
		http.DefaultClient = oldClient
	}()

	body := &trackingReadCloser{reader: strings.NewReader("denied")}
	http.DefaultClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodHead {
				return nil, errors.New("unexpected method: " + req.Method)
			}
			if req.Header.Get("User-Agent") != UserAgent {
				return nil, errors.New("unexpected User-Agent header: " + req.Header.Get("User-Agent"))
			}
			return &http.Response{
				StatusCode: http.StatusTeapot,
				Status:     "418 I'm a teapot",
				Header:     make(http.Header),
				Body:       body,
				Request:    req,
			}, nil
		}),
	}

	header := make(http.Header)
	header.Set("X-Test", "head")
	got, err := Head(context.Background(), "https://example.test/head", header)
	if got != nil {
		t.Fatalf("headers = %v, want nil", got)
	}
	if header.Get("User-Agent") != "" {
		t.Fatalf("caller header mutated User-Agent = %q, want empty", header.Get("User-Agent"))
	}
	var statusErr *StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("error = %T, want *StatusError", err)
	}
	if statusErr.Code != http.StatusTeapot {
		t.Fatalf("status error code = %d, want %d", statusErr.Code, http.StatusTeapot)
	}
	if statusErr.URL != "https://example.test/head" {
		t.Fatalf("status error URL = %q", statusErr.URL)
	}
	if !body.closed {
		t.Fatal("expected response body to be closed")
	}
}

func TestSizeWithHeaderUsesHEAD(t *testing.T) {
	errCh := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			errCh <- errors.New("unexpected method: " + r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if got := r.Header.Get("X-Test"); got != "head" {
			errCh <- errors.New("unexpected X-Test header: " + got)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if got := r.Header.Get("User-Agent"); got != UserAgent {
			errCh <- errors.New("unexpected User-Agent header: " + got)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Length", "123")
		w.WriteHeader(http.StatusOK)
		errCh <- nil
	}))
	defer server.Close()

	header := make(http.Header)
	header.Set("X-Test", "head")
	size, err := SizeWithHeader(context.Background(), server.URL, header)
	if err != nil {
		t.Fatal(err)
	}
	if size != 123 {
		t.Fatalf("size = %d, want 123", size)
	}
	if got := header.Get("User-Agent"); got != "" {
		t.Fatalf("caller header mutated User-Agent = %q, want empty", got)
	}
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
}

func TestSizeWithHeaderRejectsBadContentLength(t *testing.T) {
	for _, tc := range []struct {
		name    string
		value   string
		wantErr string
	}{
		{name: "missing", value: "", wantErr: "Content-Length is not present"},
		{name: "invalid", value: "abc", wantErr: "invalid syntax"},
		{name: "negative", value: "-1", wantErr: "invalid Content-Length \"-1\""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			oldClient := http.DefaultClient
			defer func() {
				http.DefaultClient = oldClient
			}()

			http.DefaultClient = &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					header := make(http.Header)
					if tc.value != "" {
						header.Set("Content-Length", tc.value)
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     header,
						Body:       io.NopCloser(strings.NewReader("")),
						Request:    req,
					}, nil
				}),
			}

			size, err := SizeWithHeader(context.Background(), "https://example.test/content-length", nil)
			if err == nil {
				t.Fatalf("size = %d, want error", size)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestHeadersUsesGETAndClosesBody(t *testing.T) {
	var getCount int
	var headCount int
	getBody := &trackingReadCloser{reader: strings.NewReader("payload")}

	setDefaultClient(t, &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Header.Get("User-Agent") != UserAgent {
				return nil, errors.New("unexpected User-Agent header: " + req.Header.Get("User-Agent"))
			}
			switch req.Method {
			case http.MethodGet:
				getCount++
				return &http.Response{
					StatusCode: http.StatusOK,
					Header: http.Header{
						"Content-Length": []string{"77"},
						"X-Test":         []string{"value"},
					},
					Body:    getBody,
					Request: req,
				}, nil
			case http.MethodHead:
				headCount++
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       &trackingReadCloser{reader: strings.NewReader("")},
					Request:    req,
				}, nil
			default:
				return nil, errors.New("unexpected method: " + req.Method)
			}
		}),
	})

	header, err := Headers("https://example.test/headers")
	if err != nil {
		t.Fatal(err)
	}
	if header.Get("X-Test") != "value" {
		t.Fatalf("X-Test = %q, want %q", header.Get("X-Test"), "value")
	}
	if getCount != 1 {
		t.Fatalf("GET count = %d, want 1", getCount)
	}
	if headCount != 0 {
		t.Fatalf("HEAD count = %d, want 0", headCount)
	}
	if !getBody.closed {
		t.Fatal("expected GET metadata body to be closed")
	}
}

func TestSizeOverflow(t *testing.T) {
	if strconv.IntSize != 32 {
		t.Skip("skipping on 64-bit: no int64 Content-Length can overflow int")
	}

	oldClient := http.DefaultClient
	defer func() {
		http.DefaultClient = oldClient
	}()

	http.DefaultClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Length": []string{"2147483648"},
				},
				Body:    io.NopCloser(strings.NewReader("")),
				Request: req,
			}, nil
		}),
	}

	size, err := Size("https://example.test/overflow")
	if err == nil {
		t.Fatalf("size = %d, want overflow error", size)
	}
	if !strings.Contains(err.Error(), "exceeds int range") {
		t.Fatalf("error = %q, want overflow evidence", err.Error())
	}
}

func TestSizeFallsBackToGETWhenHEADCannotProvideLength(t *testing.T) {
	for _, tc := range []struct {
		name       string
		headStatus int
		headHeader http.Header
	}{
		{name: "method not allowed", headStatus: http.StatusMethodNotAllowed, headHeader: make(http.Header)},
		{name: "not implemented", headStatus: http.StatusNotImplemented, headHeader: make(http.Header)},
		{name: "missing content length", headStatus: http.StatusOK, headHeader: make(http.Header)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var headCount int
			var getCount int
			headBody := &trackingReadCloser{reader: strings.NewReader("")}
			getBody := &trackingReadCloser{reader: strings.NewReader("payload")}

			setDefaultClient(t, &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.Header.Get("User-Agent") != UserAgent {
						return nil, errors.New("unexpected User-Agent header: " + req.Header.Get("User-Agent"))
					}
					switch req.Method {
					case http.MethodHead:
						headCount++
						return &http.Response{
							StatusCode: tc.headStatus,
							Header:     tc.headHeader.Clone(),
							Body:       headBody,
							Request:    req,
						}, nil
					case http.MethodGet:
						getCount++
						return &http.Response{
							StatusCode: http.StatusOK,
							Header: http.Header{
								"Content-Length": []string{"123"},
							},
							Body:    getBody,
							Request: req,
						}, nil
					default:
						return nil, errors.New("unexpected method: " + req.Method)
					}
				}),
			})

			size, err := Size("https://example.test/fallback")
			if err != nil {
				t.Fatal(err)
			}
			if size != 123 {
				t.Fatalf("size = %d, want 123", size)
			}
			if headCount != 1 {
				t.Fatalf("HEAD count = %d, want 1", headCount)
			}
			if getCount != 1 {
				t.Fatalf("GET count = %d, want 1", getCount)
			}
			if !headBody.closed {
				t.Fatal("expected HEAD response body to be closed")
			}
			if !getBody.closed {
				t.Fatal("expected GET fallback body to be closed")
			}
		})
	}
}

func TestSizeDoesNotFallbackOnForbiddenHEAD(t *testing.T) {
	var headCount int
	var getCount int
	headBody := &trackingReadCloser{reader: strings.NewReader("")}

	setDefaultClient(t, &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch req.Method {
			case http.MethodHead:
				headCount++
				return &http.Response{
					StatusCode: http.StatusForbidden,
					Header:     make(http.Header),
					Body:       headBody,
					Request:    req,
				}, nil
			case http.MethodGet:
				getCount++
				return &http.Response{
					StatusCode: http.StatusOK,
					Header: http.Header{
						"Content-Length": []string{"123"},
					},
					Body:    &trackingReadCloser{reader: strings.NewReader("payload")},
					Request: req,
				}, nil
			default:
				return nil, errors.New("unexpected method: " + req.Method)
			}
		}),
	})

	size, err := Size("https://example.test/no-fallback")
	if size != 0 {
		t.Fatalf("size = %d, want 0 on error", size)
	}
	var statusErr *StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("error = %T, want *StatusError", err)
	}
	if statusErr.Code != http.StatusForbidden {
		t.Fatalf("status error code = %d, want %d", statusErr.Code, http.StatusForbidden)
	}
	if headCount != 1 {
		t.Fatalf("HEAD count = %d, want 1", headCount)
	}
	if getCount != 0 {
		t.Fatalf("GET count = %d, want 0", getCount)
	}
	if !headBody.closed {
		t.Fatal("expected HEAD response body to be closed")
	}
}
