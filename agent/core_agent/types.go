package coreagent

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

const (
	PartTypeText               = "text"
	PartTypeReasoning          = "reasoning"
	PartTypeTool               = "tool"
	PartTypeFinish             = "finish"
	PartTypeError              = "error"
	PartTypeStartStep          = "start-step"
	PartTypeFinishStep         = "finish-step"
	PartTypeTextDelta          = "text-delta"
	PartTypeReasoningDelta     = "reasoning-delta"
	PartTypeToolDelta          = "tool-delta"
	PartTypeCompaction         = "compaction"
	PartTypeMemoryOrganization = "memory-organization"
	PartTypePatch              = "patch"
)

type Message struct {
	ID         string          `json:"id"`
	SessionID  string          `json:"sessionID"`
	Role       Role            `json:"role"`
	ParentID   string          `json:"parentID,omitempty"`
	Occupation string          `json:"occupation,omitempty"`
	Worker     string          `json:"worker,omitempty"`
	Provider   string          `json:"provider,omitempty"`
	Model      string          `json:"model,omitempty"`
	ProviderID string          `json:"providerID,omitempty"`
	ModelID    string          `json:"modelID,omitempty"`
	System     string          `json:"system,omitempty"`
	Tools      map[string]bool `json:"tools,omitempty"`
	Variant    string          `json:"variant,omitempty"`
	Summary    interface{}     `json:"summary,omitempty"`
	Finish     string          `json:"finish,omitempty"`
	Cost       float64         `json:"cost,omitempty"`
	Memory     any             `json:"memory,omitempty"`
	Tokens     *MessageTokens  `json:"tokens,omitempty"`
	Error      *MessageError   `json:"error,omitempty"`
	Path       *MessagePath    `json:"path,omitempty"`
	Time       MessageTime     `json:"time"`
	State      string          `json:"state,omitempty"`
	Snapshot   string          `json:"snapshot,omitempty"`
	Phase      string          `json:"phase,omitempty"`
	ResponsesOutputMessageRaw  string   `json:"responsesOutputMessageRaw,omitempty"`
	ResponsesReasoningItemRaws []string `json:"responsesReasoningItemRaws,omitempty"`
}

type MessagePath struct {
	Cwd  string `json:"cwd"`
	Root string `json:"root"`
}

type MessageTime struct {
	Created   int64 `json:"created"`
	Completed int64 `json:"completed,omitempty"`
}

type Part struct {
	ID          string                 `json:"id"`
	MessageID   string                 `json:"messageID"`
	SessionID   string                 `json:"sessionID"`
	Type        string                 `json:"type"`
	Text        string                 `json:"text,omitempty"`
	Tool        *ToolPart              `json:"tool,omitempty"`
	Reasoning   string                 `json:"reasoning,omitempty"`
	Synthetic   bool                   `json:"synthetic,omitempty"`
	Ignored     bool                   `json:"ignored,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Snapshot    string                 `json:"snapshot,omitempty"`
	Hash        string                 `json:"hash,omitempty"`
	Files       []string               `json:"files,omitempty"`
	Mime        string                 `json:"mime,omitempty"`
	Filename    string                 `json:"filename,omitempty"`
	Path        string                 `json:"path,omitempty"`
	InputSource string                 `json:"inputSource,omitempty"`
	URL         string                 `json:"url,omitempty"`
	Source      interface{}            `json:"source,omitempty"`
	AgentName   string                 `json:"name,omitempty"`
	Auto        bool                   `json:"auto,omitempty"`
	Description string                 `json:"description,omitempty"`
	Subagent    string                 `json:"agent,omitempty"`
	Model       *ModelRef              `json:"model,omitempty"`
	Command     string                 `json:"command,omitempty"`
	Attempt     int                    `json:"attempt,omitempty"`
	Error       *MessageError          `json:"error,omitempty"`
	Reason      string                 `json:"reason,omitempty"`
	Cost        float64                `json:"cost,omitempty"`
	Tokens      *MessageTokens         `json:"tokens,omitempty"`
	Time        *PartTime              `json:"time,omitempty"`
}

type PartTime struct {
	Start     int64 `json:"start,omitempty"`
	End       int64 `json:"end,omitempty"`
	Created   int64 `json:"created,omitempty"`
	Compacted int64 `json:"compacted,omitempty"`
}

type ToolPart struct {
	Name     string                 `json:"tool"`
	CallID   string                 `json:"callID"`
	State    ToolState              `json:"state"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type ToolState struct {
	Status         string                 `json:"status"`
	Input          interface{}            `json:"input,omitempty"`
	Raw            string                 `json:"raw,omitempty"`
	Title          string                 `json:"title,omitempty"`
	SystemMessage  string                 `json:"systemMessage,omitempty"`
	Output         string                 `json:"output,omitempty"`
	Error          string                 `json:"error,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	Attachments    []Part                 `json:"attachments,omitempty"`
	Time           PartTime               `json:"time"`
	MemoryMetadata map[string]interface{} `json:"-"`
	FullOutput     string                 `json:"-"`
}

type MessageTokens struct {
	Input     int        `json:"input"`
	Output    int        `json:"output"`
	Reasoning int        `json:"reasoning"`
	Cache     TokenCache `json:"cache"`
}

type TokenCache struct {
	Read  int `json:"read"`
	Write int `json:"write"`
}

type MessageError struct {
	Name            string            `json:"name"`
	Message         string            `json:"message,omitempty"`
	ProviderID      string            `json:"providerID,omitempty"`
	StatusCode      int               `json:"statusCode,omitempty"`
	IsRetryable     bool              `json:"isRetryable,omitempty"`
	ResponseBody    string            `json:"responseBody,omitempty"`
	ResponseHeaders map[string]string `json:"responseHeaders,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

type ModelRef struct {
	ProviderID string `json:"providerID,omitempty"`
	ModelID    string `json:"modelID,omitempty"`
	Name       string `json:"name,omitempty"`
}
