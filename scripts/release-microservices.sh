#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
OUTPUT_DIR="${OUTPUT_DIR:-$ROOT_DIR/deploy/releases/microservices}"
GOOS_TARGET="${GOOS_TARGET:-linux}"
GOARCH_TARGET="${GOARCH_TARGET:-amd64}"
GOCACHE_DIR="${GOCACHE:-$ROOT_DIR/.cache/go-build}"
BUILD_TIME_UTC="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
GIT_COMMIT="$(git -C "$ROOT_DIR" rev-parse --short HEAD 2>/dev/null || echo unknown)"

usage() {
  cat <<EOF
Usage: ./scripts/release-microservices.sh [service...]

Env:
  OUTPUT_DIR     Output root, default: deploy/releases/microservices
  GOOS_TARGET    Build GOOS, default: linux
  GOARCH_TARGET  Build GOARCH, default: amd64
  GOCACHE        Go build cache, default: .cache/go-build
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    printf '[release][error] missing command: %s\n' "$1" >&2
    exit 1
  }
}

service_port() {
  case "$1" in
    gateway-service) echo 8080 ;;
    identity-service) echo 8081 ;;
    tenant-service) echo 8082 ;;
    agent-service) echo 8083 ;;
    relation-service) echo 8084 ;;
    game-service) echo 8085 ;;
    rule-service) echo 8086 ;;
    activity-service) echo 8087 ;;
    recharge-service) echo 8088 ;;
    settlement-service) echo 8089 ;;
    account-service) echo 8090 ;;
    withdrawal-service) echo 8091 ;;
    risk-service) echo 8092 ;;
    report-service) echo 8093 ;;
    audit-service) echo 8094 ;;
    notification-service) echo 8095 ;;
    data-platform-sync-service) echo 8096 ;;
    *)
      return 1
      ;;
  esac
}

default_services() {
  (
    cd "$BACKEND_DIR"
    go run ./cmd/service-manifest all
  )
}

resolve_services() {
  if [[ $# -eq 0 ]]; then
    default_services
    return
  fi
  local service
  for service in "$@"; do
    if ! service_port "$service" >/dev/null && [[ "$service" != "migrate-service" ]]; then
      printf '[release][error] unknown service: %s\n' "$service" >&2
      exit 64
    fi
    printf '%s\n' "$service"
  done
}

collect_services() {
  local items=()
  local line
  while IFS= read -r line; do
    [[ -n "$line" ]] || continue
    items+=("$line")
  done
  printf '%s\n' "${items[@]}"
}

service_modules() {
  case "$1" in
    gateway-service) echo gateway ;;
    identity-service) echo auth,rbac ;;
    tenant-service) echo tenant ;;
    agent-service) echo agent ;;
    relation-service) echo relation ;;
    game-service) echo game ;;
    rule-service) echo rule ;;
    activity-service) echo activity ;;
    recharge-service) echo recharge,openapi ;;
    settlement-service) echo settlement ;;
    account-service) echo account ;;
    withdrawal-service) echo withdrawal ;;
    risk-service) echo risk ;;
    report-service) echo report ;;
    audit-service) echo audit ;;
    notification-service) echo notification-consumer ;;
    data-platform-sync-service) echo data-platform-sync-consumer ;;
    migrate-service) echo migrate ;;
  esac
}

write_env_example() {
  local service="$1"
  local service_dir="$2"
  local port="${3:-8080}"
  cat >"$service_dir/.env.example" <<EOF
APP_ENV=production
SERVICE_NAME=$service
SERVICE_MODULES=$(service_modules "$service")
SERVICE_VERSION=$GIT_COMMIT
HTTP_PORT=$port
METRICS_PORT=$port
DATABASE_DRIVER=mysql
DATABASE_DSN=gameadmin:gameadmin@tcp(mysql:3306)/game_admin?charset=utf8mb4&parseTime=True&loc=Local
REDIS_ENABLED=true
REDIS_ADDR=redis:6379
REDIS_PASSWORD=
REDIS_DB=0
REDIS_PREFIX=game-admin:
MQ_ENABLED=true
MQ_DRIVER=rabbitmq
MQ_URL=amqp://guest:guest@rabbitmq:5672/
MQ_TOPIC=game-admin.events
TELEMETRY_ENABLED=false
TELEMETRY_EXPORTER=none
OTEL_EXPORTER_OTLP_ENDPOINT=
OTEL_EXPORTER_OTLP_INSECURE=false
EOF
}

write_run_script() {
  local service="$1"
  local service_dir="$2"
  cat >"$service_dir/run.sh" <<EOF
#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="\$(cd "\$(dirname "\${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="\$SCRIPT_DIR/.env"
if [[ -f "\$ENV_FILE" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "\$ENV_FILE"
  set +a
fi

exec "\$SCRIPT_DIR/bin/$service"
EOF
  chmod +x "$service_dir/run.sh"
}

write_health_scripts() {
  local service_dir="$1"
  cat >"$service_dir/healthcheck.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

PORT="${1:-${HTTP_PORT:-8080}}"
curl -fsS "http://127.0.0.1:${PORT}/healthz"
EOF
  cat >"$service_dir/readycheck.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

PORT="${1:-${HTTP_PORT:-8080}}"
curl -fsS "http://127.0.0.1:${PORT}/readyz"
EOF
  chmod +x "$service_dir/healthcheck.sh" "$service_dir/readycheck.sh"
}

write_manifest() {
  local service="$1"
  local service_dir="$2"
  local port="$3"
  local binary_name="$4"
  cat >"$service_dir/manifest.json" <<EOF
{
  "service": "$service",
  "modules": "$(service_modules "$service")",
  "targetOS": "$GOOS_TARGET",
  "targetArch": "$GOARCH_TARGET",
  "port": "$port",
  "binary": "bin/$binary_name",
  "builtAtUTC": "$BUILD_TIME_UTC",
  "gitCommit": "$GIT_COMMIT"
}
EOF
}

write_checksums() {
  local service_dir="$1"
  (
    cd "$service_dir"
    shasum -a 256 "bin/"* ".env.example" "run.sh" "healthcheck.sh" "readycheck.sh" "manifest.json" > SHA256SUMS
  )
}

main() {
  require_cmd go
  require_cmd shasum
  mkdir -p "$OUTPUT_DIR" "$GOCACHE_DIR"
  services=()
  while IFS= read -r service; do
    [[ -n "$service" ]] || continue
    services+=("$service")
  done < <(resolve_services "$@")

  local service service_dir bin_dir port binary_name
  for service in "${services[@]}"; do
    [[ -n "$service" ]] || continue
    service_dir="$OUTPUT_DIR/$service"
    bin_dir="$service_dir/bin"
    rm -rf "$service_dir"
    mkdir -p "$bin_dir"

    binary_name="$service"
    port="${HTTP_PORT:-$(service_port "$service" 2>/dev/null || echo 8080)}"

    printf '[release] build %s\n' "$service"
    (
      cd "$BACKEND_DIR"
      GOCACHE="$GOCACHE_DIR" CGO_ENABLED=0 GOOS="$GOOS_TARGET" GOARCH="$GOARCH_TARGET" go build -o "$bin_dir/$binary_name" "./cmd/$service"
    )
    write_env_example "$service" "$service_dir" "$port"
    write_run_script "$service" "$service_dir"
    write_health_scripts "$service_dir"
    write_manifest "$service" "$service_dir" "$port" "$binary_name"
    write_checksums "$service_dir"
  done

  printf '[release] done -> %s\n' "$OUTPUT_DIR"
}

main "$@"
