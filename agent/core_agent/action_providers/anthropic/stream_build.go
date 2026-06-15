package anthropic

import (
	"encoding/json"
	"fmt"
	"strings"

	agentprovider "matrixops-agent/provider"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"pkgs/db/models"

	"matrixops.local/core_agent/streamtypes"
)

func toolInputSchemaFromMap(schema map[string]interface{}) (anthropic.ToolInputSchemaParam, error) {
	if len(schema) == 0 {
		schema = map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}
	}
	b, err := json.Marshal(schema)
	if err != nil {
		return anthropic.ToolInputSchemaParam{}, err
	}
	var out anthropic.ToolInputSchemaParam
	if err := json.Unmarshal(b, &out); err != nil {
		return anthropic.ToolInputSchemaParam{}, err
	}
	return out, nil
}

func buildAnthropicTools(defs []streamtypes.ToolDefinition) ([]anthropic.ToolUnionParam, error) {
	if len(defs) == 0 {
		return nil, nil
	}
	out := make([]anthropic.ToolUnionParam, 0, len(defs))
	for _, def := range defs {
		name := strings.TrimSpace(def.Name)
		if name == "" {
			continue
		}
		schema := def.Schema
		if len(schema) == 0 {
			schema = map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}
		}
		inputSchema, err := toolInputSchemaFromMap(schema)
		if err != nil {
			return nil, fmt.Errorf("anthropic tools: tool %q schema: %w", name, err)
		}
		tool := anthropic.ToolParam{
			Name:        name,
			InputSchema: inputSchema,
			Strict:      param.NewOpt(true),
		}
		if d := strings.TrimSpace(def.Description); d != "" {
			tool.Description = param.NewOpt(d)
		}
		out = append(out, anthropic.ToolUnionParam{OfTool: &tool})
	}
	return out, nil
}

func buildAnthropicSystemBlocks(systemPrompt, instruction string) []anthropic.TextBlockParam {
	var blocks []anthropic.TextBlockParam
	if t := strings.TrimSpace(instruction); t != "" {
		blocks = append(blocks, anthropic.TextBlockParam{Text: t})
	}
	if t := strings.TrimSpace(systemPrompt); t != "" {
		blocks = append(blocks, anthropic.TextBlockParam{Text: t})
	}
	return blocks
}

func buildAnthropicHistoryMessages(history []*streamtypes.ModelMessage) ([]anthropic.MessageParam, error) {
	if len(history) == 0 {
		return nil, nil
	}
	var out []anthropic.MessageParam
	var pendingToolResults []anthropic.ContentBlockParamUnion

	flushToolResults := func() {
		if len(pendingToolResults) == 0 {
			return
		}
		out = append(out, anthropic.NewUserMessage(pendingToolResults...))
		pendingToolResults = nil
	}

	for _, message := range history {
		if message == nil {
			continue
		}
		role := strings.TrimSpace(message.Role)
		switch role {
		case "system":
			// System prompts are merged into top-level System by the caller.
			continue
		case "assistant":
			flushToolResults()
			blocks := make([]anthropic.ContentBlockParamUnion, 0, 2+len(message.ToolCalls))
			sig := strings.TrimSpace(message.ThinkingSignature)
			thinking := strings.TrimSpace(message.ReasoningContent)
			if thinking != "" || sig != "" {
				blocks = append(blocks, anthropic.NewThinkingBlock(sig, thinking))
			}
			if text := strings.TrimSpace(streamtypes.RenderMessageTextContent(message.Content)); text != "" {
				blocks = append(blocks, anthropic.NewTextBlock(text))
			}
			for _, tc := range message.ToolCalls {
				if strings.TrimSpace(tc.Name) == "" || strings.TrimSpace(tc.ID) == "" {
					continue
				}
				argsJSON, err := json.Marshal(tc.Arguments)
				if err != nil {
					return nil, fmt.Errorf("anthropic history: marshal tool %q args: %w", tc.Name, err)
				}
				blocks = append(blocks, anthropic.NewToolUseBlock(tc.ID, json.RawMessage(argsJSON), tc.Name))
			}
			if len(blocks) == 0 {
				continue
			}
			out = append(out, anthropic.NewAssistantMessage(blocks...))
		case "tool":
			toolUseID := strings.TrimSpace(message.ToolCallID)
			content := streamtypes.RenderMessageTextContent(message.Content)
			if toolUseID == "" {
				continue
			}
			pendingToolResults = append(pendingToolResults, anthropic.NewToolResultBlock(toolUseID, content, false))
		default:
			flushToolResults()
			prompt, imageParts := splitModelMessageContent(message.Content)
			if msg, err := buildAnthropicUserMessage(prompt, imageParts); err != nil {
				return nil, err
			} else if len(msg.Content) > 0 {
				out = append(out, msg)
			}
		}
	}
	flushToolResults()
	return out, nil
}

func buildAnthropicUserMessage(prompt string, parts []agentprovider.CommonContentPart) (anthropic.MessageParam, error) {
	var blocks []anthropic.ContentBlockParamUnion
	if t := strings.TrimSpace(prompt); t != "" {
		blocks = append(blocks, anthropic.NewTextBlock(t))
	}
	for _, p := range parts {
		switch strings.TrimSpace(p.Type) {
		case "text":
			if strings.TrimSpace(p.Text) != "" {
				blocks = append(blocks, anthropic.NewTextBlock(strings.TrimSpace(p.Text)))
			}
		case "image_url":
			if p.ImageURL == nil || strings.TrimSpace(p.ImageURL.URL) == "" {
				continue
			}
			blocks = append(blocks, anthropic.NewImageBlock(anthropic.URLImageSourceParam{
				URL: strings.TrimSpace(p.ImageURL.URL),
			}))
		}
	}
	if len(blocks) == 0 {
		return anthropic.MessageParam{}, nil
	}
	return anthropic.NewUserMessage(blocks...), nil
}

func splitModelMessageContent(content interface{}) (string, []agentprovider.CommonContentPart) {
	switch typed := content.(type) {
	case string:
		return strings.TrimSpace(typed), nil
	case []agentprovider.CommonContentPart:
		var textParts []string
		var imageParts []agentprovider.CommonContentPart
		for _, part := range typed {
			switch strings.TrimSpace(part.Type) {
			case "text":
				if t := strings.TrimSpace(part.Text); t != "" {
					textParts = append(textParts, t)
				}
			case "image_url":
				if part.ImageURL != nil && strings.TrimSpace(part.ImageURL.URL) != "" {
					imageParts = append(imageParts, part)
				}
			}
		}
		return strings.Join(textParts, "\n"), imageParts
	default:
		return strings.TrimSpace(streamtypes.RenderMessageTextContent(content)), nil
	}
}

func buildAnthropicMessageParams(
	input streamtypes.StreamInput,
	model anthropic.Model,
	maxTokens int64,
) (anthropic.MessageNewParams, error) {
	history, err := buildAnthropicHistoryMessages(input.HistoryMessages)
	if err != nil {
		return anthropic.MessageNewParams{}, err
	}
	messages := make([]anthropic.MessageParam, 0, len(history)+1)
	messages = append(messages, history...)
	userMsg, err := buildAnthropicUserMessage(input.Prompt, input.UserContentParts)
	if err != nil {
		return anthropic.MessageNewParams{}, err
	}
	if len(userMsg.Content) > 0 {
		messages = append(messages, userMsg)
	} else if len(messages) == 0 {
		messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock("")))
	}

	tools, err := buildAnthropicTools(input.Tools)
	if err != nil {
		return anthropic.MessageNewParams{}, err
	}

	params := anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: maxTokens,
		Messages:  messages,
		System:    buildAnthropicSystemBlocks(input.SystemPrompt, input.Instruction),
		Tools:     tools,
	}
	if input.Temperature != 0 {
		params.Temperature = param.NewOpt(input.Temperature)
	}
	if input.TopP != 0 {
		params.TopP = param.NewOpt(input.TopP)
	}
	if !input.ParallelToolCalls {
		params.ToolChoice = anthropic.ToolChoiceUnionParam{
			OfAuto: &anthropic.ToolChoiceAutoParam{
				DisableParallelToolUse: param.NewOpt(true),
			},
		}
	}
	if anthropicShouldSendThinkingConfig(input, input.HistoryMessages) && input.BudgetTokens != nil {
		params.Thinking = anthropic.ThinkingConfigParamOfEnabled(int64(*input.BudgetTokens))
	}
	return params, nil
}

func anthropicShouldSendThinkingConfig(input streamtypes.StreamInput, history []*streamtypes.ModelMessage) bool {
	if historyNeedsAnthropicThinkingReplay(history) {
		return true
	}
	if input.EnableThinking != nil && *input.EnableThinking {
		return true
	}
	return models.NormalizeLLMThinkingType(input.ThinkingType) == models.LLMThinkingTypeEnabled
}

func historyNeedsAnthropicThinkingReplay(history []*streamtypes.ModelMessage) bool {
	for _, m := range history {
		if m == nil || strings.TrimSpace(m.Role) != "assistant" {
			continue
		}
		if strings.TrimSpace(m.ReasoningContent) != "" || strings.TrimSpace(m.ThinkingSignature) != "" {
			return true
		}
	}
	return false
}

func messageDeltaUsageToStreamUsage(u anthropic.MessageDeltaUsage) *streamtypes.Usage {
	if u.InputTokens == 0 && u.OutputTokens == 0 && u.CacheCreationInputTokens == 0 && u.CacheReadInputTokens == 0 {
		return nil
	}
	return &streamtypes.Usage{
		InputTokens:       int(u.InputTokens + u.CacheCreationInputTokens),
		OutputTokens:      int(u.OutputTokens),
		CachedInputTokens: int(u.CacheReadInputTokens),
	}
}
