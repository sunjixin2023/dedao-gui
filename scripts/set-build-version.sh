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
