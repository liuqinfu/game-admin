#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE_ROOT="${SOURCE_ROOT:-$ROOT_DIR/deploy/releases/microservices}"
RUNTIME_ROOT="${RUNTIME_ROOT:-$ROOT_DIR/deploy/runtime/microservices}"

usage() {
  cat <<'EOF'
Usage:
  ./scripts/promote-release.sh <service> [service...]

Env:
  SOURCE_ROOT   Candidate release root, default: deploy/releases/microservices
  RUNTIME_ROOT  Runtime release root, default: deploy/runtime/microservices
EOF
}

fail() {
  printf '[promote][error] %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

require_file() {
  local path="$1"
  [[ -f "$path" ]] || fail "required file not found: ${path#$ROOT_DIR/}"
}

service_exists() {
  [[ -d "$SOURCE_ROOT/$1" ]]
}

require_service() {
  service_exists "$1" || fail "candidate release not found for service: $1"
}

extract_manifest_field() {
  local file="$1"
  local key="$2"
  sed -n "s/.*\"${key}\": \"\\([^\"]*\\)\".*/\\1/p" "$file" | head -n 1
}

sanitize_release_value() {
  printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | tr -cd 'a-z0-9._-'
}

checksum_verify() {
  local service_dir="$1"
  (
    cd "$service_dir"
    shasum -a 256 -c SHA256SUMS >/dev/null
  )
}

ensure_runtime_layout() {
  mkdir -p "$RUNTIME_ROOT/releases" "$RUNTIME_ROOT/current" "$RUNTIME_ROOT/previous"
}

promote_service() {
  local service="$1"
  local source_dir="$SOURCE_ROOT/$service"
  local manifest="$source_dir/manifest.json"
  local built_at git_commit release_stamp release_commit release_id service_root releases_dir target_dir current_link previous_link current_target

  require_service "$service"
  require_file "$manifest"
  require_file "$source_dir/SHA256SUMS"
  require_cmd shasum

  checksum_verify "$source_dir" || fail "checksum verification failed for $service"

  built_at="$(extract_manifest_field "$manifest" builtAtUTC)"
  git_commit="$(extract_manifest_field "$manifest" gitCommit)"
  [[ -n "$built_at" ]] || fail "manifest missing builtAtUTC for $service"
  [[ -n "$git_commit" ]] || fail "manifest missing gitCommit for $service"

  release_stamp="$(sanitize_release_value "$(printf '%s' "$built_at" | tr -d ':-')")"
  release_commit="$(sanitize_release_value "$git_commit")"
  release_id="${release_stamp}-${release_commit}"

  service_root="$RUNTIME_ROOT"
  releases_dir="$service_root/releases/$service"
  target_dir="$releases_dir/$release_id"
  current_link="$service_root/current/$service"
  previous_link="$service_root/previous/$service"

  mkdir -p "$releases_dir" "$(dirname "$current_link")" "$(dirname "$previous_link")"

  if [[ -L "$current_link" ]] && [[ "$(basename "$(readlink "$current_link")")" == "$release_id" ]]; then
    printf '[promote][skip] %s already points to %s\n' "$service" "$release_id"
    return 0
  fi

  rm -rf "$target_dir"
  cp -R "$source_dir" "$target_dir"

  if [[ ! -f "$target_dir/.env" ]]; then
    cp "$target_dir/.env.example" "$target_dir/.env"
  fi

  if [[ -L "$current_link" ]]; then
    current_target="$(readlink "$current_link")"
    if [[ -n "$current_target" ]]; then
      ln -sfn "$current_target" "$previous_link"
    fi
  fi

  ln -sfn "../releases/$service/$release_id" "$current_link"
  printf '[promote][ok] %s -> %s\n' "$service" "$release_id"
}

main() {
  [[ $# -gt 0 ]] || {
    usage >&2
    exit 64
  }

  ensure_runtime_layout

  local service
  for service in "$@"; do
    promote_service "$service"
  done
}

main "$@"
