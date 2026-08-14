package downloader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/yann0917/dedao-gui/backend/request"
	"github.com/yann0917/dedao-gui/backend/utils"
	"golang.org/x/sync/errgroup"
)

var errContentLengthMissing = errors.New("Content-Length is not present")

type SaveOptions struct {
	ChunkSizeBytes int64
	MaxRetries     int
	RetryDelay     time.Duration
	Header         http.Header
}

type contentRange struct {
	Start int64
	End   int64
	Total int64
}

// Download download data
func Download(v Datum, stream, path string) error {
	return DownloadWithContext(context.Background(), v, stream, path)
}

// DownloadWithContext downloads data and cancels sibling work when one part fails.
func DownloadWithContext(ctx context.Context, v Datum, stream, path string) error {
	// 按大到小排序
	v.genSortedStreams()

	title := utils.FileName(v.Title, "")
	if v.OrderNum > 0 {
		title = fmt.Sprintf("%03d.%s", v.OrderNum, title)
	}
	filePreName := filepath.Join(path, title)

	if stream == "" {
		if len(v.sortedStreams) == 0 {
			return fmt.Errorf("未找到可下载资源：%s", v.Title)
		}
		stream = v.sortedStreams[0].name
	}
	data, ok := v.Streams[stream]
	if !ok {
		return fmt.Errorf("指定要下载的类型不存在：%s", stream)
	}

	// 判断下载连接是否存在
	if v.Type == "video" && len(data.URLs) == 0 && v.M3U8URL != "" {
		mp4FileName, err := utils.FilePath(filePreName, "mp4", false)
		if err != nil {
			return err
		}
		return utils.MergeAudioContext(ctx, []string{v.M3U8URL}, mp4FileName)
	}

	// 判断下载连接是否存在
	if len(data.URLs) == 0 {
		return nil
	}

	fileName, err := utils.FilePath(filePreName, "mp3", false)
	if err != nil {
		return err
	}

	if v.Type == "audio" && v.M3U8URL != "" {
		fmt.Println(fileName)
		if err := downloadAudioContext(ctx, v.M3U8URL, fileName); err != nil {
			fmt.Println(err)
			return err
		}
	}
	_, mergedFileExists, err := utils.FileSize(fileName)
	if err != nil {
		return err
	}

	// After the merge, the file size has changed, so we do not check whether the size matches
	if mergedFileExists {
		return nil
	}

	saveOpts := SaveOptions{
		ChunkSizeBytes: 1 * 1024 * 1024,
		MaxRetries:     3,
		RetryDelay:     time.Second,
	}

	if len(data.URLs) == 1 {
		return SaveWithContext(ctx, data.URLs[0], filePreName, saveOpts)
	}

	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(10)

	parts := make([]string, len(data.URLs))
	for index, url := range data.URLs {
		partFileName := fmt.Sprintf("%s[%d]", filePreName, index)
		partFilePath, err := utils.FilePath(partFileName, url.Ext, false)
		if err != nil {
			return err
		}
		parts[index] = partFilePath

		url, partFileName := url, partFileName
		group.Go(func() error {
			return SaveWithContext(groupCtx, url, partFileName, saveOpts)
		})
	}

	if err := group.Wait(); err != nil {
		return err
	}

	switch v.Type {
	case "audio":
		return utils.MergeAudioContext(ctx, parts, fileName)
	case "video":
		return utils.MergeAudioAndVideoContext(ctx, parts, fileName)
	default:
		return nil
	}
}

func downloadAudio(m3u8 string, fname string) error {
	return downloadAudioContext(context.Background(), m3u8, fname)
}

func downloadAudioContext(ctx context.Context, m3u8 string, fname string) error {
	return utils.MergeAudioContext(ctx, []string{m3u8}, fname)
}

// Save url file
func Save(urlData URL, fileName string, chunkSizeMB int) error {
	return SaveWithContext(context.Background(), urlData, fileName, SaveOptions{
		ChunkSizeBytes: int64(chunkSizeMB) * 1024 * 1024,
		MaxRetries:     3,
		RetryDelay:     time.Second,
	})
}

func SaveWithContext(ctx context.Context, urlData URL, fileName string, opts SaveOptions) error {
	if opts.MaxRetries < 0 {
		opts.MaxRetries = 0
	}

	expectedSize := int64(urlData.Size)
	if expectedSize == 0 {
		size, err := discoverExpectedSize(ctx, urlData.URL, opts.Header)
		if err != nil {
			return err
		}
		expectedSize = size
	}

	finalPath, err := utils.FilePath(fileName, urlData.Ext, false)
	if err != nil {
		return err
	}
	if complete, err := checkFinalPath(finalPath, expectedSize); err != nil {
		return err
	} else if complete {
		return nil
	}

	tempPath := finalPath + ".download"
	forceUnrangedRestart := false
	retries := 0

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		localSize, err := statFileSize(tempPath)
		if err != nil {
			return err
		}
		if localSize > expectedSize {
			return fmt.Errorf("temporary file size %d exceeds expected %d", localSize, expectedSize)
		}
		requestStartSize := localSize

		file, err := os.OpenFile(tempPath, os.O_CREATE|os.O_RDWR, 0o644)
		if err != nil {
			return err
		}

		if forceUnrangedRestart || localSize == 0 {
			if err := file.Truncate(0); err != nil {
				_ = file.Close()
				return err
			}
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				_ = file.Close()
				return err
			}
		} else if _, err := file.Seek(localSize, io.SeekStart); err != nil {
			_ = file.Close()
			return err
		}

		rangeHeader := buildRangeHeader(localSize, expectedSize, opts.ChunkSizeBytes, !forceUnrangedRestart)
		header := opts.Header.Clone()
		if header == nil {
			header = make(http.Header)
		}
		if rangeHeader != "" {
			header.Set("Range", rangeHeader)
		} else {
			header.Del("Range")
		}

		expectedStatuses := []int{http.StatusOK, http.StatusPartialContent}
		if rangeHeader != "" {
			expectedStatuses = append(expectedStatuses, http.StatusRequestedRangeNotSatisfiable)
		}

		body, response, err := request.GetWithOptions(ctx, urlData.URL, request.GetOptions{
			Header:         header,
			ExpectedStatus: expectedStatuses,
		})
		if err != nil {
			_ = file.Close()
			if shouldRetryDownload(err) && retries < opts.MaxRetries {
				retries++
				if waitErr := waitRetry(ctx, opts.RetryDelay); waitErr != nil {
					return waitErr
				}
				continue
			}
			return err
		}

		restart, complete, parsedRange, err := validateResumeResponse(response, localSize, expectedSize)
		if err != nil {
			_ = body.Close()
			_ = file.Close()
			return err
		}

		if restart {
			closeErr := body.Close()
			if err := file.Truncate(0); err != nil {
				_ = file.Close()
				return err
			}
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				_ = file.Close()
				return err
			}
			if err := file.Sync(); err != nil {
				_ = file.Close()
				return err
			}
			if err := file.Close(); err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
			forceUnrangedRestart = true
			continue
		}
		forceUnrangedRestart = false

		if !complete {
			_, copyErr := copyResponse(ctx, file, body)
			closeErr := body.Close()
			if copyErr != nil {
				_ = file.Sync()
				_ = file.Close()
				if shouldRetryDownload(copyErr) && retries < opts.MaxRetries {
					retries++
					if waitErr := waitRetry(ctx, opts.RetryDelay); waitErr != nil {
						return waitErr
					}
					continue
				}
				return copyErr
			}
			if closeErr != nil {
				_ = file.Close()
				if shouldRetryDownload(closeErr) && retries < opts.MaxRetries {
					retries++
					if waitErr := waitRetry(ctx, opts.RetryDelay); waitErr != nil {
						return waitErr
					}
					continue
				}
				return closeErr
			}
		} else if err := body.Close(); err != nil {
			_ = file.Close()
			return err
		}

		if err := file.Sync(); err != nil {
			_ = file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}

		localSize, err = statFileSize(tempPath)
		if err != nil {
			return err
		}
		if localSize > expectedSize {
			return fmt.Errorf("temporary file size %d exceeds expected %d", localSize, expectedSize)
		}
		if response.StatusCode == http.StatusPartialContent {
			expectedSpan := parsedRange.End - parsedRange.Start + 1
			actualSpan := localSize - requestStartSize
			switch {
			case actualSpan < expectedSpan:
				shortErr := fmt.Errorf("resume body shorter than declared span: %w", io.ErrUnexpectedEOF)
				if shouldRetryDownload(shortErr) && retries < opts.MaxRetries {
					retries++
					if waitErr := waitRetry(ctx, opts.RetryDelay); waitErr != nil {
						return waitErr
					}
					continue
				}
				return shortErr
			case actualSpan > expectedSpan:
				return fmt.Errorf("resume body wrote %d bytes for declared span %d", actualSpan, expectedSpan)
			}
		}
		if localSize > requestStartSize {
			retries = 0
		}
		if localSize == expectedSize {
			return finalizeDownload(tempPath, finalPath, expectedSize)
		}
		if response.StatusCode == http.StatusOK {
			return fmt.Errorf("download size %d does not match expected %d", localSize, expectedSize)
		}
	}
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

func validateResumeResponse(resp *http.Response, localSize, expectedSize int64) (restart bool, complete bool, parsed contentRange, err error) {
	switch resp.StatusCode {
	case http.StatusPartialContent:
		parsed, parseErr := parseContentRange(resp.Header.Get("Content-Range"))
		if parseErr != nil {
			return false, false, contentRange{}, fmt.Errorf("resume Content-Range does not start at %d: %w", localSize, parseErr)
		}
		if parsed.Start != localSize {
			return false, false, contentRange{}, fmt.Errorf("resume Content-Range does not start at %d", localSize)
		}
		if parsed.Total != expectedSize {
			return false, false, contentRange{}, fmt.Errorf("resume Content-Range total %d does not match expected total %d", parsed.Total, expectedSize)
		}
		if resp.ContentLength >= 0 && resp.ContentLength != parsed.End-parsed.Start+1 {
			return false, false, contentRange{}, fmt.Errorf("resume Content-Length %d does not match Content-Range span %d", resp.ContentLength, parsed.End-parsed.Start+1)
		}
		return false, false, parsed, nil
	case http.StatusOK:
		return localSize > 0, false, contentRange{}, nil
	case http.StatusRequestedRangeNotSatisfiable:
		if localSize == 0 {
			return false, false, contentRange{}, fmt.Errorf("unexpected 416 without local partial")
		}
		total, parseErr := parseUnsatisfiedTotal(resp.Header.Get("Content-Range"))
		if parseErr == nil && total == localSize {
			if localSize != expectedSize {
				return false, false, contentRange{}, fmt.Errorf("download size %d does not match expected %d", localSize, expectedSize)
			}
			return false, true, contentRange{}, nil
		}
		return true, false, contentRange{}, nil
	default:
		if resp.Request != nil && resp.Request.URL != nil {
			return false, false, contentRange{}, &request.StatusError{Code: resp.StatusCode, URL: resp.Request.URL.String()}
		}
		return false, false, contentRange{}, &request.StatusError{Code: resp.StatusCode}
	}
}

func copyResponse(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	buf := make([]byte, 32*1024)
	var writtenTotal int64

	for {
		if err := ctx.Err(); err != nil {
			return writtenTotal, err
		}

		readN, readErr := src.Read(buf)
		if readN > 0 {
			writeN, writeErr := dst.Write(buf[:readN])
			writtenTotal += int64(writeN)
			if writeErr != nil {
				return writtenTotal, writeErr
			}
			if writeN != readN {
				return writtenTotal, io.ErrShortWrite
			}
		}

		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return writtenTotal, nil
			}
			return writtenTotal, readErr
		}
	}
}

func finalizeDownload(tempPath, finalPath string, expectedSize int64) error {
	info, err := os.Stat(tempPath)
	if err != nil {
		return err
	}
	if expectedSize >= 0 && info.Size() != expectedSize {
		return fmt.Errorf("download size %d does not match expected %d", info.Size(), expectedSize)
	}
	if err := os.Link(tempPath, finalPath); err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("destination already exists: %s", finalPath)
		}
		return fmt.Errorf("link completed download: %w", err)
	}
	if err := os.Remove(tempPath); err != nil {
		return fmt.Errorf("remove published temp file: %w", err)
	}
	return nil
}

func parseRangeNumber(value string, whole string) (int64, error) {
	parsed := strings.TrimSpace(value)
	if parsed == "" {
		return 0, fmt.Errorf("invalid Content-Range %q", whole)
	}
	number, err := strconv.ParseInt(parsed, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid Content-Range %q", whole)
	}
	if number < 0 {
		return 0, fmt.Errorf("invalid Content-Range %q", whole)
	}
	return number, nil
}

func statFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	return info.Size(), nil
}

func contentLengthFromHeader(header http.Header) (int64, error) {
	value := header.Get("Content-Length")
	if value == "" {
		return 0, errContentLengthMissing
	}
	size, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, err
	}
	if size < 0 {
		return 0, fmt.Errorf("invalid Content-Length %q", value)
	}
	return size, nil
}

func discoverExpectedSize(ctx context.Context, rawURL string, header http.Header) (int64, error) {
	headers, err := request.Head(ctx, rawURL, header)
	if err == nil {
		size, sizeErr := contentLengthFromHeader(headers)
		if sizeErr == nil {
			return size, nil
		}
		if !errors.Is(sizeErr, errContentLengthMissing) {
			return 0, sizeErr
		}
		return discoverExpectedSizeFromGet(ctx, rawURL, header)
	}

	var statusErr *request.StatusError
	if errors.As(err, &statusErr) && (statusErr.Code == http.StatusMethodNotAllowed || statusErr.Code == http.StatusNotImplemented) {
		return discoverExpectedSizeFromGet(ctx, rawURL, header)
	}

	return 0, err
}

func discoverExpectedSizeFromGet(ctx context.Context, rawURL string, header http.Header) (int64, error) {
	body, response, err := request.GetWithOptions(ctx, rawURL, request.GetOptions{
		Header:         header,
		ExpectedStatus: []int{http.StatusOK},
	})
	if err != nil {
		return 0, err
	}
	defer body.Close()
	return contentLengthFromHeader(response.Header)
}

func checkFinalPath(path string, expectedSize int64) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if info.Size() == expectedSize {
		return true, nil
	}
	return false, fmt.Errorf("existing file size %d does not match expected %d", info.Size(), expectedSize)
}

func buildRangeHeader(localSize, expectedSize, chunkSize int64, allowRange bool) string {
	if !allowRange {
		return ""
	}
	if chunkSize <= 0 || expectedSize <= 0 {
		if localSize == 0 {
			return ""
		}
		return fmt.Sprintf("bytes=%d-", localSize)
	}
	if localSize >= expectedSize {
		return fmt.Sprintf("bytes=%d-", localSize)
	}
	remaining := expectedSize - localSize
	var end int64
	if chunkSize >= remaining {
		end = expectedSize - 1
	} else {
		end = localSize + chunkSize - 1
	}
	if end < localSize {
		return fmt.Sprintf("bytes=%d-", localSize)
	}
	return fmt.Sprintf("bytes=%d-%d", localSize, end)
}

func shouldRetryDownload(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	var statusErr *request.StatusError
	if errors.As(err, &statusErr) {
		return statusErr.Code >= 500 && statusErr.Code <= 599
	}

	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.ErrShortWrite) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	var opErr *net.OpError
	return errors.As(err, &opErr)
}

func waitRetry(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
