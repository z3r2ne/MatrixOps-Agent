package semreg

// Tier 表示语义回归层级。
type Tier string

const (
	TierL0 Tier = "L0"
	TierL1 Tier = "L1"
	TierL2 Tier = "L2"
)

// Kind 表示场景执行方式。
type Kind string

const (
	KindPromptRender Kind = "prompt_render"
	KindTaskRunner   Kind = "task_runner"
	KindBehavior     Kind = "behavior"
	KindSemantic     Kind = "semantic"
)

// Scenario 定义一条语义回归用例。
type Scenario struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Tier        Tier   `yaml:"tier"`
	Kind        Kind   `yaml:"kind"`

	PromptRender *PromptRenderSpec `yaml:"prompt_render,omitempty"`
	TaskRunner   *TaskRunnerSpec   `yaml:"task_runner,omitempty"`
	Behavior     *BehaviorSpec     `yaml:"behavior,omitempty"`
	Semantic     *SemanticSpec     `yaml:"semantic,omitempty"`
	Assert       AssertSpec        `yaml:"assert"`
}

// PromptRenderSpec 通过 RenderV2TaskPrompt 做 L0 结构回归（无 LLM）。
type PromptRenderSpec struct {
	GlobalPrompt         string         `yaml:"global_prompt"`
	UserInput            string         `yaml:"user_input"`
	ToolNames            []string       `yaml:"tool_names"`
	History              []HistoryEntry `yaml:"history"`
	ContextLimit         int            `yaml:"context_limit"`
	ContextCurrentTokens int            `yaml:"context_current_tokens"`
	ContextCurrentBytes  int            `yaml:"context_current_bytes"`
}

type HistoryEntry struct {
	Role    string `yaml:"role"`
	Content string `yaml:"content"`
}

// TaskRunnerSpec 通过 mock LLM 跑完整 task，检查首轮 prompt 结构。
type TaskRunnerSpec struct {
	TaskInput  string `yaml:"task_input"`
	WorkerName string `yaml:"worker_name"`
}

// AssertSpec 结构断言（L0）。
type AssertSpec struct {
	SystemPromptContains    []string `yaml:"system_prompt_contains"`
	SystemPromptNotContains []string `yaml:"system_prompt_not_contains"`
	UserInputEquals         string   `yaml:"user_input_equals"`
	TaskCompletes           bool     `yaml:"task_completes"`
}

// BehaviorSpec L1 行为回归：真实 LLM 跑 worker，对比 tool trace 基线。
type BehaviorSpec struct {
	Worker       string `yaml:"worker"`
	Prompt       string `yaml:"prompt"`
	PromptFile   string `yaml:"prompt_file"`
	BaselineFile string `yaml:"baseline_file"`
	WorkDirEnv   string `yaml:"work_dir_env"`
	WorkspaceEnv string `yaml:"workspace_id_env"`
	ProjectEnv   string `yaml:"project_id_env"`
	TimeoutSec   int    `yaml:"timeout_sec"`
}

// SemanticSpec L2 语义回归：主任务 + verification judge。
type SemanticSpec struct {
	ReuseScenario string `yaml:"reuse_scenario"`
	TaskInput     string `yaml:"task_input"`
	VerifyPrompt  string `yaml:"verify_prompt"`
	ExpectStatus  string `yaml:"expect_status"`
}
