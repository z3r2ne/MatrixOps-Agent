# MatrixOps Agent

**English** | [简体中文](./README.zh-CN.md)

![MatrixOps Agent — composable AI Agent framework](./docs/matrixops-hero.png)

> **MatrixOps Agent** is a general-purpose AI Agent framework: **`core_agent`** is the engine; memory, queues, workers, and tools plug in as modules. The desktop app in this repo is a reference implementation.

MatrixOps Agent is a **high-freedom, general-purpose AI Agent framework**. Memory lives in a separate module with pluggable SQLite or JSON backends and swappable compression / recall strategies. **`core_agent` is the engine** — it parses model output and drives the tool loop (the “brain and heart”). Everything else connects through interfaces so you can build the “body” for your scenario and plug it in.

The MatrixOps desktop app in this repo is a **reference implementation** of that framework: multi-worker orchestration, coding-agent tooling, long-term memory, semantic regression, and more.

## Product vision

### Framework first, scenarios are pluggable

The goal is not a monolithic assistant locked to one UI or one workflow. MatrixOps provides a **composable agent runtime**:

| Layer | Role | Responsibility |
|-------|------|----------------|
| **`core_agent` (brain & heart)** | AI engine | Stream LLM calls, parse actions / tool calls, run the execution loop, emit state updates — **without** binding to a specific database, frontend protocol, or business semantics |
| **Memory (`agent/memory`)** | Pluggable long-term memory | `Store` interface: SQLite (`DBStore`), JSON files (`JSONFileStore`), etc.; compaction and recall are injected by the host app, not baked into the engine |
| **Session / queue / watchdogs (body)** | Scenario adapters | Task queues, critical-info injection, multi-agent / multi-model coordination, permissions, UI protocols — wired in via hooks and adapters |
| **MatrixOps app** | Reference product | Workspaces, workers, desktop UI, semreg test workspace — proof that the framework supports real shipping software |

Development treats **the Agent as a first-class citizen**: a run, a message stream, and a tool loop are the core abstractions. The **message queue** is not a simple chat FIFO — it is a pipe to **inject critical context into the model** and to coordinate **multi-agent / multi-model** work (watchdog warnings, async tool results, critical-info replay, and more).

For a new scenario you typically implement the “body” (storage, tools, prompt layers, external APIs) and attach it to the same `core_agent` engine — no need to fork or rewrite the agent loop.

### Memory: extracted and customizable

`agent/memory` is decoupled from `core_agent`. Host apps persist through a `Store` interface:

- **SQLite** — `DBStore`, suited to desktop / server deployments
- **JSON files** — `JSONFileStore`, suited to debugging, migration, or lightweight single-machine setups

**Compaction** (e.g. graduated levels) and **recall** (semantic search, critical-info re-injection) are policy hooks in the session layer and can be replaced per product without changing the engine core.

### MatrixOps as the reference app

In this repository, MatrixOps applies the framework to a **local-first AI dev workbench**: multi-worker orchestration (explore, plan, verification, frontend engineer, …), Git worktrees, diff review, simulation view, iLink WeChat bridge, and similar features are **bodies plugged into the same brain** — not hard constraints of the framework itself.

## Highlights

### Watchdog safeguards

MatrixOps does not rely on the model to self-correct every failure mode. Built-in watchdogs observe the agent loop and push **supplement system messages** into the task queue when something looks stuck:

| Watchdog | What it watches | What it does |
|----------|-----------------|--------------|
| **Stall** | A tool call runs longer than the configured timeout | Cancels the call and queues a warning so the model can change strategy |
| **Silent tool** | Many consecutive tool calls with no assistant text | Nudges the model to explain progress or answer the user |
| **Tool repeat** | The same tool + arguments called repeatedly | Warns about possible loops and suggests a different approach |

These messages are delivered through the supplement pipeline (see below) without tearing down the session.

### Message & queue pipeline

Conversation is not only “user sends, model replies”. MatrixOps uses a **task message queue** with several delivery modes:

- **User / append messages** — normal input and follow-ups while a task is running.
- **Supplement messages** — system-side injections (watchdog warnings, async tool results, empty-stream retries) written into session memory at safe points in the agent loop.
- **Auto-run** — when a task finishes, queued messages can automatically start the next run.

This keeps long-running agents responsive to background events (tool completion, watchdogs, subtask results) without manual copy-paste.

### Critical info survival

Long sessions are compacted to stay within context limits. **Critical info** is a per-session list of facts that must survive compaction — for example async tool handles (`bash_job_id`, subtask `task_id`), user-visible placeholders, and tool-call fingerprints.

Before each agent step, the runtime checks whether each critical item still appears in the memory transcript. If compaction removed it, the item is **re-injected as a synthetic user message** so the model does not lose track of in-flight work.

### Semantic regression testing

Quality is guarded by a three-tier **semantic regression** suite (`pkgs/semreg`, `tests/semantic_regression`):

| Tier | Focus | Typical run |
|------|-------|-------------|
| **L0** | Prompt structure, first LLM request shape, task status (mock LLM) | Every PR in CI |
| **L1** | Tool-call trace metrics vs. baselines (real LLM) | Nightly / manual |
| **L2** | End-to-end scenarios judged by a verification worker (real LLM) | Nightly / manual |

The desktop app also exposes a **test workspace** to browse scenarios and launch L1/L2 runs from the UI.

### Layered prompts

System instructions are composed in layers instead of a single blob:

1. **Global** — baseline rules for every task (lowest priority among configured layers).
2. **Occupation** — role templates (`coder`, `analyst`, `planner`, …).
3. **Project** — repository-specific guidance.
4. **Worker** — per-worker system prompt (`explore`, `plan`, `leader`, …).
5. **Model settings** — model-family prompt fragments.
6. **Dynamic runtime layers** — environment (cwd, git, shell, date), tool priority, session guidance, and output style injected per run.

Layers are editable in Settings → Prompts and merged when building each LLM request.

### Compatible `action_provider` (tool-call emulation)

Many LLM APIs expose only chat completions — no `tools` / `tool_calls` field. MatrixOps ships two streaming paths:

- **Native** — OpenAI / Anthropic tool-calling APIs when the model config enables it.
- **Compatible** — the default for generic chat endpoints.

In compatible mode, `ToolPromptAdapter` **strips native tool fields from the HTTP request** and **injects tool + action schemas into the system prompt**. The model is asked to emit JSON action envelopes such as `{"@action":"call_tool","data":{...}}` or `{"@action":"answer","data":"..."}`. A streaming parser turns those envelopes into the same internal tool-call pipeline used by native APIs.

That means one agent runtime, one tool registry, and one UI — regardless of whether your provider officially supports function calling.

## Core capabilities

- **Multi-agent orchestration** — Route work to specialized workers; workers can call other workers when needed.
- **Long-term memory** — Session memory, memory libraries, compaction, and semantic search over stored knowledge.
- **OpenClaw-inspired interaction** — Durable tasks, reminders, and assistant-style workflows that accumulate context over time.
- **Coding-agent execution** — File tools, terminal, diffs, git worktrees, and project-scoped permissions in a real workspace.
- **Desktop + local-first** — Electron app bundles the backend; data stays on your machine (`~/.matrixops` by default).
- **Extensible tooling** — MCP servers, skills, custom workers, and LLM provider configuration.

## Screenshots

### New task composer

Pick a project, worker, branch, and optional Git worktree before starting a task. RAG can be toggled per run.

![New task composer](./docs/screenshots/new-task-composer.png)

### Multi-agent chat workspace

Run tasks in a chat-style workspace. Workers such as `explore` can be delegated automatically while you follow tool calls and streamed answers in real time.

![Multi-agent chat workspace](./docs/screenshots/multi-agent-chat.png)

### Agent simulation office

Switch to the simulation view to see subtasks as agents in a virtual office — useful for monitoring parallel worker progress at a glance.

![Agent simulation office](./docs/screenshots/agent-simulation-office.png)

### Snapshot diff review

Review every file change on a timeline, compare unified or split diffs, and undo or restore snapshots before committing.

![Snapshot diff review](./docs/screenshots/snapshot-diff-review.png)

### Session memory management

Inspect, compress, or delete conversation memory per session. See token size, level, and tool-call entries in one table.

![Session memory management](./docs/screenshots/session-memory-management.png)

### Usage statistics

Track LLM calls, first-token latency, cache hits, token throughput, and tool usage with trend charts and provider rankings.

![Usage statistics](./docs/screenshots/usage-statistics.png)

### iLink WeChat bot

Bind a WeChat bot account, scan to sign in, and route incoming messages to a workspace session for hands-free assistant access.

![iLink WeChat bot](./docs/screenshots/ilink-wechat-bot.png)

### Worker configuration

Configure specialized workers (explore, plan, leader, verification, frontend engineer, …) with per-role models and prompts.

![Worker configuration](./docs/screenshots/worker-configuration.png)

### Skills marketplace

Browse, install, and manage Skills from multiple sources — document tools, research workflows, frontend design helpers, and more.

![Skills marketplace](./docs/screenshots/skills-marketplace.png)

### Prompt management

Edit global, occupation, and project-level system prompts. Markdown is supported and layered into every worker’s context.

![Prompt management](./docs/screenshots/prompt-management.png)

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
