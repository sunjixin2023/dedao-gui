#!/usr/bin/env bash
set -euo pipefail

version="${1:-}"
if [[ ! "${version}" =~ ^[0-9]+[.][0-9]+[.][0-9]+$ ]]; then
	echo "version must be X.Y.Z" >&2
	exit 1
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
project_file="${WAILS_PROJECT_FILE:-${repo_root}/wails.json}"
project_dir="$(dirname "${project_file}")"
project_name="$(basename "${project_file}")"
temp_file="$(mktemp "${project_dir}/.${project_name}.set-build-version.XXXXXX")"

cleanup() {
	rm -f -- "${temp_file}"
}

trap cleanup EXIT INT TERM HUP

node - "${project_file}" "${temp_file}" "${version}" <<'NODE'
const fs = require('node:fs')
const [path, tempPath, version] = process.argv.slice(2)
const project = JSON.parse(fs.readFileSync(path, 'utf8'))
project.info = { ...(project.info || {}), productVersion: version }
fs.writeFileSync(tempPath, JSON.stringify(project, null, 2) + '\n')
fs.chmodSync(tempPath, fs.statSync(path).mode)
if (process.env.SET_BUILD_VERSION_TEST_FAIL_AFTER_WRITE === '1') {
  throw new Error('simulated failure after temp write')
}
NODE

mv -f "${temp_file}" "${project_file}"
temp_file=""
