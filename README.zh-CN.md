# MatrixOps Agent

[English](./README.md) | **简体中文**

融合多个 Agent、具备**长期记忆**的 AI 辅助工具。在使用方式上，将 **OpenClaw 式持久协助** 与 **Coding Agent 式项目编码** 结合在同一套工作流里。

你可以在同一个系统中长期积累上下文、委派给不同 Worker，也可以在需要时直接进入某个代码仓库编写或修改项目。

## 产品定位

多数工具只能二选一：要么偏「有记忆的助手」，要么偏「绑定仓库的编程 Agent」。MatrixOps Agent 希望把这两种用法放在同一条链路里。

| 模式 | 体验 | 典型场景 |
|------|------|----------|
| **持久型助手** | 长会话、记忆库、提醒、多步工作流 | 调研、规划、重复性任务、需要跨重启保留的知识 |
| **项目型编码 Agent** | 绑定工作区，使用读文件/编辑/终端/Git 等工具 | 在真实代码库里实现功能、重构、排错 |

底层由多个 **Worker**（explore、plan、verification、clawbot、frontend engineer 等）组合编排，而不是单一巨型 Agent 包办一切。

## 核心能力

- **多 Agent 编排** — 将任务路由到专用 Worker，必要时 Worker 之间可互相调用。
- **长期记忆** — 会话记忆、记忆库、压缩整理，以及对已存知识的语义检索。
- **OpenClaw 风格交互** — 可持续的任务、提醒与助手式流程，上下文随时间累积。
- **Coding Agent 执行** — 文件工具、终端、Diff、Git worktree，以及项目级权限控制。
- **桌面 + 本地优先** — Electron 内嵌后端，数据默认保存在本机（`~/.matrixops`）。
- **可扩展工具链** — MCP 服务、Skills、自定义 Worker、LLM 提供商配置。

## 架构概览

```
┌─────────────────────────────────────────────────────────────┐
│  桌面 / Web UI（React + Electron）                           │
└───────────────────────────┬─────────────────────────────────┘
                            │ HTTP / WebSocket
┌───────────────────────────▼─────────────────────────────────┐
│  API 服务（Go）— 工作区、任务、会话、工具                      │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────┐
│  Agent 运行时 — Worker、记忆、流式输出、工具执行               │
└─────────────────────────────────────────────────────────────┘
```

## 快速开始

### 桌面应用（推荐）

```bash
# 编译 CLI
go build -o build/matrixops ./cmd

# 启动桌面应用（按需构建前端并启动 Electron）
./build/matrixops app
```

或使用 [Task](https://taskfile.dev)：

```bash
task app
```

### Web 开发模式

终端 1 — 后端：

```bash
task backend
```

终端 2 — 前端：

```bash
task start
```

浏览器访问 http://localhost:3010

### 仅启动服务端

```bash
./build/matrixops server --host localhost --port 8080
```

先构建前端（`cd frontend && npm run build`）后，服务端也会托管 UI 静态资源。

## 环境要求

- **Go** 1.24+
- **Node.js** 18+
- **npm** 9+

## 安装

```bash
git clone https://github.com/z3r2ne/MatrixOps-Agent.git
cd MatrixOps-Agent

# 安装依赖
go mod download
cd frontend && npm install --legacy-peer-deps
cd ../web-server && go mod download

# 编译 CLI
cd ..
go build -o build/matrixops ./cmd
```

## CLI 命令

```bash
./build/matrixops --help
./build/matrixops app                      # 桌面应用
./build/matrixops server                   # API 服务（默认 :8080）
./build/matrixops chat "帮我总结这个仓库"
./build/matrixops version
```

更多服务端参数、对话选项与 pprof 说明见 [cmd/README.md](./cmd/README.md)。

## 常用 Task

```bash
task --list

task app                  # 桌面应用
task start                # 前端开发服务
task backend              # 后端开发服务
task electron:dev         # Electron 壳（开发模式需单独启动前后端）
task electron:build:mac   # 打包 macOS 应用
task clean                # 清理构建产物
```

## 项目结构

```
MatrixOps-Agent/
├── agent/           # Agent 运行时、Worker、工具、会话/记忆逻辑
├── cmd/             # CLI 入口（app、server、chat）
├── frontend/        # React UI + Electron 壳
├── web-server/      # HTTP API、处理器、嵌入式前端构建
├── pkgs/            # 共享 Go 包（db、search、mcp、skills 等）
├── tests/           # 集成与回归测试
└── Taskfile.yml
```

## 配置

### 数据目录

应用数据（SQLite 数据库、工作区、worktree、skills 等）默认位于：

```
~/.matrixops/
```

可通过环境变量覆盖：

```bash
export MATRIXOPS_HOME=/path/to/data
```

### 环境变量

```bash
PORT=8080
HOST=localhost
CORS_ALLOW_ALL=true
CORS_ALLOW_ORIGINS=http://localhost:3010

# 前端（开发模式）
VITE_USE_MESSAGE_V2=true
```

Electron 生产包请保持 API 为相对路径 `/api`，**不要**在构建时写死 `VITE_API_URL=http://localhost:8080/api`。

## 构建发布包

```bash
# macOS 桌面版
task electron:build:mac

# Windows / Linux
task electron:build:win
task electron:build:linux
```

产物输出在 `frontend/dist-electron/`。

## 故障排除

**端口被占用**

```bash
lsof -i :8080
./build/matrixops server --port 8081
```

**打包后的 Electron 连不上后端**

- 使用 `task electron:build:mac` 重新打包，确保 `build/matrixops` 与 `web-server/web/dist` 已打入安装包。
- 生产构建不要设置固定的 `VITE_API_URL`。

**重置前端依赖**

```bash
cd frontend
rm -rf node_modules
npm install --legacy-peer-deps
```

## 技术栈

| 层级 | 技术 |
|------|------|
| UI | React 19、TypeScript、Vite、Tailwind CSS、Electron |
| API | Go、Gin、GORM、SQLite、WebSocket |
| Agent | 自研 Worker 运行时、工具注册表、MCP、记忆检索 |

## 参与贡献

1. Fork 本仓库
2. 创建功能分支（`git checkout -b feature/my-change`）
3. 提交更改
4. 发起 Pull Request

## 许可证

本项目采用 [GNU Affero General Public License v3.0](./LICENSE)（AGPL-3.0）。

若你将修改后的版本作为网络服务运行，须向通过网络与之交互的用户提供相应源代码。
