#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PORT="${1:-4173}"

if command -v python3 >/dev/null 2>&1; then
  exec python3 -m http.server "$PORT" --directory "$SCRIPT_DIR/frontend/dist"
fi

if command -v npx >/dev/null 2>&1; then
  exec npx serve -s "$SCRIPT_DIR/frontend/dist" -l "$PORT"
fi

printf '[deploy][error] python3 or npx is required to preview frontend assets\n' >&2
exit 1
