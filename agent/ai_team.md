# FSM + P&E + BT

* **FSM** 管“项目阶段”和“质量门禁”（不允许乱跳）
* **P&E** 管“拆解任务/依赖/验收标准”（把需求变成可执行 backlog）
* **BT** 管“每个阶段内怎么稳定执行、怎么重试、怎么兜底、怎么分派人/工具”

下面给你一套可直接落地的架构模板（你可以用 Go/TS 都好实现）。

---

## 1) 三层架构怎么放到 AI Team 里

### FSM（外层：项目生命周期）

建议项目级 FSM：

1. `INTAKE`（接需求）
2. `PLAN`（产出执行计划/Backlog）
3. `DISPATCH`（派发子任务）
4. `IMPLEMENT`（Coder 实现）
5. `TEST`（Tester 测试）
6. `FIX`（Coder 修复）
7. `INTEGRATE`（合并/集成/回归）
8. `DELIVER`（交付）
9. `DONE / ERROR / ESCALATE`

> 核心：**TEST 不通过一定回到 FIX**，不允许 Leader 直接“口头认为好了就交付”。

---

### P&E（中层：Backlog + Work Items）

Leader 在 `PLAN` 阶段产出结构化计划：

* Epic / Story / Task
* 每个 task 有：输入、输出、验收标准、依赖、风险、负责人角色（coder/tester）

并且计划是 **冻结点**：进入执行后不能随意改，**只能走 REPLAN 状态受控改**（次数上限）。

---

### BT（内层：每个状态的执行策略）

每个 FSM 状态绑定一棵 BT（或一个 BT 模板），用来做：

* 条件门禁（是否具备执行前置条件）
* 具体动作（调用 LLM/工具/子代理）
* 失败重试、降级、记录、兜底
* 把结果写进 ctx（黑板）供 FSM 做跳转判断

---

## 2) 你需要的核心数据结构（黑板 ctx）

建议最少这些字段：

* `project_goal`
* `requirements`（结构化需求，Leader 从你输入抽取）
* `plan`（P&E 输出）
* `work_items`（执行中的任务列表）
* `artifacts`（代码改动、PR、测试报告、发布包等）
* `quality`（质量门禁结果：单测、lint、e2e、覆盖率等）
* `state_meta`（visited 状态、重试计数、replan 次数、预算等）
* `history`（关键日志：谁做了什么、产物链接、失败原因）

---

## 3) Plan 的推荐 Schema（最重要）

你要保证“Leader 的计划”是机器可执行的，不是自然语言。

```json
{
  "goal": "实现 AI Team 的任务分配与交付",
  "constraints": {
    "tools_allowed": ["repo_read", "repo_write", "run_tests", "lint", "issue_tracker"],
    "max_replan": 2,
    "quality_gates": ["unit_test_pass", "lint_pass"]
  },
  "items": [
    {
      "id": "T1",
      "title": "实现任务分配器（leader->workers）",
      "type": "code",
      "owner_role": "coder",
      "inputs": ["requirements", "existing_arch"],
      "outputs": ["scheduler_module"],
      "acceptance": [
        "能创建 work_item 并分配 owner_role",
        "状态流转可追踪"
      ],
      "dependencies": []
    },
    {
      "id": "T2",
      "title": "为任务分配器写单元测试",
      "type": "test",
      "owner_role": "tester",
      "inputs": ["scheduler_module"],
      "outputs": ["test_report"],
      "acceptance": [
        "覆盖核心分配逻辑",
        "测试可稳定复现"
      ],
      "dependencies": ["T1"]
    }
  ]
}
```

---

## 4) FSM 跳转规则（谁决定下一状态）

**FSM 只看事实字段**，不听 LLM “建议”。

例子（核心部分）：

* `INTAKE → PLAN`：`ctx.requirements.valid == true`
* `PLAN → DISPATCH`：`ctx.plan.valid == true`
* `DISPATCH → IMPLEMENT`：存在 `owner_role=coder` 的 ready item
* `IMPLEMENT → TEST`：所有 coder items 达到 `DONE` 或 `READY_FOR_TEST`
* `TEST → FIX`：`ctx.quality.passed == false`
* `TEST → INTEGRATE`：`ctx.quality.passed == true`
* `INTEGRATE → DELIVER`：集成/回归门禁通过
* `DELIVER → DONE`：交付物齐全（release notes / 变更说明 / 版本号）

---

## 5) 每个状态内部用 BT 怎么写（模板化）

### A) PLAN 状态的 BT（Leader 规划）

目标：把你的需求变成 plan + backlog（并校验）

**BT：**

* Sequence

  * Action: LLM 抽取结构化需求（requirements）
  * Action: 生成 Plan（JSON）
  * Condition: Plan schema 校验通过
  * Condition: 每个 item 都有 acceptance
  * Action: plan 冻结（ctx.plan_frozen=true）

> 这里强烈建议：**Plan 必须过 JSON schema 校验**，否则回退让 LLM 修正，最多重试 2 次。

---

### B) DISPATCH 状态的 BT（派发任务给 Coder/Tester）

* Sequence

  * Condition: plan_frozen == true
  * Action: 从 ctx.plan.items 挑选 “可执行（依赖满足）” 的 items
  * Action: 生成 work_item（含 owner、输入、期望输出、验收标准）
  * Action: 投递给对应 worker（coder/tester）

**关键点：**派发时把“验收标准”原封不动传给 worker，避免 worker 自己想象“做完了”。

---

### C) IMPLEMENT 状态的 BT（Coder 执行单个任务）

给每个 work_item 用同一个 step-BT 模板：

* Sequence

  * Condition: item.owner_role == coder
  * Condition: item.dependencies 都完成
  * Action: coder 读取上下文（需求+相关代码）
  * Action: coder 产出修改（patch/PR）
  * Action: coder 自检（lint/unit）
  * Condition: 自检通过
  * Action: 标记 item = READY_FOR_TEST

> 你可以让 coder 在 Action 里用 ReAct，但**工具白名单**与**最大调用次数**由 BT 控。

---

### D) TEST 状态的 BT（Tester 验收）

* Sequence

  * Condition: 有 READY_FOR_TEST items
  * Action: tester 执行测试计划（按 acceptance）
  * Action: 产出 test_report（失败用例/复现步骤/日志）
  * Selector

    * Sequence（通过）

      * Condition: all_passed == true
      * Action: 标记对应 item = DONE
      * Action: ctx.quality.passed = true
    * Sequence（不通过）

      * Action: 生成缺陷列表（issues）
      * Action: 把 issues 绑定回对应 item
      * Action: ctx.quality.passed = false

---

### E) FIX 状态的 BT（修复循环）

* Sequence

  * Condition: ctx.quality.passed == false
  * Action: 把失败用例/复现步骤/日志派回 coder（绑定到原 item）
  * Action: coder 修复并自检
  * Action: 标记 item = READY_FOR_TEST

然后 FSM 会再走 `TEST`，直到通过或触发升级策略（见下面）。

---

## 6) 你必须加的三类“刹车系统”（否则会无限循环）

### ① 每个 item 的测试失败次数上限

* `max_fix_cycles_per_item = 3`
  超过就 `ESCALATE`（Leader 介入：重写需求、降级验收、拆分任务、或 REPLAN）

### ② 全局 replan 次数上限

* `max_replan = 2`
  只有在这些条件下允许 REPLAN：
* 需求变更明确
* 发现依赖缺失/不可行
* 工具/环境限制导致无法完成

### ③ 工具/成本预算

* `max_llm_calls`
* `max_tool_calls`
* `timeout_ms`

---

## 7) 角色怎么实现：Leader / Coder / Tester 是“子代理”还是“同一代理的模式”？

两种都行：

### 方案 A：单进程多角色（推荐 MVP）

* 一个 Agent，通过“role prompt + 工具白名单 + ctx 约束”切换角色
* 好处：实现简单、状态一致性更强

### 方案 B：多子代理（更像 team）

* Leader 负责 FSM + Plan
* Coder/Tester 是独立 worker，有各自记忆/工具权限
* 好处：可并行、多工位；坏处：实现复杂、要处理通信和一致性

你做 AI Team 产品，后续大概率要上 B，但 MVP 用 A 更快。

---

## 8) 最小落地路线（建议你按这个顺序做）

1. **先做 FSM 骨架**（状态、跳转、visited、计数器）
2. **PLAN 状态先跑通**（输出 plan JSON + schema 校验）
3. **做 DISPATCH + 单 item 的 IMPLEMENT**（只支持 coder）
4. **加 TEST + FIX 循环**（tester 先只做规则验收/脚本跑测）
5. **加质量门禁**（unit/lint/e2e、产物归档）
6. **最后再做并行/多 worker/优先级/依赖图**

---

如果你愿意，我可以直接给你“可复制粘贴”的三份东西（按你现在的技术栈来写）：

1. **FSM 转移表 + 状态定义**（含 ESCALATE / REPLAN 规则）
2. **Plan JSON Schema**（可用 jsonschema 校验）
3. **Step-BT 通用模板代码骨架**（Go 或 TS，带重试/超时/预算/去重）

你更偏向用 **Go** 还是 **TypeScript/Node** 来实现这个 AI Team？
