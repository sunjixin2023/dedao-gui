package app

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestResolveTerminalProgressCompletesSuccessfulDownload(t *testing.T) {
	progress := resolveTerminalProgress(Progress{ID: 7}, nil)

	if progress.State != DownloadCompleted {
		t.Fatalf("state = %q, want %q", progress.State, DownloadCompleted)
	}
	if progress.Pct != 100 {
		t.Fatalf("pct = %d, want 100", progress.Pct)
	}
	if progress.Value != "下载完成" {
		t.Fatalf("value = %q, want %q", progress.Value, "下载完成")
	}
	if progress.Detail != "" {
		t.Fatalf("detail = %q, want empty", progress.Detail)
	}
}

func TestResolveTerminalProgressMapsCanceledErrorToCancelledState(t *testing.T) {
	progress := resolveTerminalProgress(Progress{ID: 9}, context.Canceled)

	if progress.State != DownloadCancelled {
		t.Fatalf("state = %q, want %q", progress.State, DownloadCancelled)
	}
	if progress.Value != "下载已取消" {
		t.Fatalf("value = %q, want %q", progress.Value, "下载已取消")
	}
	if progress.Detail != "已取消当前下载" {
		t.Fatalf("detail = %q, want %q", progress.Detail, "已取消当前下载")
	}
}

func TestResolveTerminalProgressSanitizesFailureDetail(t *testing.T) {
	rawErr := errors.New("download https://signed.example.com/file.mp4?token=abc123 failed: open /Users/jasonsun/Downloads/output.mp4: permission denied")
	progress := resolveTerminalProgress(Progress{ID: 12}, rawErr)

	if progress.State != DownloadFailed {
		t.Fatalf("state = %q, want %q", progress.State, DownloadFailed)
	}
	if progress.Value != "下载失败" {
		t.Fatalf("value = %q, want %q", progress.Value, "下载失败")
	}
	if progress.Detail == "" {
		t.Fatal("detail is empty, want user-safe guidance")
	}
	for _, forbidden := range []string{"https://", "/Users/jasonsun", "token=abc123"} {
		if strings.Contains(progress.Detail, forbidden) {
			t.Fatalf("detail %q unexpectedly contains %q", progress.Detail, forbidden)
		}
	}
	if !strings.Contains(progress.Detail, "下载目录") {
		t.Fatalf("detail = %q, want download-directory guidance", progress.Detail)
	}
}

func TestOdobDownloadNilDataDoesNotPanic(t *testing.T) {
	d := OdobDownload{DownloadType: 1, Data: nil}
	err := d.Download()
	if err == nil {
		t.Fatal("expected error for nil odob payload")
	}
}

func TestEbookDownloadEmptyEnID(t *testing.T) {
	d := EBookDownload{DownloadType: 1, EnID: ""}
	err := d.Download()
	if err == nil {
		t.Fatal("expected error for empty ebook id")
	}
}

func TestCourseDownloadEmptyEnID(t *testing.T) {
	d := CourseDownload{DownloadType: 1, EnId: ""}
	err := d.Download()
	if err == nil {
		t.Fatal("expected error for empty course id")
	}
}
