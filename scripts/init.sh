#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
FRONTEND_DIR="$ROOT_DIR/frontend"
ENV_FILE="$ROOT_DIR/.env"
ENV_EXAMPLE="$ROOT_DIR/.env.example"
BACKEND_DATA_DIR="$BACKEND_DIR/data"
FRONTEND_NODE_MODULES_DIR="$FRONTEND_DIR/node_modules"
FRONTEND_LOCK_FILE="$FRONTEND_DIR/package-lock.json"
GO_MOD_FILE="$BACKEND_DIR/go.mod"
GO_SUM_FILE="$BACKEND_DIR/go.sum"

RUN_TESTS=1
RUN_BUILD=1
FORCE_NPM_INSTALL=0

log() {
  printf '[init] %s\n' "$*"
}

warn() {
  printf '[init] WARN: %s\n' "$*" >&2
}

fail() {
  printf '[init] ERROR: %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<EOF
用法: ./scripts/init.sh [options]

选项:
  --skip-tests         跳过后端测试
  --skip-build         跳过前端构建
  --force-npm-install  无论是否已安装依赖，强制执行 npm install
  -h, --help           显示帮助
EOF
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "缺少命令: $1"
}

parse_args() {
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --skip-tests)
        RUN_TESTS=0
        ;;
      --skip-build)
        RUN_BUILD=0
        ;;
      --force-npm-install)
        FORCE_NPM_INSTALL=1
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        fail "未知参数: $1（使用 --help 查看帮助）"
        ;;
    esac
    shift
  done
}

copy_env_if_needed() {
  if [ ! -f "$ENV_FILE" ]; then
    log "创建 .env（从 .env.example 复制）"
    cp "$ENV_EXAMPLE" "$ENV_FILE"
  else
    log ".env 已存在，跳过复制"
  fi
}

ensure_backend_data_dir() {
  mkdir -p "$BACKEND_DATA_DIR"
  log "已确保 backend/data 目录存在"
}

frontend_dependencies_need_install() {
  [ "$FORCE_NPM_INSTALL" -eq 1 ] && return 0
  [ ! -d "$FRONTEND_NODE_MODULES_DIR" ] && return 0
  [ ! -f "$FRONTEND_LOCK_FILE" ] && return 0
  [ "$FRONTEND_LOCK_FILE" -nt "$FRONTEND_NODE_MODULES_DIR" ] && return 0
  return 1
}

install_frontend_deps() {
  if frontend_dependencies_need_install; then
    if [ "$FORCE_NPM_INSTALL" -eq 1 ]; then
      log "按要求强制执行 npm install"
    elif [ ! -d "$FRONTEND_NODE_MODULES_DIR" ]; then
      log "未发现 frontend/node_modules，安装前端依赖"
    elif [ ! -f "$FRONTEND_LOCK_FILE" ]; then
      warn "未发现 frontend/package-lock.json，回退执行 npm install"
    else
      log "检测到前端依赖可能过期，执行 npm install"
    fi
    npm install --prefix "$FRONTEND_DIR"
  else
    log "frontend/node_modules 与 package-lock.json 已就绪，跳过 npm install"
  fi
}

run_backend_tests() {
  if [ "$RUN_TESTS" -ne 1 ]; then
    log "按要求跳过后端测试"
    return 0
  fi

  log "执行后端测试"
  (
    cd "$BACKEND_DIR"
    go test ./...
  )
}

build_frontend() {
  if [ "$RUN_BUILD" -ne 1 ]; then
    log "按要求跳过前端构建"
    return 0
  fi

  log "执行前端构建"
  (
    cd "$FRONTEND_DIR"
    npm run build
  )
}

print_next_steps() {
  cat <<EOF

初始化完成。可用启动方式：
1. 后端
   cd backend && go run ./cmd/server
2. 前端
   cd frontend && npm run dev
3. 一键联调
   docker compose up --build
4. 微服务分模块迁移
   ./scripts/migrate-microservices.sh
5. 微服务二进制打包
   ./scripts/package-microservices.sh
6. 微服务镜像构建
   ./scripts/build-microservice-images.sh
EOF
}

main() {
  parse_args "$@"

  [ -d "$BACKEND_DIR" ] || fail "未找到 backend 目录: $BACKEND_DIR"
  [ -d "$FRONTEND_DIR" ] || fail "未找到 frontend 目录: $FRONTEND_DIR"
  [ -f "$ENV_EXAMPLE" ] || fail "未找到环境变量模板: $ENV_EXAMPLE"
  [ -f "$GO_MOD_FILE" ] || fail "未找到后端 go.mod: $GO_MOD_FILE"
  [ -f "$GO_SUM_FILE" ] || fail "未找到后端 go.sum: $GO_SUM_FILE"
  [ -f "$FRONTEND_DIR/package.json" ] || fail "未找到前端 package.json: $FRONTEND_DIR/package.json"

  require_cmd go
  require_cmd npm

  copy_env_if_needed
  ensure_backend_data_dir
  install_frontend_deps
  run_backend_tests
  build_frontend

  log "初始化脚本执行成功"
  print_next_steps
}

main "$@"
