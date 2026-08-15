#!/usr/bin/env bash

set -euo pipefail

echo -e "Start running the script..."
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
WAILS_VERSION="$(tr -d '[:space:]' < "${ROOT_DIR}/.wails-version")"

cd "${ROOT_DIR}"

echo -e "Current Go version: \c"
go version

echo -e "Install the Wails command line tool..."
go install "github.com/wailsapp/wails/v2/cmd/wails@${WAILS_VERSION}"

echo -e "Successful installation!"

echo -e "End running the script!"
