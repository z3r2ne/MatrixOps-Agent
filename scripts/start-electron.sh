#!/bin/bash

# MatrixOps Electron 启动脚本

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "🚀 正在启动 MatrixOps 桌面应用..."
echo ""

cd "$ROOT_DIR"

if [ ! -f "build/matrixops" ]; then
    echo "📦 编译后端..."
    mkdir -p build
    go build -o build/matrixops ./cmd/main.go
    echo ""
fi

cd frontend
npm run electron:dev
