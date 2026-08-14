# Session, Configuration, and Minimal UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep complete login cookies in the Go backend, recover safely from damaged configuration, and expose clear login/download recovery states through the existing UI without redesigning the product.

**Architecture:** Cookie parsing produces only named cookie values; raw `Set-Cookie` attributes never enter persisted state. Configuration uses short-lived file handles and same-directory temporary files, with explicit corruption backup metadata. The frontend hydrates one session store from a backend `SessionStatus` contract and never treats LocalStorage as authentication authority. One cancellable download context backs the existing download dialog.

**Tech Stack:** Go 1.23, Wails v2.10.1 bindings, Vue 3, Pinia, Element Plus, TypeScript.

---

## Files

- Modify: `backend/services/service.go`
- Modify: `backend/services/sunflower.go`
- Create: `backend/services/cookies_test.go`
- Modify: `backend/app/base.go`
- Modify: `backend/app/login.go`
- Modify: `backend/config/config.go`
- Create: `backend/config/config_test.go`
- Modify: `backend/login.go`
- Create: `backend/login_test.go`
- Modify: `backend/app.go`
- Modify: `backend/download.go`
- Create: `backend/download_test.go`
- Regenerate: `frontend/wailsjs/go/backend/App.d.ts`
- Regenerate: `frontend/wailsjs/go/backend/App.js`
- Regenerate: `frontend/wailsjs/go/models.ts`
- Modify: `frontend/src/stores/user.ts`
- Modify: `frontend/src/App.vue`
- Modify: `frontend/src/components/QrLogin.vue`
- Modify: `frontend/src/components/DownloadDialog.vue`
- Modify: `frontend/src/views/Home.vue`
- Modify: `frontend/src/views/Course.vue`
- Modify: `frontend/src/views/Ebook.vue`
- Modify: `frontend/src/views/Odob.vue`
- Modify: `frontend/src/views/Compass.vue`

### Task 1: Parse cookie values without truncation or persisting response attributes

- [ ] Add failing table tests in `backend/services/cookies_test.go` for an equals sign in a value, case-insensitive field matching, multiple `Set-Cookie` headers, ignored attributes, and map output used by PDF generation.

```go
func TestParseCookiesPreservesEqualsInValue(t *testing.T) {
	var got CookieOptions
	if err := ParseCookies("token=alpha=beta==; csrfToken=csrf-value", &got); err != nil {
		t.Fatal(err)
	}
	if got.Token != "alpha=beta==" {
		t.Fatalf("Token = %q", got.Token)
	}
}

func TestCookieHeaderFromSetCookiesDropsAttributes(t *testing.T) {
	got, err := CookieHeaderFromSetCookies([]string{
		"csrfToken=csrf-value; Path=/; HttpOnly; Secure",
		"token=alpha=beta==; Path=/; SameSite=Lax",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "csrfToken=csrf-value; token=alpha=beta==" {
		t.Fatalf("header = %q", got)
	}
}
```

Also assert that `Path`, `Domain`, `Expires`, `HttpOnly`, `Secure`, and `SameSite` do not appear in the returned header, and that marshalled `CookieOptions` has no redundant `cookieStr` field.

- [ ] Run the tests and confirm the value-truncation/new-helper failure.

```bash
go test ./backend/services -run 'Test(ParseCookies|CookieHeader)' -count=1
```

Expected: failure because the current parser uses unbounded `strings.Split` and the Set-Cookie helper does not exist.

- [ ] Change `ParseCookies` to use `strings.Cut`, preserving everything after the first equals sign. Support both a pointer-to-struct and `*map[string]string` because `backend/app/base.go::LoginedCookies` passes a map.

```go
name, value, found := strings.Cut(strings.TrimSpace(item), "=")
if !found || name == "" || value == "" {
	continue
}
cookieValues[strings.ToLower(name)] = value
```

For map output, copy only parsed name/value pairs. For struct output, retain the existing JSON-tag mapping and return an error for any other target type.

- [ ] Add structured Set-Cookie conversion using `http.Response.Cookies()`:

```go
func CookieHeaderFromSetCookies(values []string) (string, error) {
	response := &http.Response{Header: make(http.Header)}
	for _, value := range values {
		response.Header.Add("Set-Cookie", value)
	}
	cookies := response.Cookies()
	pairs := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie.Name == "" {
			continue
		}
		pairs = append(pairs, cookie.Name+"="+cookie.Value)
	}
	return strings.Join(pairs, "; "), nil
}
```

Add `MergeCookieHeaders(existing, update string) string` that parses both transient headers into a map, lets new values replace old values, and serializes only the cookie names used by `CookieOptions` in a stable order. The merged header exists only long enough to populate `CookieOptions`.

- [ ] Remove `CookieStr` from `CookieOptions`. Add a structured helper for the chromedp/PDF caller and update `backend/app/base.go`:

```go
func CookieMap(options *CookieOptions) map[string]string {
	result := make(map[string]string)
	if options == nil {
		return result
	}
	value := reflect.ValueOf(options).Elem()
	typeOfValue := value.Type()
	for index := 0; index < value.NumField(); index++ {
		field := value.Field(index)
		name := strings.Split(typeOfValue.Field(index).Tag.Get("json"), ",")[0]
		if field.Kind() == reflect.String && name != "" && name != "-" && field.String() != "" {
			result[name] = field.String()
		}
	}
	return result
}
```

`LoginedCookies` must return an empty map when `ActiveUser()` is nil; otherwise it calls `services.CookieMap(&active.CookieOptions)` and no longer parses a stored full-cookie string.

- [ ] Update `backend/services/sunflower.go` to obtain `csrfToken` from `resp.Cookies()` rather than splitting raw headers, and update `backend/app/login.go` to merge structured cookie headers before calling `ParseCookies`. Remove the assignment to `u.CookieStr`; persistence is the named fields in `CookieOptions`, not a duplicated complete header.

- [ ] Run focused tests and the affected packages.

```bash
gofmt -w backend/services/service.go backend/services/sunflower.go backend/services/cookies_test.go backend/app/base.go backend/app/login.go
go test ./backend/services ./backend/app -count=1
```

Expected: PASS.

- [ ] Commit cookie handling.

```bash
git add backend/services/service.go backend/services/sunflower.go backend/services/cookies_test.go backend/app/base.go backend/app/login.go
git commit -m "Keep authentication material structured inside the backend" \
  -m "Cookie values retain embedded equals signs while Set-Cookie attributes are discarded before persistence." \
  -m "Constraint: System keychain integration is outside this release, so config-file access still relies on user-directory permissions.
Rejected: Passing raw Set-Cookie strings through the frontend | it expands the credential trust boundary and stores response attributes
Confidence: high
Scope-risk: moderate
Directive: Parse cookie pairs at the first equals sign and never persist Path, Domain, or expiry attributes.
Tested: Equals values, multiple Set-Cookie headers, attribute removal, struct output, and map output"
```

### Task 2: Make configuration saves atomic and recover corrupt JSON

- [ ] Add failing tests in `backend/config/config_test.go` using a per-test temporary directory:

```text
TestConfigInitializesEmptyFile
TestConfigSaveProducesValidJSONAndMode0600
TestConfigRecoversTruncatedJSONAndKeepsBackup
TestConfigLoadReturnsReadFailure
TestConfigSaveReturnsPermissionError
TestConfigSaveLeavesPreviousFileWhenReplacementFails
```

The corruption test must write `{"AcitveUID":` to `config.json`, call `init`, and assert:

```go
recovery := cfg.Recovery()
if recovery == nil || recovery.BackupPath == "" {
	t.Fatal("expected recovery metadata with a backup path")
}
if got := mustReadFile(t, recovery.BackupPath); got != `{"AcitveUID":` {
	t.Fatalf("backup content = %q", got)
}
```

Use a small internal filesystem-operation interface for the permission and replacement-failure tests; return `fs.ErrPermission` from the fake and do not change global `os.Rename` variables that would race across tests.

- [ ] Confirm the tests expose the current panic/silent-decode/open-handle behavior.

```bash
go test ./backend/config -run TestConfig -count=1
```

Expected: failures because recovery metadata and atomic replacement do not exist.

- [ ] Remove the long-lived `configFile *os.File` and `lazyOpenConfigFile` model. Add recovery state and explicit accessors:

```go
type RecoveryInfo struct {
	BackupPath string `json:"backupPath"`
	Message    string `json:"message"`
}

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
}

type configFS interface {
	MkdirAll(string, os.FileMode) error
	ReadFile(string) ([]byte, error)
	CreateTemp(string, string) (*os.File, error)
	Rename(string, string) error
	Remove(string) error
	Stat(string) (os.FileInfo, error)
}

func (c *ConfigsData) Recovery() *RecoveryInfo { return c.recovery }
func (c *ConfigsData) InitError() error         { return c.initErr }
```

The package `init()` must store an initialization error instead of calling `log.Fatal`; application startup must remain possible so the UI can show recovery state.

- [ ] Implement atomic write as: marshal → `CreateTemp` in the same directory → chmod `0600` → write → sync → close → move current file to `.bak` → rename temp to final → remove `.bak`. If the final rename fails, restore `.bak` and return the original error.

```go
func (c *ConfigsData) Save() error {
	c.fileMu.Lock()
	defer c.fileMu.Unlock()

	data, err := jsoniter.MarshalIndent(configJSONExport{AcitveUID: c.AcitveUID, Users: c.Users}, "", " ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return c.writeAtomic(data)
}
```

Every `Write`, `Sync`, `Close`, `Rename`, `Remove`, and restore operation must be checked. No `panic`, `log.Fatal`, or ignored decode error may remain.

- [ ] Implement corrupt-file recovery:

```go
func (c *ConfigsData) recoverCorruptFile(decodeErr error) error {
	stamp := time.Now().UTC().Format("20060102T150405Z")
	backupPath := c.configFilePath + ".corrupt-" + stamp
	if err := c.fs.Rename(c.configFilePath, backupPath); err != nil {
		return fmt.Errorf("backup corrupt config: %w", err)
	}
	c.AcitveUID = ""
	c.Users = DedaoUsers{}
	c.recovery = &RecoveryInfo{
		BackupPath: backupPath,
		Message:    "配置已备份，需要重新登录",
	}
	if err := c.Save(); err != nil {
		return fmt.Errorf("create clean config after %v: %w", decodeErr, err)
	}
	return nil
}
```

The final implementation must avoid deadlocking by not calling `Save` while holding the same mutex. Parse the file before locking for the replacement, or call an internal already-locked write helper deliberately.

- [ ] Run config tests on the host platform and retain a Windows-specific acceptance item for the release matrix.

```bash
gofmt -w backend/config/config.go backend/config/config_test.go
go test ./backend/config -count=1
go test -race ./backend/config -count=1
```

Expected: PASS; the corrupt bytes remain at the reported backup path and the new config is valid JSON.

- [ ] Commit atomic configuration.

```bash
git add backend/config/config.go backend/config/config_test.go
git commit -m "Let the app recover instead of terminating on damaged configuration" \
  -m "Configuration writes use checked same-directory replacement, and invalid JSON is preserved under a reported backup path before a clean session is created." \
  -m "Constraint: Rename behavior must be verified separately on Windows and Unix.
Rejected: Truncating the existing open handle in place | interruption can leave an empty or partial configuration
Confidence: high
Scope-risk: broad
Directive: Keep config handles short-lived and surface every decode and replacement error.
Tested: Empty init, permissions, valid save, corrupt backup, rollback, and race test
Not-tested: Windows rename semantics until CI"
```

### Task 3: Expose backend-owned session state and delete frontend Cookie storage

- [ ] Add `backend/login_test.go` tests that build state through a small `sessionSource` interface and a fake source, so the contract tests never call a live user endpoint:

```go
func TestBuildSessionStatusLoggedOut(t *testing.T)
func TestBuildSessionStatusUsesStoredDisplayUserWithoutCookie(t *testing.T)
func TestLoginResultJSONDoesNotContainCookie(t *testing.T)
func TestSessionStatusIncludesConfigRecovery(t *testing.T)
```

Use this internal boundary:

```go
type sessionSource interface {
	ActiveUser() *config.Dedao
	LoginUserCount() int
	Recovery() *config.RecoveryInfo
}

func buildSessionStatus(source sessionSource) SessionStatus
```

The JSON assertion must marshal the public contract and fail if the bytes contain `cookie`, `token`, `csrf`, or `sid`, case-insensitively.

- [ ] Remove `Cookie` from `LoginResult` and add display-only contracts in `backend/login.go`:

```go
type LoginResult struct {
	Status int            `json:"status"`
	User   *services.User `json:"user"`
}

type SessionUser struct {
	UIDHazy  string `json:"uid_hazy"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

type SessionStatus struct {
	LoggedIn bool                 `json:"loggedIn"`
	User     *SessionUser         `json:"user,omitempty"`
	Recovery *config.RecoveryInfo `json:"recovery,omitempty"`
}

func (a *App) SessionStatus() SessionStatus {
	return buildSessionStatus(config.Instance)
}
```

`buildSessionStatus` must use the backend’s stored active user and UID, not call a live endpoint. `CheckLogin` still persists the Cookie through `app.LoginByCookie`, but returns only status and user display data.

- [ ] Regenerate the Wails bindings with the pinned CLI version:

```bash
go run github.com/wailsapp/wails/v2/cmd/wails@v2.10.1 generate module
```

Expected: `SessionStatus` appears in `frontend/wailsjs/go/backend/App.d.ts`, generated models contain no `LoginResult.cookie`, and generated files contain the new session types.

- [ ] Rewrite `frontend/src/stores/user.ts` as the single session authority:

```ts
import { defineStore } from 'pinia'
import { Logout, SessionStatus } from '../../wailsjs/go/backend/App'
import { services } from '../../wailsjs/go/models'

export const userStore = defineStore('userStore', {
  state: () => ({
    userList: [] as services.User[],
    user: null as services.User | null,
    loggedIn: false,
    sessionLoaded: false,
    recoveryMessage: '',
    recoveryBackupPath: '',
  }),
  actions: {
    async refreshSession() {
      const status = await SessionStatus()
      this.loggedIn = Boolean(status.loggedIn)
      this.user = status.user ? Object.assign(new services.User(), status.user) : null
      this.recoveryMessage = status.recovery?.message || ''
      this.recoveryBackupPath = status.recovery?.backupPath || ''
      this.sessionLoaded = true
    },
    acceptLogin(user: services.User | null) {
      this.user = user
      this.loggedIn = Boolean(user)
      this.sessionLoaded = true
    },
    clearSession() {
      this.user = null
      this.loggedIn = false
      localStorage.removeItem('cookies')
    },
    async classifySessionError(error: unknown) {
      const message = String(error || '')
      if (/\b(401|403)\b/.test(message)) {
        try {
          await Logout()
        } catch (logoutError) {
          console.warn('Backend session cleanup failed:', logoutError)
        }
        this.clearSession()
        return '登录已失效，请重新扫码登录'
      }
      if (/\b496\b/.test(message)) {
        return '需要验证，请先在得到官网完成验证码后重试'
      }
      return ''
    },
    async logout() {
      await Logout()
      this.clearSession()
    },
  },
  persist: { pick: ['userList'] },
})
```

Use the generated field casing produced by Wails if it differs; do not hand-edit generated model constructors to force the snippet.

Page request/download catch paths must call `classifySessionError` before generic retry handling. A 401 or 403 clears the backend-owned session and routes to login; 496 leaves the session intact, stops automatic retries, and tells the user to complete official-site verification.

- [ ] Hydrate the session once in `frontend/src/App.vue` on mount and display an `el-alert` when `recoveryMessage` is non-empty. The alert text must include the backup path and “需要重新登录”.

- [ ] Update `QrLogin.vue` so `syncUserAfterLogin` calls `store.acceptLogin(user)` and never reads `loginResult.cookie` or calls `Local.set("cookies", ...)`. `resetLocalSession` calls the store action and the visible helper no longer shows `rm -rf`; keep destructive shell advice only in developer documentation.

- [ ] Replace all authentication reads and duplicate cleanup paths in `Home.vue`, `Course.vue`, `Ebook.vue`, `Odob.vue`, and `Compass.vue` with `store.loggedIn` and `store.clearSession()`/`store.logout()`.

- [ ] Prove no frontend Cookie authority remains.

```bash
rg -n --glob '!frontend/src/assets/**' 'Local\.(get|set|remove)\(["'"']cookies|localStorage\.(getItem|setItem)\(["'"']cookies|loginResult[?]?[.]cookie' frontend/src
```

Expected: no output. A one-time `localStorage.removeItem('cookies')` migration in `clearSession` is allowed only if the scan expression is adjusted to distinguish removal from reads/writes.

- [ ] Run the backend contracts and frontend build.

```bash
gofmt -w backend/login.go backend/login_test.go
go test ./backend -run 'Test(BuildSessionStatus|LoginResult|SessionStatus)' -count=1
npm --prefix frontend run build
```

Expected: PASS.

- [ ] Commit session ownership.

```bash
git add backend/login.go backend/login_test.go frontend/wailsjs frontend/src/stores/user.ts frontend/src/App.vue frontend/src/components/QrLogin.vue frontend/src/views/Home.vue frontend/src/views/Course.vue frontend/src/views/Ebook.vue frontend/src/views/Odob.vue frontend/src/views/Compass.vue
git commit -m "Keep complete login cookies outside the webview" \
  -m "The frontend now hydrates a display-only session contract and all pages use one reactive login authority." \
  -m "Constraint: Persisted legacy cookie data is deleted during session migration.
Rejected: A frontend SessionStatus containing the Cookie | that would preserve the original credential exposure under a new type
Confidence: high
Scope-risk: broad
Directive: Public Wails login responses must remain free of Cookie and token fields.
Tested: Contract JSON scan, session states, generated bindings, frontend typecheck and build"
```

### Task 4: Add a cancellable active download and durable dialog states

- [ ] Add `backend/download_test.go` tests around a pure `runDownload` helper owned by `App`:

```text
TestRunDownloadRejectsSecondConcurrentDialogDownload
TestCancelDownloadCancelsActiveContext
TestRunDownloadClearsCancelFunctionAfterCompletion
```

- [ ] Extend `backend.App` with guarded active-download state:

```go
type App struct {
	Ctx            context.Context
	downloadMu     sync.Mutex
	downloadCancel context.CancelFunc
}

func (a *App) runDownload(run func(context.Context) error) error {
	a.downloadMu.Lock()
	if a.downloadCancel != nil {
		a.downloadMu.Unlock()
		return errors.New("已有下载任务正在运行")
	}
	ctx, cancel := context.WithCancel(a.Ctx)
	a.downloadCancel = cancel
	a.downloadMu.Unlock()
	defer func() {
		cancel()
		a.downloadMu.Lock()
		a.downloadCancel = nil
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
```

- [ ] Change `CourseDownload`, `OdobDownload`, and `EbookDownload` bindings to call `runDownload` and pass the per-operation context into their existing structs. This relies on the context-aware downloader work from the preceding plan.

- [ ] Extend progress payloads with a stable state string:

```go
type DownloadState string

const (
	DownloadQueued    DownloadState = "queued"
	DownloadRunning   DownloadState = "downloading"
	DownloadVerifying DownloadState = "verifying"
	DownloadCompleted DownloadState = "completed"
	DownloadFailed    DownloadState = "failed"
	DownloadCancelled DownloadState = "cancelled"
)
```

Emit state changes around the existing progress events. Map `context.Canceled` to `cancelled`; preserve the original error for detail.

- [ ] Regenerate Wails bindings, then update `DownloadDialog.vue`:

```ts
type DownloadState = 'queued' | 'downloading' | 'verifying' | 'completed' | 'failed' | 'cancelled'
const state = ref<DownloadState>('queued')
const errorDetail = ref('')

const cancelDownload = async () => {
  await CancelDownload()
  state.value = 'cancelled'
  content.value = '已取消，可稍后继续下载'
}
```

Replace clickable format `<div>` elements with `el-radio-group`/`el-radio-button`. Keep the dialog open on failure and cancellation. Close automatically only after `completed`; show a retry button when `failed`.

- [ ] Run Go tests and frontend build.

```bash
gofmt -w backend/app.go backend/download.go backend/download_test.go
go test ./backend -run 'Test(RunDownload|CancelDownload)' -count=1
npm --prefix frontend run build
```

Expected: PASS; TypeScript recognizes `CancelDownload`; failure no longer reaches an unconditional `finally { closeDialog() }`.

- [ ] Commit cancellation and status UX.

```bash
git add backend/app.go backend/download.go backend/download_test.go frontend/wailsjs frontend/src/components/DownloadDialog.vue
git commit -m "Keep failed and cancelled downloads visible and recoverable" \
  -m "The existing dialog now owns one cancellable backend context and exposes stable queued, transfer, verification, completion, failure, and cancellation states." \
  -m "Constraint: This release supports one dialog-owned foreground download at a time.
Rejected: Hiding the dialog during an active download | users would lose error and cancellation context
Confidence: high
Scope-risk: moderate
Directive: Only a completed download may close the dialog automatically.
Tested: Cancellation lifecycle, concurrent-start rejection, generated bindings, and frontend build"
```

### Task 5: Surface home loading and retry without a visual rewrite

- [ ] In `Home.vue`, add `loadError` and one `loadHome` action that sets `loading`, clears the previous error, awaits all existing initial requests, and maps rejection to a visible Chinese message.

```ts
const authStore = userStore()
const loading = ref(false)
const loadError = ref('')

const loadHome = async () => {
  loading.value = true
  loadError.value = ''
  try {
    const [homeState, ebookLabels, courseLabels, resources] = await Promise.all([
      GetHomeInitialState(),
      SunflowerLabelList(2),
      SunflowerLabelList(4),
      SunflowerResourceList(),
    ])
    Object.assign(initial, homeState)
    Object.assign(ebookLabelList, ebookLabels)
    Object.assign(courseLabelList, courseLabels)
    Object.assign(freeResourceList, resources)
    if (ebookLabels.list?.[0]) Object.assign(currentEbook, ebookLabels.list[0])
    if (courseLabels.list?.[0]) Object.assign(currentCourse, courseLabels.list[0])

    const [ebooks, courses, profile] = await Promise.all([
      SunflowerLabelContent('', 2, 0, 10),
      SunflowerLabelContent('', 4, 0, 4),
      authStore.loggedIn ? UserInfo() : Promise.resolve(null),
    ])
    Object.assign(ebookContentList, ebooks)
    Object.assign(courseContentList, courses)
    if (profile) Object.assign(user, profile)
  } catch (error) {
    loadError.value = `首页数据加载失败：${String(error)}`
  } finally {
    loading.value = false
  }
}
```

Replace the split `onBeforeMount`, `onMounted`, and eager `getFreeResourceList()` launch paths with `onMounted(loadHome)`. Keep the existing imports and data objects, and let `loadHome` own error propagation instead of swallowing each failure in `console.log`.

- [ ] Add a visible `el-skeleton` while loading and an `el-result` or `el-alert` with a keyboard-accessible “重新加载” button when `loadError` is non-empty. Do not change the existing card hierarchy or visual theme.

- [ ] Run the frontend contract scan and build.

```bash
rg -n --glob '!frontend/src/assets/**' 'rm -rf ~/.config/dedao|Local\.get\(["'"']cookies|Local\.set\(["'"']cookies' frontend/src
npm --prefix frontend run build
git diff --check
```

Expected: the scan produces no output; the build passes.

- [ ] Commit the minimal recovery feedback.

```bash
git add frontend/src/views/Home.vue frontend/src/components/QrLogin.vue frontend/src/components/DownloadDialog.vue
git commit -m "Show users when reliable recovery needs their action" \
  -m "Home loading, login reset, and download retry remain inside the existing screens with visible keyboard-accessible actions." \
  -m "Constraint: This is reliability feedback, not a visual redesign.
Confidence: high
Scope-risk: narrow
Directive: Keep errors visible until the user retries or dismisses them.
Tested: TypeScript check and production frontend build"
```

## Final verification

- [ ] Run all deterministic checks:

```bash
go test ./... -count=1
go test -race ./backend/... -count=1
go vet ./...
npm --prefix frontend run build
bash scripts/secret-check.sh
git diff --check
```

Expected: PASS.

- [ ] On macOS, build and launch the app, then manually verify: startup session hydration; legacy Cookie removal; QR failure/retry/reset; config recovery alert with backup path; format controls reachable by Tab; failed download stays open; cancel changes to “已取消”; completed download closes.

## Completion criteria

- No public Wails response contains full Cookie data.
- No frontend authentication branch reads or writes Cookie LocalStorage.
- Corrupt configuration is preserved and reported instead of terminating the process.
- Download states and cancellation are visible in the existing dialog.
- Home failures have a visible retry path.
- Backend tests, race tests, and frontend build pass.
