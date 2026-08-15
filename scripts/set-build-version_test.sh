#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
test_root="$(mktemp -d /private/tmp/dedao-version-test.XXXXXX)"
trap 'rm -rf "${test_root}"' EXIT

cp "${repo_root}/wails.json" "${test_root}/wails.json"

WAILS_PROJECT_FILE="${test_root}/wails.json" bash "${repo_root}/scripts/set-build-version.sh" 1.2.3
actual="$(node -p "require(process.argv[1]).info.productVersion" "${test_root}/wails.json")"
test "${actual}" = "1.2.3"
test -z "$(find "${test_root}" -maxdepth 1 -type f -name '.wails.json.set-build-version.*' -print -quit)"

before_failure="$(cat "${test_root}/wails.json")"
if SET_BUILD_VERSION_TEST_FAIL_AFTER_WRITE=1 WAILS_PROJECT_FILE="${test_root}/wails.json" \
	bash "${repo_root}/scripts/set-build-version.sh" 2.3.4; then
	echo "simulated write failure was not surfaced" >&2
	exit 1
fi
after_failure="$(cat "${test_root}/wails.json")"
test "${after_failure}" = "${before_failure}"
test -z "$(find "${test_root}" -maxdepth 1 -type f -name '.wails.json.set-build-version.*' -print -quit)"

if WAILS_PROJECT_FILE="${test_root}/wails.json" bash "${repo_root}/scripts/set-build-version.sh" 1.2; then
	echo "invalid version was accepted" >&2
	exit 1
fi
