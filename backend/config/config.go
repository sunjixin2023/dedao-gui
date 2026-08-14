package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/yann0917/dedao-gui/backend/services"
)

const (
	// EnvConfigDir 配置路径环境变量
	EnvConfigDir = "DEDAO_GO_CONFIG_DIR"
	// Name 配置文件名
	Name = "config.json"
)

var (
	configFilePath = filepath.Join(GetConfigDir(), Name)

	// Instance 配置信息 全局调用
	Instance *ConfigsData
)

func init() {
	Instance = new(ConfigsData)
	Instance.configFilePath = configFilePath
	Instance.fs = osConfigFS{}
	Instance.initErr = Instance.init()
}

// DedaoUsers user
type DedaoUsers []*Dedao

// ConfigsData Configs data
type ConfigsData struct {
	AcitveUID      string
	DownloadPath   string
	Users          DedaoUsers
	activeUser     *Dedao
	configFilePath string
	fileMu         sync.Mutex
	service        *services.Service
	recovery       *RecoveryInfo
	initErr        error
	fs             configFS
	now            func() time.Time
}

type configJSONExport struct {
	AcitveUID string
	Users     DedaoUsers
}

type RecoveryInfo struct {
	BackupPath string `json:"backupPath"`
	Message    string `json:"message"`
}

type configFS interface {
	MkdirAll(string, os.FileMode) error
	ReadFile(string) ([]byte, error)
	CreateTemp(string, string) (*os.File, error)
	Rename(string, string) error
	Remove(string) error
	Stat(string) (os.FileInfo, error)
}

type osConfigFS struct{}

func (osConfigFS) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (osConfigFS) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (osConfigFS) CreateTemp(dir, pattern string) (*os.File, error) {
	return os.CreateTemp(dir, pattern)
}

func (osConfigFS) Rename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}

func (osConfigFS) Remove(path string) error {
	return os.Remove(path)
}

func (osConfigFS) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (c *ConfigsData) Recovery() *RecoveryInfo { return c.recovery }

func (c *ConfigsData) InitError() error { return c.initErr }

// Init 初始化配置
func (c *ConfigsData) init() error {
	c.ensureFS()
	c.initErr = nil

	if c.configFilePath == "" {
		c.initErr = errors.New("配置文件未找到")
		return c.initErr
	}

	// 从配置文件中加载配置
	err := c.loadConfigFromFile()
	if err != nil {
		c.initErr = err
		return err
	}

	// 初始化登陆用户信息
	err = c.initActiveUser()
	if err != nil {
		return nil
	}

	if c.activeUser != nil {
		c.service = c.activeUser.New()
	}

	return nil
}

func (c *ConfigsData) initActiveUser() error {
	// 如果已经初始化过，则跳过
	if c.AcitveUID != "" && c.activeUser != nil && c.activeUser.UIDHazy == c.AcitveUID {
		return nil
	}

	if c.AcitveUID == "" && c.activeUser != nil {
		c.AcitveUID = c.activeUser.UIDHazy
		return nil
	}

	if c.AcitveUID != "" {
		for _, user := range c.Users {
			if user.UIDHazy == c.AcitveUID {
				c.activeUser = user
				return nil
			}
		}
	}

	if c.AcitveUID == "" && len(c.Users) == 0 {
		c.activeUser = new(Dedao)
	}

	if len(c.Users) > 0 {
		return errors.New("存在登录的用户，可以进行切换登录用户")
	}

	return errors.New("未登陆")
}

// Save 保存配置
func (c *ConfigsData) Save() error {
	c.fileMu.Lock()
	defer c.fileMu.Unlock()

	// 保存配置的数据
	conf := configJSONExport{
		AcitveUID: c.AcitveUID,
		Users:     c.Users,
	}

	data, err := jsoniter.MarshalIndent(conf, "", " ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return c.writeAtomic(data)
}

func (c *ConfigsData) loadConfigFromFile() error {
	c.ensureFS()

	if err := c.fs.MkdirAll(filepath.Dir(c.configFilePath), 0o700); err != nil {
		return err
	}

	data, err := c.fs.ReadFile(c.configFilePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			restored, restoreErr := c.restoreBackupIfPresent()
			if restoreErr != nil {
				return restoreErr
			}
			if restored {
				data, err = c.fs.ReadFile(c.configFilePath)
				if err != nil {
					return err
				}
			} else {
				return c.Save()
			}
		} else {
			return err
		}
	}

	if len(data) == 0 {
		return c.Save()
	}

	var conf configJSONExport
	if err := jsoniter.Unmarshal(data, &conf); err != nil {
		return c.recoverCorruptFile(err)
	}

	c.AcitveUID = conf.AcitveUID
	c.Users = conf.Users
	c.recovery = nil
	return nil
}

func (c *ConfigsData) restoreBackupIfPresent() (bool, error) {
	backupPath := c.configFilePath + ".bak"
	if _, err := c.fs.Stat(backupPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}

	if err := c.fs.Rename(backupPath, c.configFilePath); err != nil {
		return false, fmt.Errorf("restore backup config: %w", err)
	}

	return true, nil
}

func (c *ConfigsData) DeleteConfigFile() (err error) {
	if c.configFilePath == "" {
		return nil
	}
	c.ensureFS()
	err = c.fs.Remove(c.configFilePath)
	if os.IsNotExist(err) {
		return nil
	}
	return
}

// Reset clears all login state (memory + config file) and recreates a blank config.
func (c *ConfigsData) Reset() error {
	if err := c.DeleteConfigFile(); err != nil {
		return err
	}

	c.AcitveUID = ""
	c.Users = DedaoUsers{}
	c.setActiveUser(&Dedao{})
	return c.Save()
}

// New config
func New(configFilePath string) *ConfigsData {
	c := &ConfigsData{
		configFilePath: configFilePath,
		fs:             osConfigFS{},
		now:            time.Now,
	}

	return c
}

func (c *ConfigsData) ensureFS() {
	if c.fs == nil {
		c.fs = osConfigFS{}
	}
	if c.now == nil {
		c.now = time.Now
	}
}

func (c *ConfigsData) writeAtomic(data []byte) error {
	c.ensureFS()

	dir := filepath.Dir(c.configFilePath)
	if err := c.fs.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	tmpFile, err := c.fs.CreateTemp(dir, Name+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()

	if err := tmpFile.Chmod(0o600); err != nil {
		return c.closeAndRemoveTemp(tmpFile, tmpPath, err, "chmod temp config")
	}

	if _, err := tmpFile.Write(data); err != nil {
		return c.closeAndRemoveTemp(tmpFile, tmpPath, err, "write temp config")
	}

	if err := tmpFile.Sync(); err != nil {
		return c.closeAndRemoveTemp(tmpFile, tmpPath, err, "sync temp config")
	}

	if err := tmpFile.Close(); err != nil {
		if removeErr := c.fs.Remove(tmpPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return fmt.Errorf("close temp config: %w (cleanup temp: %v)", err, removeErr)
		}
		return fmt.Errorf("close temp config: %w", err)
	}

	backupPath := c.configFilePath + ".bak"
	hasBackup := false
	if _, err := c.fs.Stat(c.configFilePath); err == nil {
		if err := c.fs.Remove(backupPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			if cleanupErr := c.fs.Remove(tmpPath); cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
				return fmt.Errorf("remove stale backup: %w (cleanup temp: %v)", err, cleanupErr)
			}
			return fmt.Errorf("remove stale backup: %w", err)
		}
		if err := c.fs.Rename(c.configFilePath, backupPath); err != nil {
			if cleanupErr := c.fs.Remove(tmpPath); cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
				return fmt.Errorf("backup existing config: %w (cleanup temp: %v)", err, cleanupErr)
			}
			return fmt.Errorf("backup existing config: %w", err)
		}
		hasBackup = true
	} else if !errors.Is(err, os.ErrNotExist) {
		if cleanupErr := c.fs.Remove(tmpPath); cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
			return fmt.Errorf("stat existing config: %w (cleanup temp: %v)", err, cleanupErr)
		}
		return fmt.Errorf("stat existing config: %w", err)
	}

	if err := c.fs.Rename(tmpPath, c.configFilePath); err != nil {
		restoreErr := error(nil)
		if hasBackup {
			restoreErr = c.fs.Rename(backupPath, c.configFilePath)
		}
		cleanupErr := c.fs.Remove(tmpPath)
		if cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
			if restoreErr != nil {
				return fmt.Errorf("replace config: %w (restore backup: %v, cleanup temp: %v)", err, restoreErr, cleanupErr)
			}
			return fmt.Errorf("replace config: %w (cleanup temp: %v)", err, cleanupErr)
		}
		if restoreErr != nil {
			return fmt.Errorf("replace config: %w (restore backup: %v)", err, restoreErr)
		}
		return err
	}

	if hasBackup {
		if err := c.fs.Remove(backupPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove backup config: %w", err)
		}
	}

	return nil
}

func (c *ConfigsData) closeAndRemoveTemp(tmpFile *os.File, tmpPath string, originalErr error, op string) error {
	closeErr := tmpFile.Close()
	removeErr := c.fs.Remove(tmpPath)

	if closeErr != nil && removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		return fmt.Errorf("%s: %w (close temp: %v, cleanup temp: %v)", op, originalErr, closeErr, removeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("%s: %w (close temp: %v)", op, originalErr, closeErr)
	}
	if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		return fmt.Errorf("%s: %w (cleanup temp: %v)", op, originalErr, removeErr)
	}
	return fmt.Errorf("%s: %w", op, originalErr)
}

func (c *ConfigsData) recoverCorruptFile(decodeErr error) error {
	c.ensureFS()

	backupPath, err := c.nextCorruptBackupPath()
	if err != nil {
		return fmt.Errorf("choose corrupt backup path: %w", err)
	}
	if err := c.fs.Rename(c.configFilePath, backupPath); err != nil {
		return fmt.Errorf("backup corrupt config: %w", err)
	}

	c.AcitveUID = ""
	c.Users = DedaoUsers{}
	c.activeUser = nil
	c.service = nil
	c.recovery = &RecoveryInfo{
		BackupPath: backupPath,
		Message:    "配置已备份，需要重新登录",
	}

	if err := c.Save(); err != nil {
		return fmt.Errorf("create clean config after corrupt config recovery: %w", errors.Join(decodeErr, err))
	}

	return nil
}

func (c *ConfigsData) nextCorruptBackupPath() (string, error) {
	base := c.configFilePath + ".corrupt-" + c.now().UTC().Format("20060102T150405.000000000Z")
	for attempt := 0; ; attempt++ {
		candidate := base
		if attempt > 0 {
			candidate = fmt.Sprintf("%s-%d", base, attempt)
		}
		if _, err := c.fs.Stat(candidate); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return candidate, nil
			}
			return "", err
		}
	}
}

// GetConfigDir config file dir
func GetConfigDir() string {
	configDir, ok := os.LookupEnv(EnvConfigDir)
	if ok {
		if filepath.IsAbs(configDir) {
			return configDir
		}
	}
	home, ok := os.LookupEnv("HOME")
	if ok {
		return filepath.Join(home, ".config", "dedao")
	}

	return filepath.Join("/tmp", "dedao")
}

// ActiveUserService user
func (c *ConfigsData) ActiveUserService() *services.Service {
	if c.activeUser == nil {
		c.activeUser = &Dedao{}
	}
	if c.service == nil {
		c.service = c.activeUser.New()
	}
	return c.service
}

// SetUser set user
func (c *ConfigsData) SetUser(u *Dedao) (*Dedao, *services.User, error) {
	ser := services.NewService(&u.CookieOptions)
	user, err := ser.User()
	if err != nil {
		return nil, nil, err
	}

	c.DeleteUser(&User{UIDHazy: user.UIDHazy})

	dedao := &Dedao{
		User: User{
			UIDHazy: user.UIDHazy,
			Name:    user.Nickname,
			Avatar:  user.Avatar,
		},
		CookieOptions: u.CookieOptions,
	}
	c.Users = append(c.Users, dedao)
	c.setActiveUser(dedao)
	return dedao, user, nil
}

// DeleteUser delete
func (c *ConfigsData) DeleteUser(u *User) {
	for k, user := range c.Users {
		if user.UIDHazy == u.UIDHazy {
			c.Users = append(c.Users[:k], c.Users[k+1:]...)
			break
		}
	}
}

// ActiveUser active user
func (c *ConfigsData) ActiveUser() *Dedao {
	return c.activeUser
}

func (c *ConfigsData) setActiveUser(u *Dedao) {
	if u == nil {
		u = &Dedao{}
	}
	c.AcitveUID = u.UIDHazy
	c.activeUser = u
	c.service = u.New()
}

// LoginUserCount 登录用户数量
func (c *ConfigsData) LoginUserCount() int {
	return len(c.Users)
}

// SwitchUser switch user
func (c *ConfigsData) SwitchUser(u *User) error {
	for _, user := range c.Users {
		if user.UIDHazy == u.UIDHazy {
			c.setActiveUser(user)
			err := c.Save()
			return err
		}
	}
	return errors.New("用户不存在")
}
