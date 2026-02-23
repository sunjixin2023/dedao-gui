#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
VERSION="${1:-}"

usage() {
  cat <<USAGE
Usage: scripts/deploy.sh <version-tag>

Example:
  scripts/deploy.sh v1.2.0

This command will:
  1) create an annotated git tag
  2) push that tag to origin
  3) trigger GitHub Actions release workflow
USAGE
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "[ERROR] Missing dependency: $1"
    exit 1
  fi
}

if [[ "${VERSION}" == "-h" ]] || [[ "${VERSION}" == "--help" ]]; then
  usage
  exit 0
fi

if [[ -z "${VERSION}" ]]; then
  usage
  exit 1
fi

if [[ ! "${VERSION}" =~ ^v[0-9]+(\.[0-9]+){2}([-.][0-9A-Za-z.-]+)?$ ]]; then
  echo "[ERROR] Invalid version tag: ${VERSION}"
  echo "        Use semantic version format like: v1.0.0"
  exit 1
fi

require_cmd git
cd "${ROOT_DIR}"

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "[ERROR] Current directory is not a git repository"
  exit 1
fi

if ! git remote get-url origin >/dev/null 2>&1; then
  echo "[ERROR] Git remote 'origin' is not configured"
  exit 1
fi

if [[ -n "$(git status --porcelain)" ]]; then
  echo "[ERROR] Working tree is not clean. Commit/stash changes before deploy."
  exit 1
fi

if git rev-parse "${VERSION}" >/dev/null 2>&1; then
  echo "[ERROR] Tag already exists locally: ${VERSION}"
  exit 1
fi

if git ls-remote --exit-code --tags origin "refs/tags/${VERSION}" >/dev/null 2>&1; then
  echo "[ERROR] Tag already exists on origin: ${VERSION}"
  exit 1
fi

echo "[INFO] Creating tag ${VERSION}"
git tag -a "${VERSION}" -m "release ${VERSION}"

echo "[INFO] Pushing tag ${VERSION} to origin"
git push origin "${VERSION}"

echo "[OK] Deploy trigger sent. Watch workflow: .github/workflows/release.yml"
