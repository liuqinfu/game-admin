#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
OUTPUT_PATH="${1:-$ROOT_DIR/docker-compose.microservices.yml}"

cd "$BACKEND_DIR"
go run ./cmd/render-microservice-compose "$OUTPUT_PATH"
echo "[compose] wrote $OUTPUT_PATH"
