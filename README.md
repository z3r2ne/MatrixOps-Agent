# MatrixOps Agent

**English** | [简体中文](./README.zh-CN.md)

An AI assistant that unifies multiple agents behind one workspace, with **long-term memory** and a usage model that blends **OpenClaw-style persistent assistance** with **coding-agent project execution**.

Use the same system to keep context across weeks, delegate to specialized workers, and drop into a repository when you need to write or change code.

## What makes it different

Most tools force a choice: a memory-heavy assistant, or a repo-scoped coding agent. MatrixOps Agent is built to do both in one flow.

| Mode | What it feels like | Typical use |
|------|-------------------|-------------|
| **Persistent assistant** | Long-lived sessions, memory libraries, reminders, multi-step workflows | Research, planning, recurring tasks, knowledge that should survive restarts |
| **Project coding agent** | Workspace-bound execution with tools for read/edit/bash/git | Implementing features, refactors, debugging in a real codebase |

Under the hood, multiple **workers** (explore, plan, verification, clawbot, frontend engineer, and more) can be composed and orchestrated instead of relying on a single monolithic agent.

## Core capabilities

- **Multi-agent orchestration** — Route work to specialized workers; workers can call other workers when needed.
- **Long-term memory** — Session memory, memory libraries, compaction, and semantic search over stored knowledge.
- **OpenClaw-inspired interaction** — Durable tasks, reminders, and assistant-style workflows that accumulate context over time.
- **Coding-agent execution** — File tools, terminal, diffs, git worktrees, and project-scoped permissions in a real workspace.
- **Desktop + local-first** — Electron app bundles the backend; data stays on your machine (`~/.matrixops` by default).
- **Extensible tooling** — MCP servers, skills, custom workers, and LLM provider configuration.

## Architecture (high level)

```
┌─────────────────────────────────────────────────────────────┐
│  Desktop / Web UI (React + Electron)                        │
└───────────────────────────┬─────────────────────────────────┘
                            │ HTTP / WebSocket
┌───────────────────────────▼─────────────────────────────────┐
│  API server (Go) — workspaces, tasks, sessions, tools       │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────┐
│  Agent runtime — workers, memory, streaming, tool execution │
└─────────────────────────────────────────────────────────────┘
```

## Quick start

### Desktop app (recommended)

```bash
# Build the CLI
go build -o build/matrixops ./cmd

# Launch the desktop app (builds frontend if needed, starts Electron)
./build/matrixops app
```

Or with [Task](https://taskfile.dev):

```bash
task app
```

### Web development

Terminal 1 — backend:

```bash
task backend
```

Terminal 2 — frontend:

```bash
task start
```

Open http://localhost:3010

### Server only

```bash
./build/matrixops server --host localhost --port 8080
```

After building the frontend (`cd frontend && npm run build`), the server also serves the UI.

## Requirements

- **Go** 1.24+
- **Node.js** 18+
- **npm** 9+

## Install

```bash
git clone https://github.com/z3r2ne/MatrixOps-Agent.git
cd MatrixOps-Agent

# Dependencies
go mod download
cd frontend && npm install --legacy-peer-deps
cd ../web-server && go mod download

# CLI binary
cd ..
go build -o build/matrixops ./cmd
```

## CLI

```bash
./build/matrixops --help
./build/matrixops app                      # Desktop app
./build/matrixops server                   # API server (default :8080)
./build/matrixops chat "Summarize this repo"
./build/matrixops version
```

See [cmd/README.md](./cmd/README.md) for server flags, chat options, and pprof.

## Common tasks

```bash
task --list

task app                  # Desktop app
task start                # Frontend dev server
task backend              # Backend dev server
task electron:dev         # Electron shell (dev; start backend + frontend separately)
task electron:build:mac   # Package macOS app
task clean                # Remove build artifacts
```

## Project layout

```
MatrixOps-Agent/
├── agent/           # Agent runtime, workers, tools, session/memory logic
├── cmd/             # CLI entrypoint (app, server, chat)
├── frontend/        # React UI + Electron shell
├── web-server/      # HTTP API, handlers, embedded web build
├── pkgs/            # Shared Go packages (db, search, mcp, skills, …)
├── tests/           # Integration and regression tests
└── Taskfile.yml
```

## Configuration

### Data directory

By default, application data (SQLite DB, workspaces, worktrees, skills) is stored under:

```
~/.matrixops/
```

Override with:

```bash
export MATRIXOPS_HOME=/path/to/data
```

### Environment variables

```bash
PORT=8080
HOST=localhost
CORS_ALLOW_ALL=true
CORS_ALLOW_ORIGINS=http://localhost:3010

# Frontend (development)
VITE_USE_MESSAGE_V2=true
```

For Electron production builds, keep the API base URL as the relative path `/api` (do not bake in `http://localhost:8080`).

## Building releases

```bash
# macOS desktop
task electron:build:mac

# Windows / Linux
task electron:build:win
task electron:build:linux
```

Artifacts are written to `frontend/dist-electron/`.

## Troubleshooting

**Port in use**

```bash
lsof -i :8080
./build/matrixops server --port 8081
```

**Electron cannot reach the backend (packaged app)**

- Rebuild with `task electron:build:mac` so `build/matrixops` and `web-server/web/dist` are bundled.
- Do not set `VITE_API_URL` to a fixed localhost port for production builds.

**Reset frontend dependencies**

```bash
cd frontend
rm -rf node_modules
npm install --legacy-peer-deps
```

## Tech stack

| Layer | Stack |
|-------|--------|
| UI | React 19, TypeScript, Vite, Tailwind CSS, Electron |
| API | Go, Gin, GORM, SQLite, WebSocket |
| Agent | Custom worker runtime, tool registry, MCP, memory search |

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/my-change`)
3. Commit your changes
4. Open a pull request

## License

This project is licensed under the [GNU Affero General Public License v3.0](./LICENSE) (AGPL-3.0).

If you run a modified version as a network service, you must offer corresponding source code to users who interact with it over the network.
