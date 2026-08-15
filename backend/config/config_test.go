package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	jsoniter "github.com/json-iterator/go"
)

func TestConfigInitializesEmptyFile(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), Name)
	if err := os.WriteFile(configPath, nil, 0o600); err != nil {
		t.Fatalf("seed empty config: %v", err)
	}

	cfg := New(configPath)

	if err := cfg.init(); err != nil {
		t.Fatalf("init: %v", err)
	}

	assertValidConfigJSON(t, configPath, "")
}

func TestConfigInitializesMissingFile(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), Name)
	cfg := New(configPath)

	if err := cfg.init(); err != nil {
		t.Fatalf("init: %v", err)
	}

	assertValidConfigJSON(t, configPath, "")
}

func TestConfigSaveProducesValidJSONAndPOSIXMode0600(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), Name)
	cfg := New(configPath)
	if err := cfg.init(); err != nil {
		t.Fatalf("init: %v", err)
	}

	cfg.AcitveUID = "user-1"
	cfg.Users = DedaoUsers{
		{
			User: User{
				UIDHazy: "user-1",
				Name:    "tester",
				Avatar:  "avatar.png",
			},
		},
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	assertValidConfigJSON(t, configPath, "user-1")
	// Windows reports synthesized POSIX mode bits; access control is inherited
	// from the user's profile directory instead of represented as chmod(0600).
	if runtime.GOOS == "windows" {
		return
	}

	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("config mode = %o, want 600", got)
	}
}

func TestConfigRestoresBackupWhenConfigMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, Name)
	backupPath := configPath + ".bak"
	backupContent := "{\n \"AcitveUID\": \"restored-user\",\n \"Users\": [\n  {\n   \"uid_hazy\": \"restored-user\",\n   \"name\": \"Restored\",\n   \"avatar\": \"avatar.png\"\n  }\n ]\n}"
	if err := os.WriteFile(backupPath, []byte(backupContent), 0o600); err != nil {
		t.Fatalf("seed backup config: %v", err)
	}

	cfg := New(configPath)
	if err := cfg.init(); err != nil {
		t.Fatalf("init: %v", err)
	}

	if cfg.AcitveUID != "restored-user" {
		t.Fatalf("AcitveUID = %q, want restored-user", cfg.AcitveUID)
	}
	if len(cfg.Users) != 1 || cfg.Users[0].UIDHazy != "restored-user" {
		t.Fatalf("users = %#v, want restored user", cfg.Users)
	}
	if got := mustReadFile(t, configPath); got != backupContent {
		t.Fatalf("restored config = %q, want %q", got, backupContent)
	}
	if _, err := os.Stat(backupPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("backup file state err = %v, want not exists", err)
	}
}

func TestConfigRecoversTruncatedJSONAndKeepsBackup(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), Name)
	if err := os.WriteFile(configPath, []byte(`{"AcitveUID":`), 0o600); err != nil {
		t.Fatalf("seed corrupt config: %v", err)
	}

	cfg := New(configPath)
	if err := cfg.init(); err != nil {
		t.Fatalf("init: %v", err)
	}

	recovery := cfg.Recovery()
	if recovery == nil || recovery.BackupPath == "" {
		t.Fatal("expected recovery metadata with a backup path")
	}
	if got := mustReadFile(t, recovery.BackupPath); got != `{"AcitveUID":` {
		t.Fatalf("backup content = %q", got)
	}

	assertValidConfigJSON(t, configPath, "")
}

func TestConfigClearRecoveryClearsMetadata(t *testing.T) {
	t.Parallel()

	cfg := New(filepath.Join(t.TempDir(), Name))
	cfg.recovery = &RecoveryInfo{
		BackupPath: "/tmp/config.json.bak",
		Message:    "配置已备份，需要重新登录",
	}

	cfg.ClearRecovery()

	if recovery := cfg.Recovery(); recovery != nil {
		t.Fatalf("recovery = %#v, want nil", recovery)
	}
}

func TestConfigResetClearsRecoveryAfterSuccessfulSave(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), Name)
	if err := os.WriteFile(configPath, []byte("{\n \"AcitveUID\": \"user-1\",\n \"Users\": []\n}"), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	cfg := New(configPath)
	cfg.recovery = &RecoveryInfo{
		BackupPath: configPath + ".bak",
		Message:    "配置已备份，需要重新登录",
	}

	if err := cfg.Reset(); err != nil {
		t.Fatalf("reset: %v", err)
	}

	if recovery := cfg.Recovery(); recovery != nil {
		t.Fatalf("recovery = %#v, want nil after successful reset", recovery)
	}
	assertValidConfigJSON(t, configPath, "")
}

func TestConfigResetPreservesRecoveryWhenBlankSaveFails(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), Name)
	if err := os.WriteFile(configPath, []byte("{\n \"AcitveUID\": \"user-1\",\n \"Users\": []\n}"), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	cfg := New(configPath)
	cfg.recovery = &RecoveryInfo{
		BackupPath: configPath + ".bak",
		Message:    "配置已备份，需要重新登录",
	}
	cfg.fs = &stubConfigFS{
		createTemp: func(string, string) (*os.File, error) {
			return nil, errReplacementFailed
		},
	}

	err := cfg.Reset()
	if !errors.Is(err, errReplacementFailed) {
		t.Fatalf("reset error = %v, want %v", err, errReplacementFailed)
	}
	if recovery := cfg.Recovery(); recovery == nil {
		t.Fatal("expected recovery metadata to remain after failed reset save")
	}
}

func TestConfigRecoversTruncatedJSONWithoutOverwritingExistingBackup(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, Name)
	corruptBytes := []byte(`{"AcitveUID":`)
	if err := os.WriteFile(configPath, corruptBytes, 0o600); err != nil {
		t.Fatalf("seed corrupt config: %v", err)
	}

	cfg := New(configPath)
	cfg.now = func() time.Time {
		return time.Date(2026, time.August, 14, 15, 4, 5, 123456789, time.UTC)
	}

	collidingBackupPath := cfg.configFilePath + ".corrupt-20260814T150405.123456789Z"
	existingBytes := []byte("existing-backup")
	if err := os.WriteFile(collidingBackupPath, existingBytes, 0o600); err != nil {
		t.Fatalf("seed colliding backup: %v", err)
	}

	if err := cfg.init(); err != nil {
		t.Fatalf("init: %v", err)
	}

	recovery := cfg.Recovery()
	if recovery == nil || recovery.BackupPath == "" {
		t.Fatal("expected recovery metadata with a backup path")
	}
	if recovery.BackupPath == collidingBackupPath {
		t.Fatalf("recovery backup path = %q, want distinct path", recovery.BackupPath)
	}
	if got := mustReadFileBytes(t, collidingBackupPath); !slices.Equal(got, existingBytes) {
		t.Fatalf("existing backup bytes = %q, want %q", got, existingBytes)
	}
	if got := mustReadFileBytes(t, recovery.BackupPath); !slices.Equal(got, corruptBytes) {
		t.Fatalf("recovered backup bytes = %q, want %q", got, corruptBytes)
	}

	assertValidConfigJSON(t, configPath, "")
}

func TestRecoverCorruptFilePreservesDecodeAndSaveErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, Name)
	corruptBytes := []byte(`{"AcitveUID":`)
	if err := os.WriteFile(configPath, corruptBytes, 0o600); err != nil {
		t.Fatalf("seed corrupt config: %v", err)
	}

	decodeErr := errors.New("decode failed")
	saveErr := errors.New("save failed")

	cfg := New(configPath)
	cfg.now = func() time.Time {
		return time.Date(2026, time.August, 14, 15, 4, 5, 123456789, time.UTC)
	}
	cfg.fs = &stubConfigFS{
		createTemp: func(string, string) (*os.File, error) {
			return nil, saveErr
		},
		renameHook: func(oldPath, newPath string) error {
			return os.Rename(oldPath, newPath)
		},
	}

	err := cfg.recoverCorruptFile(decodeErr)
	if !errors.Is(err, decodeErr) {
		t.Fatalf("recover error = %v, want decode err in chain", err)
	}
	if !errors.Is(err, saveErr) {
		t.Fatalf("recover error = %v, want save err in chain", err)
	}

	recovery := cfg.Recovery()
	if recovery == nil || recovery.BackupPath == "" {
		t.Fatal("expected recovery metadata with a backup path")
	}
	if got := mustReadFileBytes(t, recovery.BackupPath); !slices.Equal(got, corruptBytes) {
		t.Fatalf("backup bytes = %q, want %q", got, corruptBytes)
	}
}

func TestConfigLoadReturnsReadFailure(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), Name)
	cfg := New(configPath)
	cfg.fs = &stubConfigFS{
		readFileErr: fs.ErrPermission,
	}

	err := cfg.init()
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("init error = %v, want %v", err, fs.ErrPermission)
	}
}

func TestConfigSaveReturnsPermissionError(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), Name)
	cfg := New(configPath)
	cfg.fs = &stubConfigFS{
		mkdirAllErr: fs.ErrPermission,
	}

	err := cfg.Save()
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("save error = %v, want %v", err, fs.ErrPermission)
	}
}

func TestConfigSaveLeavesPreviousFileWhenReplacementFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, Name)
	oldContent := "{\n \"AcitveUID\": \"original\",\n \"Users\": []\n}"
	if err := os.WriteFile(configPath, []byte(oldContent), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	cfg := New(configPath)
	cfg.fs = &stubConfigFS{
		renameHook: func(oldPath, newPath string) error {
			if strings.HasSuffix(oldPath, ".tmp") && newPath == configPath {
				return errReplacementFailed
			}
			return os.Rename(oldPath, newPath)
		},
	}
	cfg.AcitveUID = "updated"

	err := cfg.Save()
	if !errors.Is(err, errReplacementFailed) {
		t.Fatalf("save error = %v, want %v", err, errReplacementFailed)
	}

	if got := mustReadFile(t, configPath); got != oldContent {
		t.Fatalf("config content = %q, want %q", got, oldContent)
	}

	if _, err := os.Stat(configPath + ".bak"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("backup file state err = %v, want not exists", err)
	}
}

var errReplacementFailed = errors.New("replacement failed")

type stubConfigFS struct {
	mkdirAllErr error
	readFileErr error
	statErr     error
	readFile    func(string) ([]byte, error)
	createTemp  func(string, string) (*os.File, error)
	renameHook  func(string, string) error
	removeHook  func(string) error
	statHook    func(string) (os.FileInfo, error)
}

func (s *stubConfigFS) MkdirAll(path string, perm os.FileMode) error {
	if s.mkdirAllErr != nil {
		return s.mkdirAllErr
	}
	return os.MkdirAll(path, perm)
}

func (s *stubConfigFS) ReadFile(path string) ([]byte, error) {
	if s.readFile != nil {
		return s.readFile(path)
	}
	if s.readFileErr != nil {
		return nil, s.readFileErr
	}
	return os.ReadFile(path)
}

func (s *stubConfigFS) CreateTemp(dir, pattern string) (*os.File, error) {
	if s.createTemp != nil {
		return s.createTemp(dir, pattern)
	}
	return os.CreateTemp(dir, pattern)
}

func (s *stubConfigFS) Rename(oldPath, newPath string) error {
	if s.renameHook != nil {
		return s.renameHook(oldPath, newPath)
	}
	return os.Rename(oldPath, newPath)
}

func (s *stubConfigFS) Remove(path string) error {
	if s.removeHook != nil {
		return s.removeHook(path)
	}
	return os.Remove(path)
}

func (s *stubConfigFS) Stat(path string) (os.FileInfo, error) {
	if s.statHook != nil {
		return s.statHook(path)
	}
	if s.statErr != nil {
		return nil, s.statErr
	}
	return os.Stat(path)
}

func assertValidConfigJSON(t *testing.T, configPath, wantActiveUID string) {
	t.Helper()

	raw := mustReadFile(t, configPath)
	var conf configJSONExport
	if err := jsoniter.Unmarshal([]byte(raw), &conf); err != nil {
		t.Fatalf("unmarshal config: %v\nraw=%q", err, raw)
	}

	if conf.AcitveUID != wantActiveUID {
		t.Fatalf("AcitveUID = %q, want %q", conf.AcitveUID, wantActiveUID)
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()

	data := mustReadFileBytes(t, path)
	return string(data)
}

func mustReadFileBytes(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}
