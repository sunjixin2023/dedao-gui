# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a desktop application built with Wails v2 (Go + Vue.js) for downloading courses from the Chinese education platform "得到" (Dedao). The application uses a QR code login system and provides various content download capabilities.

**Important Note**: This application downloads copyrighted educational content. Any modifications should respect intellectual property rights and terms of service.

## Development Commands

### Prerequisites
- Go 1.25+
- Node.js 22.23.1
- Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@v2.14.0`

### Build & Development
```bash
# Install frontend dependencies
npm ci --prefix frontend --no-fund --no-audit

# Development mode (hot reload)
wails dev

# Build production binary
wails build

# Build for specific platforms
# macOS Intel
wails build -platform darwin/amd64
# macOS ARM
wails build -platform darwin/arm64
# Windows
wails build -platform windows/amd64
```

### Testing
```bash
# Run backend tests
go test ./backend/...

# Run frontend unit tests (volc playback + chrome contracts)
npm --prefix frontend test

# Run specific test
go test ./backend/utils -v
```

## Architecture

### Backend Structure (Go)
- **Entry Point**: `main.go` - Wails app initialization
- **Core App**: `backend/app.go` - Application lifecycle management
- **Services** (`backend/services/`): Business logic layer
  - `service.go` - Base service with HTTP client configuration
  - `requester.go` - All API endpoint implementations
  - Content-specific services: `course.go`, `ebook.go`, `article.go`, etc.
- **Downloader** (`backend/downloader/`): File download implementations
- **Utils** (`backend/utils/`): Shared utilities and BadgerDB management

### Frontend Structure (Vue 3 + TypeScript)
- **Framework**: Vue 3 with Element Plus UI
- **State Management**: Pinia with persistence
- **Build Tool**: Vite
- **Key Directories**:
  - `src/views/` - Page components
  - `src/components/` - Reusable components
  - `src/stores/` - Pinia stores
  - `wailsjs/` - Wails-generated bindings

### Key Patterns

1. **Authentication**: Cookie-based with multiple tokens (GAT, ISID, csrfToken)
2. **Content Types**:
   - Courses (bauhinia)
   - Daily audiobooks (odob)
   - Ebooks (ebook)
   - Premium content (compass)
3. **Anti-Scraping**: The ebook service implements sophisticated rate limiting with token bucket algorithm, exponential backoff, and request jitter
4. **Data Storage**: BadgerDB for local caching and configuration

### External Dependencies
- **wkhtmltopdf**: Required for PDF generation
- **ffmpeg**: Required for audio processing

## Common Tasks

### Adding a New Service
1. Create new service file in `backend/services/`
2. Implement methods following the existing pattern from other services
3. Add corresponding methods to `requester.go`
4. Generate TypeScript bindings: `wails generate module`

### Modifying Frontend Components
1. Vue components use `<script setup lang="ts">` syntax
2. Element Plus components are auto-imported
3. Use Pinia stores for state management
4. Access backend via `window.go.backend.ServiceName.MethodName()`

### Handling API Responses
All services follow this pattern:
```go
func (s *Service) MethodName() (result *ResultType, err error) {
    body, err := s.reqMethodName()
    if err != nil {
        return
    }
    defer body.Close()
    if err = handleJSONParse(body, &result); err != nil {
        return
    }
    return
}
```

### Configuration Management
- User config: Stored in BadgerDB at `~/.config/dedao/`
- App config: `wails.json` for Wails settings
- Frontend config: `vite.config.ts` for build settings

### Signed V2 private DASH (desktop)

Civilization-course playback on `wails://` is a local VePlayer 1.15.1 + DashPlugin path. Do not regress these constraints:

1. **Private DRM, not Widevine.** `desktopPlayerChrome` aside, `privateDashPluginOptions()` must keep `useEME: false`. VePlayer defaults `useEME: true`, which waits for a CDM license and spins forever while segments still download.
2. **SDK license.** VePlayer 1.11+ checks a domain-bound WASM license. Install `_needExemptions` *before* the script loads (`installVePlayerLicenseExemption`). Do **not** put `_Module` on `Object.prototype` — that hangs DASH decrypt WASM. Re-apply `installVePlayerSdkLicenseBypass` *after* `destroyPlayer()`; destroy must not restore the bypass before `new VePlayer()`.
3. **Fake XHR.** Native `status` / `readyState` are getter-only. Assigning them in a module throws and skips `send()` (error 7200). Shadow fields with `defineProperty` via the volc URL bridge.
4. **Media/license proxy.** `ProxyVolcVodGet` and `ProxyMediaGet` exist because `wails://` cannot CORS-fetch `vod.volcengineapi.com` or `*.umiwi.com`. Allowlist hosts only; rewrite `http://` to `https://`. Probe log: macOS temp `dedao-playback-probe.log` (no tokens).
5. **Fullscreen / PiP.** macOS WKWebView fullscreen is off unless `DesktopMacWindowPolicy().FullscreenEnabled` sets `mac.Preferences.FullscreenEnabled`. On `wails:`, force CSS fullscreen and a CSS PiP fallback (`desktopPlayerChromeOptions`, `installDesktopPipAvailability`). Document PiP is https/file only.
6. **Definition.** Keep `volcDefinitionMap()` covering 360p–1080p/`original`, `definition.isShowIcon: true`, and `autoBitrateOpts.enable: false` so the user can pick a rung instead of ABR.
7. **Probe log.** Do not record every successful `media_proxy` line. Rotate `dedao-playback-probe.log` after 256KiB. Never write tokens.

Frontend tests: `npm --prefix frontend test`. Backend: `go test ./backend/...`.
