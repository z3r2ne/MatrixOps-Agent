package openai_native

import (
	"encoding/json"
	"sort"
	"strings"

	agentprovider "matrixops-agent/provider"

	"pkgs/ansi"

	"matrixops.local/core_agent/streamtypes"

	"github.com/openai/openai-go/packages/param"
	openairesponses "github.com/openai/openai-go/responses"
)
func responseInputEasyMessageWithType[T string | openairesponses.ResponseInputMessageContentListParam](content T, role openairesponses.EasyInputMessageRole) openairesponses.ResponseInputItemUnionParam {
	item := openairesponses.ResponseInputItemParamOfMessage(content, role)
	if item.OfMessage != nil {
		item.OfMessage.Type = openairesponses.EasyInputMessageTypeMessage
	}
	return item
}
type openAIResponsesOutputMessageEnvelope struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Status  string `json:"status"`
	Phase   string `json:"phase"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}
func buildOpenAIResponsesInput(systemPrompt string, prompt string, parts []agentprovider.CommonContentPart, historyMessages []*streamtypes.ModelMessage) openairesponses.ResponseNewParamsInputUnion {
	systemPrompt = strings.TrimSpace(systemPrompt)
	content := make(openairesponses.ResponseInputMessageContentListParam, 0, len(parts)+1)
	if strings.TrimSpace(prompt) != "" {
		content = append(content, openairesponses.ResponseInputContentParamOfInputText(prompt))
	}
	for _, p := range parts {
		switch strings.TrimSpace(p.Type) {
		case "text":
			if strings.TrimSpace(p.Text) != "" {
				content = append(content, openairesponses.ResponseInputContentParamOfInputText(p.Text))
			}
		case "image_url":
			if p.ImageURL == nil || strings.TrimSpace(p.ImageURL.URL) == "" {
				continue
			}
			content = append(content, openairesponses.ResponseInputContentParamOfInputText("<image>"))
			part := openairesponses.ResponseInputImageParam{
				Detail: openairesponses.ResponseInputImageDetailAuto,
			}
			part.ImageURL = param.NewOpt(strings.TrimSpace(p.ImageURL.URL))
			content = append(content, openairesponses.ResponseInputContentUnionParam{OfInputImage: &part})
			content = append(content, openairesponses.ResponseInputContentParamOfInputText("</image>"))
		}
	}
	items := make(openairesponses.ResponseInputParam, 0, 2)
	if systemPrompt != "" {
		items = append(items, responseInputEasyMessageWithType(
			openairesponses.ResponseInputMessageContentListParam{
				openairesponses.ResponseInputContentParamOfInputText(systemPrompt),
			},
			openairesponses.EasyInputMessageRoleSystem,
		))
	}
	items = append(items, buildOpenAIResponsesHistoryItems(historyMessages)...)
	if len(content) > 0 {
		items = append(items, responseInputEasyMessageWithType(content, openairesponses.EasyInputMessageRoleUser))
	}
	return openairesponses.ResponseNewParamsInputUnion{
		OfInputItemList: items,
	}
}

func buildOpenAIResponsesHistoryItems(messages []*streamtypes.ModelMessage) openairesponses.ResponseInputParam {
	if len(messages) == 0 {
		return nil
	}

	items := make(openairesponses.ResponseInputParam, 0, len(messages))
	for i := 0; i < len(messages); i++ {
		message := messages[i]
		if message == nil {
			continue
		}
		for _, raw := range message.ResponsesReasoningItemRaws {
			if item := buildOpenAIResponsesReasoningHistoryItem(raw); item != nil {
				items = append(items, *item)
			}
		}
		role := strings.TrimSpace(message.Role)
		switch role {
		case "system":
			content := streamtypes.RenderMessageTextContent(message.Content)
			if content == "" {
				continue
			}
			items = append(items, responseInputEasyMessageWithType(content, openairesponses.EasyInputMessageRoleSystem))
		case "assistant":
			if len(message.ToolCalls) > 0 {
				txt := strings.TrimSpace(streamtypes.RenderMessageTextContent(message.Content))
				rc := strings.TrimSpace(message.ReasoningContent)
				if txt != "" || rc != "" {
					u := responseInputEasyMessageWithType(txt, openairesponses.EasyInputMessageRoleAssistant)
					if rc != "" && u.OfMessage != nil {
						u.OfMessage.SetExtraFields(map[string]any{"reasoning_content": rc})
					}
					items = append(items, u)
				}
				callIDs := make(map[string]struct{}, len(message.ToolCalls))
				for _, toolCall := range message.ToolCalls {
					if id := strings.TrimSpace(toolCall.ID); id != "" {
						callIDs[id] = struct{}{}
					}
				}
				consumed := 0
				for j := i + 1; j < len(messages); j++ {
					tm := messages[j]
					if tm == nil || strings.TrimSpace(tm.Role) != "tool" {
						break
					}
					tid := strings.TrimSpace(tm.ToolCallID)
					if tid == "" {
						break
					}
					if _, ok := callIDs[tid]; !ok {
						break
					}
					consumed++
				}
				toolOutByID := make(map[string]string, consumed)
				for k := 0; k < consumed; k++ {
					tm := messages[i+1+k]
					if tm == nil {
						continue
					}
					tid := strings.TrimSpace(tm.ToolCallID)
					toolOutByID[tid] = streamtypes.RenderMessageTextContent(tm.Content)
				}
				for _, toolCall := range message.ToolCalls {
					args, _ := json.Marshal(toolCall.Arguments)
					id := strings.TrimSpace(toolCall.ID)
					items = append(items, openairesponses.ResponseInputItemParamOfFunctionCall(string(args), id, strings.TrimSpace(toolCall.Name)))
					if out := toolOutByID[id]; out != "" {
						items = append(items, openairesponses.ResponseInputItemParamOfFunctionCallOutput(id, ansi.StripTerminal(out)))
					}
				}
				i += consumed
				continue
			}
			mergedOut := mergeReasoningContentIntoResponsesOutputMessageRaw(message.ResponsesOutputMessageRaw, message.ReasoningContent)
			if item := buildOpenAIResponsesOutputMessageHistoryItem(mergedOut); item != nil {
				items = append(items, *item)
				continue
			}
			content := streamtypes.RenderMessageTextContent(message.Content)
			if content == "" && strings.TrimSpace(message.ReasoningContent) == "" {
				continue
			}
			u := responseInputEasyMessageWithType(content, openairesponses.EasyInputMessageRoleAssistant)
			if rc := strings.TrimSpace(message.ReasoningContent); rc != "" && u.OfMessage != nil {
				u.OfMessage.SetExtraFields(map[string]any{"reasoning_content": rc})
			}
			items = append(items, u)
		case "tool":
			content := ansi.StripTerminal(streamtypes.RenderMessageTextContent(message.Content))
			id := strings.TrimSpace(message.ToolCallID)
			if content == "" || id == "" {
				continue
			}
			items = append(items, openairesponses.ResponseInputItemParamOfFunctionCallOutput(id, content))
		default:
			content := streamtypes.RenderMessageTextContent(message.Content)
			if content == "" {
				continue
			}
			items = append(items, responseInputEasyMessageWithType(content, openairesponses.EasyInputMessageRoleUser))
		}
	}
	return items
}

func buildOpenAIResponsesReasoningHistoryItem(raw string) *openairesponses.ResponseInputItemUnionParam {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(raw), &probe); err != nil || strings.TrimSpace(probe.Type) != "reasoning" {
		return nil
	}
	item := param.Override[openairesponses.ResponseReasoningItemParam](json.RawMessage(raw))
	return &openairesponses.ResponseInputItemUnionParam{OfReasoning: &item}
}

func buildOpenAIResponsesOutputMessageHistoryItem(raw string) *openairesponses.ResponseInputItemUnionParam {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(raw), &probe); err != nil || strings.TrimSpace(probe.Type) != "message" {
		return nil
	}
	item := param.Override[openairesponses.ResponseOutputMessageParam](json.RawMessage(raw))
	return &openairesponses.ResponseInputItemUnionParam{OfOutputMessage: &item}
}

func buildOpenAIResponsesTools(defs []streamtypes.ToolDefinition) []openairesponses.ToolUnionParam {
	if len(defs) == 0 {
		return nil
	}
	tools := make([]openairesponses.ToolUnionParam, 0, len(defs))
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
		fn := openairesponses.FunctionToolParam{
			Name:       name,
			Parameters: schema,
			Strict:     param.NewOpt(true),
		}
		if desc := strings.TrimSpace(def.Description); desc != "" {
			fn.Description = param.NewOpt(desc)
		}
		tools = append(tools, openairesponses.ToolUnionParam{OfFunction: &fn})
	}
	return tools
}

func normalizeOpenAIResponsesSchema(schema map[string]interface{}) map[string]interface{} {
	if len(schema) == 0 {
		return map[string]interface{}{
			"type":                 "object",
			"properties":           map[string]interface{}{},
			"additionalProperties": false,
		}
	}
	normalized, _ := normalizeOpenAIResponsesSchemaValue(schema).(map[string]interface{})
	if normalized == nil {
		return map[string]interface{}{
			"type":                 "object",
			"properties":           map[string]interface{}{},
			"additionalProperties": false,
		}
	}
	return normalized
}

func normalizeOpenAIResponsesSchemaValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(typed)+1)
		for k, v := range typed {
			out[k] = normalizeOpenAIResponsesSchemaValue(v)
		}
		if openAIResponsesObjectSchema(out) {
			props, _ := out["properties"].(map[string]interface{})
			requiredSet := map[string]struct{}{}
			for _, name := range openAIResponsesRequiredFields(out) {
				requiredSet[name] = struct{}{}
			}
			if len(props) > 0 {
				keys := make([]string, 0, len(props))
				for key, prop := range props {
					keys = append(keys, key)
					if _, ok := requiredSet[key]; !ok {
						props[key] = makeOpenAIResponsesSchemaNullable(prop)
					}
				}
				sort.Strings(keys)
				required := make([]interface{}, 0, len(keys))
				for _, key := range keys {
					required = append(required, key)
				}
				out["required"] = required
			} else if _, exists := out["required"]; !exists {
				out["required"] = []interface{}{}
			}
			out["additionalProperties"] = false
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(typed))
		for i, item := range typed {
			out[i] = normalizeOpenAIResponsesSchemaValue(item)
		}
		return out
	case []string:
		out := make([]string, len(typed))
		copy(out, typed)
		return out
	default:
		return value
	}
}

func makeOpenAIResponsesSchemaNullable(value interface{}) interface{} {
	schema, ok := value.(map[string]interface{})
	if !ok || len(schema) == 0 {
		return value
	}
	out := make(map[string]interface{}, len(schema)+1)
	for k, v := range schema {
		out[k] = v
	}
	switch typed := out["type"].(type) {
	case string:
		if strings.TrimSpace(typed) != "" && typed != "null" {
			out["type"] = []interface{}{typed, "null"}
		}
	case []interface{}:
		hasNull := false
		next := make([]interface{}, 0, len(typed)+1)
		for _, item := range typed {
			next = append(next, item)
			if s, ok := item.(string); ok && s == "null" {
				hasNull = true
			}
		}
		if !hasNull {
			next = append(next, "null")
		}
		out["type"] = next
	}
	if enumVals, ok := out["enum"].([]interface{}); ok {
		hasNull := false
		for _, item := range enumVals {
			if item == nil {
				hasNull = true
				break
			}
		}
		if !hasNull {
			out["enum"] = append(enumVals, nil)
		}
	}
	return out
}

func openAIResponsesObjectSchema(schema map[string]interface{}) bool {
	if schema == nil {
		return false
	}
	if typ, ok := schema["type"].(string); ok && strings.TrimSpace(typ) == "object" {
		return true
	}
	_, hasProps := schema["properties"].(map[string]interface{})
	return hasProps
}

func openAIResponsesRequiredFields(schema map[string]interface{}) []string {
	raw := schema["required"]
	switch typed := raw.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
func responseUsageToCoreUsage(u openairesponses.ResponseUsage) *streamtypes.Usage {
	if u.TotalTokens == 0 && u.InputTokens == 0 && u.OutputTokens == 0 {
		return nil
	}
	out := &streamtypes.Usage{
		InputTokens:     int(u.InputTokens),
		OutputTokens:    int(u.OutputTokens),
		ReasoningTokens: int(u.OutputTokensDetails.ReasoningTokens),
	}
	if u.InputTokensDetails.CachedTokens > 0 {
		out.CachedInputTokens = int(u.InputTokensDetails.CachedTokens)
	}
	return out
}
