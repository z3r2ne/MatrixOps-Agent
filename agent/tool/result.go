package tool

type Result struct {
	Content        string
	// Message 为发给大模型的 <system> 摘要（文件信息、错误说明、空输出提示等），与 Content 正文分离。
	Message        string
	FullContent    string
	IsError        bool
	Truncated      bool
	// PreserveFullOutput 为 true 时跳过统一输出截断，完整内容进入对话历史（如 load_skill）。
	PreserveFullOutput bool
	OutputPath     string
	Title          string
	Metadata       map[string]interface{}
	MemoryMetadata map[string]interface{}
	Vars           map[string]interface{}
	Name           string
}

func NewResult() *Result {
	return &Result{
		Vars:     make(map[string]interface{}),
		Metadata: make(map[string]interface{}),
	}
}
