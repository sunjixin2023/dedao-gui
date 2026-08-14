package utils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
)

var FfmpegDir = ""

func runMergeCmd(cmd *exec.Cmd, paths []string, mergeFilePath string) error {
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("%s\n%s", err, stderr.String())
	}

	if mergeFilePath != "" {
		os.Remove(mergeFilePath) // nolint
	}
	// remove parts
	for _, path := range paths {
		os.Remove(path) // nolint
	}
	return nil
}

// MergeAudio merge audio
func MergeAudio(paths []string, mergedFilePath string) error {
	return MergeAudioContext(context.Background(), paths, mergedFilePath)
}

// MergeAudioContext merges audio files and respects cancellation.
func MergeAudioContext(ctx context.Context, paths []string, mergedFilePath string) error {
	cmds := []string{
		"-y",
	}
	for _, path := range paths {
		cmds = append(cmds, "-i", path)
	}
	cmds = append(cmds, "-c:v", "copy", mergedFilePath)
	err := runMergeCmd(exec.CommandContext(ctx, FfmpegDir, cmds...), paths, "")
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

// MergeAudioAndVideo merge audio and video
func MergeAudioAndVideo(paths []string, mergedFilePath string) error {
	return MergeAudioAndVideoContext(context.Background(), paths, mergedFilePath)
}

// MergeAudioAndVideoContext merges audio and video files and respects cancellation.
func MergeAudioAndVideoContext(ctx context.Context, paths []string, mergedFilePath string) error {
	cmds := []string{
		"-y",
	}
	for _, path := range paths {
		cmds = append(cmds, "-i", path)
	}
	cmds = append(cmds, "-c:v", "copy", "-c:a", "copy", mergedFilePath)
	err := runMergeCmd(exec.CommandContext(ctx, FfmpegDir, cmds...), paths, "")
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

// MergeToMP4 merges video parts to an MP4 file.
func MergeToMP4(paths []string, mergedFilePath string, filename string) error {
	mergeFilePath := filename + ".txt" // merge list file should be in the current directory
	// write ffmpeg input file list
	mergeFile, _ := os.Create(mergeFilePath)
	for _, path := range paths {
		_, _ = mergeFile.Write([]byte(fmt.Sprintf("file '%s'\n", path))) // nolint
	}
	err := mergeFile.Close() // nolint
	if err != nil {
		return err
	}

	cmd := exec.Command(
		FfmpegDir, "-y", "-f", "concat", "-safe", "-1",
		"-i", mergeFilePath, "-c", "copy", "-bsf:a", "aac_adtstoasc", mergedFilePath,
	)
	return runMergeCmd(cmd, paths, mergeFilePath)
}
