package backend

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/yann0917/dedao-gui/backend/app"
	"github.com/yann0917/dedao-gui/backend/services"
	"github.com/yann0917/dedao-gui/backend/utils"
)

var errActiveDownload = errors.New("已有下载任务正在运行")

func (a *App) OpenDirectoryDialog(title string) (dir string, err error) {
	home, _ := os.LookupEnv("HOME")
	dialogOptions := wailsruntime.OpenDialogOptions{
		DefaultDirectory:           home,
		Title:                      title,
		ShowHiddenFiles:            false,
		CanCreateDirectories:       true,
		ResolvesAliases:            false,
		TreatPackagesAsDirectories: false,
	}
	dir, err = wailsruntime.OpenDirectoryDialog(a.Ctx, dialogOptions)
	app.SetOutputDir(dir)
	return
}

func (a *App) OpenFileDialog(title string) (file string, err error) {
	home, _ := os.LookupEnv("HOME")
	dialogOptions := wailsruntime.OpenDialogOptions{
		DefaultDirectory:           home,
		Title:                      title,
		ShowHiddenFiles:            false,
		CanCreateDirectories:       false,
		ResolvesAliases:            false,
		TreatPackagesAsDirectories: false,
	}
	file, err = wailsruntime.OpenFileDialog(a.Ctx, dialogOptions)
	return
}

type DirConfig struct {
	OutputDir  string `json:"outputDir"`
	FfmpegDir  string `json:"ffmpegDir"`
	WkToPdfDir string `json:"wkToPdfDir"`
}

func (a *App) SetDir(dir []string) (err error) {
	if len(dir) > 0 {
		app.OutputDir = dir[0]
	}
	if len(dir) > 1 {
		utils.FfmpegDir = dir[1]
	}
	if len(dir) > 2 {
		utils.WkToPdfDir = dir[2]
	}
	if len(dir) > 1 {
		if err = validateExecutablePath(utils.FfmpegDir, "ffmpeg"); err != nil {
			return err
		}
	}
	if len(dir) > 2 {
		if err = validateExecutablePath(utils.WkToPdfDir, "wkhtmltopdf"); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) SetDirConfig(cfg DirConfig) (err error) {
	if cfg.OutputDir != "" {
		app.SetOutputDir(cfg.OutputDir)
	}
	if cfg.FfmpegDir != "" {
		utils.FfmpegDir = cfg.FfmpegDir
		if err = validateExecutablePath(utils.FfmpegDir, "ffmpeg"); err != nil {
			return err
		}
	}
	if cfg.WkToPdfDir != "" {
		utils.WkToPdfDir = cfg.WkToPdfDir
		if err = validateExecutablePath(utils.WkToPdfDir, "wkhtmltopdf"); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) runDownload(run func(context.Context) error) error {
	a.downloadMu.Lock()
	if a.downloadCancel != nil {
		a.downloadMu.Unlock()
		return errActiveDownload
	}

	baseCtx := a.Ctx
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	ctx, cancel := context.WithCancel(baseCtx)
	a.downloadGeneration++
	generation := a.downloadGeneration
	a.downloadCancel = cancel
	a.downloadMu.Unlock()

	defer func() {
		cancel()
		a.downloadMu.Lock()
		if a.downloadGeneration == generation {
			a.downloadCancel = nil
		}
		a.downloadMu.Unlock()
	}()

	return run(ctx)
}

func (a *App) CancelDownload() {
	a.downloadMu.Lock()
	cancel := a.downloadCancel
	a.downloadMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *App) CourseDownload(id, aid, dType int, enid string) (err error) {
	return a.runDownload(func(ctx context.Context) error {
		app.EmitDownloadState(ctx, "courseDownload", app.Progress{ID: id, Value: "准备下载"}, app.DownloadQueued, "")

		d := app.CourseDownload{
			Ctx:          ctx,
			ID:           id,
			AID:          aid,
			EnId:         enid,
			DownloadType: dType,
		}

		err := d.Download()
		app.EmitTerminalDownloadState(ctx, "courseDownload", app.Progress{ID: id}, err)
		return err
	})
}

func (a *App) OdobDownload(id, dType int, data *services.Course) (err error) {
	return a.runDownload(func(ctx context.Context) error {
		app.EmitDownloadState(ctx, "odobDownload", app.Progress{ID: id, Value: "准备下载"}, app.DownloadQueued, "")

		d := app.OdobDownload{
			Ctx:          ctx,
			ID:           id,
			DownloadType: dType,
			Data:         data,
		}

		err := d.Download()
		app.EmitTerminalDownloadState(ctx, "odobDownload", app.Progress{ID: id}, err)
		return err
	})
}

func (a *App) EbookDownload(id, dType int, enid string) (err error) {
	return a.runDownload(func(ctx context.Context) error {
		app.EmitDownloadState(ctx, "ebookDownload", app.Progress{ID: id, Value: "准备下载"}, app.DownloadQueued, "")

		d := app.EBookDownload{
			Ctx:          ctx,
			ID:           id,
			DownloadType: dType,
			EnID:         enid,
		}

		err := d.Download()
		app.EmitTerminalDownloadState(ctx, "ebookDownload", app.Progress{ID: id}, err)
		return err
	})
}

func validateExecutablePath(path string, label string) error {
	if path == "" {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s 路径无效", label)
	}
	if info.IsDir() {
		return fmt.Errorf("%s 必须是可执行文件", label)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0111 == 0 {
		return fmt.Errorf("%s 没有执行权限", label)
	}
	return nil
}
