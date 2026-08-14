# Download and Concurrency Correctness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make full downloads, resumed downloads, batch resource downloads, and concurrent ebook chapter retrieval deterministic, cancellable, and safe from silent corruption or data races.

**Architecture:** Add one streaming HTTP primitive that accepts context, headers, and expected status codes. Keep existing public entry points as compatibility wrappers while moving file writes to same-directory `.download` files that are validated before rename. Batch tasks use `errgroup.WithContext`; ebook results are preallocated and written by index.

**Tech Stack:** Go 1.23 standard library, existing `golang.org/x/sync/errgroup` and `semaphore`, `httptest.Server`.

---

## Files

- Modify: `backend/request/http.go`
- Create: `backend/request/http_test.go`
- Modify: `backend/downloader/downloader.go`
- Create: `backend/downloader/downloader_test.go`
- Modify: `backend/utils/ffmpeg.go`
- Create: `backend/utils/ffmpeg_test.go`
- Modify: `backend/request/download.go`
- Create: `backend/request/download_test.go`
- Modify: `backend/utils/html2epub.go`
- Modify: `backend/services/requester.go`
- Modify: `backend/services/ebook.go`
- Modify: `backend/app/ebook.go`
- Create: `backend/app/ebook_test.go`

## Required invariants

- A non-empty partial file is appended only after a valid `206` whose `Content-Range` starts at the local byte count.
- A server that ignores Range and returns `200` causes a truncate-and-restart, never an append.
- A `416` is accepted only when its advertised total equals the local partial size.
- The final path appears only after close, size validation, and successful rename.
- Cancellation closes response bodies and stops tasks that have not started.
- Concurrent ebook completion order never changes chapter output order.

### Task 1: Add a streaming, status-aware HTTP request primitive

- [ ] Add failing tests to `backend/request/http_test.go` for Range forwarding, HEAD size, unexpected status, and context cancellation.

```go
func TestGetWithOptionsSendsRangeHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Range"); got != "bytes=3-5" {
			t.Fatalf("Range = %q, want bytes=3-5", got)
		}
		w.Header().Set("Content-Range", "bytes 3-5/10")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("345"))
	}))
	defer server.Close()

	header := make(http.Header)
	header.Set("Range", "bytes=3-5")
	body, response, err := GetWithOptions(context.Background(), server.URL, GetOptions{
		Header: header, ExpectedStatus: []int{http.StatusPartialContent},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	if response.StatusCode != http.StatusPartialContent {
		t.Fatalf("status = %d", response.StatusCode)
	}
}
```

Add equivalent focused tests named:

```text
TestSizeWithHeaderUsesHEAD
TestGetWithOptionsRejectsUnexpectedStatus
TestGetWithOptionsHonoursCancellation
```

- [ ] Run the focused tests and confirm they fail because the new API does not exist.

```bash
go test ./backend/request -run 'Test(GetWithOptions|SizeWithHeader)' -count=1
```

Expected: compile failure referring to `GetOptions`, `GetWithOptions`, or `SizeWithHeader`.

- [ ] Implement the API in `backend/request/http.go` without buffering the response body:

```go
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

func GetWithOptions(ctx context.Context, rawURL string, opts GetOptions) (io.ReadCloser, *http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header = opts.Header.Clone()
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", UserAgent)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	for _, code := range opts.ExpectedStatus {
		if resp.StatusCode == code {
			return resp.Body, resp, nil
		}
	}
	_ = resp.Body.Close()
	return nil, resp, &StatusError{Code: resp.StatusCode, URL: rawURL}
}

func Get(rawURL string) (io.ReadCloser, error) {
	body, _, err := GetWithOptions(context.Background(), rawURL, GetOptions{
		ExpectedStatus: []int{http.StatusOK},
	})
	return body, err
}
```

Implement `Head`/`SizeWithHeader` with `http.NewRequestWithContext`, cloned headers, and an explicitly closed body. Return `int64`, not `int`, internally; retain `Size(string) (int, error)` only as a checked compatibility wrapper.

- [ ] Run the focused tests and formatting.

```bash
gofmt -w backend/request/http.go backend/request/http_test.go
go test ./backend/request -run 'Test(GetWithOptions|SizeWithHeader)' -count=1
```

Expected: PASS.

- [ ] Commit the HTTP contract.

```bash
git add backend/request/http.go backend/request/http_test.go
git commit -m "Make download requests carry their declared range contract" \
  -m "The streaming request primitive now forwards headers, honours cancellation, and rejects unexpected response codes." \
  -m "Constraint: Existing callers of request.Get remain source compatible.
Rejected: Extending the old buffered Resty helper | large media responses must remain streaming
Confidence: high
Scope-risk: moderate
Directive: Do not append bytes unless the caller has validated response status and range metadata.
Tested: Range forwarding, HEAD size, status rejection, and cancellation"
```

### Task 2: Rebuild `Save` around validated partial files and atomic completion

- [ ] Add `backend/downloader/downloader_test.go` with one local server helper that can return full content, valid ranges, ignored ranges, malformed ranges, and `416`.

Create these tests first:

```text
TestSaveWithContextDownloadsToTemporaryFileThenRenames
TestSaveWithContextResumesFromValidPartialContent
TestSaveWithContextRestartsWhenServerIgnoresRange
TestSaveWithContextRejectsMismatchedContentRange
TestSaveWithContextAcceptsCompletePartialAfter416
TestSaveWithContextRejectsWrongSizeAfter416
TestSaveWithContextRejectsTruncatedBody
TestSaveWithContextStopsAtContextDeadline
TestSaveWithContextPreservesPartialOnCancellation
TestSaveWithContextDoesNotRetry401403Or496
TestDownloadStopsBeforeMergeWhenOnePartFails
```

The ignored-Range assertion must verify exact final bytes and ensure the output is not `partial + full`:

```go
if got := string(mustReadFile(t, finalPath)); got != content {
	t.Fatalf("final content = %q, want %q", got, content)
}
if _, err := os.Stat(finalPath + ".download"); !os.IsNotExist(err) {
	t.Fatalf("completed temporary file still exists: %v", err)
}
```

- [ ] Confirm the new tests fail against the existing implementation.

```bash
go test ./backend/downloader -run 'Test(SaveWithContext|DownloadStops)' -count=1
```

Expected: compile failures for the new context API, followed by behavioral failures as it is introduced.

- [ ] Introduce compatibility wrappers and explicit options in `backend/downloader/downloader.go`:

```go
type SaveOptions struct {
	ChunkSizeBytes int64
	MaxRetries     int
	RetryDelay     time.Duration
	Header         http.Header
}

func Save(urlData URL, fileName string, chunkSizeMB int) error {
	return SaveWithContext(context.Background(), urlData, fileName, SaveOptions{
		ChunkSizeBytes: int64(chunkSizeMB) * 1024 * 1024,
		MaxRetries:     3,
		RetryDelay:     time.Second,
	})
}
```

Add `func SaveWithContext(ctx context.Context, urlData URL, fileName string, opts SaveOptions) error` and implement it by resolving final/temporary paths, inspecting the partial size, issuing and validating the correct request, closing/syncing the file, validating the total size, and finally renaming. Never defer a rename whose error would be discarded.

The implementation must replace the comment-only body above with real code. Keep helpers small and testable:

```go
type contentRange struct {
	Start int64
	End   int64
	Total int64
}

func parseContentRange(value string) (contentRange, error)
func validateResumeResponse(resp *http.Response, localSize int64) (restart bool, complete bool, err error)
func copyResponse(ctx context.Context, dst io.Writer, src io.Reader) (int64, error)
func finalizeDownload(tempPath, finalPath string, expectedSize int64) error
```

- [ ] Implement the exact response decisions:

```go
switch resp.StatusCode {
case http.StatusPartialContent:
	parsed, err := parseContentRange(resp.Header.Get("Content-Range"))
	if err != nil || parsed.Start != localSize {
		return fmt.Errorf("resume Content-Range does not start at %d: %w", localSize, err)
	}
	_, err = file.Seek(0, io.SeekEnd)
	return false, false, err
case http.StatusOK:
	if err := file.Truncate(0); err != nil {
		return false, false, err
	}
	_, err := file.Seek(0, io.SeekStart)
	return true, false, err
case http.StatusRequestedRangeNotSatisfiable:
	total, err := parseUnsatisfiedTotal(resp.Header.Get("Content-Range"))
	if err == nil && total == localSize {
		return false, true, nil
	}
	return true, false, nil
default:
	return false, false, &request.StatusError{Code: resp.StatusCode, URL: resp.Request.URL.String()}
}
```

For a restart, close the ranged response, truncate, and issue a new request without `Range`. Do not consume a `200 OK` response into an append-positioned file. The authentication/verification test must table-test 401, 403, and 496 and assert exactly one request for each status.

- [ ] Validate bytes before rename and surface every filesystem error:

```go
func finalizeDownload(tempPath, finalPath string, expectedSize int64) error {
	info, err := os.Stat(tempPath)
	if err != nil {
		return err
	}
	if expectedSize >= 0 && info.Size() != expectedSize {
		return fmt.Errorf("download size %d does not match expected %d", info.Size(), expectedSize)
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		return fmt.Errorf("rename completed download: %w", err)
	}
	return nil
}
```

Close and `Sync` the file before calling this helper. Treat 401/403 as login invalidation, 496 as official-site verification required, and all filesystem errors as non-retryable. Retry only interrupted reads and explicitly transient transport/5xx errors, never beyond `MaxRetries`.

- [ ] Update multipart download internals to use `errgroup.WithContext` so the first part failure prevents merge. Keep `Download(v, stream, path)` as a wrapper and add `DownloadWithContext(ctx, v, stream, path)` for app callers.

- [ ] Before changing FFmpeg helpers, add `backend/utils/ffmpeg_test.go::TestRunMergeCommandStopsOnContextCancel`. Use `os.Args[0]` as a helper subprocess selected by an environment flag, cancel its context, and assert the command returns `context.Canceled` instead of waiting for the helper timeout. This keeps the test cross-platform and does not require FFmpeg.

- [ ] Add context-aware merge wrappers in `backend/utils/ffmpeg.go` and use them from `DownloadWithContext`:

```go
func MergeAudio(paths []string, mergedFilePath string) error {
	return MergeAudioContext(context.Background(), paths, mergedFilePath)
}

func MergeAudioContext(ctx context.Context, paths []string, mergedFilePath string) error {
	args := []string{"-y"}
	for _, path := range paths {
		args = append(args, "-i", path)
	}
	args = append(args, "-c:v", "copy", mergedFilePath)
	err := runMergeCmd(exec.CommandContext(ctx, FfmpegDir, args...), paths, "")
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}
```

Add the equivalent `MergeAudioAndVideoContext` wrapper. On cancellation, preserve downloaded part files and do not run success cleanup.

- [ ] Run focused tests, the race detector, and formatting.

```bash
gofmt -w backend/downloader/downloader.go backend/downloader/downloader_test.go backend/utils/ffmpeg.go backend/utils/ffmpeg_test.go
go test ./backend/downloader ./backend/utils -count=1
go test -race ./backend/downloader ./backend/utils -count=1
```

Expected: PASS; no final file is present after a failed or cancelled transfer, while a valid `.download` partial remains resumable.

- [ ] Commit the atomic download path.

```bash
git add backend/downloader/downloader.go backend/downloader/downloader_test.go backend/utils/ffmpeg.go backend/utils/ffmpeg_test.go
git commit -m "Prevent resumed media from silently appending the wrong bytes" \
  -m "Range responses are validated, ignored ranges restart safely, and completed files become visible only after size checks and close." \
  -m "Constraint: Completed user files are never rewritten by migration; only new tasks and .download partials use the new contract.
Rejected: Trusting a successful HTTP call | status alone cannot prove the requested byte interval was returned
Confidence: high
Scope-risk: broad
Directive: Preserve the 200, 206, and 416 branch tests when modifying downloader behavior.
Tested: Full, resumed, ignored-range, malformed-range, 416, cancellation, multipart failure, and race tests"
```

### Task 3: Propagate batch task failures and cancellation

- [ ] Add failing tests in `backend/request/download_test.go`:

```text
TestBatchRejectsNonPositiveConcurrency
TestBatchPropagatesTaskFailure
TestBatchStopsWaitingAfterCancellation
TestDownloadWithContextResumesOnlyOnValid206
TestDownloadWithContextRestartsOnIgnoredRange
TestShouldSkipUsesOkMarker
```

- [ ] Change the package and receiver signatures:

```go
func Batch(ctx context.Context, tasks *DownloadTasks, concurrent int, eachTimeout time.Duration) error {
	return one.Batch(ctx, tasks, concurrent, eachTimeout)
}

func (g *GetDownload) Batch(ctx context.Context, tasks *DownloadTasks, concurrent int, eachTimeout time.Duration) error {
	if concurrent <= 0 {
		return fmt.Errorf("concurrent must be greater than zero")
	}
	group, groupCtx := errgroup.WithContext(ctx)
	sema := semaphore.NewWeighted(int64(concurrent))
	tasks.ForEach(func(task *DownloadTask) {
		task := task
		group.Go(func() error {
			if err := sema.Acquire(groupCtx, 1); err != nil {
				task.Err = err
				return err
			}
			defer sema.Release(1)
			taskCtx, cancel := context.WithTimeout(groupCtx, eachTimeout)
			defer cancel()
			task.Err = g.DownloadWithContext(taskCtx, task)
			return task.Err
		})
	})
	return group.Wait()
}
```

- [ ] Apply the same 200/206/416 rules from Task 2 inside `DownloadWithContext`, and create the `.ok` marker only after close and expected-size validation. A `416` must not truncate a complete local file.

- [ ] Update `backend/utils/html2epub.go` so `saveImages` returns `(map[string]string, error)`, its `add` caller checks that error, and a failed batch aborts the export rather than only logging item errors:

```go
images, err := h.saveImages(doc)
if err != nil {
	return err
}
```

```go
if err = request.Batch(context.Background(), tasks, 3, 2*time.Minute); err != nil {
	return downloads, fmt.Errorf("download EPUB assets: %w", err)
}
return downloads, nil
```

The first block is the `add` caller; the second is the replacement tail inside `saveImages`. Thread an existing caller context where one is available; use `context.Background()` only at the current API boundary and document it for later widening.

- [ ] Run focused tests and all affected packages.

```bash
gofmt -w backend/request/download.go backend/request/download_test.go backend/utils/html2epub.go
go test ./backend/request ./backend/utils -count=1
go test -race ./backend/request -count=1
```

Expected: PASS, including immediate error for concurrency zero and propagated HTTP failure.

- [ ] Commit batch semantics.

```bash
git add backend/request/download.go backend/request/download_test.go backend/utils/html2epub.go
git commit -m "Make failed resource batches fail their parent export" \
  -m "Batch acquisition, task errors, timeouts, and cancellation now return through one errgroup-owned path." \
  -m "Constraint: Existing task.Err remains populated for per-item diagnostics.
Confidence: high
Scope-risk: moderate
Directive: Never discard Acquire or Wait errors in batch orchestration.
Tested: Parameter validation, task failure, cancellation, resume, ignored range, and skip marker"
```

### Task 4: Remove ebook result races and make retry waits cancellable

- [ ] Add a pure coordinator test seam in `backend/app/ebook_test.go` and write failing tests:

```go
type chapterFetcher func(context.Context, services.EbookOrders, int) (*utils.SvgContent, error)
```

Create `TestFetchEbookChaptersPreservesOrder` with two channels so chapter two completes before chapter one, `TestFetchEbookChaptersReturnsFirstError` with a named sentinel error, and `TestFetchEbookChaptersCancelsPendingWork` with a blocked fetch selecting on `ctx.Done()`.

The order assertion must compare `ChapterID` values at indexes 0 and 1, not sort the result after the fact.

- [ ] Implement the coordinator in `backend/app/ebook.go`:

```go
func fetchEbookChapters(ctx context.Context, orders []services.EbookOrders, limit int, fetch chapterFetcher) (utils.SvgContents, error) {
	results := make(utils.SvgContents, len(orders))
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(limit)
	for index, order := range orders {
		index, order := index, order
		group.Go(func() error {
			result, err := fetch(groupCtx, order, index)
			if err != nil {
				return err
			}
			results[index] = result
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}
```

- [ ] Add context-aware wrappers in `backend/services/ebook.go` and `backend/services/requester.go`:

```go
func (s *Service) EbookPages(chapterID, token string, index, count, offset int) (*EbookPage, error) {
	return s.EbookPagesContext(context.Background(), chapterID, token, index, count, offset)
}

func (s *Service) EbookPagesContext(ctx context.Context, chapterID, token string, index, count, offset int) (*EbookPage, error)
func (s *Service) reqEbookPagesContext(ctx context.Context, chapterID, token string, index, count, offset int) (io.ReadCloser, error)
```

Use `s.client.R().SetContext(ctx)` in the requester. Replace `time.Sleep` in the ebook limiter/retry path with a cancellable timer helper:

```go
func waitContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
```

Keep the old methods as background-context wrappers so unrelated service callers remain compatible.

- [ ] Change `generateEbookPages` to accept context and update its two callers. Check `ctx.Err()` before cache/network work and before recursive pagination.

```go
func generateEbookPages(ctx context.Context, enid, chapterID, token string, index, count, offset int) ([]string, error)
```

- [ ] Rewrite `EbookPage` to call `fetchEbookChapters`. Emit progress after each fixed-index result succeeds, using `atomic.Int64` for the completed count. Do not append to shared slices or write a shared error variable.

- [ ] Run focused tests and the race detector.

```bash
gofmt -w backend/services/requester.go backend/services/ebook.go backend/app/ebook.go backend/app/ebook_test.go
go test ./backend/app ./backend/services -count=1
go test -race ./backend/app ./backend/services -count=1
```

Expected: PASS; delayed chapter completion still yields input order and no race report.

- [ ] Commit ebook concurrency.

```bash
git add backend/services/requester.go backend/services/ebook.go backend/app/ebook.go backend/app/ebook_test.go
git commit -m "Keep ebook chapter output stable under concurrent retrieval" \
  -m "Each worker owns one result slot, first failure cancels the group, and network/retry waits now observe the caller context." \
  -m "Constraint: The existing process-wide anti-spider limiter remains shared across downloads.
Rejected: Protecting append with a mutex | it would remove the race but preserve completion-order output
Confidence: high
Scope-risk: moderate
Directive: Preserve fixed-index writes and context-aware waits.
Tested: Ordering, first-error propagation, cancellation, package tests, and race detector"
```

### Task 5: Verify the combined download boundary

- [ ] Run the complete offline and race suites.

```bash
go test ./... -count=1
go test -race ./backend/... -count=1
go vet ./...
git diff --check
```

Expected: all commands pass with no external account or service.

- [ ] Verify the original defect pattern no longer exists.

```bash
rg -n 'headers\["Range"\]|context\.TODO\(\)|svgContent = append|err = err1' backend/downloader backend/request backend/app/ebook.go
```

Expected: no ignored Range header path, no batch `context.TODO`, and no concurrent ebook append/shared error assignment. A legitimate local `Range` construction may remain only when it is passed to `GetWithOptions`.

## Completion criteria

- All Range response branches have deterministic local-server tests.
- Partial files cannot be appended after an ignored or mismatched Range.
- Final paths are created only after close and size validation.
- Batch failures reach HTML/EPUB callers.
- Ebook results are fixed-order and pass `go test -race`.
- No public Wails download method has been removed; compatibility wrappers remain where needed.
