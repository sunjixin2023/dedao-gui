package backend

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRunDownloadRejectsSecondConcurrentDialogDownload(t *testing.T) {
	app := &App{Ctx: context.Background()}
	started := make(chan struct{})
	release := make(chan struct{})
	errCh := make(chan error, 1)

	go func() {
		errCh <- app.runDownload(func(ctx context.Context) error {
			close(started)
			<-release
			return nil
		})
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("first download did not start")
	}

	err := app.runDownload(func(ctx context.Context) error { return nil })
	if err == nil {
		t.Fatal("second download unexpectedly started")
	}
	if got, want := err.Error(), "已有下载任务正在运行"; got != want {
		t.Fatalf("second download error = %q, want %q", got, want)
	}

	close(release)

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("first download error = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("first download did not complete")
	}
}

func TestCancelDownloadCancelsActiveContext(t *testing.T) {
	app := &App{}
	started := make(chan struct{})
	errCh := make(chan error, 1)

	go func() {
		errCh <- app.runDownload(func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		})
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("download did not start")
	}

	app.CancelDownload()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("runDownload error = %v, want %v", err, context.Canceled)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("download did not observe cancellation")
	}
}

func TestRunDownloadClearsCancelFunctionAfterCompletion(t *testing.T) {
	app := &App{Ctx: context.Background()}

	if err := app.runDownload(func(ctx context.Context) error { return nil }); err != nil {
		t.Fatalf("first runDownload error = %v", err)
	}
	if app.downloadCancel != nil {
		t.Fatal("downloadCancel not cleared after completion")
	}

	if err := app.runDownload(func(ctx context.Context) error { return nil }); err != nil {
		t.Fatalf("second runDownload error = %v", err)
	}
}

func TestRunDownloadRejectsNilCallback(t *testing.T) {
	app := &App{Ctx: context.Background()}

	err := app.runDownload(nil)
	if err == nil {
		t.Fatal("runDownload unexpectedly accepted nil callback")
	}
	if got, want := err.Error(), "下载任务无效"; got != want {
		t.Fatalf("runDownload error = %q, want %q", got, want)
	}
}
