# Credentials and Offline Test Baseline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove credential-bearing tests from the current tree, replace network-dependent tests with deterministic fixtures, and add a repository secret gate that fails before sensitive values can be committed again.

**Architecture:** Default tests use only the Go standard library and local `httptest.Server` instances. Any manual live-account smoke test is isolated behind the `integration` build tag and reads credentials from the environment. A shell gate scans tracked source now and can scan every Git revision later; the actual history rewrite is intentionally deferred to the release/history plan.

**Tech Stack:** Go 1.23, `testing`, `net/http/httptest`, Bash, ripgrep, Git.

---

## Baseline evidence

- `backend/services/service_test.go` contains a full real Cookie, token-like values, phone/user data, and live requests that mostly print errors instead of asserting.
- `backend/utils/utils_test.go::TestM3u8URLs` contacts a real CDN and is the only reason `go test ./...` currently fails offline.
- `go test ./backend/...` and `go test ./...` currently fail at `TestM3u8URLs` when external networking is unavailable.
- This plan does not claim the exposed credentials become safe after deletion; rotation and Git history rewriting remain required.

## Files

- Delete: `backend/services/service_test.go`
- Create: `backend/services/service_integration_test.go`
- Modify: `backend/utils/utils_test.go`
- Create: `scripts/secret-check.sh`
- Create: `scripts/secret-check_test.sh`
- Modify later in the release plan: `.github/workflows/release.yml`

### Task 1: Replace credential-bearing pseudo-tests with one explicit manual smoke test

- [ ] Record the current failure without printing secret values.

Run:

```bash
go test ./backend/services -count=1
```

Expected: the package may report success even when live calls fail because the current tests do not assert; this is the behavior defect being removed.

- [ ] Delete `backend/services/service_test.go` completely. Do not copy any string literal or personal value from it into a fixture, plan, commit message, or issue.

- [ ] Create `backend/services/service_integration_test.go` with an explicit build tag and environment-only credential input:

```go
//go:build integration

package services

import (
	"os"
	"testing"
)

func TestIntegrationUserProfile(t *testing.T) {
	cookie := os.Getenv("DEDAO_TEST_COOKIE")
	if cookie == "" {
		t.Skip("DEDAO_TEST_COOKIE is required for the manual integration test")
	}

	var opts CookieOptions
	if err := ParseCookies(cookie, &opts); err != nil {
		t.Fatalf("parse integration cookie: %v", err)
	}
	user, err := NewService(&opts).User()
	if err != nil {
		t.Fatalf("load user profile: %v", err)
	}
	if user == nil || user.UIDHazy == "" {
		t.Fatal("expected a non-empty user profile")
	}
}
```

- [ ] Verify the manual test is excluded by default and does not embed credentials.

Run:

```bash
go test ./backend/services -count=1
go test -tags=integration ./backend/services -run TestIntegrationUserProfile -count=1
```

Expected: the first command passes without network access; the second command skips when `DEDAO_TEST_COOKIE` is unset.

- [ ] Commit the test-boundary change.

```bash
git add backend/services/service_test.go backend/services/service_integration_test.go
git commit -m "Prevent account credentials from serving as test fixtures" \
  -m "Live account coverage is now an explicit environment-driven smoke test, while the default suite remains offline." \
  -m "Constraint: Exposed credentials must still be rotated and removed from Git history.
Rejected: Redacting only the visible cookie literal | the remaining live tests still lacked assertions and required external state
Confidence: high
Scope-risk: narrow
Directive: Never add account cookies or response dumps to repository tests.
Tested: Default services tests and integration-tag skip path
Not-tested: Live account endpoint behavior"
```

### Task 2: Lock the playlist parser to a local HTTP fixture

- [ ] Replace the existing `TestM3u8URLs` with a failing-first table test. The test must serve both relative and absolute media paths and must not call an external host.

```go
package utils

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestM3u8URLsResolvesMediaSegments(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		_, _ = fmt.Fprintf(w, "#EXTM3U\nsegment-1.ts\n%s/absolute.ts\n", server.URL)
	}))
	defer server.Close()

	got, err := M3u8URLs(server.URL + "/media/index.m3u8")
	if err != nil {
		t.Fatalf("M3u8URLs returned error: %v", err)
	}
	want := []string{
		server.URL + "/media/segment-1.ts",
		server.URL + "/absolute.ts",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("M3u8URLs() = %#v, want %#v", got, want)
	}
}
```

- [ ] Add the empty-address contract as a separate test:

```go
func TestM3u8URLsRejectsEmptyAddress(t *testing.T) {
	if _, err := M3u8URLs(""); err == nil {
		t.Fatal("expected an error for an empty playlist address")
	}
}
```

- [ ] Run the focused and repository tests.

```bash
go test ./backend/utils -run TestM3u8URLs -count=1
go test ./backend/... -count=1
go test ./... -count=1
```

Expected: all commands pass without external network access.

- [ ] Commit the offline fixture.

```bash
git add backend/utils/utils_test.go
git commit -m "Make the default test suite independent of the public CDN" \
  -m "The playlist parser now runs against a deterministic local server and asserts URL resolution." \
  -m "Constraint: Default tests must run without an account or internet access.
Confidence: high
Scope-risk: narrow
Directive: Keep live API checks behind the integration build tag.
Tested: Focused playlist tests and full Go suite"
```

### Task 3: Add a source and history secret gate

- [ ] Create `scripts/secret-check.sh`. It must print only commit IDs and file paths, never matched secret text.

```bash
#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${repo_root}"

pattern='(GAT|csrfToken|_sid|iget)=[^";[:space:]]{16,}|eyJ[A-Za-z0-9_-]{20,}[.]eyJ[A-Za-z0-9_-]{20,}[.][A-Za-z0-9_-]{16,}|(cookie|token|sign)[[:space:]]*:=[[:space:]]*"[^"[:space:]]{20,}"|"1[3-9][0-9]{9}"'
paths=(backend frontend scripts .github)
exclude=(":(exclude)scripts/secret-check.sh" ":(exclude)scripts/secret-check_test.sh" ":(exclude)frontend/src/assets/**")

scan_tree() {
	local ref="$1"
	local output
	if output="$(git grep -I -l -E "${pattern}" "${ref}" -- "${paths[@]}" "${exclude[@]}" 2>/dev/null)"; then
		printf 'credential-like content found in %s:\n%s\n' "${ref}" "${output}"
		return 1
	fi
}

scan_worktree() {
	local files=()
	local output
	while IFS= read -r -d '' file; do
		files+=("${file}")
	done < <(git ls-files --cached --others --exclude-standard -z -- \
		backend frontend scripts .github \
		':(exclude)scripts/secret-check.sh' \
		':(exclude)scripts/secret-check_test.sh' \
		':(exclude)frontend/src/assets/**')
	if [[ "${#files[@]}" -eq 0 ]]; then
		return 0
	fi
	if output="$(rg -I -l -e "${pattern}" -- "${files[@]}")"; then
		printf 'credential-like content found in working tree:\n%s\n' "${output}"
		return 1
	fi
}

if [[ "${1:-}" == "--history" ]]; then
	failed=0
	while IFS= read -r commit; do
		if ! scan_tree "${commit}"; then
			failed=1
		fi
	done < <(git rev-list --all)
	exit "${failed}"
fi

scan_worktree
printf 'secret-check passed\n'
```

- [ ] Create `scripts/secret-check_test.sh` to prove both a safe tree and a synthetic secret are classified correctly without touching the real working tree:

```bash
#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${repo_root}"

bash scripts/secret-check.sh

fixture="backend/services/secret_check_fixture_test.go"
trap 'rm -f "${fixture}"' EXIT
fixture_value="0123456789abcdefghijklmnop"
printf 'package services\nvar leaked = "csrfToken=%s"\n' "${fixture_value}" > "${fixture}"

if bash scripts/secret-check.sh; then
	echo "secret-check accepted a credential fixture" >&2
	exit 1
fi
```

- [ ] Make both scripts executable, run the failing baseline before deleting the old secret-bearing file if task ordering permits, then run the clean-tree gate.

```bash
chmod +x scripts/secret-check.sh scripts/secret-check_test.sh
bash scripts/secret-check_test.sh
bash scripts/secret-check.sh
```

Expected: the synthetic fixture is rejected, the fixture is removed by the trap, and the clean repository passes. `bash scripts/secret-check.sh --history` is expected to fail until the history-rewrite plan is executed.

- [ ] Commit the gate.

```bash
git add scripts/secret-check.sh scripts/secret-check_test.sh
git commit -m "Block credential-shaped literals before they become repository history" \
  -m "The scanner covers tracked and untracked source without echoing matched values, and exposes a separate all-revisions audit mode." \
  -m "Constraint: The current public history remains contaminated until the final rewrite step.
Rejected: A hosted secret scanner dependency | the project needs a local deterministic gate with no new service dependency
Confidence: medium
Scope-risk: narrow
Directive: Keep scanner output limited to paths and revision IDs.
Tested: Safe tree and synthetic leak rejection
Not-tested: Historical pass before rewrite"
```

### Task 4: Establish the clean offline baseline receipt

- [ ] Run formatting and the complete backend suite.

```bash
gofmt -w backend/services/service_integration_test.go backend/utils/utils_test.go
go test ./backend/... -count=1
go test ./... -count=1
bash scripts/secret-check.sh
git diff --check
```

Expected: every command passes; no test contacts a real 得到 host; `git diff --check` produces no output.

- [ ] Record the remaining security boundary in the implementation handoff:

```text
Current tree: sanitized and protected by the local gate.
Public history: still unsafe until the dedicated mirror rewrite and fresh-clone verification.
Credential status: exposed values must be treated as revoked independently of Git cleanup.
```

## Completion criteria

- `backend/services/service_test.go` no longer exists in the current tree.
- The only live-account test is tagged `integration` and reads `DEDAO_TEST_COOKIE` at runtime.
- `go test ./...` passes with networking unavailable.
- `scripts/secret-check_test.sh` proves a synthetic leak is rejected.
- `scripts/secret-check.sh` passes on the current tree and does not print matched content.
- History cleanup remains explicitly open for `2026-08-14-ci-release-history-rewrite.md`.
