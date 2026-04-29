#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
COMPOSE_FILE="$ROOT_DIR/docker-compose.microservices.yml"

MYSQL_HOST="${MYSQL_HOST:-127.0.0.1}"
MYSQL_PORT="${MYSQL_PORT:-3306}"
REDIS_HOST="${REDIS_HOST:-127.0.0.1}"
REDIS_PORT="${REDIS_PORT:-6379}"
RABBITMQ_HOST="${RABBITMQ_HOST:-127.0.0.1}"
RABBITMQ_PORT="${RABBITMQ_PORT:-5672}"

usage() {
  cat <<'EOF'
Usage:
  ./scripts/microservice-ops.sh deps
  ./scripts/microservice-ops.sh start [service...]
  ./scripts/microservice-ops.sh stop [service...]
  ./scripts/microservice-ops.sh migrate [service...]
  ./scripts/microservice-ops.sh health [service...]
  ./scripts/microservice-ops.sh ready [service...]
  ./scripts/microservice-ops.sh metrics [service...]

Examples:
  ./scripts/microservice-ops.sh start identity-service tenant-service gateway-service
  ./scripts/microservice-ops.sh migrate identity-service tenant-service game-service audit-service
  ./scripts/microservice-ops.sh ready gateway-service notification-service
EOF
}

compose() {
  docker compose -f "$COMPOSE_FILE" "$@"
}

service_exists() {
  service_port "$1" >/dev/null
}

is_worker() {
  case "$1" in
    notification-service|data-platform-sync-service)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
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
  printf '%s\n' \
    identity-service \
    tenant-service \
    agent-service \
    relation-service \
    game-service \
    rule-service \
    activity-service \
    recharge-service \
    settlement-service \
    account-service \
    withdrawal-service \
    risk-service \
    report-service \
    audit-service \
    notification-service \
    data-platform-sync-service \
    gateway-service
}

resolve_services() {
  if [[ $# -eq 0 ]]; then
    default_services
    return
  fi
  local service
  for service in "$@"; do
    if ! service_exists "$service"; then
      printf '[ops][error] unknown service: %s\n' "$service" >&2
      exit 64
    fi
    printf '%s\n' "$service"
  done
}

dependency_required() {
  return 0
}

check_port() {
  local name="$1"
  local host="$2"
  local port="$3"
  if command -v nc >/dev/null 2>&1; then
    if nc -z "$host" "$port" >/dev/null 2>&1; then
      printf '[deps][ok] %s %s:%s\n' "$name" "$host" "$port"
      return 0
    fi
  elif (echo >/dev/tcp/"$host"/"$port") >/dev/null 2>&1; then
    printf '[deps][ok] %s %s:%s\n' "$name" "$host" "$port"
    return 0
  fi
  printf '[deps][error] %s unavailable at %s:%s\n' "$name" "$host" "$port" >&2
  return 1
}

check_dependencies() {
  check_port mysql "$MYSQL_HOST" "$MYSQL_PORT"
  check_port redis "$REDIS_HOST" "$REDIS_PORT"
  check_port rabbitmq "$RABBITMQ_HOST" "$RABBITMQ_PORT"
}

curl_check() {
  local service="$1"
  local endpoint="$2"
  local expected="${3:-200}"
  local port
  port="$(service_port "$service")"
  local url="http://127.0.0.1:${port}/${endpoint}"
  local body_file
  body_file="$(mktemp)"
  local code
  code="$(curl -sS -o "$body_file" -w '%{http_code}' "$url" || true)"
  if [[ ",$expected," != *",$code,"* ]]; then
    printf '[ops][error] %s %s -> HTTP %s\n' "$service" "$endpoint" "$code" >&2
    cat "$body_file" >&2 || true
    rm -f "$body_file"
    return 1
  fi
  printf '[ops][ok] %s %s -> HTTP %s\n' "$service" "$endpoint" "$code"
  rm -f "$body_file"
}

start_services() {
  services=()
  while IFS= read -r service; do
    [[ -n "$service" ]] || continue
    services+=("$service")
  done < <(resolve_services "$@")
  check_dependencies
  compose up -d mysql redis rabbitmq
  local service
  for service in "${services[@]}"; do
    if ! is_worker "$service" && [[ "$service" != "gateway-service" ]]; then
      compose --profile migrate up "migrate-${service}"
    fi
  done
  compose up -d "${services[@]}"
}

stop_services() {
  if [[ $# -eq 0 ]]; then
    compose down --remove-orphans
    return
  fi
  services=()
  while IFS= read -r service; do
    [[ -n "$service" ]] || continue
    services+=("$service")
  done < <(resolve_services "$@")
  compose stop "${services[@]}"
}

migrate_services() {
  services=()
  while IFS= read -r service; do
    [[ -n "$service" ]] || continue
    services+=("$service")
  done < <(resolve_services "$@")
  check_dependencies
  local service
  for service in "${services[@]}"; do
    if is_worker "$service" || [[ "$service" == "gateway-service" ]]; then
      printf '[ops][skip] %s does not own DB migrations\n' "$service"
      continue
    fi
    printf '[ops][migrate] %s\n' "$service"
    (
      cd "$BACKEND_DIR"
      GOCACHE="${ROOT_DIR}/.cache/go-build" go run ./cmd/migrate-service "$service"
    )
  done
}

check_endpoint() {
  local endpoint="$1"
  shift
  services=()
  while IFS= read -r service; do
    [[ -n "$service" ]] || continue
    services+=("$service")
  done < <(resolve_services "$@")
  local service
  for service in "${services[@]}"; do
    case "$endpoint" in
      ready)
        curl_check "$service" "readyz" "200,503"
        ;;
      health)
        curl_check "$service" "healthz" "200"
        ;;
      metrics)
        curl_check "$service" "metrics" "200"
        ;;
    esac
  done
}

cmd="${1:-}"
shift || true

case "$cmd" in
  deps)
    check_dependencies
    ;;
  start)
    start_services "$@"
    ;;
  stop)
    stop_services "$@"
    ;;
  migrate)
    migrate_services "$@"
    ;;
  health)
    check_endpoint health "$@"
    ;;
  ready)
    check_endpoint ready "$@"
    ;;
  metrics)
    check_endpoint metrics "$@"
    ;;
  *)
    usage
    exit 64
    ;;
esac
