package utils

import (
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var (
	helperOverwrite = flag.Bool("y", false, "")
	helperInputs    multiStringFlag
	helperCopyVideo = flag.String("c:v", "", "")
	helperCopyAudio = flag.String("c:a", "", "")
)

type multiStringFlag []string

func (m *multiStringFlag) String() string {
	return ""
}

func (m *multiStringFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func init() {
	flag.Var(&helperInputs, "i", "")
}

func TestMain(m *testing.M) {
	if os.Getenv("GO_WANT_FFMPEG_HELPER_PROCESS") == "1" {
		time.Sleep(5 * time.Second)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestRunMergeCommandStopsOnContextCancel(t *testing.T) {
	part := filepath.Join(t.TempDir(), "part.mp3")
	if err := os.WriteFile(part, []byte("part"), 0o644); err != nil {
		t.Fatalf("write part: %v", err)
	}
	out := filepath.Join(t.TempDir(), "merged.mp3")

	oldDir := FfmpegDir
	oldOverwrite := *helperOverwrite
	oldCopyVideo := *helperCopyVideo
	oldCopyAudio := *helperCopyAudio
	oldInputs := append([]string(nil), helperInputs...)
	FfmpegDir = os.Args[0]
	*helperOverwrite = false
	*helperCopyVideo = ""
	*helperCopyAudio = ""
	helperInputs = nil
	t.Cleanup(func() {
		FfmpegDir = oldDir
		*helperOverwrite = oldOverwrite
		*helperCopyVideo = oldCopyVideo
		*helperCopyAudio = oldCopyAudio
		helperInputs = oldInputs
	})
	if err := os.Setenv("GO_WANT_FFMPEG_HELPER_PROCESS", "1"); err != nil {
		t.Fatalf("set helper env: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Unsetenv("GO_WANT_FFMPEG_HELPER_PROCESS")
	})

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- MergeAudioContext(ctx, []string{part}, out)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want %v", err, context.Canceled)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("MergeAudioContext did not return after cancellation")
	}

	if _, err := os.Stat(part); err != nil {
		t.Fatalf("part file removed on cancellation: %v", err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("merged file should not exist: %v", err)
	}
}
