package openai_native

import (
	"encoding/json"
	"strings"

	agentprovider "matrixops-agent/provider"

	"matrixops.local/core_agent/streamtypes"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/shared"
)
func buildOpenAIChatCompletionMessages(
	systemPrompt string,
	instruction string,
	prompt string,
	parts []agentprovider.CommonContentPart,
	historyMessages []*streamtypes.ModelMessage,
) []openai.ChatCompletionMessageParamUnion {
	messages := make([]openai.ChatCompletionMessageParamUnion, 0, len(historyMessages)+2)

	switch {
	case strings.TrimSpace(instruction) != "":
		messages = append(messages, openai.DeveloperMessage(strings.TrimSpace(instruction)))
	case strings.TrimSpace(systemPrompt) != "":
		messages = append(messages, openai.SystemMessage(strings.TrimSpace(systemPrompt)))
	}

	messages = append(messages, buildOpenAIChatCompletionHistoryMessages(historyMessages)...)

	if userMessage := buildOpenAIChatCompletionUserMessage(prompt, parts); userMessage != nil {
		messages = append(messages, *userMessage)
	} else if len(messages) == 0 {
		messages = append(messages, openai.UserMessage(""))
	}
	return messages
}

func buildOpenAIChatCompletionHistoryMessages(historyMessages []*streamtypes.ModelMessage) []openai.ChatCompletionMessageParamUnion {
	if len(historyMessages) == 0 {
		return nil
	}

	out := make([]openai.ChatCompletionMessageParamUnion, 0, len(historyMessages))
	for _, message := range historyMessages {
		if message == nil {
			continue
		}
		role := strings.TrimSpace(message.Role)
		switch role {
		case "system":
			content := streamtypes.RenderMessageTextContent(message.Content)
			if content != "" {
				out = append(out, openai.SystemMessage(content))
			}
		case "assistant":
			rc := strings.TrimSpace(message.ReasoningContent)
			if len(message.ToolCalls) > 0 {
				assistant := openai.ChatCompletionAssistantMessageParam{}
				assistant.ToolCalls = buildOpenAIChatCompletionToolCalls(message.ToolCalls)
				if content := streamtypes.RenderMessageTextContent(message.Content); strings.TrimSpace(content) != "" {
					assistant.Content.OfString = param.NewOpt(strings.TrimSpace(content))
				}
				if rc != "" {
					assistant.SetExtraFields(map[string]any{"reasoning_content": rc})
				}
				out = append(out, openai.ChatCompletionMessageParamUnion{OfAssistant: &assistant})
				continue
			}
			content := streamtypes.RenderMessageTextContent(message.Content)
			if content == "" && rc == "" {
				break
			}
			if content != "" && rc == "" {
				out = append(out, openai.ChatCompletionMessageParamOfAssistant(content))
				break
			}
			assistant := openai.ChatCompletionAssistantMessageParam{}
			if content != "" {
				assistant.Content.OfString = param.NewOpt(content)
			}
			if rc != "" {
				assistant.SetExtraFields(map[string]any{"reasoning_content": rc})
			}
			out = append(out, openai.ChatCompletionMessageParamUnion{OfAssistant: &assistant})
		case "tool":
			callID := strings.TrimSpace(message.ToolCallID)
			if callID == "" {
				continue
			}
			if parts, ok := message.Content.([]agentprovider.CommonContentPart); ok && len(parts) > 0 {
				contentParts := make([]openai.ChatCompletionContentPartTextParam, 0, len(parts))
				for _, part := range parts {
					if strings.TrimSpace(part.Type) != "text" || strings.TrimSpace(part.Text) == "" {
						continue
					}
					contentParts = append(contentParts, openai.ChatCompletionContentPartTextParam{
						Text: strings.TrimSpace(part.Text),
						Type: "text",
					})
				}
				if len(contentParts) > 0 {
					out = append(out, openai.ChatCompletionMessageParamUnion{
						OfTool: &openai.ChatCompletionToolMessageParam{
							Role:       "tool",
							ToolCallID: callID,
							Content: openai.ChatCompletionToolMessageParamContentUnion{
								OfArrayOfContentParts: contentParts,
							},
						},
					})
				}
				continue
			}
			content := streamtypes.RenderMessageTextContent(message.Content)
			if content != "" {
				out = append(out, openai.ToolMessage(content, callID))
			}
		default:
			if userMessage := buildOpenAIChatCompletionUserMessage(streamtypes.RenderMessageTextContent(message.Content), nil); userMessage != nil {
				out = append(out, *userMessage)
			}
		}
	}
	return out
}

func buildOpenAIChatCompletionUserMessage(prompt string, parts []agentprovider.CommonContentPart) *openai.ChatCompletionMessageParamUnion {
	contentParts := make([]openai.ChatCompletionContentPartUnionParam, 0, len(parts)+1)
	if strings.TrimSpace(prompt) != "" {
		contentParts = append(contentParts, openai.TextContentPart(strings.TrimSpace(prompt)))
	}
	for _, p := range parts {
		switch strings.TrimSpace(p.Type) {
		case "text":
			if strings.TrimSpace(p.Text) != "" {
				contentParts = append(contentParts, openai.TextContentPart(strings.TrimSpace(p.Text)))
			}
		case "image_url":
			if p.ImageURL == nil || strings.TrimSpace(p.ImageURL.URL) == "" {
				continue
			}
			contentParts = append(contentParts, openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
				URL:    strings.TrimSpace(p.ImageURL.URL),
				Detail: "auto",
			}))
		}
	}
	if len(contentParts) == 0 {
		if strings.TrimSpace(prompt) == "" {
			return nil
		}
		msg := openai.UserMessage(strings.TrimSpace(prompt))
		return &msg
	}
	if len(contentParts) == 1 && strings.TrimSpace(prompt) == "" && contentParts[0].OfText != nil {
		msg := openai.UserMessage(strings.TrimSpace(contentParts[0].OfText.Text))
		return &msg
	}
	msg := openai.UserMessage(contentParts)
	return &msg
}

func buildOpenAIChatCompletionTools(defs []streamtypes.ToolDefinition) []openai.ChatCompletionToolParam {
	if len(defs) == 0 {
		return nil
	}
	tools := make([]openai.ChatCompletionToolParam, 0, len(defs))
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
		schema = normalizeOpenAIResponsesSchema(schema)
		tools = append(tools, openai.ChatCompletionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        name,
				Description: param.NewOpt(strings.TrimSpace(def.Description)),
				Parameters:  shared.FunctionParameters(schema),
				Strict:      param.NewOpt(true),
			},
		})
	}
	return tools
}

func buildOpenAIChatCompletionToolCalls(calls []streamtypes.ToolCall) []openai.ChatCompletionMessageToolCallParam {
	if len(calls) == 0 {
		return nil
	}
	out := make([]openai.ChatCompletionMessageToolCallParam, 0, len(calls))
	for _, call := range calls {
		args, _ := json.Marshal(call.Arguments)
		out = append(out, openai.ChatCompletionMessageToolCallParam{
			ID: strings.TrimSpace(call.ID),
			Function: openai.ChatCompletionMessageToolCallFunctionParam{
				Name:      strings.TrimSpace(call.Name),
				Arguments: string(args),
			},
		})
	}
	return out
}

func chatCompletionUsageToCoreUsage(u openai.CompletionUsage) *streamtypes.Usage {
	if u.PromptTokens == 0 && u.CompletionTokens == 0 && u.TotalTokens == 0 {
		return nil
	}
	out := &streamtypes.Usage{
		InputTokens:       int(u.PromptTokens),
		OutputTokens:      int(u.CompletionTokens),
		ReasoningTokens:   int(u.CompletionTokensDetails.ReasoningTokens),
		CachedInputTokens: int(u.PromptTokensDetails.CachedTokens),
	}
	if out.CachedInputTokens > 0 {
		out.InputTokens -= out.CachedInputTokens
		if out.InputTokens < 0 {
			out.InputTokens = 0
		}
	}
	return out
}
func buildOpenAIUserMessageContent(prompt string, parts []agentprovider.CommonContentPart) openai.ChatCompletionUserMessageParamContentUnion {
	if len(parts) == 0 {
		return openai.ChatCompletionUserMessageParamContentUnion{
			OfString: openai.String(prompt),
		}
	}
	oaParts := make([]openai.ChatCompletionContentPartUnionParam, 0, len(parts)+1)
	if strings.TrimSpace(prompt) != "" {
		oaParts = append(oaParts, openai.TextContentPart(prompt))
	}
	for _, p := range parts {
		switch strings.TrimSpace(p.Type) {
		case "text":
			if strings.TrimSpace(p.Text) != "" {
				oaParts = append(oaParts, openai.TextContentPart(p.Text))
			}
		case "image_url":
			if p.ImageURL != nil && strings.TrimSpace(p.ImageURL.URL) != "" {
				oaParts = append(oaParts, openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
					URL: p.ImageURL.URL,
				}))
			}
		}
	}
	if len(oaParts) == 0 {
		return openai.ChatCompletionUserMessageParamContentUnion{
			OfString: openai.String(prompt),
		}
	}
	if len(oaParts) == 1 && strings.TrimSpace(prompt) == "" {
		switch {
		case oaParts[0].OfText != nil:
			return openai.ChatCompletionUserMessageParamContentUnion{
				OfString: openai.String(oaParts[0].OfText.Text),
			}
		}
	}
	return openai.ChatCompletionUserMessageParamContentUnion{
		OfArrayOfContentParts: oaParts,
	}
}

func completionUsageToCoreUsage(u openai.CompletionUsage) *streamtypes.Usage {
	if u.TotalTokens == 0 && u.PromptTokens == 0 && u.CompletionTokens == 0 {
		return nil
	}
	out := &streamtypes.Usage{
		InputTokens:     int(u.PromptTokens),
		OutputTokens:    int(u.CompletionTokens),
		ReasoningTokens: int(u.CompletionTokensDetails.ReasoningTokens),
	}
	if u.PromptTokensDetails.CachedTokens > 0 {
		out.CachedInputTokens = int(u.PromptTokensDetails.CachedTokens)
	}
	return out
}
