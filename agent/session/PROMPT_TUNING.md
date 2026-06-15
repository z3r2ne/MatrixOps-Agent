# Prompt Tuning Notes

This file records every prompt adjustment made in `agent/session`, including:

- what changed
- why it changed
- which bad behavior it tries to prevent
- how we expect the new prompt to behave

## 2026-04-21 · Explore-First Guidance For Broad Repo Analysis

**Observed issue**

- In a real `chat` CLI debug session against `/Users/z3/Code/go/yaklang-ai-config`, the main `chat` worker repeatedly used local `tree` / `rg` / `read`.
- It did **not** delegate to `run_worker_task(explore)` even though:
  - `run_worker_task` was enabled for `chat`
  - `explore` / `plan` / `verification` were available in prompt context
  - the task clearly required broad cross-directory investigation before producing a plan

**Why this is a problem**

- The main worker burned many iterations on repository-wide reconnaissance that is better handled by a read-only subworker.
- This makes the agent slower, less structured, and harder to debug.
- It also weakens the intended role split:
  - main worker = orchestration / integration
  - `explore` = read-only reconnaissance
  - `plan` = implementation planning

**Prompt changes**

- Strengthened `# Tool Priority` in `agent/session/prompt_layers.go` to say:
  - broad repo analysis should default to `run_worker_task(explore)`
  - main worker should avoid doing many consecutive `rg/read` calls itself for cross-directory discovery
  - planning tasks should usually follow an `explore` pass rather than guessing
- Strengthened `# Session Guidance` in `agent/session/prompt_layers.go` to say:
  - for unfamiliar repos or cross-module questions, the first round should usually delegate reconnaissance to a subworker
  - when the user asks to “read code first, then make a plan”, the preferred flow is `explore` first, then `plan`

**Intended outcome**

- For requests like:
  - “看一下当前项目的相关代码”
  - “梳理实现链路”
  - “定位触发点和影响面”
  - “先看代码再给方案”
- the agent should first call `run_worker_task` with `explore`, then optionally use `plan`, instead of directly expanding a long local search sequence in the main worker.

**Validation**

- Added unit tests in `agent/session/prompt_layers_test.go` to ensure the strengthened delegation language stays present.

## 2026-04-21 · Stronger And Earlier Explore-First Constraint

**Observed issue after first tuning pass**

- Even after adding softer delegation guidance, the `chat` worker still started with local `tree` / `rg` / `read` in the same broad repo-analysis prompt.
- The likely reason was prompt ordering and wording:
  - local file-search guidance appeared earlier and more concretely
  - `explore` delegation sounded like a preference, not an operating rule

**Prompt changes**

- Moved broad-analysis delegation guidance to the top of `# Tool Priority`
- Upgraded the wording from “prefer” to a stronger default rule:
  - first round should default to `run_worker_task(explore)`
  - main worker should not start with repeated repo-wide local search
  - after 2+ broad local searches without clarity, it should switch to `explore`
- Added a clearer default workflow for “read code first, then make a plan”:
  - `explore` → local verification if needed → `plan`

**Intended outcome**

- The model should treat `explore` as the default reconnaissance path for broad repo tasks, instead of as an optional optimization.

## 2026-04-21 · Promote Explore-First Rule Into Chat Worker Base Prompt

**Observed issue after second tuning pass**

- Even with stronger dynamic prompt layers, the model still started broad repo analysis with local `tree` / `rg` / `read`.
- This suggested the later injected prompt sections were still being outranked by the base `chat` worker system prompt and the model's habitual local-search pattern.

**Prompt changes**

- Added a dedicated `## 仓库探索委派` section near the top of `agent/core_agent/workersv2/generic/chat.yaml`
- The new section makes the rule explicit at worker-base level:
  - broad repo analysis should start with `run_worker_task(explore)`
  - main worker should not begin with repeated local repo-wide search
  - “先看代码再给方案” should follow `explore` → local verification if needed → `plan`

**Why this change exists**

- Dynamic prompt layers are helpful, but the base worker prompt is more stable and consistently present.
- The explore-first rule is important enough to live in both places:
  - base worker prompt = stable default behavior
  - dynamic layers = environment-specific reinforcement

## 2026-04-21 · Remove Auto-Explore Fallback

**Why it was removed**

- A runtime auto-triggered `explore` fallback was tested, but it is still a hardcoded mechanism based on heuristic matching.
- That approach is not stable enough for long-term behavior shaping:
  - it may miss valid cases
  - it may trigger on the wrong cases
  - it hides the real model behavior instead of improving it

**What replaced it**

- Reverted the strategy fallback and returned to prompt/tool-driven behavior.
- Kept the stronger delegation guidance in:
  - `agent/session/prompt_layers.go`
  - `agent/core_agent/workersv2/generic/chat.yaml`
- Improved `run_worker_task` tool wording so the model sees clearer usage intent:
  - `explore` for broad read-only repo analysis
  - `plan` for implementation planning
  - `verification` for independent validation
- Improved CLI debug visibility so subtask usage is easier to inspect when it does happen naturally.

**Design principle**

- Prefer making the agent *want* to choose the right path via prompt, tool semantics, and observability.
- Avoid silent strategy injections that alter behavior behind the scenes unless there is a very strong product reason.

## 2026-04-21 · Add Delegation Examples To Main Task Template

**Observed issue**

- Even after stronger rules, the model still often started with local `tree/rg/read`.
- This suggests abstract guidance alone is not enough; the model benefits from a concrete action pattern.

**Prompt changes**

- Added a short `<delegation_examples>` section to `agent/core_agent/promptbuilder/templates/v2_task.tmpl`
- It only appears when `run_worker_task` is actually available
- The examples explicitly map broad repo-analysis requests to:
  - first step: `run_worker_task(explore)`
  - later planning: `run_worker_task(plan)`
  - skip delegation only when the task is clearly limited to 1-2 known files

**Why this exists**

- This is still prompt-only guidance, but more concrete than general policy text.
- The intent is to teach the model a reusable action pattern, not to force behavior via hidden runtime control.

## 2026-04-21 · Add Concrete Subtask Briefing Examples To `run_worker_task`

**Observed issue**

- Even after adding delegation examples to the main prompt template, the model still often stayed in local search mode.
- One likely reason is that `run_worker_task` still looked like a generic orchestration tool, without enough concrete “how to use me” guidance.

**Prompt/tool changes**

- Expanded `agent/tool/run_worker_task.go`
  - description now includes a concrete workflow:
    - `explore` first for repo reconnaissance
    - main worker integrates findings
    - `plan` later if a structured implementation plan is needed
  - `content` field description now includes an example of a good `explore` brief

**Why this exists**

- Tool descriptions are one of the most direct places where the model learns operational intent.
- The goal is to reduce ambiguity around when `run_worker_task` is the correct first move, and how to phrase a useful subtask brief.

## 2026-04-21 · Fix Tool Definition Export: Real Descriptions And Stable Ordering

**Root cause discovered**

- In `agent/session/utils.go`, tool definitions exported to the model were using `tool.Name()` as the description instead of `tool.Description()`.
- Tool ordering was also unstable in practice because names were first filtered into a map and then iterated back out.

**Why this matters**

- This severely weakens tool selection quality:
  - the model does not see the actual semantic guidance for `run_worker_task`
  - tools can appear in inconsistent order between runs
- For the `explore` delegation problem, this is especially important because `run_worker_task` depends heavily on its description to explain when it should be chosen.

**Fix**

- `resolveTools(...)` now:
  - exports the real `tool.Description()`
  - keeps deterministic ordering
  - prioritizes `run_worker_task` before local search tools like `read` / `rg` / `glob` / `list` / `tree`

**Expected outcome**

- The model sees a clearer and more stable tool list.
- `run_worker_task` becomes more discoverable and semantically understandable during the first decision step.

## 2026-04-21 · Add Boundary Hints To Local Search Tools

**Observed issue**

- The model keeps preferring local primitives such as `tree`, `rg`, `read`, and `list` for broad repo-analysis tasks.
- As long as those tools look like the easiest generic option, prompt-only delegation rules compete with their default appeal.

**Tool changes**

- Updated descriptions for:
  - `tree`
  - `rg`
  - `read`
  - `list`
- Each now explicitly says:
  - it is suitable for local confirmation / narrowed scope work
  - it is **not** the preferred first step for broad unfamiliar-repo reconnaissance
  - broad read-only exploration should prefer `run_worker_task` + `explore`

**Why this exists**

- Tool descriptions shape local choice pressure.
- If the “wrong” tools do not advertise their boundary, the model tends to overuse them.

## 2026-04-24 · Make `frontend_engineer` Callable And Preferred For Frontend Work

**Observed issue**

- `frontend_engineer` existed as a builtin worker, but it was not surfaced in the main worker's prompt-time “known callable workers” list.
- Its occupation was set to `frontend_engineer`, while the default occupation table only contains `analyst` / `coder` / `reviewer` / `orchestrator` / `planner`.
- As a result:
  - the main worker had weak awareness that `frontend_engineer` should be delegated to
  - the worker itself did not inherit the standard `coder` occupation prompt layer

**Changes**

- Changed `frontend_engineer` builtin worker occupation from `frontend_engineer` to `coder`
- Strengthened the worker prompt in `agent/core_agent/workersv2/frontend_engineer/frontend_engineer.yaml` so it explicitly frames the worker as the preferred subworker for:
  - React / TSX / JSX
  - CSS / Tailwind
  - Dialog / Sheet / layout overlap issues
  - component interaction and accessibility
- Added `frontend_engineer` to the prompt-visible callable worker shortlist in `agent/session/prompt_layers.go`
- Added delegation guidance so frontend/UI tasks prefer `run_worker_task(frontend_engineer)`
- Updated `run_worker_task` tool description and schema help text to advertise `frontend_engineer` as the preferred frontend worker

**Intended outcome**

- When the user task is clearly frontend-oriented, the main worker should prefer delegating to `frontend_engineer` instead of handling it as a generic `chat` coding task.
- `frontend_engineer` should also benefit from the normal `coder` occupation prompt layer rather than referencing a missing occupation code.

## 2026-04-24 · Add Explicit "Do Not Overstep" Rule For Frontend Changes

**Observed issue**

- Even after making `frontend_engineer` callable and preferred for frontend tasks, the main worker may still be tempted to keep implementation work locally.
- The missing piece is an explicit role-boundary rule: delegation preference alone is weaker than a “do not overstep” constraint.

**Prompt changes**

- Added a dedicated `## 任务边界` section to `agent/core_agent/workersv2/generic/chat.yaml`
- Reinforced in `agent/session/prompt_layers.go` that:
  - when the task becomes frontend code modification, the main worker should not overstep
  - frontend implementation should default to `frontend_engineer`
  - the main worker should stay in orchestration / context / integration role
- Extended the `v2_task` delegation examples with a concrete frontend handoff example

**Why this exists**

- The model needs both:
  - “this subworker is appropriate”
  - and “you should not keep doing this yourself once the work is clearly in that subworker's lane”

**Intended outcome**

- Once a task is clearly about modifying frontend code, the main worker should stop treating it as generic coding work and hand it to `frontend_engineer` by default.
