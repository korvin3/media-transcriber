#!/usr/bin/env bash
set -euo pipefail

APP_NAME="${APP_NAME:-media-transcriber}"
PLATFORM="${WAILS_PLATFORM:-windows/amd64}"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"
export GOCACHE="${GOCACHE:-/tmp/go-build}"

if ! command -v wails >/dev/null 2>&1; then
  echo "wails CLI not found. Install: go install github.com/wailsapp/wails/v2/cmd/wails@latest"
  exit 1
fi

echo "==> Building $APP_NAME for $PLATFORM"
if wails build -clean -platform "$PLATFORM"; then
  echo "==> Wails build completed"
else
  echo "==> Wails build failed. Trying Go fallback build (no Wails packaging/signing)."
  mkdir -p build/bin
  GOOS=windows GOARCH=amd64 go build -o "build/bin/${APP_NAME}.exe" .
fi

echo "==> Windows release artifacts"
ls -la build/bin | sed -n '1,120p'
