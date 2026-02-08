#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DATE="$(date +%F)"
OS_NAME="${1:-$(uname -s | tr '[:upper:]' '[:lower:]')}"
OUT_FILE="$ROOT_DIR/docs/smoke-results/${DATE}-${OS_NAME}-smoke.md"

if [[ -f "$OUT_FILE" ]]; then
  echo "Smoke report already exists: $OUT_FILE"
  exit 1
fi

cp "$ROOT_DIR/docs/smoke-results/TEMPLATE.md" "$OUT_FILE"
echo "Created: $OUT_FILE"
