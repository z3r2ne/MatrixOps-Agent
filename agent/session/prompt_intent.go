package session

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"matrixops-agent/llm"
	database "pkgs/db"
)

// OutputSchema 定义输出的结构化 schema
type OutputSchema struct {
	Params []*OutputSchemaParam
}

// OutputSchemaParam 定义输出参数
type OutputSchemaParam struct {
	Name        string // 参数名称
	Type        string // 参数类型（如 string, number, boolean）
	Description string // 参数描述
	Required    bool   // 是否必需
}

// chatWithAI 与 AI 进行简单对话
// 支持结构化输出（通过 resultPrompt 指定）
func (r *AgentRunner) chatWithAI(runtimeConfig *RuntimeConfig, input string, resultPrompt *OutputSchema) (map[string]any, error) {
	// 构建用户消息
	userMessages := []*llm.ModelMessage{
		{
			Role:    "user",
			Content: input,
		},
	}

	// 如果提供了 resultPrompt，构建结构化输出的提示词
	if resultPrompt != nil && len(resultPrompt.Params) > 0 {
		schemaPrompt := "\n\n请以JSON格式返回结果，包含以下字段："
		for _, param := range resultPrompt.Params {
			requiredStr := ""
			if param.Required {
				requiredStr = "（必需）"
			}
			schemaPrompt += fmt.Sprintf("\n- %s (%s)%s: %s", param.Name, param.Type, requiredStr, param.Description)
		}
		userMessages[0].Content = input + schemaPrompt
	}

	modelOut := 0
	if runtimeConfig.ModelSettings != nil {
		modelOut = runtimeConfig.ModelSettings.OutputLimit
	}
	temperature := 0.0
	topP := 0.0
	if runtimeConfig.Worker != nil {
		if runtimeConfig.Worker.Temperature != nil {
			temperature = *runtimeConfig.Worker.Temperature
		}
		topP = runtimeConfig.Worker.TopP
	}
	// 调用 LLM（需要 LLM 客户端已配置）
	chatReq := llm.ChatRequest{
		Messages:        userMessages,
		ProviderOptions: runtimeConfig.LLMConfig,
		Model:           runtimeConfig.Model,
		Temperature:     temperature,
		TopP:            topP,
		MaxOutputTokens: database.EffectiveLLMMaxOutputTokens(r.db, modelOut),
	}

	response, err := runtimeConfig.LLMClient.Chat(chatReq)
	if err != nil {
		return nil, fmt.Errorf("chat with AI: %w", err)
	}

	// 提取响应文本
	text := strings.TrimSpace(renderContent(response.Message.Content))
	if text == "" {
		return nil, fmt.Errorf("chat with AI: empty response")
	}

	// 如果没有指定输出 schema，直接返回文本内容
	if resultPrompt == nil || len(resultPrompt.Params) == 0 {
		return map[string]any{"text": text}, nil
	}

	// 尝试从响应中提取 JSON
	result, err := extractJSON(text)
	if err != nil {
		// 如果解析失败，返回原始文本
		return map[string]any{"text": text}, nil
	}

	return result, nil
}

// extractJSON 从文本中提取 JSON 内容
// 支持多种格式：直接 JSON、JSON 代码块、{} 包裹的内容
func extractJSON(text string) (map[string]any, error) {
	result := make(map[string]any)

	// 尝试直接解析整个响应为 JSON
	if err := json.Unmarshal([]byte(text), &result); err == nil {
		return result, nil
	}

	// 如果直接解析失败，尝试提取 JSON 代码块
	jsonBlockRegex := regexp.MustCompile("(?s)```(?:json)?\n?(.*?)\n?```")
	matches := jsonBlockRegex.FindStringSubmatch(text)
	if len(matches) > 1 {
		jsonText := strings.TrimSpace(matches[1])
		if err := json.Unmarshal([]byte(jsonText), &result); err == nil {
			return result, nil
		}
	}

	// 如果仍然失败，尝试提取 {} 包裹的内容
	braceStart := strings.Index(text, "{")
	braceEnd := strings.LastIndex(text, "}")
	if braceStart >= 0 && braceEnd > braceStart {
		jsonText := text[braceStart : braceEnd+1]
		if err := json.Unmarshal([]byte(jsonText), &result); err == nil {
			return result, nil
		}
	}

	return nil, fmt.Errorf("failed to extract JSON from text")
}
