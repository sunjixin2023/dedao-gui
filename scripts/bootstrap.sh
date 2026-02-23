#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "[ERROR] Missing dependency: $1"
    exit 1
  fi
}

cd "${ROOT_DIR}"

echo "[INFO] Checking required commands..."
require_cmd go
require_cmd node
require_cmd npm

if ! command -v wails >/dev/null 2>&1; then
  echo "[INFO] Wails CLI not found, installing..."
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  export PATH="$(go env GOPATH)/bin:${PATH}"
fi

if ! command -v wails >/dev/null 2>&1; then
  echo "[ERROR] Wails CLI is still unavailable after installation."
  echo "        Add \"$(go env GOPATH)/bin\" to PATH and retry."
  exit 1
fi

echo "[INFO] Downloading Go dependencies..."
go mod download

echo "[INFO] Installing frontend dependencies..."
if [[ -f "${ROOT_DIR}/frontend/package-lock.json" ]]; then
  npm --prefix frontend ci --no-fund --no-audit
else
  npm --prefix frontend install --no-fund --no-audit
fi

echo "[OK] Environment is ready."
