package request

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestBatchRejectsNonPositiveConcurrency(t *testing.T) {
	tasks := NewDownloadTasks()

	err := Batch(context.Background(), tasks, 0, time.Second)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "greater than zero") {
		t.Fatalf("error = %v, want concurrency validation", err)
	}
}

func TestBatchPropagatesTaskFailure(t *testing.T) {
	slowStarted := make(chan struct{})
	handlerErr := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/fail":
			select {
			case <-slowStarted:
			case <-time.After(2 * time.Second):
				handlerErr <- errors.New("slow handler did not start")
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("boom"))
		case "/slow":
			close(slowStarted)
			<-r.Context().Done()
			handlerErr <- r.Context().Err()
		default:
			handlerErr <- fmt.Errorf("unexpected path %q", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	g := Default()
	g.Client = *server.Client()

	dir := t.TempDir()
	failTask := NewDownloadTask(server.URL+"/fail", filepath.Join(dir, "fail.bin"))
	slowTask := NewDownloadTask(server.URL+"/slow", filepath.Join(dir, "slow.bin"))
	tasks := NewDownloadTasks()
	tasks.tasks = append(tasks.tasks, failTask, slowTask)

	err := g.Batch(context.Background(), tasks, 2, 5*time.Second)
	if err == nil {
		t.Fatal("expected error")
	}

	var statusErr *StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("error = %T, want *StatusError", err)
	}
	if statusErr.Code != http.StatusInternalServerError {
		t.Fatalf("status code = %d, want %d", statusErr.Code, http.StatusInternalServerError)
	}
	if !errors.As(failTask.Err, &statusErr) {
		t.Fatalf("failTask.Err = %T, want *StatusError", failTask.Err)
	}
	if slowTask.Err == nil {
		t.Fatal("slowTask.Err is nil, want cancellation or transport error")
	}

	select {
	case got := <-handlerErr:
		if !errors.Is(got, context.Canceled) {
			t.Fatalf("handler context error = %v, want %v", got, context.Canceled)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("slow handler did not observe cancellation")
	}
}

func TestBatchStopsWaitingAfterCancellation(t *testing.T) {
	started := make(chan struct{}, 1)
	handlerErr := make(chan error, 1)
	var totalRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		totalRequests.Add(1)
		started <- struct{}{}
		<-r.Context().Done()
		handlerErr <- r.Context().Err()
	}))
	defer server.Close()

	g := Default()
	g.Client = *server.Client()

	dir := t.TempDir()
	firstTask := NewDownloadTask(server.URL+"/one", filepath.Join(dir, "first.bin"))
	secondTask := NewDownloadTask(server.URL+"/two", filepath.Join(dir, "second.bin"))
	tasks := NewDownloadTasks()
	tasks.tasks = append(tasks.tasks, firstTask, secondTask)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultCh := make(chan error, 1)
	go func() {
		resultCh <- g.Batch(ctx, tasks, 1, 5*time.Second)
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("first handler did not start")
	}

	cancel()

	select {
	case err := <-resultCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want %v", err, context.Canceled)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Batch did not return after cancellation")
	}

	if !errors.Is(firstTask.Err, context.Canceled) {
		t.Fatalf("firstTask.Err = %v, want %v", firstTask.Err, context.Canceled)
	}
	if secondTask.Err == nil {
		t.Fatal("secondTask.Err is nil, want cancellation")
	}
	if got := totalRequests.Load(); got != 1 {
		t.Fatalf("request count = %d, want 1", got)
	}

	select {
	case got := <-handlerErr:
		if !errors.Is(got, context.Canceled) {
			t.Fatalf("handler context error = %v, want %v", got, context.Canceled)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("hold handler did not observe cancellation")
	}
}

func TestBatchLeavesSuccessfulSkippedTaskErrNilWhenSiblingFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer server.Close()

	g := Default()
	g.Client = *server.Client()

	dir := t.TempDir()
	skippedTask := NewDownloadTask(server.URL+"/skip", filepath.Join(dir, "skip.bin"))
	mustWriteDownloadFile(t, skippedTask.Path, "done")
	if err := os.WriteFile(skippedTask.Path+".ok", []byte("ok"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	failedTask := NewDownloadTask(server.URL+"/fail", filepath.Join(dir, "fail.bin"))

	tasks := NewDownloadTasks()
	tasks.tasks = append(tasks.tasks, skippedTask, failedTask)

	err := g.Batch(context.Background(), tasks, 2, time.Second)
	if err == nil {
		t.Fatal("expected error")
	}
	if skippedTask.Err != nil {
		t.Fatalf("skippedTask.Err = %v, want nil", skippedTask.Err)
	}
	if failedTask.Err == nil {
		t.Fatal("failedTask.Err is nil, want failure")
	}
}

func TestBatchRejectsNilTaskWithoutPanic(t *testing.T) {
	tasks := NewDownloadTasks()
	tasks.tasks = append(tasks.tasks, nil)

	err := Batch(context.Background(), tasks, 1, time.Second)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "nil download task") {
		t.Fatalf("error = %v, want nil task error", err)
	}
}

func TestBatchWithoutPerTaskTimeoutUsesGroupContext(t *testing.T) {
	started := make(chan struct{}, 1)
	handlerErr := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		<-r.Context().Done()
		handlerErr <- r.Context().Err()
	}))
	defer server.Close()

	g := Default()
	g.Client = *server.Client()

	task := newTempDownloadTask(t, server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultCh := make(chan error, 1)
	go func() {
		resultCh <- g.Batch(ctx, &DownloadTasks{tasks: []*DownloadTask{task}}, 1, 0)
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not start")
	}

	cancel()

	select {
	case err := <-resultCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want %v", err, context.Canceled)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Batch did not return after cancellation")
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

func TestDownloadWithContextResumesOnlyOnValid206(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		handlerErr := make(chan error, 1)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("Range"); got != "bytes=3-" {
				handlerErr <- fmt.Errorf("unexpected Range header %q", got)
				http.Error(w, "bad range", http.StatusBadRequest)
				return
			}
			if got := r.Header.Get("X-Test"); got != "resume" {
				handlerErr <- fmt.Errorf("unexpected X-Test header %q", got)
				http.Error(w, "bad header", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Range", "bytes 3-7/8")
			w.Header().Set("Content-Length", "5")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte("defgh"))
			handlerErr <- nil
		}))
		defer server.Close()

		g := Default()
		g.Client = *server.Client()
		g.Header.Set("X-Test", "resume")

		task := newTempDownloadTask(t, server.URL)
		mustWriteDownloadFile(t, task.Path, "abc")

		if err := g.DownloadWithContext(context.Background(), task); err != nil {
			t.Fatalf("DownloadWithContext returned error: %v", err)
		}
		if got := mustReadDownloadFile(t, task.Path); got != "abcdefgh" {
			t.Fatalf("download content = %q, want %q", got, "abcdefgh")
		}
		if _, err := os.Stat(task.Path + ".ok"); err != nil {
			t.Fatalf(".ok marker missing: %v", err)
		}
		requireDownloadNoHandlerError(t, handlerErr)
	})

	t.Run("invalid-content-range", func(t *testing.T) {
		handlerErr := make(chan error, 1)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("Range"); got != "bytes=3-" {
				handlerErr <- fmt.Errorf("unexpected Range header %q", got)
				http.Error(w, "bad range", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Range", "bytes 4-7/8")
			w.Header().Set("Content-Length", "4")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte("defg"))
			handlerErr <- nil
		}))
		defer server.Close()

		g := Default()
		g.Client = *server.Client()

		task := newTempDownloadTask(t, server.URL)
		mustWriteDownloadFile(t, task.Path, "abc")

		err := g.DownloadWithContext(context.Background(), task)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "Content-Range") {
			t.Fatalf("error = %v, want Content-Range detail", err)
		}
		if got := mustReadDownloadFile(t, task.Path); got != "abc" {
			t.Fatalf("download content = %q, want %q", got, "abc")
		}
		if _, err := os.Stat(task.Path + ".ok"); !os.IsNotExist(err) {
			t.Fatalf(".ok marker should not exist: %v", err)
		}
		requireDownloadNoHandlerError(t, handlerErr)
	})
}

func TestDownloadWithContextRestartsOnIgnoredRange(t *testing.T) {
	handlerErr := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Range"); got != "bytes=3-" {
			handlerErr <- fmt.Errorf("unexpected Range header %q", got)
			http.Error(w, "bad range", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Length", "8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("abcdefgh"))
		handlerErr <- nil
	}))
	defer server.Close()

	g := Default()
	g.Client = *server.Client()

	task := newTempDownloadTask(t, server.URL)
	mustWriteDownloadFile(t, task.Path, "abc")

	if err := g.DownloadWithContext(context.Background(), task); err != nil {
		t.Fatalf("DownloadWithContext returned error: %v", err)
	}
	if got := mustReadDownloadFile(t, task.Path); got != "abcdefgh" {
		t.Fatalf("download content = %q, want %q", got, "abcdefgh")
	}
	if _, err := os.Stat(task.Path + ".ok"); err != nil {
		t.Fatalf(".ok marker missing: %v", err)
	}
	requireDownloadNoHandlerError(t, handlerErr)
}

func TestDownloadWithContextAcceptsComplete416(t *testing.T) {
	const content = "already-complete"
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if got := r.Header.Get("Range"); got != fmt.Sprintf("bytes=%d-", len(content)) {
			http.Error(w, "bad range", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", len(content)))
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
	}))
	defer server.Close()

	g := Default()
	g.Client = *server.Client()

	task := newTempDownloadTask(t, server.URL)
	mustWriteDownloadFile(t, task.Path, content)

	if err := g.DownloadWithContext(context.Background(), task); err != nil {
		t.Fatalf("DownloadWithContext returned error: %v", err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("request count = %d, want 1", got)
	}
	if got := mustReadDownloadFile(t, task.Path); got != content {
		t.Fatalf("download content = %q, want %q", got, content)
	}
	if _, err := os.Stat(task.Path + ".ok"); err != nil {
		t.Fatalf(".ok marker missing: %v", err)
	}
}

func TestDownloadWithContextRestartsAfterMalformed416(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Header.Get("Range"))
		switch len(requests) {
		case 1:
			w.Header().Set("Content-Range", "not-a-range")
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		case 2:
			if got := r.Header.Get("Range"); got != "" {
				http.Error(w, "unexpected range on restart", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Length", "8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("abcdefgh"))
		default:
			http.Error(w, "too many requests", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	g := Default()
	g.Client = *server.Client()

	task := newTempDownloadTask(t, server.URL)
	mustWriteDownloadFile(t, task.Path, "abc")

	if err := g.DownloadWithContext(context.Background(), task); err != nil {
		t.Fatalf("DownloadWithContext returned error: %v", err)
	}
	if got := strings.Join(requests, ","); got != "bytes=3-," {
		t.Fatalf("ranges = %q, want %q", got, "bytes=3-,")
	}
	if got := mustReadDownloadFile(t, task.Path); got != "abcdefgh" {
		t.Fatalf("download content = %q, want %q", got, "abcdefgh")
	}
	if _, err := os.Stat(task.Path + ".ok"); err != nil {
		t.Fatalf(".ok marker missing: %v", err)
	}
}

func TestDownloadWithContextReturnsMarkerCreationError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "4")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data"))
	}))
	defer server.Close()

	g := Default()
	g.Client = *server.Client()

	task := newTempDownloadTask(t, server.URL)
	if err := os.Mkdir(task.Path+".ok", 0o755); err != nil {
		t.Fatalf("mkdir marker path: %v", err)
	}

	err := g.DownloadWithContext(context.Background(), task)
	if err == nil {
		t.Fatal("expected error")
	}
	if got := mustReadDownloadFile(t, task.Path); got != "data" {
		t.Fatalf("download content = %q, want %q", got, "data")
	}
	info, statErr := os.Stat(task.Path + ".ok")
	if statErr != nil {
		t.Fatalf("stat marker path: %v", statErr)
	}
	if !info.IsDir() {
		t.Fatalf(".ok path type changed, want preexisting directory")
	}
}

func TestDownloadWithContextReturnsTypedStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("unauthorized"))
	}))
	defer server.Close()

	g := Default()
	g.Client = *server.Client()

	task := newTempDownloadTask(t, server.URL)
	err := g.DownloadWithContext(context.Background(), task)
	if err == nil {
		t.Fatal("expected error")
	}

	var statusErr *StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("error = %T, want *StatusError", err)
	}
	if !statusErr.AuthenticationRequired() {
		t.Fatalf("AuthenticationRequired = %v, want true", statusErr.AuthenticationRequired())
	}
	if !errors.As(task.Err, &statusErr) {
		t.Fatalf("task.Err = %T, want *StatusError", task.Err)
	}
	if _, statErr := os.Stat(task.Path + ".ok"); !os.IsNotExist(statErr) {
		t.Fatalf(".ok marker should not exist: %v", statErr)
	}
}

func TestDownloadWithContextRejectsUnknownLength200WithoutMarker(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		_, _ = w.Write([]byte("abcdefgh"))
	}))
	defer server.Close()

	g := Default()
	g.Client = *server.Client()

	task := newTempDownloadTask(t, server.URL)
	err := g.DownloadWithContext(context.Background(), task)
	if err != nil {
		t.Fatalf("DownloadWithContext returned error: %v", err)
	}
	if got := mustReadDownloadFile(t, task.Path); got != "abcdefgh" {
		t.Fatalf("download content = %q, want %q", got, "abcdefgh")
	}
	if _, statErr := os.Stat(task.Path + ".ok"); !os.IsNotExist(statErr) {
		t.Fatalf(".ok marker should not exist: %v", statErr)
	}
}

func TestDownloadWithContextRejectsNilTaskWithoutPanic(t *testing.T) {
	g := Default()

	err := g.DownloadWithContext(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "nil download task") {
		t.Fatalf("error = %v, want nil task error", err)
	}

	err = DownloadWithContext(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "nil download task") {
		t.Fatalf("global error = %v, want nil task error", err)
	}
}

func TestDownloadWithContextRejectsShortUnknownLength206Span(t *testing.T) {
	task := newTempDownloadTask(t, "https://example.test/range")
	mustWriteDownloadFile(t, task.Path, "abc")

	g := Default()
	g.Client = http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("Range"); got != "bytes=3-" {
				return nil, fmt.Errorf("unexpected range %q", got)
			}
			return &http.Response{
				StatusCode:    http.StatusPartialContent,
				ContentLength: -1,
				Header:        http.Header{"Content-Range": []string{"bytes 3-5/8"}},
				Body:          io.NopCloser(strings.NewReader("de")),
				Request:       req,
			}, nil
		}),
	}

	err := g.DownloadWithContext(context.Background(), task)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "declared span") {
		t.Fatalf("error = %v, want span validation", err)
	}
	if got := mustReadDownloadFile(t, task.Path); got != "abcde" {
		t.Fatalf("download content = %q, want %q", got, "abcde")
	}
	if _, statErr := os.Stat(task.Path + ".ok"); !os.IsNotExist(statErr) {
		t.Fatalf(".ok marker should not exist: %v", statErr)
	}
}

func TestDownloadWithContextPropagatesBodyCloseError(t *testing.T) {
	t.Run("success-before-marker", func(t *testing.T) {
		task := newTempDownloadTask(t, "https://example.test/file")
		closeErr := errors.New("close failed")

		g := Default()
		g.Client = http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode:    http.StatusOK,
					ContentLength: 4,
					Header:        http.Header{"Content-Length": []string{"4"}},
					Body: &errorCloseReadCloser{
						Reader: strings.NewReader("data"),
						Err:    closeErr,
					},
					Request: req,
				}, nil
			}),
		}

		err := g.DownloadWithContext(context.Background(), task)
		if !errors.Is(err, closeErr) {
			t.Fatalf("error = %v, want %v", err, closeErr)
		}
		if got := mustReadDownloadFile(t, task.Path); got != "data" {
			t.Fatalf("download content = %q, want %q", got, "data")
		}
		if _, statErr := os.Stat(task.Path + ".ok"); !os.IsNotExist(statErr) {
			t.Fatalf(".ok marker should not exist: %v", statErr)
		}
	})

	t.Run("typed-status-through-join", func(t *testing.T) {
		task := newTempDownloadTask(t, "https://example.test/protected")
		closeErr := errors.New("close unauthorized")

		g := Default()
		g.Client = http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusUnauthorized,
					Body: &errorCloseReadCloser{
						Reader: strings.NewReader("blocked"),
						Err:    closeErr,
					},
					Request: req,
				}, nil
			}),
		}

		err := g.DownloadWithContext(context.Background(), task)
		if err == nil {
			t.Fatal("expected error")
		}
		var statusErr *StatusError
		if !errors.As(err, &statusErr) {
			t.Fatalf("error = %T, want *StatusError", err)
		}
		if !errors.Is(err, closeErr) {
			t.Fatalf("error = %v, want joined close error %v", err, closeErr)
		}
		if _, statErr := os.Stat(task.Path + ".ok"); !os.IsNotExist(statErr) {
			t.Fatalf(".ok marker should not exist: %v", statErr)
		}
	})
}

func TestDownloadWithContextClearsStaleTaskErrOnSkip(t *testing.T) {
	task := newTempDownloadTask(t, "https://example.test/skip")
	mustWriteDownloadFile(t, task.Path, "done")
	if err := os.WriteFile(task.Path+".ok", []byte("ok"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	task.Err = errors.New("stale")

	g := Default()
	err := g.DownloadWithContext(context.Background(), task)
	if err != nil {
		t.Fatalf("DownloadWithContext returned error: %v", err)
	}
	if task.Err != nil {
		t.Fatalf("task.Err = %v, want nil", task.Err)
	}
}

func TestShouldSkipUsesOkMarker(t *testing.T) {
	var headRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			headRequests.Add(1)
			w.Header().Set("Content-Length", "3")
			return
		}
		http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
	}))
	defer server.Close()

	g := Default()
	g.Client = *server.Client()

	task := newTempDownloadTask(t, server.URL)
	mustWriteDownloadFile(t, task.Path, "abc")

	if skip := g.shouldSkip(context.Background(), task); skip {
		t.Fatal("shouldSkip returned true without .ok marker")
	}
	if got := headRequests.Load(); got != 0 {
		t.Fatalf("HEAD request count = %d, want 0", got)
	}

	if err := os.WriteFile(task.Path+".ok", []byte("ok"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	if skip := g.shouldSkip(context.Background(), task); !skip {
		t.Fatal("shouldSkip returned false with .ok marker")
	}
	if got := headRequests.Load(); got != 0 {
		t.Fatalf("HEAD request count after marker = %d, want 0", got)
	}
}

func newTempDownloadTask(t *testing.T, link string) *DownloadTask {
	t.Helper()
	return NewDownloadTask(link, filepath.Join(t.TempDir(), "download.bin"))
}

func mustWriteDownloadFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func mustReadDownloadFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	return string(body)
}

func requireDownloadNoHandlerError(t *testing.T, errCh <-chan error) {
	t.Helper()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	default:
	}
}

type errorCloseReadCloser struct {
	Reader io.Reader
	Err    error
}

func (e *errorCloseReadCloser) Read(p []byte) (int, error) {
	return e.Reader.Read(p)
}

func (e *errorCloseReadCloser) Close() error {
	return e.Err
}
