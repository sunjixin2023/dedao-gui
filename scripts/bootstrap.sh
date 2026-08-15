#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
WAILS_VERSION="$(tr -d '[:space:]' < "${ROOT_DIR}/.wails-version")"

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
export PATH="$(go env GOPATH)/bin:${PATH}"

if ! command -v wails >/dev/null 2>&1; then
  echo "[INFO] Wails CLI not found, installing..."
  go install "github.com/wailsapp/wails/v2/cmd/wails@${WAILS_VERSION}"
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
npm --prefix frontend ci --no-fund --no-audit

echo "[OK] Environment is ready."
