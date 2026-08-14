# CI, Release, and Git History Rewrite Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Require one deterministic quality gate for macOS, Windows, and Linux releases; make tag, application metadata, asset names, and SHA-256 agree; then remove the exposed test file from every public revision and verify the rewritten GitHub repository from a fresh clone.

**Architecture:** Repository files pin Node and Wails versions. A metadata job validates one semantic version and every matrix build consumes it. Release publication depends on successful verification and build jobs and explicitly labels unsigned artifacts. History cleanup runs last in a private mirror with an immutable local backup, explicit expected remote object IDs, and post-push fresh-clone scanning.

**Tech Stack:** GitHub Actions, Go 1.23, Node 22.23.1, Wails v2.10.1, Bash/PowerShell, Git, `git-filter-repo`, GitHub CLI.

---

## Baseline evidence and operational gates

- `.github/workflows/release.yml` currently publishes with `always()`, so a failed build matrix does not reliably prevent the release job.
- Wails is installed with `@latest` in the workflow and local scripts, while `go.mod` uses Wails v2.10.1.
- `wails.json` invokes `npm install`, which can drift from `frontend/package-lock.json`.
- Wails defaults `info.productVersion` to `1.0.0`; the installed v1.0.1 macOS app was observed with 1.0.0 metadata.
- Remote `main` and remote tag `v1.0.1` currently resolve to `e486c2ccf108355d6971b4d9dbb0280782d33c8c`; local `main` is ahead by two design/setup commits.
- `git filter-repo` is not installed and `gh auth status` currently reports an invalid token. History rewriting must not start until both operational gates are resolved.
- The user has authorized rewriting public history and force-pushing branches/tags. This does not remove the need for exact-ref checks and a private backup.

## Files

- Create: `.nvmrc`
- Create: `.wails-version`
- Modify: `frontend/package.json`
- Modify: `wails.json`
- Create: `backend/version.go`
- Create: `backend/version_test.go`
- Create: `scripts/set-build-version.sh`
- Create: `scripts/set-build-version_test.sh`
- Modify: `scripts/bootstrap.sh`
- Modify: `scripts/release.sh`
- Modify: `scripts/install-wails-cli.sh`
- Modify: `README.md`
- Modify: `CLAUDE.md`
- Create: `.github/workflows/quality.yml`
- Modify: `.github/workflows/release.yml`
- Operational only: private backup mirror, rewrite mirror, fresh verification clone, GitHub refs/releases.

### Task 1: Pin build tools and expose one application version contract

- [ ] Add exact tool source files:

```text
# .nvmrc
22.23.1
```

```text
# .wails-version
v2.10.1
```

- [ ] Add a Node engine constraint to `frontend/package.json` without changing dependency versions:

```json
"engines": {
  "node": ">=22.23.1 <23"
}
```

- [ ] Change `wails.json` to use the lock file and an explicit development version:

```json
{
  "name": "dedao",
  "outputfilename": "dedao",
  "frontend:install": "npm ci --no-audit --no-fund",
  "frontend:build": "npm run build",
  "info": {
    "productVersion": "0.0.0"
  }
}
```

Preserve every existing author/bindings field when applying this fragment.

- [ ] Add `backend/version.go` and a unit test:

```go
package backend

var BuildVersion = "0.0.0-dev"

func (a *App) AppVersion() string {
	return BuildVersion
}
```

```go
func TestAppVersionReturnsBuildVersion(t *testing.T) {
	original := BuildVersion
	t.Cleanup(func() { BuildVersion = original })
	BuildVersion = "1.2.3"
	if got := NewApp().AppVersion(); got != "1.2.3" {
		t.Fatalf("AppVersion() = %q", got)
	}
}
```

- [ ] Create `scripts/set-build-version.sh`. Validate strict `X.Y.Z` input, update only `wails.json.info.productVersion`, and leave a newline-terminated JSON file:

```bash
#!/usr/bin/env bash
set -euo pipefail

version="${1:-}"
if [[ ! "${version}" =~ ^[0-9]+[.][0-9]+[.][0-9]+$ ]]; then
	echo "version must be X.Y.Z" >&2
	exit 1
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
project_file="${WAILS_PROJECT_FILE:-${repo_root}/wails.json}"
node - "${project_file}" "${version}" <<'NODE'
const fs = require('node:fs')
const [path, version] = process.argv.slice(2)
const project = JSON.parse(fs.readFileSync(path, 'utf8'))
project.info = { ...(project.info || {}), productVersion: version }
fs.writeFileSync(path, JSON.stringify(project, null, 2) + '\n')
NODE
```

- [ ] Create `scripts/set-build-version_test.sh` that copies `wails.json` into a temporary directory, runs the script with `WAILS_PROJECT_FILE` set to that copy, asserts `productVersion == 1.2.3`, and asserts `1.2` is rejected. The test must never edit the working tree.

```bash
#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
test_root="$(mktemp -d /private/tmp/dedao-version-test.XXXXXX)"
trap 'rm -rf "${test_root}"' EXIT
cp "${repo_root}/wails.json" "${test_root}/wails.json"

WAILS_PROJECT_FILE="${test_root}/wails.json" bash "${repo_root}/scripts/set-build-version.sh" 1.2.3
actual="$(node -p "require(process.argv[1]).info.productVersion" "${test_root}/wails.json")"
test "${actual}" = "1.2.3"

if WAILS_PROJECT_FILE="${test_root}/wails.json" bash "${repo_root}/scripts/set-build-version.sh" 1.2; then
	echo "invalid version was accepted" >&2
	exit 1
fi
```

- [ ] Replace every Wails `@latest` installation in `.github/workflows/release.yml`, `scripts/bootstrap.sh`, `scripts/release.sh`, `scripts/install-wails-cli.sh`, `README.md`, and `CLAUDE.md` with the version from `.wails-version` or the literal documentation value `v2.10.1`.

Shell scripts must use:

```bash
wails_version="$(tr -d '[:space:]' < "${ROOT_DIR}/.wails-version")"
go install "github.com/wailsapp/wails/v2/cmd/wails@${wails_version}"
```

- [ ] Make `scripts/release.sh` require `--version X.Y.Z` for packaged builds, run `scripts/set-build-version.sh`, pass `-ldflags`, and include the version in the archive name:

```bash
wails build --clean --platform "${TARGET}" \
  -ldflags "-X github.com/yann0917/dedao-gui/backend.BuildVersion=${VERSION}"
BASENAME="dedao-${VERSION}-${TARGET//\//-}"
```

- [ ] Verify pins and version mutation.

```bash
bash scripts/set-build-version_test.sh
go test ./backend -run TestAppVersion -count=1
test "$(tr -d '[:space:]' < .wails-version)" = "v2.10.1"
test "$(tr -d '[:space:]' < .nvmrc)" = "22.23.1"
rg -n '@latest|npm install' .github scripts README.md CLAUDE.md wails.json
```

Expected: tests pass and the final `rg` has no install-command match; prose discussing why floating versions are unsafe may remain only if it cannot be mistaken for a command.

- [ ] Commit deterministic tool and version sources.

```bash
git add .nvmrc .wails-version frontend/package.json wails.json backend/version.go backend/version_test.go scripts/set-build-version.sh scripts/set-build-version_test.sh scripts/bootstrap.sh scripts/release.sh scripts/install-wails-cli.sh README.md CLAUDE.md
git commit -m "Make release metadata derive from one reviewed version" \
  -m "Node and Wails are pinned, lockfile installation is mandatory, and local packaged builds stamp both Wails metadata and the backend version." \
  -m "Constraint: Release versions are strict numeric X.Y.Z values for platform metadata compatibility.
Rejected: Wails @latest | a moving CLI makes historical releases irreproducible
Confidence: high
Scope-risk: moderate
Directive: Update .wails-version and go.mod together after compatibility testing.
Tested: Version script contract, backend version binding, pin scans, and frontend build"
```

### Task 2: Add a pull-request quality gate

- [ ] Create `.github/workflows/quality.yml` with checkout, pinned Go/Node setup, dependency caches, security scan, formatting, vet, tests, race tests, and frontend build.

```yaml
name: Quality

on:
  pull_request:
  push:
    branches: [main]

jobs:
  verify:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true
      - uses: actions/setup-node@v4
        with:
          node-version-file: .nvmrc
          cache: npm
          cache-dependency-path: frontend/package-lock.json
      - run: go mod download
      - run: npm --prefix frontend ci --no-audit --no-fund
      - name: Check credentials
        run: bash scripts/secret-check.sh
      - name: Check Go formatting
        shell: bash
        run: |
          files="$(gofmt -l $(git ls-files '*.go'))"
          test -z "${files}"
      - run: go vet ./...
      - run: go test ./... -count=1
      - run: npm --prefix frontend run build
  race:
    strategy:
      matrix:
        os: [ubuntu-24.04, macos-14]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true
      - run: go mod download
      - run: go test -race ./backend/... -count=1
```

- [ ] Validate YAML structure locally without adding a parser dependency. Review indentation, then use GitHub’s workflow parser on the pushed implementation branch. Before push, run every shell command from the job locally.

```bash
bash scripts/secret-check.sh
test -z "$(gofmt -l $(git ls-files '*.go'))"
go vet ./...
go test ./... -count=1
go test -race ./backend/... -count=1
npm --prefix frontend ci --no-audit --no-fund
npm --prefix frontend run build
git diff --check
```

Expected: PASS; `npm ci` does not change `frontend/package-lock.json`.

- [ ] Commit the quality workflow.

```bash
git add .github/workflows/quality.yml
git commit -m "Stop unverified changes before the release matrix" \
  -m "Pull requests and main now share deterministic credential, formatting, vet, offline test, race, and frontend build checks." \
  -m "Constraint: Race testing runs on Linux here; macOS race coverage remains in the release matrix verification.
Confidence: high
Scope-risk: narrow
Directive: Release jobs must depend on an equivalent or stronger verification set.
Tested: Every workflow command locally
Not-tested: GitHub workflow parsing until branch push"
```

### Task 3: Make the three-platform release fail closed and publish checksums

- [ ] Rewrite `.github/workflows/release.yml` into `metadata -> verify -> build -> release` dependencies. Add a required manual-dispatch version input and validate either the input or tag:

```yaml
on:
  push:
    tags: ['v*']
  workflow_dispatch:
    inputs:
      version:
        description: Version in X.Y.Z form
        required: true

jobs:
  metadata:
    runs-on: ubuntu-24.04
    outputs:
      version: ${{ steps.version.outputs.version }}
    steps:
      - id: version
        shell: bash
        env:
          INPUT_VERSION: ${{ inputs.version }}
        run: |
          version="${INPUT_VERSION:-${GITHUB_REF_NAME#v}}"
          [[ "${version}" =~ ^[0-9]+[.][0-9]+[.][0-9]+$ ]]
          echo "version=${version}" >> "${GITHUB_OUTPUT}"
```

- [ ] Make `verify` reproduce the quality checks and add a `race` job on `ubuntu-24.04` and `macos-14` before any platform build. Make `build` declare `needs: [metadata, verify, race]`. Set Node with `.nvmrc` and install Wails from `.wails-version`:

```yaml
- name: Install Wails
  shell: bash
  run: |
    version="$(tr -d '[:space:]' < .wails-version)"
    go install "github.com/wailsapp/wails/v2/cmd/wails@${version}"
    echo "$(go env GOPATH)/bin" >> "${GITHUB_PATH}"
```

- [ ] Before each matrix build, stamp the Wails project and backend:

```yaml
- name: Verify tests on build host
  run: go test ./... -count=1

- name: Stamp version
  shell: bash
  run: bash scripts/set-build-version.sh "${{ needs.metadata.outputs.version }}"

- name: Build application
  shell: bash
  run: |
    wails build --clean --platform "${{ matrix.platform }}" \
      -ldflags "-X github.com/yann0917/dedao-gui/backend.BuildVersion=${{ needs.metadata.outputs.version }}"
```

- [ ] Name every archive `dedao-${version}-${platform}` and generate an adjacent `.sha256`. On Unix use `shasum -a 256`; on Windows use `Get-FileHash` and write lowercase hash plus basename.

```powershell
$archive = "release/dedao-${{ needs.metadata.outputs.version }}-windows-amd64.zip"
$hash = (Get-FileHash $archive -Algorithm SHA256).Hash.ToLower()
"$hash  $([System.IO.Path]::GetFileName($archive))" | Set-Content "$archive.sha256"
```

- [ ] Add platform metadata assertions before upload:

macOS:

```bash
test "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' build/bin/dedao.app/Contents/Info.plist)" = "${VERSION}"
test "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleVersion' build/bin/dedao.app/Contents/Info.plist)" = "${VERSION}"
```

Windows:

```powershell
if ((Get-Item build/bin/dedao.exe).VersionInfo.ProductVersion -ne $env:VERSION) { exit 1 }
```

Linux: assert that the archive basename contains the exact version and that `build/bin/dedao` exists and is executable.

- [ ] Add a five-second process smoke check before packaging. Extend the Linux dependency step with `xvfb`. A process that exits early fails the build; each check must stop the process it launched.

Linux:

```bash
xvfb-run -a build/bin/dedao > smoke.log 2>&1 &
smoke_pid=$!
sleep 5
kill -0 "${smoke_pid}"
kill "${smoke_pid}"
wait "${smoke_pid}" || true
```

macOS:

```bash
open build/bin/dedao.app
sleep 5
smoke_pid="$(pgrep -f '/dedao.app/Contents/MacOS/dedao' | head -n 1)"
test -n "${smoke_pid}"
kill "${smoke_pid}"
```

Windows:

```powershell
$process = Start-Process -FilePath build/bin/dedao.exe -PassThru
Start-Sleep -Seconds 5
if ($process.HasExited) { throw "dedao exited during smoke check with $($process.ExitCode)" }
Stop-Process -Id $process.Id
```

- [ ] Remove `always()` from the release job. Require `needs: [metadata, build]`, publish only for tag refs, and place this statement in release notes:

```text
这些自动构建产物未配置 Apple 公证或 Windows 代码签名证书，属于未签名测试包。请使用随附的 SHA-256 文件校验下载完整性。
```

Do not imply notarization or code signing. Keep signing as a later protected-credential job, not a repository secret.

- [ ] Run local release-equivalent checks for macOS before pushing:

```bash
bash scripts/set-build-version.sh 1.0.2
go run github.com/wailsapp/wails/v2/cmd/wails@v2.10.1 build --clean --platform darwin/universal \
  -ldflags "-X github.com/yann0917/dedao-gui/backend.BuildVersion=1.0.2"
/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' build/bin/dedao.app/Contents/Info.plist
/usr/libexec/PlistBuddy -c 'Print :CFBundleVersion' build/bin/dedao.app/Contents/Info.plist
git restore wails.json
```

Expected: both plist values are `1.0.2`; `git restore` reverts only the generated version stamp, not unrelated changes.

- [ ] Commit the release gate.

```bash
git add .github/workflows/release.yml
git commit -m "Publish only artifacts that prove their source version and integrity" \
  -m "Three platform builds depend on verification, assert embedded metadata, include SHA-256 files, and fail closed before release creation." \
  -m "Constraint: Public artifacts remain explicitly unsigned until protected signing credentials exist.
Rejected: Keeping release always() | it can publish an incomplete matrix after failure
Confidence: high
Scope-risk: broad
Directive: Tag, embedded version, asset basename, and checksum basename must remain identical.
Tested: Local quality gate, macOS universal build/smoke, plist version assertions, and checksum generation
Not-tested: Windows and Linux runners until GitHub Actions"
```

### Task 4: Obtain clean remote authority and prepare the immutable backup

- [ ] Re-authenticate GitHub CLI interactively and verify repository write authority. This is a required external-authority gate, not optional cleanup.

```bash
gh auth login -h github.com
gh auth status
gh repo view sunjixin2023/dedao-gui --json nameWithOwner,viewerPermission
```

Expected: active authentication and `viewerPermission` of `ADMIN`, `MAINTAIN`, or `WRITE`. Do not proceed with rewriting if authentication remains invalid.

- [ ] Install `git-filter-repo` only after explicit one-time tooling approval if it is still absent. Verify the executable before using it:

```bash
git filter-repo --version
```

Expected: a version string. Do not substitute deprecated `git filter-branch` to avoid the approval gate.

- [ ] Finish and merge all implementation commits into local `main`, then require a clean tree and capture exact local/remote refs:

```bash
git -C /Users/jasonsun/dedao-gui status --short --branch
git -C /Users/jasonsun/dedao-gui rev-parse main
git -C /Users/jasonsun/dedao-gui ls-remote --heads --tags origin
git -C /Users/jasonsun/dedao-gui show-ref --head --dereference
```

Expected: no uncommitted changes; record final local `main`, remote `main`, and peeled tag object IDs in a timestamped receipt.

- [ ] Create a private mirror and bundle outside the public checkout. Use a concrete timestamp value assigned once; do not leave command substitution unresolved in destructive commands.

```bash
umask 077
backup_root="/Users/jasonsun/dedao-gui-private-backups/20260814T-rewrite"
mkdir -p "${backup_root}"
git clone --mirror /Users/jasonsun/dedao-gui "${backup_root}/original.git"
git --git-dir="${backup_root}/original.git" show-ref --head --dereference > "${backup_root}/refs-before.txt"
git --git-dir="${backup_root}/original.git" bundle create "${backup_root}/original.bundle" --all
shasum -a 256 "${backup_root}/original.bundle" > "${backup_root}/original.bundle.sha256"
chmod -R a-w "${backup_root}"
```

Replace `20260814T-rewrite` with the actual once-recorded timestamp directory during execution. Expected: mirror, ref receipt, bundle, and checksum exist with owner-only access. This backup remains private because it intentionally contains the exposed history.

### Task 5: Rewrite every public ref in an isolated mirror

- [ ] Clone a separate rewrite mirror from the completed local repository into a newly created temporary directory:

```bash
rewrite_root="$(mktemp -d /private/tmp/dedao-rewrite.XXXXXX)"
git clone --mirror /Users/jasonsun/dedao-gui "${rewrite_root}/sanitized.git"
```

- [ ] Remove `backend/services/service_test.go` from every branch and tag. The sanitized integration test has a different filename and remains intact.

```bash
git -C "${rewrite_root}/sanitized.git" filter-repo --force \
  --path backend/services/service_test.go \
  --invert-paths
```

- [ ] Create a disposable worktree clone from the sanitized mirror and run current-tree plus all-history scanning:

```bash
git clone "${rewrite_root}/sanitized.git" "${rewrite_root}/verify"
cd "${rewrite_root}/verify"
bash scripts/secret-check.sh
bash scripts/secret-check.sh --history
git log --all -- backend/services/service_test.go
```

Expected: both scans pass; the final log command produces no commits.

- [ ] Record rewritten refs and an old-to-new mapping for `main`, `v1.0.0`, and `v1.0.1` in the private receipt directory. Temporarily restore owner write permission only for receipt files, then make the directory read-only again.

- [ ] Re-add the exact public remote after `filter-repo` removes it, fetch current refs, and compare them with the previously recorded expected values:

```bash
git -C "${rewrite_root}/sanitized.git" remote add origin https://github.com/sunjixin2023/dedao-gui.git
git -C "${rewrite_root}/sanitized.git" fetch origin '+refs/heads/*:refs/remotes/origin/*' '+refs/tags/*:refs/remotes/origin-tags/*'
git -C "${rewrite_root}/sanitized.git" ls-remote --heads --tags origin
```

Expected: remote refs still match the pre-rewrite receipt. If any ref moved, stop and recompute the rewrite from the new remote state; do not overwrite unexpected work.

### Task 6: Force-update GitHub and verify from a fresh clone

- [ ] Push rewritten `main` with an explicit lease against the recorded old remote SHA, then update the two existing release tags explicitly:

```bash
old_remote_main="e486c2ccf108355d6971b4d9dbb0280782d33c8c"
git -C "${rewrite_root}/sanitized.git" push origin \
  --force-with-lease="refs/heads/main:${old_remote_main}" \
  refs/heads/main:refs/heads/main
git -C "${rewrite_root}/sanitized.git" push origin --force \
  refs/tags/v1.0.0:refs/tags/v1.0.0 \
  refs/tags/v1.0.1:refs/tags/v1.0.1
```

Before execution, replace `old_remote_main` with the live recorded value if it differs. Do not use a broad branch wildcard.

- [ ] Verify GitHub refs and release/tag association:

```bash
git -C "${rewrite_root}/sanitized.git" ls-remote --heads --tags origin
gh release view v1.0.0 --repo sunjixin2023/dedao-gui --json tagName,url
gh release view v1.0.1 --repo sunjixin2023/dedao-gui --json tagName,url
```

- [ ] Fresh-clone the public remote into a second new temporary directory and rerun the evidence suite:

```bash
fresh_root="$(mktemp -d /private/tmp/dedao-fresh.XXXXXX)"
git clone https://github.com/sunjixin2023/dedao-gui.git "${fresh_root}/dedao-gui"
cd "${fresh_root}/dedao-gui"
bash scripts/secret-check.sh
bash scripts/secret-check.sh --history
go test ./... -count=1
go test -race ./backend/... -count=1
npm --prefix frontend ci --no-audit --no-fund
npm --prefix frontend run build
```

Expected: all checks pass against the GitHub clone, not the pre-rewrite local object store.

- [ ] Quarantine the old working repository rather than merging it back. Move it to the private backup area, clone the rewritten public repository anew at `/Users/jasonsun/dedao-gui`, and set the quarantined directory owner-only/read-only. Do not copy `.git`, old refs, or old worktrees into the clean clone.

- [ ] Publish the next patch release as `v1.0.2` only after the rewritten `main` and three-platform workflow are green:

```bash
cd /Users/jasonsun/dedao-gui
scripts/deploy.sh v1.0.2
run_id="$(gh run list --repo sunjixin2023/dedao-gui --workflow release.yml --branch v1.0.2 --limit 1 --json databaseId --jq '.[0].databaseId')"
test -n "${run_id}"
gh run watch "${run_id}" --repo sunjixin2023/dedao-gui --exit-status
gh release view v1.0.2 --repo sunjixin2023/dedao-gui --json url,assets,body
```

Expected: macOS universal, Windows amd64, and Linux amd64 archives plus matching `.sha256` files; release notes explicitly say the artifacts are unsigned.

## Final acceptance

- [ ] Download every v1.0.2 asset and verify its adjacent checksum.
- [ ] Compare tag `1.0.2`, macOS plist `1.0.2`, Windows ProductVersion `1.0.2`, Linux/archive asset names, and `AppVersion()` output.
- [ ] Launch the downloaded macOS app on the actual machine and repeat login/download/recovery smoke checks.
- [ ] Preserve the GitHub Actions run URL, release URL, fresh-clone commit SHA, ref mapping, and local checksum receipt.
- [ ] Notify all clone holders: old commit IDs are invalid; delete/reclone; never merge or push an old clone.

## Completion criteria

- No workflow or script installs Wails from a floating version.
- Pull requests and releases run offline tests, race tests, security scan, vet, and frontend build.
- Failed matrix jobs cannot publish a release.
- Each asset has a matching SHA-256 and explicit unsigned status when signing credentials are absent.
- Rewritten public branches/tags and a fresh GitHub clone pass history scanning.
- The next release’s tag, embedded metadata, asset names, and checksums all use `1.0.2`.
- The old contaminated repository remains private and cannot accidentally be pushed.
