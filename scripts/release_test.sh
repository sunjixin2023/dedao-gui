#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
test_root="$(mktemp -d /private/tmp/dedao-release-test.XXXXXX)"
fake_bin="${test_root}/bin"
pause_file="${test_root}/resume"
wails_backup="${test_root}/wails.json.backup"
release_workflow="${repo_root}/.github/workflows/release.yml"
quality_workflow="${repo_root}/.github/workflows/quality.yml"
codeql_workflow="${repo_root}/.github/workflows/codeql.yml"
windows_info="${repo_root}/build/windows/info.json"
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

assert_not_contains() {
	local needle="$1"
	local file="$2"
	if grep -Fq -- "$needle" "$file"; then
		echo "unexpected content in ${file}: ${needle}" >&2
		exit 1
	fi
}

line_of() {
	local needle="$1"
	local file="$2"
	local match
	match="$(grep -nF -- "$needle" "$file" | head -n 1 || true)"
	if [[ -z "${match}" ]]; then
		echo "missing expected line in ${file}: ${needle}" >&2
		exit 1
	fi
	echo "${match%%:*}"
}

line_of_occurrence() {
	local needle="$1"
	local file="$2"
	local occurrence="$3"
	local match
	match="$(grep -nF -- "$needle" "$file" | sed -n "${occurrence}p" || true)"
	if [[ -z "${match}" ]]; then
		echo "missing expected occurrence ${occurrence} in ${file}: ${needle}" >&2
		exit 1
	fi
	echo "${match%%:*}"
}

assert_line_before() {
	local earlier="$1"
	local later="$2"
	local file="$3"
	local earlier_line
	local later_line
	earlier_line="$(line_of "$earlier" "$file")"
	later_line="$(line_of "$later" "$file")"
	if (( earlier_line >= later_line )); then
		echo "expected line order in ${file}: ${earlier} before ${later}" >&2
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

assert_contains 'build_tags: webkit2_41' "${release_workflow}"
assert_contains 'libwebkit2gtk-4.1-dev' "${release_workflow}"
assert_not_contains 'libwebkit2gtk-4.0-dev' "${release_workflow}"
assert_contains 'build_args=(--clean -skipbindings --platform "${{ matrix.platform }}")' "${release_workflow}"
assert_contains 'build_args+=(-tags "${BUILD_TAGS}")' "${release_workflow}"
assert_contains 'wails build "${build_args[@]}"' "${release_workflow}"
assert_contains '"product_version": "{{.Info.ProductVersion}}"' "${windows_info}"
assert_contains '"0409": {' "${windows_info}"
assert_contains '"FileVersion": "{{.Info.ProductVersion}}"' "${windows_info}"
assert_contains '$versionInfo.ProductMajorPart' "${release_workflow}"
assert_contains '$versionInfo.ProductMinorPart' "${release_workflow}"
assert_contains '$versionInfo.ProductBuildPart' "${release_workflow}"
assert_contains 'uses: actions/upload-artifact@v7' "${release_workflow}"
assert_contains 'uses: actions/download-artifact@v8' "${release_workflow}"
assert_not_contains 'uses: actions/upload-artifact@v4' "${release_workflow}"
assert_not_contains 'uses: actions/download-artifact@v4' "${release_workflow}"
assert_contains '[System.IO.File]::WriteAllText("$archive.sha256", $checksum, [System.Text.UTF8Encoding]::new($false))' "${release_workflow}"
assert_not_contains '| Set-Content "$archive.sha256"' "${release_workflow}"
assert_contains "grep -q \$'\\r' \"dedao-\${VERSION}-windows-amd64.zip.sha256\"" "${release_workflow}"
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

for workflow in "${quality_workflow}" "${release_workflow}" "${codeql_workflow}"; do
	assert_contains 'uses: actions/checkout@v7' "${workflow}"
	assert_not_contains 'uses: actions/checkout@v4' "${workflow}"
done
for workflow in "${quality_workflow}" "${release_workflow}"; do
	assert_contains 'uses: actions/setup-go@v7' "${workflow}"
	assert_contains 'uses: actions/setup-node@v7' "${workflow}"
	assert_not_contains 'uses: actions/setup-go@v5' "${workflow}"
	assert_not_contains 'uses: actions/setup-node@v4' "${workflow}"
done

assert_line_before '- run: npm --prefix frontend run build' '- run: go vet ./...' "${quality_workflow}"
assert_line_before '- run: npm --prefix frontend run build' '- run: go test ./... -count=1' "${quality_workflow}"
assert_line_before '- run: npm --prefix frontend run build' '- run: go vet ./...' "${release_workflow}"
assert_line_before '- run: npm --prefix frontend run build' '- run: go test ./... -count=1' "${release_workflow}"

build_step_line_2="$(line_of_occurrence '- run: npm --prefix frontend run build' "${release_workflow}" 2)"
host_test_line="$(line_of '      - name: Verify tests on build host' "${release_workflow}")"
if (( build_step_line_2 >= host_test_line )); then
	echo "expected second frontend build in ${release_workflow} before Verify tests on build host" >&2
	exit 1
fi
