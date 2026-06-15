package models

import "time"

// GlobalConfig 全局配置
type GlobalConfig struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Key       string    `json:"key" gorm:"unique;not null"` // 配置键
	Value     string    `json:"value"`                      // 配置值
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// 配置键常量
const (
	ConfigKeyKeepProcessAlive                 = "keep_process_alive"                  // 常驻进程开关
	ConfigKeyMatrixopsAgent                      = "matrixops_agent"                   // MatrixOps Agent 配置
	ConfigKeyDefaultLLMConfig                 = "default_llm_config"                  // 默认 LLM 配置 ID
	ConfigKeyGlobalPrompt                     = "global_prompt"                       // 全局提示词
	ConfigKeyDefaultShell                     = "default_shell"                       // 默认 shell
	ConfigKeyCustomShellCommand               = "custom_shell_command"                // 自定义 shell 命令
	ConfigKeyLLMCustomHeaders                 = "llm_custom_headers"                  // 全局附加到 LLM HTTP 请求的自定义 Header（JSON 对象）
	ConfigKeyLLMHTTPTimeoutSeconds            = "llm_http_timeout_seconds"            // 调用大模型 API 的 http.Client 总超时（秒）；≤0 或未配置表示不限制
	ConfigKeyLLMHTTPConnectTimeoutSeconds     = "llm_http_connect_timeout_seconds"    // 调用大模型 API 的连接超时（秒），控制从连接建立到读取第一个响应字符；≤0 或未配置表示不限制
	ConfigKeyLLMMaxOutputTokens               = "llm_max_output_tokens"               // 单次补全最大输出 token；未配置或无效时使用 DefaultLLMMaxOutputTokens
	ConfigKeyStallWatchdogTimeoutSeconds      = "stall_watchdog_timeout_seconds"      // 工具 stall watchdog 触发超时（秒）；≤0 或未配置时使用默认值 10s
	ConfigKeyAgentMaxSteps                              = "agent_max_steps"                               // Agent 单轮任务最大迭代步数；≤0 或未配置时使用 DefaultAgentMaxSteps
	ConfigKeyMemoryCompactionTriggerThresholdPercent  = "memory_compaction_trigger_threshold_percent"   // 自动记忆压缩触发阈值（上下文占用百分比）
	ConfigKeyMemoryCompactionTargetPercent          = "memory_compaction_target_percent"              // 记忆压缩目标占用率（current/limit ≤ 该值）
	ConfigKeyMemoryCompactionL2ScopePercent           = "memory_compaction_l2_scope_percent"            // L2 摘要 batch 在可压缩记忆中的 token 占比
	ConfigKeyMemoryCompactionScopePercent             = "memory_compaction_scope_percent"               // 已废弃，迁移至 l2_scope_percent
	ConfigKeyDefaultTaskListGroupMode         = "default_task_list_group_mode"        // 新工作区任务列表默认分组方式
	ConfigKeyDefaultProjectToolPermissions    = "default_project_tool_permissions"    // 新建项目默认工具权限模板（JSON 对象）
)

// DefaultLLMMaxOutputTokens 为 llm_max_output_tokens 未配置或无效时的默认上限。
const DefaultLLMMaxOutputTokens = 100000

// DefaultAgentMaxSteps 为 agent_max_steps 未配置或无效时的默认最大迭代步数。
const DefaultAgentMaxSteps = 1000

// DefaultMemoryCompactionTriggerThresholdPercent 为自动记忆压缩触发阈值默认值（百分比）。
const DefaultMemoryCompactionTriggerThresholdPercent = 80

// DefaultMemoryCompactionTargetPercent 为记忆压缩目标占用率默认值（百分比）。
const DefaultMemoryCompactionTargetPercent = 60

// DefaultMemoryCompactionL2ScopePercent 为 L2 摘要范围默认值（百分比）。
const DefaultMemoryCompactionL2ScopePercent = 80

// DefaultMemoryCompactionScopePercent 已废弃，与 DefaultMemoryCompactionL2ScopePercent 相同。
const DefaultMemoryCompactionScopePercent = DefaultMemoryCompactionL2ScopePercent
