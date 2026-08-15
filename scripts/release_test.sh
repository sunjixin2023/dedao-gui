#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
test_root="$(mktemp -d /private/tmp/dedao-release-test.XXXXXX)"
fake_bin="${test_root}/bin"
pause_file="${test_root}/resume"
wails_backup="${test_root}/wails.json.backup"
release_workflow="${repo_root}/.github/workflows/release.yml"
mkdir -p "${fake_bin}"

expected_package_md5="$(tr -d '[:space:]' < "${repo_root}/frontend/package.json.md5")"
actual_package_md5="$(node -e 'const crypto = require("node:crypto"); const fs = require("node:fs"); process.stdout.write(crypto.createHash("md5").update(fs.readFileSync(process.argv[1])).digest("hex"))' "${repo_root}/frontend/package.json")"
if [[ "${expected_package_md5}" != "${actual_package_md5}" ]]; then
	echo "frontend/package.json.md5 is stale" >&2
	exit 1
fi

cp "${repo_root}/wails.json" "${wails_backup}"
cleanup() {
	cp "${wails_backup}" "${repo_root}/wails.json"
	rm -rf "${test_root}"
}
trap cleanup EXIT

cat > "${fake_bin}/go" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "env" && "${2:-}" == "GOPATH" ]]; then
	echo "/tmp/dedao-release-test-gopath"
	exit 0
fi
echo "unexpected go invocation: $*" >&2
exit 1
EOF

cat > "${fake_bin}/npm" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
exit 0
EOF

cat > "${fake_bin}/wails" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ " $* " != *" -skipbindings "* ]]; then
	echo "release build must use the reviewed committed bindings" >&2
	exit 92
fi
if [[ -n "${FAKE_WAILS_WAIT_FILE:-}" ]]; then
	while [[ ! -f "${FAKE_WAILS_WAIT_FILE}" ]]; do
		sleep 0.1
	done
fi
exit "${FAKE_WAILS_EXIT_CODE:-0}"
EOF

chmod +x "${fake_bin}/go" "${fake_bin}/npm" "${fake_bin}/wails"

assert_contains() {
	local needle="$1"
	local file="$2"
	if ! grep -Fq -- "$needle" "$file"; then
		echo "missing expected content in ${file}: ${needle}" >&2
		exit 1
	fi
}

run_release() {
	PATH="${fake_bin}:${PATH}" bash "${repo_root}/scripts/release.sh" auto --skip-install --no-package --version 1.2.3
}

run_release
cmp -s "${repo_root}/wails.json" "${wails_backup}"

if FAKE_WAILS_EXIT_CODE=91 run_release; then
	echo "release script unexpectedly succeeded with failing wails" >&2
	exit 1
fi
cmp -s "${repo_root}/wails.json" "${wails_backup}"

rm -f "${pause_file}"
RELEASE_TEST_PAUSE_AFTER_VERSION_STAMP_FILE="${pause_file}" FAKE_WAILS_WAIT_FILE="${pause_file}" run_release &
first_pid=$!

sleep 1

if PATH="${fake_bin}:${PATH}" bash "${repo_root}/scripts/release.sh" auto --skip-install --no-package --version 1.2.3; then
	echo "concurrent release invocation unexpectedly succeeded" >&2
	kill "${first_pid}" 2>/dev/null || true
	exit 1
fi

touch "${pause_file}"
wait "${first_pid}"
cmp -s "${repo_root}/wails.json" "${wails_backup}"

assert_contains 'wails build --clean -skipbindings --platform "${{ matrix.platform }}"' "${release_workflow}"
assert_contains 'app_exec="${app_path}/Contents/MacOS/dedao"' "${release_workflow}"
assert_contains '"${app_exec}" > smoke.log 2>&1 &' "${release_workflow}"
assert_contains 'smoke_pid=$!' "${release_workflow}"
assert_contains 'if [[ -n "${smoke_pid}" ]] && kill -0 "${smoke_pid}" 2>/dev/null; then' "${release_workflow}"
assert_contains 'xvfb-run -a build/bin/dedao > smoke.log 2>&1 &' "${release_workflow}"
assert_contains 'kill -0 "${smoke_pid}"' "${release_workflow}"
assert_contains 'sha256sum -c "dedao-${VERSION}-macos-universal.tar.gz.sha256"' "${release_workflow}"
assert_contains 'sha256sum -c "dedao-${VERSION}-windows-amd64.zip.sha256"' "${release_workflow}"
assert_contains 'sha256sum -c "dedao-${VERSION}-linux-amd64.tar.gz.sha256"' "${release_workflow}"
assert_contains 'try {' "${release_workflow}"
assert_contains '} finally {' "${release_workflow}"
assert_contains 'if ($process -and -not $process.HasExited)' "${release_workflow}"
