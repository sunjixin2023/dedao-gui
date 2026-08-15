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
