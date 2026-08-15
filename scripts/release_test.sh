#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
test_root="$(mktemp -d /private/tmp/dedao-release-test.XXXXXX)"
fake_bin="${test_root}/bin"
pause_file="${test_root}/resume"
wails_backup="${test_root}/wails.json.backup"
mkdir -p "${fake_bin}"

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
