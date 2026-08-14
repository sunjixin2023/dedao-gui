package downloader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yann0917/dedao-gui/backend/request"
)

type saveResponse struct {
	status        int
	body          string
	contentRange  string
	contentLength int
	omitLength    bool
	expectedRange string
	forbidRange   bool
	firstChunk    int
	release       <-chan struct{}
	started       chan<- struct{}
	waitForCancel bool
	cancelErr     chan<- error
}

func newSaveServer(t *testing.T, responses []saveResponse) (*httptest.Server, *atomic.Int32, <-chan error) {
	t.Helper()

	var requests atomic.Int32
	errCh := make(chan error, len(responses)+2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		index := int(requests.Add(1)) - 1
		if index >= len(responses) {
			errCh <- fmt.Errorf("unexpected request %d", index+1)
			http.Error(w, "unexpected request", http.StatusInternalServerError)
			return
		}

		resp := responses[index]
		gotRange := r.Header.Get("Range")
		switch {
		case resp.expectedRange != "" && gotRange != resp.expectedRange:
			errCh <- fmt.Errorf("request %d Range = %q, want %q", index+1, gotRange, resp.expectedRange)
			http.Error(w, "bad range", http.StatusBadRequest)
			return
		case resp.forbidRange && gotRange != "":
			errCh <- fmt.Errorf("request %d Range = %q, want empty", index+1, gotRange)
			http.Error(w, "unexpected range", http.StatusBadRequest)
			return
		}

		if resp.contentRange != "" {
			w.Header().Set("Content-Range", resp.contentRange)
		}
		if resp.omitLength {
			w.Header().Del("Content-Length")
		} else if resp.contentLength > 0 {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", resp.contentLength))
		}

		w.WriteHeader(resp.status)
		if resp.omitLength {
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}

		body := resp.body
		if resp.firstChunk > len(body) {
			resp.firstChunk = len(body)
		}
		if resp.firstChunk > 0 {
			_, _ = w.Write([]byte(body[:resp.firstChunk]))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			if resp.started != nil {
				resp.started <- struct{}{}
			}
			body = body[resp.firstChunk:]
		} else if resp.started != nil {
			resp.started <- struct{}{}
		}

		if resp.waitForCancel {
			<-r.Context().Done()
			if resp.cancelErr != nil {
				resp.cancelErr <- r.Context().Err()
			}
			return
		}

		if resp.release != nil {
			<-resp.release
		}

		if body != "" {
			_, _ = w.Write([]byte(body))
		}
	}))
	t.Cleanup(server.Close)

	return server, &requests, errCh
}

func requireNoHandlerError(t *testing.T, errCh <-chan error) {
	t.Helper()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	default:
	}
}

func savePaths(t *testing.T, ext string) (string, string, string) {
	t.Helper()

	base := filepath.Join(t.TempDir(), "download")
	finalPath := base + "." + ext
	return base, finalPath, finalPath + ".download"
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestSaveWithContextDownloadsToTemporaryFileThenRenames(t *testing.T) {
	content := "hello world"
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	server, _, errCh := newSaveServer(t, []saveResponse{{
		status:      http.StatusOK,
		body:        content,
		firstChunk:  len("hello "),
		started:     started,
		release:     release,
		forbidRange: true,
	}})

	base, finalPath, tempPath := savePaths(t, "mp3")
	resultCh := make(chan error, 1)
	go func() {
		resultCh <- SaveWithContext(context.Background(), URL{
			URL:  server.URL,
			Size: len(content),
			Ext:  "mp3",
		}, base, SaveOptions{})
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("download did not start")
	}

	if _, err := os.Stat(finalPath); !os.IsNotExist(err) {
		t.Fatalf("final file visible before completion: %v", err)
	}
	if _, err := os.Stat(tempPath); err != nil {
		t.Fatalf("temporary file missing during download: %v", err)
	}

	close(release)

	select {
	case err := <-resultCh:
		if err != nil {
			t.Fatalf("SaveWithContext returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SaveWithContext did not finish")
	}

	if got := mustReadFile(t, finalPath); got != content {
		t.Fatalf("final content = %q, want %q", got, content)
	}
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Fatalf("completed temporary file still exists: %v", err)
	}
	requireNoHandlerError(t, errCh)
}

func TestSaveWithContextResumesFromValidPartialContent(t *testing.T) {
	content := "hello world"
	partial := "hello "
	server, requests, errCh := newSaveServer(t, []saveResponse{{
		status:        http.StatusPartialContent,
		body:          content[len(partial):],
		contentRange:  fmt.Sprintf("bytes %d-%d/%d", len(partial), len(content)-1, len(content)),
		expectedRange: fmt.Sprintf("bytes=%d-", len(partial)),
	}})

	base, finalPath, tempPath := savePaths(t, "mp3")
	mustWriteFile(t, tempPath, partial)

	err := SaveWithContext(context.Background(), URL{
		URL:  server.URL,
		Size: len(content),
		Ext:  "mp3",
	}, base, SaveOptions{})
	if err != nil {
		t.Fatalf("SaveWithContext returned error: %v", err)
	}

	if got := requests.Load(); got != 1 {
		t.Fatalf("request count = %d, want 1", got)
	}
	if got := mustReadFile(t, finalPath); got != content {
		t.Fatalf("final content = %q, want %q", got, content)
	}
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Fatalf("completed temporary file still exists: %v", err)
	}
	requireNoHandlerError(t, errCh)
}

func TestSaveWithContextRestartsWhenServerIgnoresRange(t *testing.T) {
	content := "restart-safe"
	partial := "res"
	server, requests, errCh := newSaveServer(t, []saveResponse{
		{
			status:        http.StatusOK,
			body:          content,
			expectedRange: fmt.Sprintf("bytes=%d-", len(partial)),
		},
		{
			status:      http.StatusOK,
			body:        content,
			forbidRange: true,
		},
	})

	base, finalPath, tempPath := savePaths(t, "mp3")
	mustWriteFile(t, tempPath, partial)

	err := SaveWithContext(context.Background(), URL{
		URL:  server.URL,
		Size: len(content),
		Ext:  "mp3",
	}, base, SaveOptions{})
	if err != nil {
		t.Fatalf("SaveWithContext returned error: %v", err)
	}

	if got := requests.Load(); got != 2 {
		t.Fatalf("request count = %d, want 2", got)
	}
	if got := mustReadFile(t, finalPath); got != content {
		t.Fatalf("final content = %q, want %q", got, content)
	}
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Fatalf("completed temporary file still exists: %v", err)
	}
	requireNoHandlerError(t, errCh)
}

func TestSaveWithContextRejectsMismatchedContentRange(t *testing.T) {
	tests := []struct {
		name         string
		contentRange string
	}{
		{name: "mismatched-start", contentRange: "bytes 1-4/8"},
		{name: "malformed", contentRange: "not-a-range"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			content := "abcdefgh"
			partial := "abc"
			server, _, errCh := newSaveServer(t, []saveResponse{{
				status:        http.StatusPartialContent,
				body:          content[len(partial):],
				contentRange:  tc.contentRange,
				expectedRange: fmt.Sprintf("bytes=%d-", len(partial)),
			}})

			base, finalPath, tempPath := savePaths(t, "mp3")
			mustWriteFile(t, tempPath, partial)

			err := SaveWithContext(context.Background(), URL{
				URL:  server.URL,
				Size: len(content),
				Ext:  "mp3",
			}, base, SaveOptions{})
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "Content-Range") {
				t.Fatalf("error = %v, want Content-Range detail", err)
			}
			if _, statErr := os.Stat(finalPath); !os.IsNotExist(statErr) {
				t.Fatalf("final file should not exist: %v", statErr)
			}
			if got := mustReadFile(t, tempPath); got != partial {
				t.Fatalf("temporary content = %q, want %q", got, partial)
			}
			requireNoHandlerError(t, errCh)
		})
	}
}

func TestSaveWithContextAcceptsCompletePartialAfter416(t *testing.T) {
	content := "already-complete"
	server, requests, errCh := newSaveServer(t, []saveResponse{{
		status:        http.StatusRequestedRangeNotSatisfiable,
		contentRange:  fmt.Sprintf("bytes */%d", len(content)),
		expectedRange: fmt.Sprintf("bytes=%d-", len(content)),
	}})

	base, finalPath, tempPath := savePaths(t, "mp3")
	mustWriteFile(t, tempPath, content)

	err := SaveWithContext(context.Background(), URL{
		URL:  server.URL,
		Size: len(content),
		Ext:  "mp3",
	}, base, SaveOptions{})
	if err != nil {
		t.Fatalf("SaveWithContext returned error: %v", err)
	}

	if got := requests.Load(); got != 1 {
		t.Fatalf("request count = %d, want 1", got)
	}
	if got := mustReadFile(t, finalPath); got != content {
		t.Fatalf("final content = %q, want %q", got, content)
	}
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Fatalf("completed temporary file still exists: %v", err)
	}
	requireNoHandlerError(t, errCh)
}

func TestSaveWithContextRejectsWrongSizeAfter416(t *testing.T) {
	partial := "hello"
	server, requests, errCh := newSaveServer(t, []saveResponse{{
		status:        http.StatusRequestedRangeNotSatisfiable,
		contentRange:  fmt.Sprintf("bytes */%d", len(partial)),
		expectedRange: fmt.Sprintf("bytes=%d-", len(partial)),
	}})

	base, finalPath, tempPath := savePaths(t, "mp3")
	mustWriteFile(t, tempPath, partial)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resultCh := make(chan error, 1)
	go func() {
		resultCh <- SaveWithContext(ctx, URL{
			URL:  server.URL,
			Size: len(partial) + 1,
			Ext:  "mp3",
		}, base, SaveOptions{})
	}()

	select {
	case err := <-resultCh:
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "does not match expected") {
			t.Fatalf("error = %v, want size mismatch", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("SaveWithContext did not return after a single 416 mismatch")
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("request count = %d, want 1", got)
	}
	if _, statErr := os.Stat(finalPath); !os.IsNotExist(statErr) {
		t.Fatalf("final file should not exist: %v", statErr)
	}
	if got := mustReadFile(t, tempPath); got != partial {
		t.Fatalf("temp content = %q, want %q", got, partial)
	}
	requireNoHandlerError(t, errCh)
}

func TestSaveWithContextRejectsTruncatedBody(t *testing.T) {
	server, _, errCh := newSaveServer(t, []saveResponse{{
		status:        http.StatusOK,
		body:          "abc",
		contentLength: 5,
		forbidRange:   true,
	}})

	base, finalPath, _ := savePaths(t, "mp3")
	err := SaveWithContext(context.Background(), URL{
		URL:  server.URL,
		Size: 5,
		Ext:  "mp3",
	}, base, SaveOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
	if _, statErr := os.Stat(finalPath); !os.IsNotExist(statErr) {
		t.Fatalf("final file should not exist: %v", statErr)
	}
	requireNoHandlerError(t, errCh)
}

func TestSaveWithContextStopsAtContextDeadline(t *testing.T) {
	server, requests, errCh := newSaveServer(t, []saveResponse{{
		status:      http.StatusInternalServerError,
		body:        "retry",
		forbidRange: true,
	}})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	base, _, _ := savePaths(t, "mp3")
	err := SaveWithContext(ctx, URL{
		URL:  server.URL,
		Size: 5,
		Ext:  "mp3",
	}, base, SaveOptions{
		MaxRetries: 5,
		RetryDelay: 2 * time.Second,
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want %v", err, context.DeadlineExceeded)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("request count = %d, want 1", got)
	}
	requireNoHandlerError(t, errCh)
}

func TestSaveWithContextPreservesPartialOnCancellation(t *testing.T) {
	content := "abcdef"
	started := make(chan struct{}, 1)
	cancelErr := make(chan error, 1)
	server, _, errCh := newSaveServer(t, []saveResponse{{
		status:        http.StatusOK,
		body:          content,
		firstChunk:    3,
		started:       started,
		waitForCancel: true,
		cancelErr:     cancelErr,
		forbidRange:   true,
	}})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	base, finalPath, tempPath := savePaths(t, "mp3")
	resultCh := make(chan error, 1)
	go func() {
		resultCh <- SaveWithContext(ctx, URL{
			URL:  server.URL,
			Size: len(content),
			Ext:  "mp3",
		}, base, SaveOptions{
			MaxRetries: 3,
			RetryDelay: time.Millisecond,
		})
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("download did not start")
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		info, err := os.Stat(tempPath)
		if err == nil && info.Size() > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("temporary file did not receive data: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()

	select {
	case err := <-resultCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want %v", err, context.Canceled)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SaveWithContext did not return after cancellation")
	}

	select {
	case err := <-cancelErr:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("handler context error = %v, want %v", err, context.Canceled)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not observe cancellation")
	}

	if _, err := os.Stat(finalPath); !os.IsNotExist(err) {
		t.Fatalf("final file should not exist: %v", err)
	}
	if got := mustReadFile(t, tempPath); got != "abc" {
		t.Fatalf("temporary content = %q, want %q", got, "abc")
	}
	requireNoHandlerError(t, errCh)
}

func TestSaveWithContextDoesNotRetry401403Or496(t *testing.T) {
	tests := []struct {
		name   string
		status int
		check  func(*request.StatusError) bool
	}{
		{
			name:   "401",
			status: http.StatusUnauthorized,
			check:  func(err *request.StatusError) bool { return err.AuthenticationRequired() },
		},
		{
			name:   "403",
			status: http.StatusForbidden,
			check:  func(err *request.StatusError) bool { return err.AuthenticationRequired() },
		},
		{
			name:   "496",
			status: 496,
			check:  func(err *request.StatusError) bool { return err.VerificationRequired() },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server, requests, errCh := newSaveServer(t, []saveResponse{{
				status:      tc.status,
				body:        "blocked",
				forbidRange: true,
			}})

			base, finalPath, _ := savePaths(t, "mp3")
			err := SaveWithContext(context.Background(), URL{
				URL:  server.URL,
				Size: 7,
				Ext:  "mp3",
			}, base, SaveOptions{
				MaxRetries: 5,
				RetryDelay: 10 * time.Millisecond,
			})
			var statusErr *request.StatusError
			if !errors.As(err, &statusErr) {
				t.Fatalf("error = %T, want *request.StatusError", err)
			}
			if !tc.check(statusErr) {
				t.Fatalf("unexpected status helper state for %d", tc.status)
			}
			if got := requests.Load(); got != 1 {
				t.Fatalf("request count = %d, want 1", got)
			}
			if _, statErr := os.Stat(finalPath); !os.IsNotExist(statErr) {
				t.Fatalf("final file should not exist: %v", statErr)
			}
			requireNoHandlerError(t, errCh)
		})
	}
}

func TestSaveWithContextDiscoversUnknownSizeWithHeadFallback(t *testing.T) {
	tests := []struct {
		name        string
		headStatus  int
		headLength  string
		wantMethods []string
	}{
		{
			name:        "head-405-falls-back-to-get",
			headStatus:  http.StatusMethodNotAllowed,
			wantMethods: []string{http.MethodHead, http.MethodGet, http.MethodGet},
		},
		{
			name:        "head-501-falls-back-to-get",
			headStatus:  http.StatusNotImplemented,
			wantMethods: []string{http.MethodHead, http.MethodGet, http.MethodGet},
		},
		{
			name:        "head-missing-length-falls-back-to-get",
			headStatus:  http.StatusOK,
			wantMethods: []string{http.MethodHead, http.MethodGet, http.MethodGet},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			content := "abcdefghij"
			methods := make([]string, 0, 3)
			var methodsMu sync.Mutex
			var metadataGets atomic.Int32
			var downloadGets atomic.Int32

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				methodsMu.Lock()
				methods = append(methods, r.Method)
				methodsMu.Unlock()
				if got := r.Header.Get("X-Test"); got != "size-discovery" {
					http.Error(w, "missing custom header", http.StatusBadRequest)
					return
				}
				if got := r.Header.Get("User-Agent"); got != request.UserAgent {
					http.Error(w, "missing default user agent", http.StatusBadRequest)
					return
				}

				switch r.Method {
				case http.MethodHead:
					if tc.headStatus == http.StatusOK && tc.headLength != "" {
						w.Header().Set("Content-Length", tc.headLength)
					}
					w.WriteHeader(tc.headStatus)
				case http.MethodGet:
					if metadataGets.Load() == 0 {
						metadataGets.Add(1)
					} else {
						downloadGets.Add(1)
					}
					w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
					_, _ = w.Write([]byte(content))
				default:
					http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
				}
			}))
			defer server.Close()

			base, finalPath, _ := savePaths(t, "mp3")
			err := SaveWithContext(context.Background(), URL{
				URL: server.URL,
				Ext: "mp3",
			}, base, SaveOptions{
				Header: http.Header{"X-Test": []string{"size-discovery"}},
			})
			if err != nil {
				t.Fatalf("SaveWithContext returned error: %v", err)
			}

			if got := metadataGets.Load(); got != 1 {
				t.Fatalf("metadata GET count = %d, want 1", got)
			}
			if got := downloadGets.Load(); got != 1 {
				t.Fatalf("download GET count = %d, want 1", got)
			}
			methodsMu.Lock()
			gotMethods := append([]string(nil), methods...)
			methodsMu.Unlock()
			if !reflect.DeepEqual(gotMethods, tc.wantMethods) {
				t.Fatalf("methods = %v, want %v", gotMethods, tc.wantMethods)
			}
			if got := mustReadFile(t, finalPath); got != content {
				t.Fatalf("final content = %q, want %q", got, content)
			}
		})
	}
}

func TestSaveWithContextUnknownSizeDoesNotFallbackOnAuthenticationHeadError(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != http.MethodHead {
			http.Error(w, "unexpected fallback", http.StatusInternalServerError)
			return
		}
		if got := r.Header.Get("X-Test"); got != "auth-head" {
			http.Error(w, "missing custom header", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	base, finalPath, tempPath := savePaths(t, "mp3")
	err := SaveWithContext(context.Background(), URL{
		URL: server.URL,
		Ext: "mp3",
	}, base, SaveOptions{
		Header: http.Header{"X-Test": []string{"auth-head"}},
	})
	var statusErr *request.StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("error = %T, want *request.StatusError", err)
	}
	if !statusErr.AuthenticationRequired() {
		t.Fatalf("unexpected status error: %v", statusErr)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("request count = %d, want 1", got)
	}
	if _, statErr := os.Stat(finalPath); !os.IsNotExist(statErr) {
		t.Fatalf("final file should not exist: %v", statErr)
	}
	if _, statErr := os.Stat(tempPath); !os.IsNotExist(statErr) {
		t.Fatalf("temporary file should not exist: %v", statErr)
	}
}

func TestSaveWithContextSkipsExistingFinalWithExpectedSize(t *testing.T) {
	content := "existing"
	server, requests, errCh := newSaveServer(t, []saveResponse{{
		status:      http.StatusOK,
		body:        "should-not-hit",
		forbidRange: true,
	}})

	base, finalPath, tempPath := savePaths(t, "mp3")
	mustWriteFile(t, finalPath, content)
	mustWriteFile(t, tempPath, "partial")

	err := SaveWithContext(context.Background(), URL{
		URL:  server.URL,
		Size: len(content),
		Ext:  "mp3",
	}, base, SaveOptions{})
	if err != nil {
		t.Fatalf("SaveWithContext returned error: %v", err)
	}

	if got := requests.Load(); got != 0 {
		t.Fatalf("request count = %d, want 0", got)
	}
	if got := mustReadFile(t, finalPath); got != content {
		t.Fatalf("final content = %q, want %q", got, content)
	}
	if got := mustReadFile(t, tempPath); got != "partial" {
		t.Fatalf("temp content = %q, want %q", got, "partial")
	}
	requireNoHandlerError(t, errCh)
}

func TestSaveWithContextRejectsExistingFinalWithWrongSizeBeforeNetwork(t *testing.T) {
	server, requests, errCh := newSaveServer(t, []saveResponse{{
		status:      http.StatusOK,
		body:        "should-not-hit",
		forbidRange: true,
	}})

	base, finalPath, tempPath := savePaths(t, "mp3")
	mustWriteFile(t, finalPath, "bad")
	mustWriteFile(t, tempPath, "partial")

	err := SaveWithContext(context.Background(), URL{
		URL:  server.URL,
		Size: len("expected"),
		Ext:  "mp3",
	}, base, SaveOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "existing file size") {
		t.Fatalf("error = %v, want existing file size detail", err)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("request count = %d, want 0", got)
	}
	if got := mustReadFile(t, finalPath); got != "bad" {
		t.Fatalf("final content = %q, want %q", got, "bad")
	}
	if got := mustReadFile(t, tempPath); got != "partial" {
		t.Fatalf("temp content = %q, want %q", got, "partial")
	}
	requireNoHandlerError(t, errCh)
}

func TestSaveWithContextRefusesToOverwriteFinalAppearingBeforeRename(t *testing.T) {
	content := "hello world"
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	server, _, errCh := newSaveServer(t, []saveResponse{{
		status:      http.StatusOK,
		body:        content,
		firstChunk:  len("hello "),
		started:     started,
		release:     release,
		forbidRange: true,
	}})

	base, finalPath, tempPath := savePaths(t, "mp3")
	resultCh := make(chan error, 1)
	go func() {
		resultCh <- SaveWithContext(context.Background(), URL{
			URL:  server.URL,
			Size: len(content),
			Ext:  "mp3",
		}, base, SaveOptions{})
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("download did not start")
	}
	mustWriteFile(t, finalPath, "appeared")
	close(release)

	err := <-resultCh
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "destination already exists") {
		t.Fatalf("error = %v, want destination already exists detail", err)
	}
	if got := mustReadFile(t, finalPath); got != "appeared" {
		t.Fatalf("final content = %q, want %q", got, "appeared")
	}
	if got := mustReadFile(t, tempPath); got != content {
		t.Fatalf("temp content = %q, want %q", got, content)
	}
	requireNoHandlerError(t, errCh)
}

func TestParseContentRangeRejectsTrailingJunk(t *testing.T) {
	tests := []string{
		"bytes 3-5x/10",
		"bytes 3-5/10x",
		"bytes 3-5/10 extra",
		"bytes 3-5/5",
		"bytes 6-5/10",
	}

	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			if _, err := parseContentRange(value); err == nil {
				t.Fatalf("parseContentRange(%q) unexpectedly succeeded", value)
			}
		})
	}
}

func TestParseContentRangeAcceptsOuterWhitespaceOnly(t *testing.T) {
	parsed, err := parseContentRange("  bytes 3-5/10  ")
	if err != nil {
		t.Fatalf("parseContentRange returned error: %v", err)
	}
	if parsed != (contentRange{Start: 3, End: 5, Total: 10}) {
		t.Fatalf("parsed range = %#v", parsed)
	}
}

func TestParseUnsatisfiedTotalRejectsTrailingJunk(t *testing.T) {
	tests := []string{
		"bytes */10x",
		"bytes */10 extra",
	}

	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			if _, err := parseUnsatisfiedTotal(value); err == nil {
				t.Fatalf("parseUnsatisfiedTotal(%q) unexpectedly succeeded", value)
			}
		})
	}
}

func TestParseUnsatisfiedTotalAcceptsOuterWhitespaceOnly(t *testing.T) {
	total, err := parseUnsatisfiedTotal("  bytes */10  ")
	if err != nil {
		t.Fatalf("parseUnsatisfiedTotal returned error: %v", err)
	}
	if total != 10 {
		t.Fatalf("total = %d, want 10", total)
	}
}

func TestValidateResumeResponseRejectsMismatchedContentLength(t *testing.T) {
	resp := &http.Response{
		StatusCode:    http.StatusPartialContent,
		Header:        http.Header{"Content-Range": []string{"bytes 3-5/10"}},
		ContentLength: 2,
		Request:       mustRequest(t, "https://example.test/file"),
		Body:          io.NopCloser(strings.NewReader("45")),
	}

	_, _, _, err := validateResumeResponse(resp, 3, 10)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Content-Length") {
		t.Fatalf("error = %v, want Content-Length detail", err)
	}
}

func TestValidateResumeResponseRejectsMismatchedContentRangeTotal(t *testing.T) {
	resp := &http.Response{
		StatusCode:    http.StatusPartialContent,
		Header:        http.Header{"Content-Range": []string{"bytes 3-5/11"}},
		ContentLength: 3,
		Request:       mustRequest(t, "https://example.test/file"),
		Body:          io.NopCloser(strings.NewReader("345")),
	}

	_, _, _, err := validateResumeResponse(resp, 3, 10)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "expected total") {
		t.Fatalf("error = %v, want expected total detail", err)
	}
}

func TestSaveWithContextRejectsMismatchedContentRangeTotal(t *testing.T) {
	content := "abcdefgh"
	partial := "abc"
	server, _, errCh := newSaveServer(t, []saveResponse{{
		status:        http.StatusPartialContent,
		body:          content[len(partial):],
		contentRange:  fmt.Sprintf("bytes %d-%d/%d", len(partial), len(content)-1, len(content)+1),
		contentLength: len(content) - len(partial),
		expectedRange: fmt.Sprintf("bytes=%d-", len(partial)),
	}})

	base, finalPath, tempPath := savePaths(t, "mp3")
	mustWriteFile(t, tempPath, partial)

	err := SaveWithContext(context.Background(), URL{
		URL:  server.URL,
		Size: len(content),
		Ext:  "mp3",
	}, base, SaveOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "expected total") {
		t.Fatalf("error = %v, want expected total detail", err)
	}
	if _, statErr := os.Stat(finalPath); !os.IsNotExist(statErr) {
		t.Fatalf("final file should not exist: %v", statErr)
	}
	if got := mustReadFile(t, tempPath); got != partial {
		t.Fatalf("temporary content = %q, want %q", got, partial)
	}
	requireNoHandlerError(t, errCh)
}

func TestSaveWithContextHonoursChunkSizeBytes(t *testing.T) {
	content := "abcdefghij"
	var ranges []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ranges = append(ranges, r.Header.Get("Range"))
		switch r.Header.Get("Range") {
		case "bytes=0-3":
			w.Header().Set("Content-Range", "bytes 0-3/10")
			w.Header().Set("Content-Length", "4")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte("abcd"))
		case "bytes=4-7":
			w.Header().Set("Content-Range", "bytes 4-7/10")
			w.Header().Set("Content-Length", "4")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte("efgh"))
		case "bytes=8-9":
			w.Header().Set("Content-Range", "bytes 8-9/10")
			w.Header().Set("Content-Length", "2")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte("ij"))
		default:
			http.Error(w, "unexpected range "+r.Header.Get("Range"), http.StatusBadRequest)
		}
	}))
	defer server.Close()

	base, finalPath, tempPath := savePaths(t, "mp3")
	err := SaveWithContext(context.Background(), URL{
		URL:  server.URL,
		Size: len(content),
		Ext:  "mp3",
	}, base, SaveOptions{
		ChunkSizeBytes: 4,
	})
	if err != nil {
		t.Fatalf("SaveWithContext returned error: %v", err)
	}

	wantRanges := []string{"bytes=0-3", "bytes=4-7", "bytes=8-9"}
	if !reflect.DeepEqual(ranges, wantRanges) {
		t.Fatalf("ranges = %v, want %v", ranges, wantRanges)
	}
	if got := mustReadFile(t, finalPath); got != content {
		t.Fatalf("final content = %q, want %q", got, content)
	}
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Fatalf("completed temporary file still exists: %v", err)
	}
}

func TestSaveWithContextResetsRetryBudgetAfterAdvancingChunk(t *testing.T) {
	content := "abcdefgh"
	var (
		reqMu    sync.Mutex
		requests []string
		hits     = map[string]int{}
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rng := r.Header.Get("Range")
		reqMu.Lock()
		requests = append(requests, rng)
		hits[rng]++
		count := hits[rng]
		reqMu.Unlock()

		switch rng {
		case "bytes=0-3":
			if count == 1 {
				http.Error(w, "retry first chunk", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Range", "bytes 0-3/8")
			w.Header().Set("Content-Length", "4")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte("abcd"))
		case "bytes=4-7":
			if count == 1 {
				http.Error(w, "retry second chunk", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Range", "bytes 4-7/8")
			w.Header().Set("Content-Length", "4")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte("efgh"))
		default:
			http.Error(w, "unexpected range "+rng, http.StatusBadRequest)
		}
	}))
	defer server.Close()

	base, finalPath, tempPath := savePaths(t, "mp3")
	err := SaveWithContext(context.Background(), URL{
		URL:  server.URL,
		Size: len(content),
		Ext:  "mp3",
	}, base, SaveOptions{
		ChunkSizeBytes: 4,
		MaxRetries:     1,
		RetryDelay:     time.Millisecond,
	})
	if err != nil {
		t.Fatalf("SaveWithContext returned error: %v", err)
	}

	wantRequests := []string{"bytes=0-3", "bytes=0-3", "bytes=4-7", "bytes=4-7"}
	reqMu.Lock()
	gotRequests := append([]string(nil), requests...)
	reqMu.Unlock()
	if !reflect.DeepEqual(gotRequests, wantRequests) {
		t.Fatalf("requests = %v, want %v", gotRequests, wantRequests)
	}
	if got := mustReadFile(t, finalPath); got != content {
		t.Fatalf("final content = %q, want %q", got, content)
	}
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Fatalf("completed temporary file still exists: %v", err)
	}
}

func TestSaveWithContextRejectsShort206BodyWithoutContentLength(t *testing.T) {
	partial := "abc"
	server, _, errCh := newSaveServer(t, []saveResponse{{
		status:        http.StatusPartialContent,
		body:          "de",
		contentRange:  "bytes 3-5/8",
		omitLength:    true,
		expectedRange: "bytes=3-5",
	}})

	base, finalPath, tempPath := savePaths(t, "mp3")
	mustWriteFile(t, tempPath, partial)

	err := SaveWithContext(context.Background(), URL{
		URL:  server.URL,
		Size: 8,
		Ext:  "mp3",
	}, base, SaveOptions{
		ChunkSizeBytes: 3,
		MaxRetries:     0,
	})
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("error = %v, want %v", err, io.ErrUnexpectedEOF)
	}
	if _, statErr := os.Stat(finalPath); !os.IsNotExist(statErr) {
		t.Fatalf("final file should not exist: %v", statErr)
	}
	if got := mustReadFile(t, tempPath); got != "abcde" {
		t.Fatalf("temp content = %q, want %q", got, "abcde")
	}
	requireNoHandlerError(t, errCh)
}

func TestSaveWithContextRejectsZeroProgress206BodyWithoutContentLength(t *testing.T) {
	partial := "abc"
	server, _, errCh := newSaveServer(t, []saveResponse{{
		status:        http.StatusPartialContent,
		body:          "",
		contentRange:  "bytes 3-5/8",
		omitLength:    true,
		expectedRange: "bytes=3-5",
	}})

	base, finalPath, tempPath := savePaths(t, "mp3")
	mustWriteFile(t, tempPath, partial)

	err := SaveWithContext(context.Background(), URL{
		URL:  server.URL,
		Size: 8,
		Ext:  "mp3",
	}, base, SaveOptions{
		ChunkSizeBytes: 3,
		MaxRetries:     0,
	})
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("error = %v, want %v", err, io.ErrUnexpectedEOF)
	}
	if _, statErr := os.Stat(finalPath); !os.IsNotExist(statErr) {
		t.Fatalf("final file should not exist: %v", statErr)
	}
	if got := mustReadFile(t, tempPath); got != partial {
		t.Fatalf("temp content = %q, want %q", got, partial)
	}
	requireNoHandlerError(t, errCh)
}

func TestSaveWithContextDoesNotResetRetryBudgetAfterFailedPartial206Progress(t *testing.T) {
	partial := "abc"
	var (
		reqMu    sync.Mutex
		requests []string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("Range")
		reqMu.Lock()
		requests = append(requests, got)
		reqMu.Unlock()
		switch got {
		case "bytes=3-5":
			w.Header().Set("Content-Range", "bytes 3-5/8")
			w.WriteHeader(http.StatusPartialContent)
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			_, _ = w.Write([]byte("de"))
		case "bytes=5-7":
			w.Header().Set("Content-Range", "bytes 5-7/8")
			w.WriteHeader(http.StatusPartialContent)
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			_, _ = w.Write([]byte("fg"))
		default:
			http.Error(w, "unexpected range "+got, http.StatusBadRequest)
		}
	}))
	defer server.Close()

	base, finalPath, tempPath := savePaths(t, "mp3")
	mustWriteFile(t, tempPath, partial)

	err := SaveWithContext(context.Background(), URL{
		URL:  server.URL,
		Size: 8,
		Ext:  "mp3",
	}, base, SaveOptions{
		ChunkSizeBytes: 3,
		MaxRetries:     1,
		RetryDelay:     time.Millisecond,
	})
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("error = %v, want %v", err, io.ErrUnexpectedEOF)
	}
	reqMu.Lock()
	gotRequests := append([]string(nil), requests...)
	reqMu.Unlock()
	wantRequests := []string{"bytes=3-5", "bytes=5-7"}
	if !reflect.DeepEqual(gotRequests, wantRequests) {
		t.Fatalf("requests = %v, want %v", gotRequests, wantRequests)
	}
	if _, statErr := os.Stat(finalPath); !os.IsNotExist(statErr) {
		t.Fatalf("final file should not exist: %v", statErr)
	}
	if got := mustReadFile(t, tempPath); got != "abcdefg" {
		t.Fatalf("temp content = %q, want %q", got, "abcdefg")
	}
}

func TestSaveWithContextRejectsOversized206BodyWithoutContentLength(t *testing.T) {
	partial := "abc"
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if got := r.Header.Get("Range"); got != "bytes=3-5" {
			http.Error(w, "unexpected range "+got, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Range", "bytes 3-5/8")
		w.WriteHeader(http.StatusPartialContent)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		_, _ = w.Write([]byte("defg"))
	}))
	defer server.Close()

	base, finalPath, tempPath := savePaths(t, "mp3")
	mustWriteFile(t, tempPath, partial)

	err := SaveWithContext(context.Background(), URL{
		URL:  server.URL,
		Size: 8,
		Ext:  "mp3",
	}, base, SaveOptions{
		ChunkSizeBytes: 3,
		MaxRetries:     3,
		RetryDelay:     time.Millisecond,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "wrote 4 bytes for declared span 3") {
		t.Fatalf("error = %v, want oversized span detail", err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("request count = %d, want 1", got)
	}
	if _, statErr := os.Stat(finalPath); !os.IsNotExist(statErr) {
		t.Fatalf("final file should not exist: %v", statErr)
	}
	if got := mustReadFile(t, tempPath); got != "abcdefg" {
		t.Fatalf("temp content = %q, want %q", got, "abcdefg")
	}
}

func TestBuildRangeHeaderHandlesLargeValuesWithoutOverflow(t *testing.T) {
	tests := []struct {
		name       string
		localSize  int64
		expected   int64
		chunkSize  int64
		allowRange bool
		want       string
	}{
		{
			name:       "caps-at-expected-minus-one",
			localSize:  math.MaxInt64 - 5,
			expected:   math.MaxInt64 - 1,
			chunkSize:  10,
			allowRange: true,
			want:       fmt.Sprintf("bytes=%d-%d", int64(math.MaxInt64-5), int64(math.MaxInt64-2)),
		},
		{
			name:       "exact-remaining",
			localSize:  math.MaxInt64 - 5,
			expected:   math.MaxInt64,
			chunkSize:  5,
			allowRange: true,
			want:       fmt.Sprintf("bytes=%d-%d", int64(math.MaxInt64-5), int64(math.MaxInt64-1)),
		},
		{
			name:       "complete-partial-stays-open-ended",
			localSize:  math.MaxInt64 - 1,
			expected:   math.MaxInt64 - 1,
			chunkSize:  8,
			allowRange: true,
			want:       fmt.Sprintf("bytes=%d-", int64(math.MaxInt64-1)),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := buildRangeHeader(tc.localSize, tc.expected, tc.chunkSize, tc.allowRange); got != tc.want {
				t.Fatalf("buildRangeHeader(...) = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFinalizeDownloadDoesNotOverwriteExistingDestination(t *testing.T) {
	dir := t.TempDir()
	tempPath := filepath.Join(dir, "file.mp3.download")
	finalPath := filepath.Join(dir, "file.mp3")
	mustWriteFile(t, tempPath, "fresh")
	mustWriteFile(t, finalPath, "existing")

	err := finalizeDownload(tempPath, finalPath, int64(len("fresh")))
	if err == nil {
		t.Fatal("expected error")
	}
	if got := mustReadFile(t, finalPath); got != "existing" {
		t.Fatalf("final content = %q, want %q", got, "existing")
	}
	if got := mustReadFile(t, tempPath); got != "fresh" {
		t.Fatalf("temp content = %q, want %q", got, "fresh")
	}
}

func TestFinalizeDownloadAllowsExactlyOneConcurrentPublisher(t *testing.T) {
	dir := t.TempDir()
	tempA := filepath.Join(dir, "a.download")
	tempB := filepath.Join(dir, "b.download")
	finalPath := filepath.Join(dir, "file.mp3")
	mustWriteFile(t, tempA, "winner-a")
	mustWriteFile(t, tempB, "winner-b")

	start := make(chan struct{})
	errCh := make(chan error, 2)
	go func() {
		<-start
		errCh <- finalizeDownload(tempA, finalPath, int64(len("winner-a")))
	}()
	go func() {
		<-start
		errCh <- finalizeDownload(tempB, finalPath, int64(len("winner-b")))
	}()
	close(start)

	err1 := <-errCh
	err2 := <-errCh
	var successCount int
	for _, err := range []error{err1, err2} {
		if err == nil {
			successCount++
			continue
		}
		if !strings.Contains(err.Error(), "destination already exists") && !strings.Contains(err.Error(), "link completed download") {
			t.Fatalf("unexpected concurrent finalize error: %v", err)
		}
	}
	if successCount != 1 {
		t.Fatalf("success count = %d, want 1", successCount)
	}
	finalData := mustReadFile(t, finalPath)
	if finalData != "winner-a" && finalData != "winner-b" {
		t.Fatalf("final content = %q, want one candidate", finalData)
	}
	if finalData == "winner-a" {
		if _, err := os.Stat(tempA); !os.IsNotExist(err) {
			t.Fatalf("tempA should be removed on success: %v", err)
		}
		if got := mustReadFile(t, tempB); got != "winner-b" {
			t.Fatalf("tempB content = %q, want %q", got, "winner-b")
		}
	} else {
		if _, err := os.Stat(tempB); !os.IsNotExist(err) {
			t.Fatalf("tempB should be removed on success: %v", err)
		}
		if got := mustReadFile(t, tempA); got != "winner-a" {
			t.Fatalf("tempA content = %q, want %q", got, "winner-a")
		}
	}
}

func TestDownloadStopsBeforeMergeWhenOnePartFails(t *testing.T) {
	slowStarted := make(chan struct{}, 1)
	cancelErr := make(chan error, 1)
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slowStarted <- struct{}{}
		w.Header().Set("Content-Range", "bytes 0-4/5")
		w.WriteHeader(http.StatusPartialContent)
		<-r.Context().Done()
		cancelErr <- r.Context().Err()
	}))
	defer slowServer.Close()

	failServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-slowStarted:
		case <-time.After(2 * time.Second):
			http.Error(w, "slow part did not start", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("blocked"))
	}))
	defer failServer.Close()

	tempDir := t.TempDir()
	err := DownloadWithContext(context.Background(), Datum{
		Title: "Episode",
		Type:  "audio",
		Streams: map[string]Stream{
			"audio": {
				URLs: []URL{
					{URL: slowServer.URL, Size: 5, Ext: "mp3"},
					{URL: failServer.URL, Size: 5, Ext: "mp3"},
				},
			},
		},
	}, "audio", tempDir)
	if err == nil {
		t.Fatal("expected error")
	}
	var statusErr *request.StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("error = %T, want *request.StatusError", err)
	}
	if !statusErr.AuthenticationRequired() {
		t.Fatalf("unexpected status error: %v", statusErr)
	}

	select {
	case observed := <-cancelErr:
		if !errors.Is(observed, context.Canceled) {
			t.Fatalf("slow handler context error = %v, want %v", observed, context.Canceled)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("slow part was not cancelled")
	}

	mergedPath := filepath.Join(tempDir, "Episode.mp3")
	if _, statErr := os.Stat(mergedPath); !os.IsNotExist(statErr) {
		t.Fatalf("merged file should not exist: %v", statErr)
	}
}

func mustRequest(t *testing.T, rawURL string) *http.Request {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		t.Fatalf("NewRequest(%q): %v", rawURL, err)
	}
	return req
}
