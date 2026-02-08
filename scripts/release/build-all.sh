#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

case "$(uname -s)" in
  Darwin)
    ./scripts/release/build-macos.sh
    ;;
  MINGW*|MSYS*|CYGWIN*)
    ./scripts/release/build-windows.sh
    ;;
  *)
    echo "Use OS-specific scripts:"
    echo "  macOS:    ./scripts/release/build-macos.sh"
    echo "  Windows:  ./scripts/release/build-windows.sh"
    exit 1
    ;;
esac
