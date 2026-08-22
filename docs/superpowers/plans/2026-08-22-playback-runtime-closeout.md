# Playback Runtime Closeout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a 1.0.3 desktop app whose V2 video path, macOS window, download entrypoints, Wails runtime, and list/notification UX are correct without a visual rewrite.

**Architecture:** Keep the existing Wails v2 + Go + Vue layers. Extract a testable macOS window policy, send Volc signed queries through `net/http` with verbatim `RawQuery`, reject nil download payloads before dereference, pin Wails to v2.14.0, and add CSS content-visibility plus download-end notifications.

**Tech Stack:** Go 1.23, Wails v2.14.0, Vue 3, Element Plus, net/http, httptest.

---

## File map

- Create: `backend/window_policy.go`, `backend/window_policy_test.go`
- Create: `backend/services/volc_http.go`, `backend/services/volc_http_test.go`
- Create: `frontend/src/utils/downloadNotify.ts`, `frontend/src/utils/downloadNotify.test.ts`
- Modify: `main.go`, `backend/app.go`, `backend/download.go`, `backend/app/download.go`, `backend/services/requester.go`, `go.mod`, `go.sum`, `.wails-version`, `CLAUDE.md`, `frontend/src/assets/css/global.css`, `frontend/src/components/DownloadDialog.vue`
- Do not modify: homepage layout, VePlayer CDN fallback order, `wails.json.info.productVersion` (stays `0.0.0` until release script)

---

### Task 1: Opaque macOS window policy

**Files:**
- Create: `backend/window_policy.go`
- Create: `backend/window_policy_test.go`
- Modify: `main.go`
- Modify: `backend/app.go`

- [ ] **Step 1: Write failing tests**

```go
package backend

import "testing"

func TestDesktopMacWindowPolicyIsOpaque(t *testing.T) {
	got := DesktopMacWindowPolicy()
	if got.WebviewIsTransparent || got.WindowIsTranslucent {
		t.Fatalf("opaque window required, got %+v", got)
	}
}

func TestFocusExistingWindowNoopsOnNilContext(t *testing.T) {
	var a App
	a.OnSecondInstanceLaunch(struct{}{}) // replaced below with real type usage in implementation
}
```

Replace the second test with a helper that does not import Wails options in the test if it makes compilation fail. The required behavior is: `focusExistingWindow(nil)` must not panic.

```go
func TestFocusExistingWindowNilContextDoesNotPanic(t *testing.T) {
	defer func() {
		if recover() != nil {
			t.Fatal("focusExistingWindow(nil) panicked")
		}
	}()
	focusExistingWindow(nil)
}
```

- [ ] **Step 2: Run tests and confirm they fail because the helpers do not exist**

Run: `go test ./backend -count=1 -run 'TestDesktopMacWindowPolicyIsOpaque|TestFocusExistingWindowNilContextDoesNotPanic'`

Expected: FAIL, undefined `DesktopMacWindowPolicy` / `focusExistingWindow`

- [ ] **Step 3: Implement**

`backend/window_policy.go`:

```go
package backend

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type MacWindowPolicy struct {
	WebviewIsTransparent bool
	WindowIsTranslucent  bool
}

func DesktopMacWindowPolicy() MacWindowPolicy {
	return MacWindowPolicy{
		WebviewIsTransparent: false,
		WindowIsTranslucent:  false,
	}
}

func focusExistingWindow(ctx context.Context) {
	if ctx == nil {
		return
	}
	runtime.WindowUnminimise(ctx)
	runtime.Show(ctx)
}
```

Note: confirm the v2.10/v2.14 API. If `runtime.Show` does not exist, use `runtime.WindowShow(ctx)` which is the v2 API.

`main.go` Mac block must use the policy:

```go
policy := backend.DesktopMacWindowPolicy()
Mac: &mac.Options{
    TitleBar: &mac.TitleBar{
        TitlebarAppearsTransparent: false,
        HideTitle:                  false,
        HideTitleBar:               false,
        FullSizeContent:            false,
        UseToolbar:                 false,
        HideToolbarSeparator:       true,
    },
    Appearance:           mac.DefaultAppearance,
    WebviewIsTransparent: policy.WebviewIsTransparent,
    WindowIsTranslucent:  policy.WindowIsTranslucent,
    About: &mac.AboutInfo{
        Title:   "dedao",
        Message: "https://github.com/yann0917/dedao-gui",
        Icon:    icon,
    },
},
```

`backend/app.go` `OnSecondInstanceLaunch`:

```go
func (a *App) OnSecondInstanceLaunch(secondInstanceData options.SecondInstanceData) {
	focusExistingWindow(a.Ctx)
}
```

- [ ] **Step 4: Re-run the two tests**

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/window_policy.go backend/window_policy_test.go main.go backend/app.go
git commit -m "$(cat <<'EOF'
Keep the macOS desktop window opaque and restorable

Transparent Wails windows on macOS 27 leave a live process with no usable surface. Apply a testable opaque policy and bring the first instance forward on a second launch.
EOF
)"
```

---

### Task 2: Verbatim Volc signed HTTP

**Files:**
- Create: `backend/services/volc_http.go`
- Create: `backend/services/volc_http_test.go`
- Modify: `backend/services/requester.go`

- [ ] **Step 1: Write failing tests**

```go
package services

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestVolcSignedGETPreservesRawQuery(t *testing.T) {
	want := "Action=GetPlayInfo&Version=2020-08-01&Vid=video-1&X-SignedQueries=Action%3BVersion%3BVid&X-Signature=sig+plus"
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ResponseMetadata":{"RequestId":"1"},"Result":{}}`))
	}))
	t.Cleanup(server.Close)

	body, err := volcSignedGET(server.URL, want, nil)
	if err != nil {
		t.Fatalf("volcSignedGET() error = %v", err)
	}
	defer body.Close()
	if _, err := io.ReadAll(body); err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if gotQuery != want {
		t.Fatalf("RawQuery = %q, want %q", gotQuery, want)
	}
}

func TestVolcSignedGETRejectsNewlines(t *testing.T) {
	_, err := volcSignedGET("https://vod.example.invalid/", "Action=GetPlayInfo\nHost: evil", nil)
	if err == nil {
		t.Fatal("expected newline rejection")
	}
}

func TestVolcSignedGETMaps496(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(496)
	}))
	t.Cleanup(server.Close)
	_, err := volcSignedGET(server.URL, "Action=GetPlayInfo&Version=2020-08-01", nil)
	if err == nil || !strings.Contains(err.Error(), "496") {
		t.Fatalf("error = %v, want 496", err)
	}
}
```

- [ ] **Step 2: Run tests, expect missing `volcSignedGET`**

Run: `go test ./backend/services -count=1 -run 'TestVolcSignedGET'`

- [ ] **Step 3: Implement**

`backend/services/volc_http.go`:

```go
package services

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func volcSignedGET(endpoint, query string, cookies []*http.Cookie) (io.ReadCloser, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("火山点播请求参数为空")
	}
	if strings.ContainsAny(query, "\r\n") {
		return nil, errors.New("火山点播请求参数包含非法换行")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("解析火山点播地址: %w", err)
	}
	parsed.RawQuery = query
	req, err := http.NewRequest(http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	req.URL.RawQuery = query
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Xi-DT", "web")
	for _, cookie := range cookies {
		if cookie != nil {
			req.AddCookie(cookie)
		}
	}
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, errors.New("404 NotFound")
	}
	if resp.StatusCode == http.StatusBadRequest {
		resp.Body.Close()
		return nil, errors.New("400 BadRequest")
	}
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		return nil, errors.New("401 Unauthorized")
	}
	if resp.StatusCode == 496 {
		resp.Body.Close()
		return nil, errors.New("496 NoCertificate")
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		resp.Body.Close()
		return nil, fmt.Errorf("火山点播 HTTP %d", resp.StatusCode)
	}
	return resp.Body, nil
}
```

`reqVolcQuery` becomes:

```go
func (s *Service) reqVolcQuery(query string) (io.ReadCloser, error) {
	return volcSignedGET("https://vod.volcengineapi.com/", query, s.client.Cookies)
}
```

Do not change `volcPlayInfoQuery` / `volcPrivateDrmQuery`; they already preserve signed prefixes.

- [ ] **Step 4: Run `go test ./backend/services -count=1`**

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/services/volc_http.go backend/services/volc_http_test.go backend/services/requester.go
git commit -m "$(cat <<'EOF'
Send Volc signed queries without Resty re-encoding

Resty reconstructs URL query maps and can alter signed V2 GetPlayInfo bytes. Issue the request with net/http and keep RawQuery identical to the already-validated token.
EOF
)"
```

---

### Task 3: Nil-safe download entrypoints

**Files:**
- Modify: `backend/app/download.go`
- Modify: `backend/download.go`
- Test: `backend/app/download_test.go` (create if missing) or extend `backend/download_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestOdobDownloadNilDataDoesNotPanic(t *testing.T) {
	d := app.OdobDownload{DownloadType: 1, Data: nil}
	err := d.Download()
	if err == nil {
		t.Fatal("expected error for nil odob payload")
	}
}

func TestEbookDownloadEmptyEnID(t *testing.T) {
	d := app.EBookDownload{DownloadType: 1, EnID: ""}
	err := d.Download()
	if err == nil {
		t.Fatal("expected error for empty ebook id")
	}
}

func TestCourseDownloadEmptyEnID(t *testing.T) {
	d := app.CourseDownload{DownloadType: 1, EnId: ""}
	err := d.Download()
	if err == nil {
		t.Fatal("expected error for empty course id")
	}
}
```

- [ ] **Step 2: Run and confirm current Odob test panics or returns success incorrectly**

Run: `go test ./backend/app -count=1 -run 'TestOdobDownloadNilData|TestEbookDownloadEmptyEnID|TestCourseDownloadEmptyEnID'`

- [ ] **Step 3: Implement guards at the top of each `Download()`**

```go
func (d *OdobDownload) Download() error {
	if d == nil || d.Data == nil {
		return errors.New("下载任务无效")
	}
	// existing body
}

func (d *EBookDownload) Download() error {
	if d == nil || strings.TrimSpace(d.EnID) == "" {
		return errors.New("下载任务无效")
	}
	// existing body
}

func (d *CourseDownload) Download() error {
	if d == nil || strings.TrimSpace(d.EnId) == "" {
		return errors.New("下载任务无效")
	}
	// existing body
}
```

Also guard `extOdobDownloadData`:

```go
func extOdobDownloadData(info *services.Course) []downloader.Datum {
	if info == nil {
		return nil
	}
	// existing body
}
```

- [ ] **Step 4: Re-run the new tests and `go test ./backend/app ./backend -count=1`**

Expected: PASS, no panic

- [ ] **Step 5: Commit**

```bash
git add backend/app/download.go backend/app/download_test.go backend/download.go backend/download_test.go
git commit -m "$(cat <<'EOF'
Reject empty download payloads before they can panic

Nil listen-book metadata and empty course/ebook ids crashed the macOS download path. Fail those entrypoints with the existing invalid-task error instead of dereferencing.
EOF
)"
```

---

### Task 4: Pin Wails v2.14.0 and bump golang.org/x/crypto

**Files:**
- Modify: `go.mod`, `go.sum`, `.wails-version`, `CLAUDE.md`

- [ ] **Step 1: Write a pin contract test in `backend/version_test.go` (extend)**

If a Wails pin test does not exist, add `scripts/wails_pin_test.sh` that asserts `.wails-version` equals `v2.14.0` and `go.mod` contains the same module version.

```bash
#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
pin="$(tr -d '[:space:]' < "${root}/.wails-version")"
test "${pin}" = "v2.14.0"
grep -q 'github.com/wailsapp/wails/v2 v2.14.0' "${root}/go.mod"
```

First run should fail.

- [ ] **Step 2: Run `bash scripts/wails_pin_test.sh` and watch it fail**

- [ ] **Step 3: Upgrade**

```bash
printf 'v2.14.0\n' > .wails-version
go get github.com/wailsapp/wails/v2@v2.14.0
go get golang.org/x/crypto@v0.45.0
go mod tidy
```

Update `CLAUDE.md` install line to `wails@v2.14.0`. Do not change `scripts/bootstrap.sh` logic; it already reads `.wails-version`.

If `go get` requires a newer Go, stop and record the constraint rather than jumping the toolchain past 1.23 unless `go.mod` already allows it.

- [ ] **Step 4: Run pin script, `go test ./... -count=1`, `go vet ./...`**

Expected: PASS. Fix `WindowShow` vs `Show` compile errors from Task 1 if the upgrade renamed APIs.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum .wails-version CLAUDE.md scripts/wails_pin_test.sh
git commit -m "$(cat <<'EOF'
Move the desktop runtime onto Wails v2.14 and current x/crypto

macOS 27 needs the Tahoe webview crash fix shipped in Wails v2.12+. Keep the CLI pin, module, and docs on one version and lift golang.org/x/crypto to the current advisory line.
EOF
)"
```

---

### Task 5: Off-screen list rendering cost

**Files:**
- Modify: `frontend/src/assets/css/global.css`
- Modify: `frontend/src/views/Ebook.vue` (class only if needed)
- Modify: `frontend/src/views/Odob.vue`
- Modify: `frontend/src/views/Course.vue`

- [ ] **Step 1: Add a source contract test**

`frontend/src/utils/listRendering.test.ts`:

```ts
import assert from 'node:assert/strict'
import test from 'node:test'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

test('card grids skip off-screen layout work', () => {
  const css = readFileSync(join(dirname(fileURLToPath(import.meta.url)), '../assets/css/global.css'), 'utf8')
  assert.match(css, /content-visibility:\s*auto/)
  assert.match(css, /contain-intrinsic-size:/)
})
```

- [ ] **Step 2: Run `npm --prefix frontend test` and confirm the new test fails**

- [ ] **Step 3: Add CSS**

```css
.ebook-card,
.course-card,
.audio-card {
  content-visibility: auto;
  contain-intrinsic-size: 280px;
}
```

Use the actual card class names already in those views. If they differ (`.ebook-card` is confirmed), inspect Odob/Course for the equivalent class and list every selector in the CSS rule. Do not add npm dependencies.

- [ ] **Step 4: Re-run frontend tests**

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add frontend/src/assets/css/global.css frontend/src/utils/listRendering.test.ts
git commit -m "$(cat <<'EOF'
Skip off-screen card layout in long shelves

Course, audiobook, and ebook grids paint every card. Mark those cards with content-visibility so the browser can skip off-screen layout without a new virtual-list dependency.
EOF
)"
```

---

### Task 6: Download completion notifications

**Files:**
- Create: `frontend/src/utils/downloadNotify.ts`
- Create: `frontend/src/utils/downloadNotify.test.ts`
- Modify: `frontend/src/components/DownloadDialog.vue`

- [ ] **Step 1: Write failing tests**

```ts
import assert from 'node:assert/strict'
import test from 'node:test'
import { downloadNotifyMessage } from './downloadNotify.ts'

test('completed downloads use a finite success copy', () => {
  assert.deepEqual(downloadNotifyMessage('completed', '得到·课程'), {
    title: '下载完成',
    message: '得到·课程',
    type: 'success',
  })
})

test('failed downloads keep the sanitized detail', () => {
  assert.equal(downloadNotifyMessage('failed', '下载失败，请检查网络连接后重试').type, 'error')
})

test('notifyDownloadEnd does not throw without Notification', () => {
  const { notifyDownloadEnd } = require('./downloadNotify.ts')
})
```

Implement `notifyDownloadEnd` to accept an injected `notify` function so tests never touch the real Notification API:

```ts
export const notifyDownloadEnd = (
  state: DownloadState,
  content: string,
  emit: (payload: { title: string; message: string; type: string }) => void,
) => {
  if (state !== 'completed' && state !== 'failed' && state !== 'cancelled') return
  emit(downloadNotifyMessage(state, content))
}
```

- [ ] **Step 2: Run frontend tests, expect missing module**

- [ ] **Step 3: Implement and wire DownloadDialog**

On `state` transition into `completed|failed|cancelled`, call `notifyDownloadEnd`. Use `ElNotification` as `emit`. If `window.Notification` exists, call `Notification.requestPermission` at most once per session (module-level flag) and `new Notification(title, { body: message })` when granted. Catch constructor errors.

- [ ] **Step 4: `npm --prefix frontend test` and `npm --prefix frontend run build`**

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add frontend/src/utils/downloadNotify.ts frontend/src/utils/downloadNotify.test.ts frontend/src/components/DownloadDialog.vue
git commit -m "$(cat <<'EOF'
Announce download outcomes without exposing internals

Minimized windows hid completion and failure. Surface the existing terminal download state through in-app notices and, when permitted, a system notification that reuses the sanitized status text.
EOF
)"
```

---

### Task 7: Full verification gate

**Files:** none new except plan checkbox updates

- [ ] **Step 1: Run the full deterministic suite from the worktree**

```bash
gofmt -w backend/window_policy.go backend/window_policy_test.go backend/services/volc_http.go backend/services/volc_http_test.go backend/app/download.go backend/app.go main.go
go vet ./...
go test ./... -count=1
go test -race ./backend/... -count=1
npm --prefix frontend test
npm --prefix frontend run build
bash scripts/secret-check.sh
bash scripts/wails_pin_test.sh
git diff --check
```

Expected: all pass. If `npm --prefix frontend` lacks `node_modules` in the worktree, `npm ci --prefix frontend --no-fund --no-audit` first.

- [ ] **Step 2: Confirm `rg 'WebviewIsTransparent: true'` in `main.go` is empty**

- [ ] **Step 3: Confirm `reqVolcQuery` calls `volcSignedGET`**

- [ ] **Step 4: Update this plan's checkboxes to `[x]`**

- [ ] **Step 5: Commit the spec/plan if not already committed**

Do not run `wails build` against the user's GUI. Do not launch `/Applications/dedao.app`.

---

## Out of scope leftovers

- Apple notarization
- Push and GitHub Release `v1.0.3` (prepare the tree; tagging is the final branch-finish step)
- Manual play of a DRM video
