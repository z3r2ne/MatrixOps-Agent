#!/bin/bash
set -e

echo "🚀 开始构建 macOS 应用..."
echo ""

# 1. 编译后端
echo "📦 编译 Go 后端..."
go build -o matrixops ./cmd/main.go
echo "✅ 后端编译完成"
echo ""

# 2. 构建前端
echo "📦 构建前端..."
cd frontend
npm run build
echo "✅ 前端构建完成"
echo ""

# 3. 打包应用
echo "📦 打包 Electron 应用..."
npm run electron:build:mac
echo "✅ 应用打包完成"
echo ""

# 4. 显示结果
echo "🎉 构建成功！"
echo ""
echo "构建产物:"
ls -lh dist-electron/*.dmg dist-electron/*.zip 2>/dev/null || true
echo ""
echo "可以运行:"
echo "  open dist-electron/MatrixOps-1.0.0-arm64.dmg"
echo ""
