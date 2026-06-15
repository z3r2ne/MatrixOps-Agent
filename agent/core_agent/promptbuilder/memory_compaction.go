package promptbuilder

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"
)

const MemoryCompactionPromptBuilderName = "memory_compaction"

// MemoryCompactionSystemPrompt is the short system role for compaction LLM calls.
// Task instructions are appended to the user message (kimi-cli style).
const MemoryCompactionSystemPrompt = "你是一个帮助压缩对话上下文的助手。"

//go:embed templates/memory_compaction.tmpl
var memoryCompactionTemplate string

var memoryCompactionTmpl *template.Template

func init() {
	var err error
	memoryCompactionTmpl, err = template.New("memory_compaction").Parse(memoryCompactionTemplate)
	if err != nil {
		panic(fmt.Sprintf("failed to parse memory_compaction template: %v", err))
	}
	Register(MemoryCompactionPromptBuilderName, NewMemoryCompactionPromptBuilder)
}

func NewMemoryCompactionPromptBuilder(params map[string]interface{}) Builder {
	return func(input Input) (string, error) {
		data := map[string]interface{}{
			"Memory":            input.Memory,
			"ContextInfo":       input.ContextInfo,
			"UserInput":         input.UserInput,
			"WorkerExtraPrompt": input.WorkerExtraPrompt,
		}
		var buf bytes.Buffer
		if err := memoryCompactionTmpl.Execute(&buf, data); err != nil {
			return "", fmt.Errorf("execute template: %w", err)
		}
		return buf.String(), nil
	}
}
