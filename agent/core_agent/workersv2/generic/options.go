package generic

import (
	"fmt"
	"strings"

	coreagent "matrixops.local/core_agent"
	database "pkgs/db"
	"pkgs/db/models"

	"gorm.io/gorm"
)

// Option 在默认 AgentConfig 之上增量修改；返回错误时 New 终止。
type Option func(*AgentConfig) error

// DefaultWorkerName 与 WithWorkerFromDB 在未指定名称时使用。
const DefaultWorkerName = "chat"

// New 先构造 defaultAgentConfig，再依次应用 opts，最后 setDefaults（补齐未设置的 LLM Client、Emitter、工具表等）并校验。
func New(opts ...Option) (*Worker, error) {
	ac := defaultAgentConfig()
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(&ac); err != nil {
			return nil, err
		}
	}
	ac.setDefaults()
	rc, err := agentConfigToRuntime(ac)
	if err != nil {
		return nil, err
	}
	return &Worker{cfg: rc, notify: make(chan struct{}, 1)}, nil
}

// WithFullAgentConfig 用整份配置覆盖当前 AgentConfig（常用于 session 已装配好的 ac）。
// 覆盖后仍会在 New 末尾执行 setDefaults，因此 ac 中为零值的字段（如 nil Emitter）会被兜底。
func WithFullAgentConfig(src AgentConfig) Option {
	return func(c *AgentConfig) error {
		*c = src
		return nil
	}
}

// WithLegacyConfig 将旧版扁平 Config 转为 AgentConfig 并整体覆盖（与 NewFromLegacy 行为一致）。
func WithLegacyConfig(leg Config) Option {
	return WithFullAgentConfig(legacyConfigToAgentConfig(leg))
}

// WithWorkerFromDB 按 worker 名从数据库合并 Worker / LLMConfig / ModelSettings（不整体替换配置）。
// 未传 name 或为空时使用 DefaultWorkerName。要求 Worker 已关联 LLMConfig。
// 会写入 LLM（GenericProviderClient）、NoEmitter、空工具表。
func WithWorkerFromDB(db *gorm.DB, name ...string) Option {
	return func(c *AgentConfig) error {
		if db == nil {
			return fmt.Errorf("WithWorkerFromDB: db is required")
		}
		n := DefaultWorkerName
		if len(name) > 0 && strings.TrimSpace(name[0]) != "" {
			n = strings.TrimSpace(name[0])
		}
		m, err := database.LoadWorkerModelContext(db, n)
		if err != nil {
			return err
		}
		w := m.Worker
		if w == nil {
			return fmt.Errorf("worker %q not loaded", n)
		}
		if m.LLMConfig == nil {
			return fmt.Errorf("worker %q has no LLMConfig: associate llmConfigId in database", n)
		}

		c.Name = w.Name
		c.Prompts.WorkerPrompt = strings.TrimSpace(w.SystemPrompt)
		c.Prompts.OccupationPrompt = strings.TrimSpace(w.Occupation)
		if m.ModelSettings != nil {
			c.Prompts.ModelPrompt = strings.TrimSpace(m.ModelSettings.Prompt)
		}

		modelOut := 0
		if m.ModelSettings != nil {
			modelOut = m.ModelSettings.OutputLimit
		}
		maxOut := database.EffectiveLLMMaxOutputTokens(db, modelOut)

		provName := strings.TrimSpace(w.Provider)
		if strings.TrimSpace(m.LLMConfig.Name) != "" {
			provName = strings.TrimSpace(m.LLMConfig.Name)
		}

		c.LLM = LLMSettings{
			Client:          coreagent.NewGenericProviderClient(),
			ProviderName:    provName,
			Model:           strings.TrimSpace(w.Model),
			Temperature:     0.0,
			TopP:            models.EffectiveTopP(m.ModelSettings),
			MaxOutputTokens: maxOut,
			ProviderOptions: m.LLMConfig,
		}
		if w.Temperature != nil {
			c.LLM.Temperature = *w.Temperature
		}
		c.Memory = MemorySystem{}
		c.Runtime.Emitter = coreagent.NoEmitter{}
		c.Runtime.Tools = coreagent.NewToolRegistry()
		if m.ModelSettings != nil {
			c.Runtime.NativeOpenAIToolCalls = m.ModelSettings.NativeOpenAIToolCalls
			c.Runtime.SystemPromptPlacement = coreagent.NormalizeSystemPromptPlacement(m.ModelSettings.SystemPromptPlacement)
			c.Runtime.ThinkingType = models.EffectiveThinkingType(m.ModelSettings)
			c.Runtime.EnableThinking = m.ModelSettings.EnableThinking
			c.Runtime.BudgetTokens = m.ModelSettings.BudgetTokens
			// 与 EnableThinking 无关：Reasoning effort / verbosity 等须从 ModelSettings 传入 Runner，否则 DB 装配的 Worker 请求里会缺失这些字段。
			c.Runtime.ReasoningEffort = models.OpenAIReasoningEffortValue(m.ModelSettings)
			c.Runtime.TextVerbosity = models.OpenAITextVerbosityValue(m.ModelSettings)
			c.Runtime.EnableEncryptedReasoning = models.EncryptedReasoningEnabled(m.ModelSettings)
			c.Runtime.ParallelToolCalls = models.ParallelToolCallsEnabled(m.ModelSettings)
		} else {
			c.Runtime.NativeOpenAIToolCalls = m.LLMConfig.NativeOpenAIToolCalls
		}
		c.Ext = ExtConfigFromJSON(w.ExtConfig)
		return nil
	}
}

// WithLLMClient 覆盖 LLM 客户端（需实现 StreamChatWithOptions 方可走流式 Runner）。
func WithLLMClient(client coreagent.ChatClient) Option {
	return func(c *AgentConfig) error {
		if client == nil {
			return fmt.Errorf("WithLLMClient: client is nil")
		}
		c.LLM.Client = client
		return nil
	}
}

// WithLLMParams 设置 provider显示名、模型与采样参数；ProviderOptions 常为 *models.LLMConfig。
func WithLLMParams(providerName, model string, temperature, topP float64, maxOutputTokens int, providerOptions any) Option {
	return func(c *AgentConfig) error {
		c.LLM.ProviderName = strings.TrimSpace(providerName)
		c.LLM.Model = strings.TrimSpace(model)
		c.LLM.Temperature = temperature
		c.LLM.TopP = topP
		if maxOutputTokens > 0 {
			c.LLM.MaxOutputTokens = maxOutputTokens
		}
		if providerOptions != nil {
			c.LLM.ProviderOptions = providerOptions
		}
		return nil
	}
}

// WithEmitter 覆盖 Emitter；nil 表示在 setDefaults 中回退为 NoEmitter。
func WithEmitter(e coreagent.Emitter) Option {
	return func(c *AgentConfig) error {
		c.Runtime.Emitter = e
		return nil
	}
}

// WithMemory 覆盖记忆钩子。
func WithMemory(m MemorySystem) Option {
	return func(c *AgentConfig) error {
		c.Memory = m
		return nil
	}
}

// WithTools 覆盖工具注册表。
func WithTools(reg *coreagent.ToolRegistry) Option {
	return func(c *AgentConfig) error {
		c.Runtime.Tools = reg
		return nil
	}
}

// WithToolRuntime 设置 ToolContextBuilder 与 ToolExecutor。
func WithToolRuntime(ctxb coreagent.ToolContextBuilder, exec coreagent.ToolExecutor) Option {
	return func(c *AgentConfig) error {
		c.Runtime.ToolContextBuilder = ctxb
		c.Runtime.ToolExecutor = exec
		return nil
	}
}

// WithMaxSteps 设置 Runner 默认最大步数（RunInput.MaxSteps 仍为优先）。
func WithMaxSteps(n int) Option {
	return func(c *AgentConfig) error {
		c.Runtime.MaxSteps = n
		return nil
	}
}

// WithConfigureRunner 在 core Runner 创建后、Run 前调用（如 RegisterAction）。
func WithConfigureRunner(fn func(*coreagent.Runner)) Option {
	return func(c *AgentConfig) error {
		c.Runtime.ConfigureRunner = fn
		return nil
	}
}

// WithCompatibleActionSchemas 覆盖兼容模式 prompt 中的 action 列表。
func WithCompatibleActionSchemas(schemas []coreagent.ActionSchema) Option {
	return func(c *AgentConfig) error {
		c.Runtime.CompatibleActionSchemas = append([]coreagent.ActionSchema(nil), schemas...)
		return nil
	}
}

// WithSystemPromptPlacement 设置 system/instruction/user_input 拆分策略。
func WithSystemPromptPlacement(placement string) Option {
	return func(c *AgentConfig) error {
		c.Runtime.SystemPromptPlacement = coreagent.NormalizeSystemPromptPlacement(placement)
		return nil
	}
}

// WithLoop 设置主循环 PromptBuilder 或名称及选项。
func WithLoop(builder coreagent.PromptBuilder, builderName string, opt coreagent.PromptBuilderOptions) Option {
	return func(c *AgentConfig) error {
		c.Loop.Builder = builder
		c.Loop.BuilderName = strings.TrimSpace(builderName)
		c.Loop.Options = opt
		return nil
	}
}

// WithPromptSections 设置分层静态/子构建器提示（与 Prompts 其它字段可组合）。
func WithPromptSections(workerPrompt, modelPrompt, occupationPrompt string, static []StaticPromptSection, sub []PromptSection) Option {
	return func(c *AgentConfig) error {
		c.Prompts.WorkerPrompt = strings.TrimSpace(workerPrompt)
		c.Prompts.ModelPrompt = strings.TrimSpace(modelPrompt)
		c.Prompts.OccupationPrompt = strings.TrimSpace(occupationPrompt)
		if len(static) > 0 {
			c.Prompts.StaticPrompts = append([]StaticPromptSection(nil), static...)
		}
		if len(sub) > 0 {
			c.Prompts.SubPromptBuilders = append([]PromptSection(nil), sub...)
		}
		return nil
	}
}

// WithName 逻辑名称（如 DB worker.name）。
func WithName(name string) Option {
	return func(c *AgentConfig) error {
		c.Name = strings.TrimSpace(name)
		return nil
	}
}

// WithExt 设置扩展 JSON。
func WithExt(ext ExtConfig) Option {
	return func(c *AgentConfig) error {
		c.Ext = ext
		return nil
	}
}

// WithEnableCallToolReason 已废弃：兼容模式与原生模式均只通过 Tools + 工具调用工作，不再使用 action schema。
func WithEnableCallToolReason(enable bool) Option {
	return func(*AgentConfig) error {
		_ = enable
		return nil
	}
}

// WithNativeOpenAIToolCalls 为 true 时，Runner 使用 openai-go 流式接口并解析原生 tool_calls（需 LLM.ProviderOptions 含 API Key）。
func WithNativeOpenAIToolCalls(enable bool) Option {
	return func(c *AgentConfig) error {
		c.Runtime.NativeOpenAIToolCalls = enable
		return nil
	}
}
