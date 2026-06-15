package generic

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	coreagent "matrixops.local/core_agent"
	"pkgs/db/models"
	"pkgs/taskqueue"
)

// ExtConfig 来自数据库 Worker.extConfig 等位置的 JSON 扩展，供上层解析（如记忆类型、额外开关）。
type ExtConfig struct {
	raw json.RawMessage
}

// ExtConfigFromJSON 从 JSON 文本构造扩展配置；空字符串表示无扩展。
func ExtConfigFromJSON(s string) ExtConfig {
	s = strings.TrimSpace(s)
	if s == "" {
		return ExtConfig{}
	}
	return ExtConfig{raw: json.RawMessage(s)}
}

// Bytes 返回原始 JSON；若无配置则为 nil。
func (e ExtConfig) Bytes() []byte {
	if len(e.raw) == 0 {
		return nil
	}
	return append([]byte(nil), e.raw...)
}

// Unmarshal 将扩展 JSON 解码到 v（与 encoding/json.Unmarshal 相同约定）。
func (e ExtConfig) Unmarshal(v any) error {
	if len(e.raw) == 0 {
		return nil
	}
	return json.Unmarshal(e.raw, v)
}

// IsEmpty 是否未配置扩展。
func (e ExtConfig) IsEmpty() bool {
	return len(strings.TrimSpace(string(e.raw))) == 0
}

// PromptSettings 静态与动态提示片段（通常由 Worker / ModelSettings / Occupation 等 DB 记录装配）。
type PromptSettings struct {
	WorkerPrompt      string
	ModelPrompt       string
	OccupationPrompt  string
	StaticPrompts     []StaticPromptSection
	SubPromptBuilders []PromptSection
}

// LoopPromptSettings 主循环使用的模板（默认 v2_task，与 Memory / Tools 等共同构成一轮提示）。
type LoopPromptSettings struct {
	// Builder 非空时直接使用，忽略 BuilderName。
	Builder coreagent.PromptBuilder
	// BuilderName 为空则使用 coreagent.DefaultPromptBuilderName。
	BuilderName string
	Options     coreagent.PromptBuilderOptions
}

// LLMSettings 模型与客户端。
type LLMSettings struct {
	ProviderName    string
	Model           string
	Temperature     float64
	TopP            float64
	MaxOutputTokens int
	ProviderOptions any
	Client          coreagent.ChatClient
}

// RuntimeSettings 工具、发射器与步数等运行期依赖。
type RuntimeSettings struct {
	Emitter               coreagent.Emitter
	Tools                 *coreagent.ToolRegistry
	ToolContextBuilder    coreagent.ToolContextBuilder
	ToolExecutor          coreagent.ToolExecutor
	MaxSteps              int
	IDGenerator           coreagent.IDGenerator
	Now                   func() time.Time
	SystemPromptPlacement string
	// ConfigureRunner 在 NewRunner 成功后、执行 Run 前调用，用于 RegisterAction 等。
	ConfigureRunner func(*coreagent.Runner)
	// CompatibleActionSchemas 兼容模式 action 列表；为空时 Runner 使用默认 SessionActionSchemas。
	CompatibleActionSchemas []coreagent.ActionSchema
	// NativeOpenAIToolCalls 为 true 时 Runner 使用 openai-go 原生 tools（见 coreagent.RunnerConfig）。
	NativeOpenAIToolCalls    bool
	ReasoningEffort          string
	TextVerbosity            string
	EnableEncryptedReasoning bool
	ParallelToolCalls        bool
	PromptCacheKey           string
	ThinkingType             string
	EnableThinking           *bool
	BudgetTokens             *int
	StallWatchdogTimeout     time.Duration
	RepeatedToolCallThreshold int
	OnRepeatedToolCall       func(state *coreagent.RunState, toolName string, args map[string]interface{}, count int) error
	SilentToolCallThreshold  int
	OnSilentToolStreak       func(state *coreagent.RunState, count int) error
	OnStallWatchdogToolCancelled func(state *coreagent.RunState, toolName, callID, reason string, elapsed time.Duration) error
	MessageQueue             *taskqueue.Queue
	ConsumeSupplement        func(state *coreagent.RunState, item models.TaskMessageQueueItem) error
}

// AgentConfig 描述一个完整可执行 Agent：记忆钩子 + 工具 + LLM + 分层提示词 + 循环 PromptBuilder。
// 装配好后使用 New(WithFullAgentConfig(ac))，或拆成多个 WithXxx Option。
type AgentConfig struct {
	Name string

	Prompts PromptSettings
	Loop    LoopPromptSettings
	Memory  MemorySystem
	LLM     LLMSettings
	Runtime RuntimeSettings
	Ext     ExtConfig
}

// defaultAgentConfig 为 New(opts...) 的起始模板；未在 Option 中覆盖的字段会保留此处或 setDefaults 中的兜底值。
func defaultAgentConfig() AgentConfig {
	return AgentConfig{
		LLM: LLMSettings{
			Client:          coreagent.NewGenericProviderClient(),
			MaxOutputTokens: models.DefaultLLMMaxOutputTokens,
		},
		Runtime: RuntimeSettings{
			Emitter: coreagent.NoEmitter{},
			Tools:   coreagent.NewToolRegistry(),
		},
	}
}

func (c *AgentConfig) setDefaults() {
	if c.LLM.Client == nil {
		c.LLM.Client = coreagent.NewGenericProviderClient()
	}
	if c.LLM.MaxOutputTokens <= 0 {
		c.LLM.MaxOutputTokens = models.DefaultLLMMaxOutputTokens
	}
	if c.Runtime.Emitter == nil {
		c.Runtime.Emitter = coreagent.NoEmitter{}
	}
	if c.Runtime.Tools == nil {
		c.Runtime.Tools = coreagent.NewToolRegistry()
	}
	if c.Loop.Builder == nil && strings.TrimSpace(c.Loop.BuilderName) == "" {
		c.Loop.BuilderName = coreagent.DefaultPromptBuilderName
	}
	if c.Runtime.Now == nil {
		c.Runtime.Now = time.Now
	}
	if c.Runtime.IDGenerator == nil {
		c.Runtime.IDGenerator = coreagent.DefaultIDGenerator
	}
}

func (c *AgentConfig) validate() error {
	if c == nil {
		return fmt.Errorf("agent config is nil")
	}
	if c.LLM.Client == nil {
		return fmt.Errorf("llm client is required")
	}
	return nil
}

// runtimeConfig 为 Runner 使用的扁平配置（由 AgentConfig 映射而来）。
type runtimeConfig struct {
	name string
	ext  ExtConfig

	mainPromptBuilder coreagent.PromptBuilder
	subPromptBuilders []PromptSection
	staticPrompts     []StaticPromptSection
	workerPrompt      string
	modelPrompt       string
	occupationPrompt  string

	memory MemorySystem

	llmClient                coreagent.ChatClient
	providerName             string
	model                    string
	temperature              float64
	topP                     float64
	maxOutputTokens          int
	providerOptions          any
	emitter                  coreagent.Emitter
	tools                    *coreagent.ToolRegistry
	toolContextBuilder       coreagent.ToolContextBuilder
	toolExecutor             coreagent.ToolExecutor
	maxSteps                 int
	idGenerator              coreagent.IDGenerator
	now                      func() time.Time
	systemPromptPlacement    string
	configureRunner          func(*coreagent.Runner)
	compatibleActionSchemas  []coreagent.ActionSchema
	nativeOpenAIToolCalls    bool
	reasoningEffort          string
	textVerbosity            string
	enableEncryptedReasoning bool
	parallelToolCalls        bool
	promptCacheKey           string
	thinkingType             string
	enableThinking           *bool
	budgetTokens             *int
	stallWatchdogTimeout     time.Duration
	repeatedToolCallThreshold int
	onRepeatedToolCall       func(state *coreagent.RunState, toolName string, args map[string]interface{}, count int) error
	silentToolCallThreshold  int
	onSilentToolStreak       func(state *coreagent.RunState, count int) error
	onStallWatchdogToolCancelled func(state *coreagent.RunState, toolName, callID, reason string, elapsed time.Duration) error
	messageQueue             *taskqueue.Queue
	consumeSupplement        func(state *coreagent.RunState, item models.TaskMessageQueueItem) error
}

func agentConfigToRuntime(ac AgentConfig) (runtimeConfig, error) {
	ac.setDefaults()
	if err := ac.validate(); err != nil {
		return runtimeConfig{}, err
	}

	var mainPB coreagent.PromptBuilder
	if ac.Loop.Builder != nil {
		mainPB = ac.Loop.Builder
	} else {
		name := strings.TrimSpace(ac.Loop.BuilderName)
		if name == "" {
			name = coreagent.DefaultPromptBuilderName
		}
		mainPB = coreagent.MustCreatePromptBuilder(name, ac.Loop.Options)
	}

	return runtimeConfig{
		name:                     strings.TrimSpace(ac.Name),
		ext:                      ac.Ext,
		mainPromptBuilder:        mainPB,
		subPromptBuilders:        append([]PromptSection(nil), ac.Prompts.SubPromptBuilders...),
		staticPrompts:            append([]StaticPromptSection(nil), ac.Prompts.StaticPrompts...),
		workerPrompt:             ac.Prompts.WorkerPrompt,
		modelPrompt:              ac.Prompts.ModelPrompt,
		occupationPrompt:         ac.Prompts.OccupationPrompt,
		memory:                   ac.Memory,
		llmClient:                ac.LLM.Client,
		providerName:             strings.TrimSpace(ac.LLM.ProviderName),
		model:                    strings.TrimSpace(ac.LLM.Model),
		temperature:              ac.LLM.Temperature,
		topP:                     ac.LLM.TopP,
		maxOutputTokens:          ac.LLM.MaxOutputTokens,
		providerOptions:          ac.LLM.ProviderOptions,
		emitter:                  ac.Runtime.Emitter,
		tools:                    ac.Runtime.Tools,
		toolContextBuilder:       ac.Runtime.ToolContextBuilder,
		toolExecutor:             ac.Runtime.ToolExecutor,
		maxSteps:                 ac.Runtime.MaxSteps,
		idGenerator:              ac.Runtime.IDGenerator,
		now:                      ac.Runtime.Now,
		systemPromptPlacement:    coreagent.NormalizeSystemPromptPlacement(ac.Runtime.SystemPromptPlacement),
		configureRunner:          ac.Runtime.ConfigureRunner,
		compatibleActionSchemas:  append([]coreagent.ActionSchema(nil), ac.Runtime.CompatibleActionSchemas...),
		nativeOpenAIToolCalls:    ac.Runtime.NativeOpenAIToolCalls,
		reasoningEffort:          strings.TrimSpace(ac.Runtime.ReasoningEffort),
		textVerbosity:            strings.TrimSpace(ac.Runtime.TextVerbosity),
		enableEncryptedReasoning: ac.Runtime.EnableEncryptedReasoning,
		parallelToolCalls:        ac.Runtime.ParallelToolCalls,
		promptCacheKey:           strings.TrimSpace(ac.Runtime.PromptCacheKey),
		thinkingType:             strings.TrimSpace(ac.Runtime.ThinkingType),
		enableThinking:           ac.Runtime.EnableThinking,
		budgetTokens:             ac.Runtime.BudgetTokens,
		stallWatchdogTimeout:     ac.Runtime.StallWatchdogTimeout,
		repeatedToolCallThreshold: ac.Runtime.RepeatedToolCallThreshold,
		onRepeatedToolCall:       ac.Runtime.OnRepeatedToolCall,
		silentToolCallThreshold:  ac.Runtime.SilentToolCallThreshold,
		onSilentToolStreak:       ac.Runtime.OnSilentToolStreak,
		onStallWatchdogToolCancelled: ac.Runtime.OnStallWatchdogToolCancelled,
		messageQueue:             ac.Runtime.MessageQueue,
		consumeSupplement:        ac.Runtime.ConsumeSupplement,
	}, nil
}

// Config 保留旧版扁平字段布局，便于渐进迁移；新代码请使用 AgentConfig。
type Config struct {
	Name string

	MainPromptBuilder coreagent.PromptBuilder
	SubPromptBuilders []PromptSection
	StaticPrompts     []StaticPromptSection

	WorkerPrompt     string
	ModelPrompt      string
	OccupationPrompt string

	MemorySystem MemorySystem

	LLMClient          coreagent.ChatClient
	Emitter            coreagent.Emitter
	Tools              *coreagent.ToolRegistry
	ToolContextBuilder coreagent.ToolContextBuilder
	ToolExecutor       coreagent.ToolExecutor
	ConfigureRunner    func(*coreagent.Runner)

	ProviderName    string
	Model           string
	Temperature     float64
	TopP            float64
	MaxOutputTokens int
	ProviderOptions any
	MaxSteps        int

	IDGenerator           coreagent.IDGenerator
	Now                   func() time.Time
	SystemPromptPlacement string
	// NativeOpenAIToolCalls 见 RuntimeSettings。
	NativeOpenAIToolCalls    bool
	ReasoningEffort          string
	TextVerbosity            string
	EnableEncryptedReasoning bool
	ParallelToolCalls        bool
	PromptCacheKey           string
	ThinkingType             string
	EnableThinking           *bool
	BudgetTokens             *int
	Ext                      ExtConfig
}

func legacyConfigToAgentConfig(c Config) AgentConfig {
	return AgentConfig{
		Name: c.Name,
		Prompts: PromptSettings{
			WorkerPrompt:      c.WorkerPrompt,
			ModelPrompt:       c.ModelPrompt,
			OccupationPrompt:  c.OccupationPrompt,
			StaticPrompts:     c.StaticPrompts,
			SubPromptBuilders: c.SubPromptBuilders,
		},
		Loop: LoopPromptSettings{
			Builder: c.MainPromptBuilder,
			Options: coreagent.PromptBuilderOptions{},
		},
		Memory: c.MemorySystem,
		LLM: LLMSettings{
			ProviderName:    c.ProviderName,
			Model:           c.Model,
			Temperature:     c.Temperature,
			TopP:            c.TopP,
			MaxOutputTokens: c.MaxOutputTokens,
			ProviderOptions: c.ProviderOptions,
			Client:          c.LLMClient,
		},
		Runtime: RuntimeSettings{
			Emitter:                  c.Emitter,
			Tools:                    c.Tools,
			ToolContextBuilder:       c.ToolContextBuilder,
			ToolExecutor:             c.ToolExecutor,
			MaxSteps:                 c.MaxSteps,
			IDGenerator:              c.IDGenerator,
			Now:                      c.Now,
			SystemPromptPlacement:    c.SystemPromptPlacement,
			ConfigureRunner:          c.ConfigureRunner,
			NativeOpenAIToolCalls:    c.NativeOpenAIToolCalls,
			ReasoningEffort:          c.ReasoningEffort,
			TextVerbosity:            c.TextVerbosity,
			EnableEncryptedReasoning: c.EnableEncryptedReasoning,
			ParallelToolCalls:        c.ParallelToolCalls,
			PromptCacheKey:           c.PromptCacheKey,
			ThinkingType:             c.ThinkingType,
			EnableThinking:           c.EnableThinking,
			BudgetTokens:             c.BudgetTokens,
		},
		Ext: c.Ext,
	}
}

func legacyConfigToRuntime(c Config) (runtimeConfig, error) {
	return agentConfigToRuntime(legacyConfigToAgentConfig(c))
}
