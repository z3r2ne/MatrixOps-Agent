# 语义回归测试（Semantic Regression）

本目录存放 MatrixOps 语义回归用例与基线。核心库见 [`pkgs/semreg`](../../pkgs/semreg)。

## 层级

| 层级 | 说明 | 是否进 PR CI |
|------|------|-------------|
| **L0** | 结构回归：prompt 片段、首轮 LLM 请求、任务状态（mock LLM） | ✅ |
| **L1** | 行为回归：tool trace 指标 vs baseline（真实 LLM） | Nightly / 手动 |
| **L2** | 语义回归：testrunner + verification judge（真实 LLM） | Nightly / 手动 |

## 运行

```bash
task semreg:struct     # L0
task semreg:behavior   # L1，需 SEMREG_ENABLE=1
task semreg:semantic   # L2，需 SEMREG_ENABLE=1
```

L1/L2 环境变量：

```bash
export SEMREG_ENABLE=1
export SEMREG_WORK_DIR=/path/to/project
export SEMREG_WORKSPACE_ID=7
export SEMREG_PROJECT_ID=8
```

## 用例类型

- `prompt_render` / `task_runner` — L0
- `behavior` — L1，对比 `baselines/*.json`
- `semantic` — L2，复用 `pkgs/testrunner` 场景（YAML `reuse_scenario`）

## CI

Nightly workflow：`.github/workflows/semantic-regression.yml`（仓库变量 `SEMREG_ENABLE=1` 启用 L1/L2）
