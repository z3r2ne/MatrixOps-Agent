@echo off
REM MatrixOps Electron 启动脚本 (Windows)

setlocal

cd /d "%~dp0\.."

echo 🚀 正在启动 MatrixOps 桌面应用...
echo.

if not exist "build\matrixops.exe" (
    echo 📦 编译后端...
    if not exist "build" mkdir build
    go build -o build\matrixops.exe ./cmd/main.go
    echo.
)

cd frontend
call npm run electron:dev
