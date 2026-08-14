package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type ebookLimiterSnapshot struct {
	limiter         *requestLimiter
	lastRequestTime time.Time
	failures        int
	inCooldown      bool
}

func snapshotEbookRequestState(t *testing.T) func() {
	t.Helper()

	antispiderMutex.Lock()
	snapshot := ebookLimiterSnapshot{
		limiter:         globalLimiter,
		lastRequestTime: lastRequestTime,
		failures:        consecutiveFailures,
		inCooldown:      antispiderCooldown,
	}
	antispiderMutex.Unlock()

	return func() {
		antispiderMutex.Lock()
		globalLimiter = snapshot.limiter
		lastRequestTime = snapshot.lastRequestTime
		consecutiveFailures = snapshot.failures
		antispiderCooldown = snapshot.inCooldown
		antispiderMutex.Unlock()
	}
}

func TestWaitContextImmediateCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err := waitContext(ctx, 5*time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want %v", err, context.Canceled)
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("elapsed = %v, want prompt cancellation", elapsed)
	}
}

func TestWaitContextCancelsInFlight(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	start := time.Now()
	go func() {
		errCh <- waitContext(ctx, 5*time.Second)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want %v", err, context.Canceled)
		}
		if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
			t.Fatalf("elapsed = %v, want prompt cancellation", elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("waitContext did not stop after cancellation")
	}
}

func TestWithRetryContextCancellationInterruptsBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var calls atomic.Int32
	firstAttempt := make(chan struct{})
	errCh := make(chan error, 1)
	start := time.Now()

	go func() {
		_, err := withRetryContext(ctx, func(ctx context.Context) (int, error) {
			if calls.Add(1) == 1 {
				close(firstAttempt)
			}
			return 0, errors.New("boom")
		}, "chapter-1")
		errCh <- err
	}()

	select {
	case <-firstAttempt:
	case <-time.After(2 * time.Second):
		t.Fatal("first attempt did not run")
	}
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want %v", err, context.Canceled)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("withRetryContext did not stop during backoff")
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("operation count = %d, want 1", got)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("elapsed = %v, want cancellation before 3s backoff", elapsed)
	}
}

func TestWaitForNextRequestContextCancelsCooldown(t *testing.T) {
	restore := snapshotEbookRequestState(t)
	defer restore()

	antispiderMutex.Lock()
	antispiderCooldown = true
	lastRequestTime = time.Now()
	consecutiveFailures = maxConsecutiveFailures
	antispiderMutex.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	start := time.Now()
	go func() {
		errCh <- waitForNextRequestContext(ctx)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want %v", err, context.Canceled)
		}
		if elapsed := time.Since(start); elapsed > time.Second {
			t.Fatalf("elapsed = %v, want prompt cancellation", elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("waitForNextRequestContext did not stop during cooldown")
	}
}

func TestWaitForNextRequestContextCancelsTokenWait(t *testing.T) {
	restore := snapshotEbookRequestState(t)
	defer restore()

	antispiderMutex.Lock()
	antispiderCooldown = false
	lastRequestTime = time.Time{}
	consecutiveFailures = 0
	antispiderMutex.Unlock()
	globalLimiter = &requestLimiter{
		tokens:         0,
		maxTokens:      1,
		refillRate:     0.5,
		lastRefillTime: time.Now(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	start := time.Now()
	go func() {
		errCh <- waitForNextRequestContext(ctx)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want %v", err, context.Canceled)
		}
		if elapsed := time.Since(start); elapsed > time.Second {
			t.Fatalf("elapsed = %v, want cancellation before token refill wait", elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("waitForNextRequestContext did not stop during token wait")
	}
}

func TestRequestLimiterAcquireDoesNotDrainTokenOnCanceledJitter(t *testing.T) {
	limiter := &requestLimiter{
		tokens:         1,
		maxTokens:      1,
		refillRate:     1,
		lastRefillTime: time.Now(),
		jitterFunc: func() time.Duration {
			return time.Hour
		},
	}

	for range 3 {
		ctx, cancel := context.WithCancel(context.Background())
		errCh := make(chan error, 1)
		go func() {
			errCh <- limiter.acquire(ctx)
		}()

		time.Sleep(20 * time.Millisecond)
		cancel()

		select {
		case err := <-errCh:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("error = %v, want %v", err, context.Canceled)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("acquire did not cancel during jitter wait")
		}

		limiter.mutex.Lock()
		tokens := limiter.tokens
		maxTokens := limiter.maxTokens
		limiter.mutex.Unlock()
		if tokens != maxTokens {
			t.Fatalf("tokens after canceled jitter wait = %d, want %d", tokens, maxTokens)
		}
	}
}

func TestRequestLimiterAcquireWaitersRecompeteAfterRefill(t *testing.T) {
	limiter := &requestLimiter{
		tokens:         0,
		maxTokens:      1,
		refillRate:     200,
		lastRefillTime: time.Now(),
		jitterFunc: func() time.Duration {
			return 0
		},
	}

	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	errCh1 := make(chan error, 1)
	errCh2 := make(chan error, 1)
	go func() {
		errCh1 <- limiter.acquire(ctx1)
	}()
	go func() {
		errCh2 <- limiter.acquire(ctx2)
	}()

	var firstErr error
	var secondErr error
	select {
	case firstErr = <-errCh1:
		cancel2()
		select {
		case secondErr = <-errCh2:
		case <-time.After(2 * time.Second):
			t.Fatal("second waiter did not return after cancellation")
		}
	case firstErr = <-errCh2:
		cancel1()
		select {
		case secondErr = <-errCh1:
		case <-time.After(2 * time.Second):
			t.Fatal("second waiter did not return after cancellation")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no waiter acquired a refilled token")
	}

	if firstErr != nil {
		t.Fatalf("first waiter error = %v, want nil", firstErr)
	}
	if !errors.Is(secondErr, context.Canceled) {
		t.Fatalf("second waiter error = %v, want %v", secondErr, context.Canceled)
	}
}

func TestReqEbookPagesContextHonoursCancellation(t *testing.T) {
	handlerErr := make(chan error, 2)
	started := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			handlerErr <- fmt.Errorf("method = %s, want POST", r.Method)
			http.Error(w, "bad method", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/ebk_web_go/v2/get_pages" {
			handlerErr <- fmt.Errorf("path = %s, want /ebk_web_go/v2/get_pages", r.URL.Path)
			http.Error(w, "bad path", http.StatusNotFound)
			return
		}
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			handlerErr <- fmt.Errorf("read request body: %w", err)
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		started <- struct{}{}
		<-r.Context().Done()
		handlerErr <- r.Context().Err()
	}))
	defer server.Close()

	service := NewService(&CookieOptions{})
	service.client.SetBaseURL(server.URL)

	ctx, cancel := context.WithCancel(context.Background())
	type result struct {
		body ioReadCloser
		err  error
	}
	resultCh := make(chan result, 1)
	go func() {
		body, err := service.reqEbookPagesContext(ctx, "chapter-1", "token", 0, 20, 0)
		resultCh <- result{body: body, err: err}
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("request did not reach server")
	}
	cancel()

	select {
	case got := <-resultCh:
		if got.body != nil {
			_ = got.body.Close()
			t.Fatal("expected nil body on cancellation")
		}
		if !errors.Is(got.err, context.Canceled) {
			t.Fatalf("error = %v, want %v", got.err, context.Canceled)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("reqEbookPagesContext did not return after cancellation")
	}

	select {
	case err := <-handlerErr:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not observe request cancellation")
	}

	select {
	case err := <-handlerErr:
		if err != nil {
			t.Fatal(err)
		}
	default:
	}
}

type ioReadCloser interface {
	Close() error
}
