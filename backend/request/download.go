package request

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

type GetDownload struct {
	OnEachStart func(t *DownloadTask)
	OnEachStop  func(t *DownloadTask)
	OnEachSkip  func(t *DownloadTask)
	Header      http.Header
	Client      http.Client
}

type DownloadTask struct {
	Link string
	Path string
	Err  error
}

type DownloadTasks struct {
	tasks []*DownloadTask
}

func Default() (g GetDownload) {
	g.Header = make(http.Header)
	g.Header.Set("user-agent", UserAgent)
	return g
}

var one = Default()

func Download(dl *DownloadTask, timeout time.Duration) (err error) {
	return one.Download(dl, timeout)
}

func DownloadWithContext(ctx context.Context, dl *DownloadTask) (err error) {
	return one.DownloadWithContext(ctx, dl)
}

func Batch(ctx context.Context, tasks *DownloadTasks, concurrent int, eachTimeout time.Duration) error {
	return one.Batch(ctx, tasks, concurrent, eachTimeout)
}

func (g *GetDownload) Download(task *DownloadTask, timeout time.Duration) (err error) {
	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()

	return g.DownloadWithContext(ctx, task)
}

func (g *GetDownload) DownloadWithContext(ctx context.Context, task *DownloadTask) (err error) {
	if task == nil {
		return errors.New("nil download task")
	}
	task.Err = nil
	if g.shouldSkip(ctx, task) {
		if g.OnEachSkip != nil {
			g.OnEachSkip(task)
		}
		return
	}
	if g.OnEachStart != nil {
		g.OnEachStart(task)
	}
	defer func() {
		task.Err = err
		if g.OnEachStop != nil {
			g.OnEachStop(task)
		}
	}()

	f, err := os.OpenFile(task.Path, os.O_RDWR|os.O_CREATE, 0766)
	if err != nil {
		return
	}
	defer func() {
		if f == nil {
			return
		}
		err = errors.Join(err, f.Close())
	}()

	localSize, err := currentFileSize(f)
	if err != nil {
		return
	}
	attemptedRestart := false
	lastModified := ""
	expectedSize := int64(-1)

	for {
		req, reqErr := newRequestWithHeader(ctx, http.MethodGet, task.Link, g.Header)
		if reqErr != nil {
			return reqErr
		}
		if localSize > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", localSize))
		}

		rsp, rspErr := g.Client.Do(req)
		if rspErr != nil {
			return rspErr
		}

		restart, complete, rspExpectedSize, rspLastModified, handleErr := g.handleDownloadResponse(f, rsp, localSize)
		if handleErr != nil {
			return handleErr
		}
		lastModified = rspLastModified
		if rspExpectedSize >= 0 {
			expectedSize = rspExpectedSize
		}
		if restart {
			if attemptedRestart {
				return fmt.Errorf("cannot recover from invalid ranged response")
			}
			attemptedRestart = true
			localSize = 0
			continue
		}
		if complete {
			break
		}

		localSize, err = currentFileSize(f)
		if err != nil {
			return err
		}
		break
	}

	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	f = nil

	localSize, err = fileSize(task.Path)
	if err != nil {
		return err
	}
	if expectedSize < 0 {
		return nil
	}
	if localSize != expectedSize {
		return fmt.Errorf("download size %d does not match expected %d", localSize, expectedSize)
	}
	if lastModified != "" {
		if mt, parseErr := http.ParseTime(lastModified); parseErr == nil {
			_ = os.Chtimes(task.Path, mt, mt)
		}
	}
	ok, err := os.Create(task.Path + ".ok")
	if err != nil {
		return err
	}
	return ok.Close()
}

func (g *GetDownload) Batch(ctx context.Context, tasks *DownloadTasks, concurrent int, eachTimeout time.Duration) error {
	if concurrent <= 0 {
		return fmt.Errorf("concurrent must be greater than zero")
	}
	if tasks == nil {
		return nil
	}

	group, groupCtx := errgroup.WithContext(ctx)
	sema := semaphore.NewWeighted(int64(concurrent))

	tasks.ForEach(func(task *DownloadTask) {
		taskCopy := task
		group.Go(func() error {
			if taskCopy == nil {
				return errors.New("nil download task")
			}
			if err := sema.Acquire(groupCtx, 1); err != nil {
				taskCopy.Err = err
				return err
			}
			defer sema.Release(1)
			if err := groupCtx.Err(); err != nil {
				taskCopy.Err = err
				return err
			}

			taskCtx := groupCtx
			cancel := func() {}
			if eachTimeout > 0 {
				taskCtx, cancel = context.WithTimeout(groupCtx, eachTimeout)
			}
			defer cancel()

			taskCopy.Err = g.DownloadWithContext(taskCtx, taskCopy)
			return taskCopy.Err
		})
	})

	return group.Wait()
}

func (g *GetDownload) shouldSkip(ctx context.Context, task *DownloadTask) (skip bool) {
	_ = ctx
	okInfo, err := os.Stat(task.Path + ".ok")
	if err != nil || okInfo.IsDir() {
		return false
	}
	fileInfo, err := os.Stat(task.Path)
	if err != nil || fileInfo.IsDir() {
		return false
	}
	return true
}

func NewDownloadTask(link, path string) *DownloadTask {
	return &DownloadTask{
		Link: link,
		Path: path,
	}
}

func (d *DownloadTasks) Add(link, path string) {
	for _, t := range d.tasks {
		if t.Link == link && t.Path == path {
			return
		}
	}
	d.tasks = append(d.tasks, NewDownloadTask(link, path))
}

func (d *DownloadTasks) ForEach(f func(t *DownloadTask)) {
	for _, t := range d.tasks {
		f(t)
	}
}

func NewDownloadTasks() *DownloadTasks {
	return &DownloadTasks{}
}

type contentRange struct {
	Start int64
	End   int64
	Total int64
}

func currentFileSize(f *os.File) (int64, error) {
	info, err := f.Stat()
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func fileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func (g *GetDownload) handleDownloadResponse(f *os.File, rsp *http.Response, localSize int64) (restart bool, complete bool, expectedSize int64, lastModified string, err error) {
	lastModified = rsp.Header.Get("Last-Modified")
	expectedSize = -1
	closeWithErr := func(baseErr error) error {
		closeErr := rsp.Body.Close()
		if closeErr == nil {
			return baseErr
		}
		if baseErr == nil {
			return closeErr
		}
		return errors.Join(baseErr, closeErr)
	}

	switch rsp.StatusCode {
	case http.StatusPartialContent:
		rng, parseErr := parseContentRange(rsp.Header.Get("Content-Range"))
		if parseErr != nil {
			return false, false, -1, lastModified, closeWithErr(fmt.Errorf("resume Content-Range does not start at %d: %w", localSize, parseErr))
		}
		if rng.Start != localSize {
			return false, false, -1, lastModified, closeWithErr(fmt.Errorf("resume Content-Range does not start at %d", localSize))
		}
		span := rng.End - rng.Start + 1
		if rsp.ContentLength >= 0 && rsp.ContentLength != span {
			return false, false, -1, lastModified, closeWithErr(fmt.Errorf("resume Content-Length %d does not match Content-Range span %d", rsp.ContentLength, span))
		}
		if _, err := f.Seek(0, io.SeekEnd); err != nil {
			return false, false, -1, lastModified, closeWithErr(err)
		}
		sizeBeforeCopy := localSize
		if _, err := io.Copy(f, rsp.Body); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return false, false, -1, lastModified, closeWithErr(err)
			}
			return false, false, -1, lastModified, closeWithErr(fmt.Errorf("copy error: %w", err))
		}
		sizeAfterCopy, err := currentFileSize(f)
		if err != nil {
			return false, false, -1, lastModified, closeWithErr(err)
		}
		actualSpan := sizeAfterCopy - sizeBeforeCopy
		if actualSpan != span {
			return false, false, rng.Total, lastModified, closeWithErr(fmt.Errorf("resume body wrote %d bytes for declared span %d", actualSpan, span))
		}
		if sizeAfterCopy != rng.Total {
			return false, false, rng.Total, lastModified, closeWithErr(fmt.Errorf("download size %d does not match expected %d", sizeAfterCopy, rng.Total))
		}
		return false, false, rng.Total, lastModified, closeWithErr(nil)
	case http.StatusOK:
		if localSize > 0 {
			if err := f.Truncate(0); err != nil {
				return false, false, -1, lastModified, closeWithErr(err)
			}
			if _, err := f.Seek(0, io.SeekStart); err != nil {
				return false, false, -1, lastModified, closeWithErr(err)
			}
			localSize = 0
		}
		if _, err := io.Copy(f, rsp.Body); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return false, false, -1, lastModified, closeWithErr(err)
			}
			return false, false, -1, lastModified, closeWithErr(fmt.Errorf("copy error: %w", err))
		}
		if rsp.ContentLength >= 0 {
			expectedSize = rsp.ContentLength
		}
		return false, false, expectedSize, lastModified, closeWithErr(nil)
	case http.StatusRequestedRangeNotSatisfiable:
		if localSize == 0 {
			return false, false, -1, lastModified, closeWithErr(fmt.Errorf("unexpected 416 without local partial"))
		}
		total, parseErr := parseUnsatisfiedTotal(rsp.Header.Get("Content-Range"))
		if parseErr == nil && total == localSize {
			return false, true, total, lastModified, closeWithErr(nil)
		}
		if err := f.Truncate(0); err != nil {
			return false, false, -1, lastModified, closeWithErr(err)
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return false, false, -1, lastModified, closeWithErr(err)
		}
		return true, false, -1, lastModified, closeWithErr(nil)
	default:
		return false, false, -1, lastModified, closeWithErr(&StatusError{Code: rsp.StatusCode, URL: taskURL(rsp)})
	}
}

func taskURL(rsp *http.Response) string {
	if rsp.Request == nil || rsp.Request.URL == nil {
		return ""
	}
	return rsp.Request.URL.String()
}

func parseContentRange(value string) (contentRange, error) {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "bytes ") {
		return contentRange{}, fmt.Errorf("invalid Content-Range %q", value)
	}
	rangeAndTotal := strings.TrimPrefix(trimmed, "bytes ")
	parts := strings.Split(rangeAndTotal, "/")
	if len(parts) != 2 {
		return contentRange{}, fmt.Errorf("invalid Content-Range %q", value)
	}
	rangePart := strings.Split(parts[0], "-")
	if len(rangePart) != 2 {
		return contentRange{}, fmt.Errorf("invalid Content-Range %q", value)
	}

	start, err := parseRangeNumber(rangePart[0], value)
	if err != nil {
		return contentRange{}, err
	}
	end, err := parseRangeNumber(rangePart[1], value)
	if err != nil {
		return contentRange{}, err
	}
	total, err := parseRangeNumber(parts[1], value)
	if err != nil {
		return contentRange{}, err
	}
	if end < start || total <= end {
		return contentRange{}, fmt.Errorf("invalid Content-Range %q", value)
	}
	return contentRange{Start: start, End: end, Total: total}, nil
}

func parseUnsatisfiedTotal(value string) (int64, error) {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "bytes */") {
		return 0, fmt.Errorf("invalid unsatisfied Content-Range %q", value)
	}
	return parseRangeNumber(strings.TrimPrefix(trimmed, "bytes */"), value)
}

func parseRangeNumber(value string, whole string) (int64, error) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("invalid Content-Range %q", whole)
	}
	return parsed, nil
}
