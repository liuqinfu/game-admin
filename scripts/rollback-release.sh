#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUNTIME_ROOT="${RUNTIME_ROOT:-$ROOT_DIR/deploy/runtime/microservices}"

usage() {
  cat <<'EOF'
Usage:
  ./scripts/rollback-release.sh <service> [service...]

Env:
  RUNTIME_ROOT  Runtime release root, default: deploy/runtime/microservices
EOF
}

fail() {
  printf '[rollback][error] %s\n' "$*" >&2
  exit 1
}

rollback_service() {
  local service="$1"
  local current_link="$RUNTIME_ROOT/current/$service"
  local previous_link="$RUNTIME_ROOT/previous/$service"
  local releases_dir="$RUNTIME_ROOT/releases/$service"
  local previous_target current_target current_id previous_id candidate

  [[ -L "$current_link" ]] || fail "current release link missing for $service"
  [[ -L "$previous_link" ]] || fail "previous release link missing for $service"

  current_target="$(readlink "$current_link")"
  previous_target="$(readlink "$previous_link")"
  [[ -n "$previous_target" ]] || fail "previous release target missing for $service"

  current_id="$(basename "$current_target")"
  previous_id="$(basename "$previous_target")"

  ln -sfn "$previous_target" "$current_link"

  candidate=""
  if [[ -d "$releases_dir" ]]; then
    candidate="$(
      find "$releases_dir" -mindepth 1 -maxdepth 1 -type d -print \
        | xargs -n1 basename 2>/dev/null \
        | sort -r \
        | while IFS= read -r item; do
            [[ "$item" == "$previous_id" ]] && continue
            [[ "$item" == "$current_id" ]] && continue
            printf '%s\n' "$item"
            break
          done
    )"
  fi

  if [[ -n "$candidate" ]]; then
    ln -sfn "../releases/$service/$candidate" "$previous_link"
  else
    rm -f "$previous_link"
  fi

  printf '[rollback][ok] %s -> %s\n' "$service" "$previous_id"
}

main() {
  [[ $# -gt 0 ]] || {
    usage >&2
    exit 64
  }

  local service
  for service in "$@"; do
    rollback_service "$service"
  done
}

main "$@"
