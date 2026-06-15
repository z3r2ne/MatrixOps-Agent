package session

import (
	"os"
	"runtime"
	"slices"
	"strings"
	"time"

	"matrixops-agent/taskctx"
	database "pkgs/db"
	"pkgs/db/models"

	"gorm.io/gorm"
)

type dynamicPromptLayers struct {
	sessionGuidance string
	outputStyle     string
	toolPriority    string
	environment     string
}

func buildDynamicPromptLayers(db *gorm.DB, worker *models.Worker, ctx taskctx.Context, aiWorkspaceDir string) dynamicPromptLayers {
	enabledTools, hasEnabledTools, err := models.ParseEnabledTools(worker.EnabledTools)
	if err != nil {
		enabledTools = nil
		hasEnabledTools = false
	}
	enabledSkills, hasEnabledSkills, err := models.ParseEnabledSkills(worker.EnabledSkills)
	if err != nil {
		enabledSkills = nil
		hasEnabledSkills = false
	}
	callableWorkers := availableWorkerNamesForPrompt(loadCallableWorkerNames(db))
	return dynamicPromptLayers{
		sessionGuidance: buildSessionGuidancePrompt(hasEnabledTools, enabledTools, hasEnabledSkills, enabledSkills, callableWorkers, aiWorkspaceDir),
		outputStyle:     buildOutputStylePrompt(),
		toolPriority:    buildToolPriorityPrompt(hasEnabledTools, enabledTools, callableWorkers),
		environment:     buildStandardEnvironmentPrompt(ctx, aiWorkspaceDir, time.Now()),
	}
}

func buildStandardEnvironmentPrompt(ctx taskctx.Context, aiWorkspaceDir string, now time.Time) string {
	isGit := "no"
	if ctx.VCS == "git" {
		isGit = "yes"
	}
	shellName := strings.TrimSpace(os.Getenv("SHELL"))
	if shellName == "" {
		shellName = "unknown"
	}
	lines := []string{
		"# Environment",
		"你当前运行在以下环境中：",
		"- Primary working directory: " + ctx.WorkDir,
		"- ai_workspace directory: " + aiWorkspaceDir,
		"- Is a git repository: " + isGit,
		"- Platform: " + runtime.GOOS,
		"- Shell: " + shellName,
		"- Today's date: " + now.Format("Mon Jan 2 2006"),
	}
	if strings.TrimSpace(ctx.Worktree) != "" && ctx.Worktree != ctx.WorkDir {
		lines = append(lines, "- Project worktree root: "+ctx.Worktree)
	}
	projectDirHint := projectDirectoryHint(ctx)
	if projectDataDir, err := database.ProjectDataDir(ctx.ProjectID, projectDirHint); err == nil && strings.TrimSpace(projectDataDir) != "" {
		lines = append(lines, "- Project data directory: "+projectDataDir)
		if codeMapPath, codeMapErr := database.ProjectCodeMapFilePath(ctx.ProjectID, projectDirHint); codeMapErr == nil && strings.TrimSpace(codeMapPath) != "" {
			lines = append(lines, "- Recommended code map file: "+codeMapPath)
		}
	}
	return strings.Join(lines, "\n")
}

func projectDirectoryHint(ctx taskctx.Context) string {
	if workDir := strings.TrimSpace(ctx.WorkDir); workDir != "" {
		return workDir
	}
	if worktree := strings.TrimSpace(ctx.Worktree); worktree != "" {
		return worktree
	}
	return ""
}

func buildOutputStylePrompt() string {
	return strings.Join([]string{
		"# Output Style",
		"- 在第一次工具调用前，用 1-2 句说明你接下来立刻要完成什么；只有在小目标切换时再补新的进度说明。",
		"- 面向用户的正文默认保持简洁、自然、协作，优先先给结论或动作，再给必要细节。",
		"- 不要原样转储大段文件内容、日志或命令输出；只提炼支撑结论的关键部分。",
		"- 最终回复优先覆盖：做了什么、验证了什么、仍有哪些风险或边界；如果没验证，必须明确说明。",
		"- 引用文件时使用独立路径，必要时附行号；避免把多个路径糅在一句泛泛描述里。",
	}, "\n")
}

func buildToolPriorityPrompt(hasEnabledTools bool, enabledTools map[string]struct{}, callableWorkers []string) string {
	if !hasEnabledTools {
		return ""
	}

	lines := []string{
		"# Tool Priority",
	}

	if hasTool(enabledTools, "run_worker_task") {
		lines = append(lines,
			"- 跨目录/陌生模块的定位与梳理：优先 `run_worker_task` → `explore`（只读摸排），再基于结果决策。",
		)
		if slices.Contains(callableWorkers, "plan") {
			lines = append(lines, "- 需要方案与拆解：优先 `run_worker_task` → `plan`（输出步骤/依赖/风险/关键文件）。")
		}
		if slices.Contains(callableWorkers, "frontend_engineer") {
			lines = append(lines, "- 复杂前端改动（跨组件/交互/样式体系/可访问性/设计质量/大范围 UI 调整）时，优先 `run_worker_task` → `frontend_engineer`；轻量局部改动主 worker 可自行完成。")
		}
		if slices.Contains(callableWorkers, "verification") {
			lines = append(lines, "- 多文件/多模块/核心逻辑/安全逻辑/发布前交付时，优先 `run_worker_task` → `verification`；轻量低风险任务主 worker 做最小验证即可。")
		}
	}

	lines = append(lines, "- 已知精确文件路径时，优先 `read`，不要用 `bash` 读大段文件。")

	if hasTool(enabledTools, "patch") {
		lines = append(lines, "- 小而聚焦的代码修改优先 `patch`。")
	}
	if hasTool(enabledTools, "edit") {
		lines = append(lines, "- 需要对已有文件做局部重写或精确替换时，使用 `edit`。")
	}
	if hasTool(enabledTools, "write") {
		lines = append(lines, "- 只有在确实需要创建新文件时才使用 `write`。")
	}
	if hasTool(enabledTools, "rg") {
		lines = append(lines, "- 搜索文件内容优先 `rg`。")
	}
	if hasTool(enabledTools, "glob") {
		lines = append(lines, "- 按路径、命名模式或文件集合查找时优先 `glob`。")
	}
	if hasTool(enabledTools, "list") || hasTool(enabledTools, "tree") {
		lines = append(lines, "- 需要理解目录结构时，先用 `list` / `tree`，再进入文件级搜索。")
	}
	if hasTool(enabledTools, "diff") {
		lines = append(lines, "- 需要理解当前改动、比对实现差异或查看 patch 时使用 `diff`。")
	}
	if hasTool(enabledTools, "load_skill") {
		lines = append(lines, "- 只有当现有上下文不够覆盖某个明确工作流时，再使用 `load_skill` 按名称加载技能；完整正文会出现在工具输出中。")
	}
	if hasTool(enabledTools, "bash") {
		lines = append(lines, "- `bash` 主要用于构建、测试、运行程序、查看系统状态，或没有 dedicated tool 可替代时再使用。")
	}

	if hasTool(enabledTools, "question") {
		lines = append(lines, "- 只有在你经过调查后仍存在关键不确定项时，再用 `question` 问用户；不要把本可通过搜索或读代码解决的问题直接抛给用户。")
	}

	return strings.Join(lines, "\n")
}

func buildSessionGuidancePrompt(hasEnabledTools bool, enabledTools map[string]struct{}, hasEnabledSkills bool, enabledSkills map[string]struct{}, callableWorkers []string, aiWorkspaceDir string) string {
	lines := []string{
		"# Session Guidance",
		"",
		"## 决策优先级补充",
		"1. 正确性优先：可以用“假设”指导探索路径，但不能把未经验证的假设当成事实、结论或直接改代码依据；涉及核心逻辑/安全/公共接口/数据结构/持久化/跨模块影响时，必须通过代码、测试、运行结果或明确上下文验证。",
		"2. 聚焦范围内重构：默认保持改动聚焦、不扩大范围；但当局部补丁会明显增加复杂度，或 bug 根因来自结构问题时，应优先在任务相关范围内重构，而不是最小表面修改。",
		"3. 轻量任务不过度委派：若目标已明确在 1-2 个文件内，或一次精准搜索即可唯一定位，主 worker 直接执行“精准定位 → 最小必要阅读 → 聚焦修改 → 最小验证”。",
		"4. 子 worker 用于降低不确定性：陌生模块/跨目录链路/影响面分析/前端复杂实现/系统性验证/仓库沉淀时，优先委派；子 worker 输出需区分 Findings / Relevant Files / Suggested Actions / Risks & Unknowns / Verification，主 worker 不把推测当事实。",
		"5. Clean Break 有边界：除非任务明确要求兼容旧行为，否则不为旧逻辑增加兼容包袱；但若影响公共 API、配置格式、数据库 schema、持久化数据、插件接口、CLI 参数、SDK 行为或跨版本协议，必须先识别为 breaking change 并说明影响范围，只有在任务目标允许时才执行。",
		"6. plan.md 只记录长期有用信息：不要为短任务创建/更新；仅在多阶段、强依赖、耗时较长或上下文可能丢失时维护；记录目标/约束/决策/重要发现/风险/验证/未完成事项，不记录琐碎工具调用日志。",
		"",
		"## 失败恢复",
		"当测试/构建/运行验证失败时，先判断失败是否由本次修改引入：相关失败回到最近假设并做最小必要探索后修复；无关失败不要擅自修复，在最终回复中说明失败范围、关键错误与判断依据。不要通过删除测试、降低校验、跳过错误或扩大兼容分支来掩盖失败，除非用户明确要求。",
		"",
		"## 停止条件",
		"如果受限于环境/权限/缺失依赖/无法复现/测试不可运行，或存在必须由用户确认的产品决策，应在完成可执行的最大范围后停止，并在最终回复中说明：已完成内容、当前阻塞点、已掌握证据、未验证风险、建议用户下一步确认或执行事项。",
		"",
		"总体目标：更快定位问题、更大胆简化实现，在保证当前任务正确前提下持续提高代码质量。",
	}

	if hasEnabledTools && hasTool(enabledTools, "run_worker_task") {
		if len(callableWorkers) > 0 {
			lines = append(lines, "- 当前可调用的子 worker: "+strings.Join(callableWorkers, ", ")+".")
		}
		lines = append(lines, "- 子 worker 用于降低不确定性、分担专业实现或执行系统性验证；不要为了形式委派。若上下文充分、范围明确、风险可控，主 worker 直接完成。")
		if slices.Contains(callableWorkers, "explore") {
			lines = append(lines, "- `explore`：只读摸排（需要时写明 `quick/medium/very thorough`）。")
		}
	}

	if hasEnabledSkills && len(enabledSkills) > 0 {
		names := make([]string, 0, len(enabledSkills))
		for name := range enabledSkills {
			names = append(names, name)
		}
		slices.Sort(names)
		lines = append(lines, "- 本 Worker 已预加载技能: "+strings.Join(names, ", ")+"。完整内容已在 `<worker_skills>` 中，无需再调用 `load_skill`。")
	} else if hasEnabledTools && hasTool(enabledTools, "load_skill") {
		lines = append(lines, "- 需要技能时用 `load_skill`，技能全文在工具结果里，勿重复加载同一技能。")
	}

	return strings.Join(lines, "\n")
}

func hasTool(enabledTools map[string]struct{}, name string) bool {
	_, ok := enabledTools[name]
	return ok
}

func availableWorkerNamesForPrompt(workers []string) []string {
	known := []string{"explore", "code_map", "frontend_engineer", "plan", "verification"}
	out := make([]string, 0, len(known))
	for _, name := range known {
		if slices.Contains(workers, name) {
			out = append(out, name)
		}
	}
	if len(out) == 0 {
		return workers
	}
	return out
}
