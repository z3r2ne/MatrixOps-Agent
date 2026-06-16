# 语义回归测试（Semantic Regression）

本目录存放 MatrixOps 语义回归用例与基线。核心库见 [`pkgs/semreg`](../../pkgs/semreg)。

## 层级

| 层级 | 说明 | 是否进 PR CI |
|------|------|-------------|
| **L0** | 结构回归：prompt 片段、首轮 LLM 请求、任务状态（mock LLM） | ✅ |
| **L1** | 行为回归：tool trace 指标 vs baseline | Nightly（待接 LLM） |
| **L2** | 语义回归：verification worker / rubric 评分 | Nightly（待扩展 testrunner） |

## 目录

```
scenarios/     # YAML 用例定义
baselines/     # 已批准的 trace / verdict 基线（进 git）
struct_test.go # L0 集成测试
trace_test.go  # baseline 加载与对比单测
```

## 用例格式（YAML）

```yaml
id: prompt_v2_regression
name: Prompt V2 基础结构
tier: L0
kind: prompt_render   # prompt_render | task_runner

prompt_render:
  global_prompt: global
  user_input: do it
  tool_names: [read_file]
  history:
    - role: user
      content: hello

assert:
  system_prompt_contains:
    - "<system_prompt>"
  user_input_equals: do it
```

`kind: task_runner` 会通过 mock LLM 跑完整 task，检查首轮 system prompt 与任务完成状态。

## 运行

```bash
# L0 结构回归（PR 可跑）
task semreg:struct

# 或单独
cd pkgs && go test ./semreg/... -count=1
cd tests && go test ./semantic_regression/... -count=1
```

`task test` 已包含 L0 语义回归（随 `cd tests && go test ./...` 执行）。

## Trace 基线格式

与 `tests/explore_comparison/collect_trace.py` / `cmd/explore-compare` 输出兼容，并增加 `version` 与 `tolerances`：

```json
{
  "version": 1,
  "summary": { "total_tool_calls": 18, "read_duplicate_ranges": [] },
  "tolerances": { "total_tool_calls": 0.2, "read_duplicate_ranges": 0 }
}
```

使用 `semreg.CompareTraceSummary` 对比实际 trace 与 baseline。

## 后续（Phase 2+）

- `task semreg:behavior`：真实 LLM + explore trace（`-tags=semanticregression`）
- `task semreg:semantic`：扩展 `pkgs/testrunner` 从 YAML 加载 L2 场景
- Nightly workflow 上传 `reports/`
