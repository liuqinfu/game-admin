#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${ROOT_DIR}/.env"
BACKEND_ENV_FILE="${ROOT_DIR}/backend/.env"
FRONTEND_ENV_FILE="${ROOT_DIR}/frontend/.env.production"
BACKEND_BIN="${ROOT_DIR}/backend/bin/server"
FRONTEND_DIST="${ROOT_DIR}/frontend/dist"
DEPLOY_DIR="${ROOT_DIR}/deploy"
RELEASE_DIR="${DEPLOY_DIR}/release"
RUNTIME_DIR="${DEPLOY_DIR}/runtime"
BACKEND_DATA_DIR="${RUNTIME_DIR}/backend/data"
LOG_DIR="${RUNTIME_DIR}/logs"
KEEP_FRONTEND_ENV_FILE=0
SKIP_FRONTEND_BUILD=0
SKIP_BACKEND_BUILD=0

log() {
  printf '[deploy] %s\n' "$*"
}

fail() {
  printf '[deploy][error] %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<EOF
Usage: $(basename "$0") [options]

Build backend and frontend release artifacts into deploy/release.

Options:
  --api-base-url URL       Override PUBLIC_API_BASE_URL for frontend build
  --skip-backend           Reuse existing backend binary without rebuilding
  --skip-frontend          Reuse existing frontend dist without rebuilding
  --keep-frontend-env      Keep existing frontend/.env.production after packaging
  -h, --help               Show this help message
EOF
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

require_file() {
  local path="$1"
  [[ -f "$path" ]] || fail "required file not found: ${path#${ROOT_DIR}/}"
}

require_dir() {
  local path="$1"
  [[ -d "$path" ]] || fail "required directory not found: ${path#${ROOT_DIR}/}"
}

cleanup() {
  if [[ "$KEEP_FRONTEND_ENV_FILE" -eq 0 ]]; then
    rm -f "$FRONTEND_ENV_FILE"
  fi
}

trap cleanup EXIT

load_env_file() {
  local file="$1"
  if [[ -f "$file" ]]; then
    log "loading environment from ${file#${ROOT_DIR}/}"
    set -a
    # shellcheck disable=SC1090
    source "$file"
    set +a
  fi
}

assert_env() {
  local name="$1"
  [[ -n "${!name:-}" ]] || fail "required environment variable is not set: $name"
}

prepare_env_files() {
  local api_base_url="$1"

  mkdir -p "$(dirname "$BACKEND_ENV_FILE")" "$(dirname "$FRONTEND_ENV_FILE")"

  cat >"$BACKEND_ENV_FILE" <<EOF
APP_ENV=${APP_ENV}
HTTP_PORT=${HTTP_PORT}
DATABASE_DRIVER=${DATABASE_DRIVER}
DATABASE_DSN=${DATABASE_DSN}
REDIS_ENABLED=${REDIS_ENABLED}
REDIS_ADDR=${REDIS_ADDR}
REDIS_PASSWORD=***
REDIS_DB=${REDIS_DB}
REDIS_PREFIX=${REDIS_PREFIX}
MQ_ENABLED=${MQ_ENABLED}
MQ_DRIVER=${MQ_DRIVER}
MQ_URL=${MQ_URL}
MQ_TOPIC=${MQ_TOPIC}
TELEMETRY_ENABLED=${TELEMETRY_ENABLED}
TELEMETRY_EXPORTER=${TELEMETRY_EXPORTER}
OTEL_EXPORTER_OTLP_ENDPOINT=${OTEL_EXPORTER_OTLP_ENDPOINT}
OTEL_EXPORTER_OTLP_INSECURE=${OTEL_EXPORTER_OTLP_INSECURE}
SERVICE_VERSION=${SERVICE_VERSION}
EOF

  cat >"$FRONTEND_ENV_FILE" <<EOF
VITE_API_BASE_URL=${api_base_url}
EOF
}

build_backend() {
  if [[ "$SKIP_BACKEND_BUILD" -eq 1 ]]; then
    log 'skipping backend build, reusing existing binary'
    require_file "$BACKEND_BIN"
    return
  fi

  log 'building backend binary'
  mkdir -p "$(dirname "$BACKEND_BIN")"
  (
    cd "$ROOT_DIR/backend"
    go build -o "$BACKEND_BIN" ./cmd/server
  )
}

build_frontend() {
  if [[ "$SKIP_FRONTEND_BUILD" -eq 1 ]]; then
    log 'skipping frontend build, reusing existing dist'
    require_dir "$FRONTEND_DIST"
    return
  fi

  log 'building frontend assets'
  (
    cd "$ROOT_DIR/frontend"
    npm run build
  )
}

assemble_release() {
  log 'assembling release bundle'
  require_file "$BACKEND_BIN"
  require_file "$BACKEND_ENV_FILE"
  require_dir "$FRONTEND_DIST"
  require_file "$ROOT_DIR/.env.example"
  require_file "$ROOT_DIR/README.md"

  rm -rf "$RELEASE_DIR"
  mkdir -p "$RELEASE_DIR/backend" "$RELEASE_DIR/frontend" "$BACKEND_DATA_DIR" "$LOG_DIR"

  cp "$BACKEND_BIN" "$RELEASE_DIR/backend/server"
  cp "$BACKEND_ENV_FILE" "$RELEASE_DIR/backend/.env"
  cp -R "$FRONTEND_DIST" "$RELEASE_DIR/frontend/dist"
  cp "$FRONTEND_ENV_FILE" "$RELEASE_DIR/frontend/.env.production"
  cp "$ROOT_DIR/.env.example" "$RELEASE_DIR/.env.example"
  cp "$ROOT_DIR/README.md" "$RELEASE_DIR/README.md"
}

write_run_scripts() {
  log 'generating runtime helper scripts'

  cat >"$RELEASE_DIR/run-backend.sh" <<'EOF'
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
EOF

  cat >"$RELEASE_DIR/run-frontend.sh" <<'EOF'
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
EOF

  chmod +x "$RELEASE_DIR/run-backend.sh" "$RELEASE_DIR/run-frontend.sh"
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --api-base-url)
        shift
        [[ $# -gt 0 ]] || fail '--api-base-url requires a value'
        PUBLIC_API_BASE_URL="$1"
        ;;
      --skip-backend)
        SKIP_BACKEND_BUILD=1
        ;;
      --skip-frontend)
        SKIP_FRONTEND_BUILD=1
        ;;
      --keep-frontend-env)
        KEEP_FRONTEND_ENV_FILE=1
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        fail "unknown option: $1"
        ;;
    esac
    shift
  done
}

main() {
  load_env_file "$ENV_FILE"
  parse_args "$@"

  if [[ "$SKIP_BACKEND_BUILD" -eq 0 ]]; then
    require_cmd go
  fi

  if [[ "$SKIP_FRONTEND_BUILD" -eq 0 ]]; then
    require_cmd npm
  fi

  : "${APP_ENV:=production}"
  : "${HTTP_PORT:=8080}"
  : "${DATABASE_DRIVER:=sqlite}"
  : "${DATABASE_DSN:=../runtime/backend/data/app.db}"
  : "${REDIS_ENABLED:=false}"
  : "${REDIS_ADDR:=localhost:6379}"
  : "${REDIS_PASSWORD:=}"
  : "${REDIS_DB:=0}"
  : "${REDIS_PREFIX:=game-admin:}"
  : "${MQ_ENABLED:=false}"
  : "${MQ_DRIVER:=rabbitmq}"
  : "${MQ_URL:=amqp://guest:***@localhost:5672/}"
  : "${MQ_TOPIC:=game-admin.events}"
  : "${TELEMETRY_ENABLED:=false}"
  : "${TELEMETRY_EXPORTER:=none}"
  : "${OTEL_EXPORTER_OTLP_ENDPOINT:=}"
  : "${OTEL_EXPORTER_OTLP_INSECURE:=false}"
  : "${SERVICE_VERSION:=dev}"
  : "${PUBLIC_API_BASE_URL:=http://localhost:${HTTP_PORT}/api}"

  assert_env APP_ENV
  assert_env HTTP_PORT
  assert_env DATABASE_DRIVER
  assert_env DATABASE_DSN
  assert_env PUBLIC_API_BASE_URL

  prepare_env_files "$PUBLIC_API_BASE_URL"
  build_backend
  build_frontend
  assemble_release
  write_run_scripts

  log "release bundle ready at ${RELEASE_DIR}"
  log "backend launcher: ${RELEASE_DIR}/run-backend.sh"
  log "frontend launcher: ${RELEASE_DIR}/run-frontend.sh"
}

main "$@"
