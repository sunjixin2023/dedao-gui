#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
TARGET="auto"
SKIP_INSTALL=0
PACKAGE_OUTPUT=1
VERSION=""
WAILS_VERSION="$(tr -d '[:space:]' < "${ROOT_DIR}/.wails-version")"

usage() {
  cat <<USAGE
Usage: scripts/release.sh [target-platform] [options]

Targets:
  auto (default)
  darwin/universal
  darwin/arm64
  darwin/amd64
  windows/amd64
  linux/amd64

Options:
  --version X.Y.Z  Set packaged build version metadata
  --skip-install   Skip frontend dependency install step
  --no-package     Do not archive build output
  -h, --help       Show help
USAGE
}

resolve_auto_target() {
  local os
  os="$(uname -s)"
  case "${os}" in
    Darwin)
      echo "darwin/universal"
      ;;
    Linux)
      echo "linux/amd64"
      ;;
    MINGW*|MSYS*|CYGWIN*)
      echo "windows/amd64"
      ;;
    *)
      echo ""
      ;;
  esac
}

normalize_target() {
  case "$1" in
    darwin)
      echo "darwin/universal"
      ;;
    windows)
      echo "windows/amd64"
      ;;
    linux)
      echo "linux/amd64"
      ;;
    *)
      echo "$1"
      ;;
  esac
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "[ERROR] Missing dependency: $1"
    exit 1
  fi
}

write_checksum() {
  local target_file="$1"
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$target_file" > "${target_file}.sha256"
    return
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$target_file" > "${target_file}.sha256"
    return
  fi
  echo "[WARN] sha256 tool not found, skip checksum"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    auto|darwin|darwin/universal|darwin/arm64|darwin/amd64|windows|windows/amd64|linux|linux/amd64)
      TARGET="$1"
      shift
      ;;
    --skip-install)
      SKIP_INSTALL=1
      shift
      ;;
    --version)
      if [[ $# -lt 2 ]]; then
        echo "[ERROR] --version requires a value"
        usage
        exit 1
      fi
      VERSION="$2"
      shift 2
      ;;
    --no-package)
      PACKAGE_OUTPUT=0
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "[ERROR] Unknown option or target: $1"
      usage
      exit 1
      ;;
  esac
done

TARGET="$(normalize_target "${TARGET}")"
if [[ "${TARGET}" == "auto" ]]; then
  TARGET="$(resolve_auto_target)"
  if [[ -z "${TARGET}" ]]; then
    echo "[ERROR] Unsupported host platform for auto mode"
    usage
    exit 1
  fi
fi

case "${TARGET}" in
  darwin/universal|darwin/arm64|darwin/amd64|windows/amd64|linux/amd64)
    ;;
  *)
    echo "[ERROR] Unsupported target platform: ${TARGET}"
    usage
    exit 1
    ;;
esac

cd "${ROOT_DIR}"

echo "[INFO] Checking build dependencies..."
require_cmd go
require_cmd node
require_cmd npm
export PATH="$(go env GOPATH)/bin:${PATH}"

if ! command -v wails >/dev/null 2>&1; then
  echo "[INFO] Wails CLI not found, installing..."
  go install "github.com/wailsapp/wails/v2/cmd/wails@${WAILS_VERSION}"
  export PATH="$(go env GOPATH)/bin:${PATH}"
fi
require_cmd wails

if [[ "${SKIP_INSTALL}" -eq 0 ]]; then
  echo "[INFO] Installing frontend dependencies..."
  npm --prefix frontend ci --no-fund --no-audit
fi

if [[ "${PACKAGE_OUTPUT}" -eq 1 && -z "${VERSION}" ]]; then
  echo "[ERROR] --version X.Y.Z is required when packaging release artifacts"
  usage
  exit 1
fi

if [[ -n "${VERSION}" ]]; then
  bash "${ROOT_DIR}/scripts/set-build-version.sh" "${VERSION}"
fi

echo "[INFO] Building application for ${TARGET}..."
wails_build_args=(build --clean --platform "${TARGET}")
if [[ -n "${VERSION}" ]]; then
  wails_build_args+=(-ldflags "-X github.com/yann0917/dedao-gui/backend.BuildVersion=${VERSION}")
fi
wails "${wails_build_args[@]}"
echo "[OK] Build finished. Output directory: ${ROOT_DIR}/build/bin"

if [[ "${PACKAGE_OUTPUT}" -eq 0 ]]; then
  exit 0
fi

if [[ ! -d "${ROOT_DIR}/build/bin" ]]; then
  echo "[ERROR] Build output directory missing: ${ROOT_DIR}/build/bin"
  exit 1
fi

mkdir -p "${ROOT_DIR}/release"
BASENAME="dedao-${VERSION}-${TARGET//\//-}"

if [[ "${TARGET}" == windows/* ]]; then
  ARCHIVE_FILE="${ROOT_DIR}/release/${BASENAME}.zip"
  if command -v zip >/dev/null 2>&1; then
    (
      cd "${ROOT_DIR}/build/bin"
      zip -r "${ARCHIVE_FILE}" . >/dev/null
    )
  elif command -v powershell >/dev/null 2>&1; then
    powershell -NoProfile -Command "Compress-Archive -Path '${ROOT_DIR}/build/bin/*' -DestinationPath '${ARCHIVE_FILE}' -Force"
  else
    echo "[ERROR] Neither zip nor powershell found for Windows archive packaging"
    exit 1
  fi
else
  ARCHIVE_FILE="${ROOT_DIR}/release/${BASENAME}.tar.gz"
  tar -czf "${ARCHIVE_FILE}" -C "${ROOT_DIR}/build/bin" .
fi

write_checksum "${ARCHIVE_FILE}"
echo "[OK] Packaged artifact: ${ARCHIVE_FILE}"
