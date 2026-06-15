package coreagent

import "strings"

// PromptToolMergeOptions 控制合并进提示词的工具列表。
type PromptToolMergeOptions struct {
	// ExcludeAnswer 为 true 时不并入 answer（原生 OpenAI：最终答复用 assistant 纯文本）。
	ExcludeAnswer bool
}

// MergePromptToolDefinitions 将 Worker 工具与内置控制类工具合并为提示词中的 <tools> 列表（去重时保留先出现的 Worker 定义）。
func MergePromptToolDefinitions(workerDefs []ToolDefinition, opt PromptToolMergeOptions) []ToolDefinition {
	out := make([]ToolDefinition, 0, len(workerDefs)+4)
	seen := map[string]struct{}{}
	for _, d := range workerDefs {
		n := strings.TrimSpace(d.Name)
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, d)
	}
	appendIfMissing := func(d ToolDefinition) {
		n := strings.TrimSpace(d.Name)
		if n == "" {
			return
		}
		if _, ok := seen[n]; ok {
			return
		}
		seen[n] = struct{}{}
		out = append(out, d)
	}
	if !opt.ExcludeAnswer {
		appendIfMissing(AnswerToolDefinition())
	}
	return out
}

// OmitAnswerToolDefinitions 返回去掉内置 answer 工具后的列表（用于原生 OpenAI tools：最终答复走纯文本，不注册 answer function）。
func OmitAnswerToolDefinitions(defs []ToolDefinition) []ToolDefinition {
	if len(defs) == 0 {
		return nil
	}
	out := make([]ToolDefinition, 0, len(defs))
	for _, d := range defs {
		if strings.TrimSpace(d.Name) == AnswerActionSchema.ActionName {
			continue
		}
		out = append(out, d)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// AnswerToolDefinition OpenAI / 兼容模式共用的 answer 工具（与 AnswerActionSchema.DataSchema 一致）。
func AnswerToolDefinition() ToolDefinition {
	schema, _ := AnswerActionSchema.DataSchema.(map[string]interface{})
	if schema == nil {
		schema = map[string]interface{}{"type": "object"}
	}
	return ToolDefinition{
		Name:        AnswerActionSchema.ActionName,
		Description: AnswerActionSchema.Description,
		Schema:      schema,
	}
}

// MessageToolDefinition 与 MessageActionSchema 一致的对象形态。
func MessageToolDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        MessageActionSchema.ActionName,
		Description: MessageActionSchema.Description,
		Schema:      messageToolSchemaCopy(),
	}
}

func messageToolSchemaCopy() map[string]interface{} {
	// MessageActionSchema.DataSchema 已是完整 object schema
	if m, ok := MessageActionSchema.DataSchema.(map[string]interface{}); ok {
		out := make(map[string]interface{}, len(m))
		for k, v := range m {
			out[k] = v
		}
		return out
	}
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}
