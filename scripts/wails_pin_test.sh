#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
pin="$(tr -d '[:space:]' < "${root}/.wails-version")"
test "${pin}" = "v2.14.0"
grep -q 'github.com/wailsapp/wails/v2 v2.14.0' "${root}/go.mod"
crypto_version="$(awk '$1 == "golang.org/x/crypto" { print $2; exit }' "${root}/go.mod")"
test -n "${crypto_version}"
# Wails v2.14 pulls x/crypto >= 0.45; reject anything older than the advisory line.
python3 - "${crypto_version}" <<'PY'
import sys
version = sys.argv[1].lstrip("v")
parts = [int(p) for p in version.split(".")[:3]]
while len(parts) < 3:
    parts.append(0)
if tuple(parts) < (0, 45, 0):
    raise SystemExit(f"golang.org/x/crypto {version} is older than 0.45.0")
PY
