#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
OUTPUT_DIR="${OUTPUT_DIR:-$ROOT_DIR/deploy/microservices}"

usage() {
  cat <<EOF
Usage: ./scripts/package-microservices.sh [service...]

Env:
  OUTPUT_DIR   Output root, default: deploy/microservices
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

SERVICES=()
while IFS= read -r service; do
  [[ -n "$service" ]] || continue
  SERVICES+=("$service")
done < <(
  if [[ $# -gt 0 ]]; then
    printf '%s\n' "$@"
  else
    (
      cd "$BACKEND_DIR"
      go run ./cmd/service-manifest all
    )
  fi
)

rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

for service in "${SERVICES[@]}"; do
  [[ -n "$service" ]] || continue
  service_dir="$OUTPUT_DIR/$service"
  mkdir -p "$service_dir"

  echo "[package] build $service"
  (
    cd "$BACKEND_DIR"
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "$service_dir/$service" "./cmd/$service"
  )
done

echo "[package] done -> $OUTPUT_DIR"
