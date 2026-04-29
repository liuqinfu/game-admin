#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/docker-compose.microservices.yml"

usage() {
  cat <<'EOF'
Usage:
  ./scripts/observability.sh start
  ./scripts/observability.sh stop
  ./scripts/observability.sh status
  ./scripts/observability.sh smoke
  ./scripts/observability.sh logs

Examples:
  ./scripts/observability.sh start
  ./scripts/observability.sh smoke
  ./scripts/observability.sh logs
EOF
}

compose() {
  docker compose -f "$COMPOSE_FILE" "$@"
}

curl_expect_ok() {
  local name="$1"
  local url="$2"
  local body_file
  body_file="$(mktemp)"
  local code
  code="$(curl -sS -o "$body_file" -w '%{http_code}' "$url" || true)"
  if [[ "$code" != "200" ]]; then
    printf '[observability][error] %s -> HTTP %s %s\n' "$name" "$code" "$url" >&2
    cat "$body_file" >&2 || true
    rm -f "$body_file"
    return 1
  fi
  printf '[observability][ok] %s -> %s\n' "$name" "$url"
  rm -f "$body_file"
}

cmd="${1:-}"

case "$cmd" in
  start)
    mkdir -p "$ROOT_DIR/deploy/observability/logs"
    compose --profile observability up -d prometheus loki promtail grafana jaeger
    ;;
  stop)
    compose stop prometheus loki promtail grafana jaeger
    ;;
  status)
    compose ps prometheus loki promtail grafana jaeger
    ;;
  smoke)
    curl_expect_ok prometheus "http://127.0.0.1:9090/-/healthy"
    curl_expect_ok loki "http://127.0.0.1:3100/ready"
    curl_expect_ok grafana "http://127.0.0.1:3000/api/health"
    curl_expect_ok jaeger-ui "http://127.0.0.1:16686/"
    ;;
  logs)
    cat <<'EOF'
Grafana: http://127.0.0.1:3000/explore
Loki API: http://127.0.0.1:3100/loki/api/v1/query_range?query=%7Bjob%3D%22game-admin-services%22%7D
Local files: deploy/observability/logs/*.log
Suggested LogQL:
  {job="game-admin-services"} | json
  {job="game-admin-services",service="gateway-service"} | json | level="ERROR"
EOF
    ;;
  -h|--help|"")
    usage
    ;;
  *)
    printf '[observability][error] unknown command: %s\n' "$cmd" >&2
    usage >&2
    exit 64
    ;;
esac
