#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"

if [ ! -f "$ROOT_DIR/backend/web/dist/index.html" ]; then
  echo "未检测到前端构建产物，正在执行 yarn build..."
  (cd "$ROOT_DIR" && yarn build)
fi

cd "$ROOT_DIR/backend"
echo "正在启动后端服务器..."
go run .
