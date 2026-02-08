#!/usr/bin/env bash
set -euo pipefail

APP_NAME="${APP_NAME:-media-transcriber}"
PLATFORM="${WAILS_PLATFORM:-darwin/universal}"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

if ! command -v wails >/dev/null 2>&1; then
  echo "wails CLI not found. Install: go install github.com/wailsapp/wails/v2/cmd/wails@latest"
  exit 1
fi

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "This script must run on macOS."
  exit 1
fi

echo "==> Building $APP_NAME for $PLATFORM"
wails build -clean -platform "$PLATFORM"

APP_BUNDLE="build/bin/${APP_NAME}.app"
DMG_FILE="build/bin/${APP_NAME}.dmg"

if [[ -n "${MAC_SIGN_IDENTITY:-}" && -d "$APP_BUNDLE" ]]; then
  echo "==> Signing app bundle"
  codesign --force --deep --options runtime --timestamp --sign "$MAC_SIGN_IDENTITY" "$APP_BUNDLE"
fi

if [[ -n "${MAC_SIGN_IDENTITY:-}" && -f "$DMG_FILE" ]]; then
  echo "==> Signing DMG"
  codesign --force --timestamp --sign "$MAC_SIGN_IDENTITY" "$DMG_FILE"
fi

if [[ -n "${APPLE_ID:-}" && -n "${APPLE_TEAM_ID:-}" && -n "${APPLE_APP_PASSWORD:-}" && -f "$DMG_FILE" ]]; then
  echo "==> Submitting DMG for notarization"
  xcrun notarytool submit "$DMG_FILE" \
    --apple-id "$APPLE_ID" \
    --team-id "$APPLE_TEAM_ID" \
    --password "$APPLE_APP_PASSWORD" \
    --wait

  echo "==> Stapling notarization ticket"
  xcrun stapler staple "$DMG_FILE"
fi

echo "==> macOS release artifacts"
ls -la build/bin | sed -n '1,120p'
