#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="$SCRIPT_DIR/backend/.env"

if [[ -f "$ENV_FILE" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
fi

: "${HTTP_PORT:=8080}"
: "${DATABASE_DSN:=../runtime/backend/data/app.db}"

mkdir -p "$(dirname "$DATABASE_DSN")"
exec "$SCRIPT_DIR/backend/server"
