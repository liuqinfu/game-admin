#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
IMAGE_PREFIX="${IMAGE_PREFIX:-game-admin}"
TAG="${TAG:-dev}"

usage() {
  cat <<EOF
Usage: ./scripts/build-microservice-images.sh [service...]

Env:
  IMAGE_PREFIX   Image name prefix, default: game-admin
  TAG            Image tag, default: dev
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

for service in "${SERVICES[@]}"; do
  [[ -n "$service" ]] || continue
  if [[ "$service" == "migrate-service" ]]; then
    continue
  fi
  echo "[images] build ${IMAGE_PREFIX}/${service}:${TAG}"
  docker build \
    -f "$BACKEND_DIR/Dockerfile.service" \
    --build-arg SERVICE_NAME="$service" \
    -t "${IMAGE_PREFIX}/${service}:${TAG}" \
    "$ROOT_DIR"
done

echo "[images] done"
