package provider

import (
	"encoding/json"
	"fmt"
	"strings"
)

func FromOaCompatibleRequest(body map[string]interface{}) CommonRequest {
	req := CommonRequest{}

	if val, ok := body["model"].(string); ok {
		req.Model = val
	}
	if val, ok := body["max_tokens"].(float64); ok {
		req.MaxTokens = int(val)
	}
	if val, ok := body["temperature"].(float64); ok {
		req.Temperature = val
	}
	if val, ok := body["top_p"].(float64); ok {
		req.TopP = val
	}
	if val, ok := body["stop"]; ok {
		req.Stop = val
	}
	if val, ok := body["stream"].(bool); ok {
		req.Stream = val
	}
	if val, ok := body["tool_choice"]; ok {
		req.ToolChoice = val
	}

	// Tools
	if tools, ok := body["tools"].([]interface{}); ok {
		for _, t := range tools {
			tm, ok := t.(map[string]interface{})
			if !ok {
				continue
			}
			ct := CommonTool{}
			if typeStr, ok := tm["type"].(string); ok {
				ct.Type = typeStr
			}
			if fn, ok := tm["function"].(map[string]interface{}); ok {
				ct.Function.Name = getString(fn, "name")
				ct.Function.Description = getString(fn, "description")
				if params, ok := fn["parameters"].(map[string]interface{}); ok {
					ct.Function.Parameters = params
				}
			}
			req.Tools = append(req.Tools, ct)
		}
	}

	// Messages
	var inMsgs []interface{}
	if val, ok := body["messages"].([]interface{}); ok {
		inMsgs = val
	}

	for _, m := range inMsgs {
		msgMap, ok := m.(map[string]interface{})
		if !ok {
			continue
		}
		role := getString(msgMap, "role")

		if role == "system" {
			if s, ok := msgMap["content"].(string); ok && s != "" {
				req.Messages = append(req.Messages, CommonMessage{Role: "system", Content: s})
			}
			continue
		}

		if role == "user" {
			content := msgMap["content"]
			if s, ok := content.(string); ok {
				req.Messages = append(req.Messages, CommonMessage{Role: "user", Content: s})
			} else if parts, ok := content.([]interface{}); ok {
				var newParts []CommonContentPart
				for _, p := range parts {
					pm, ok := p.(map[string]interface{})
					if !ok {
						continue
					}
					pType := getString(pm, "type")
					if pType == "text" {
						if txt, ok := pm["text"].(string); ok {
							newParts = append(newParts, CommonContentPart{Type: "text", Text: txt})
						}
					}
					if pType == "image_url" {
						if urlData, ok := pm["image_url"].(map[string]interface{}); ok {
							if url, ok := urlData["url"].(string); ok {
								newParts = append(newParts, CommonContentPart{Type: "image_url", ImageURL: &CommonImageURL{URL: url}})
							}
						}
					}
				}
				if len(newParts) > 0 {
					if len(newParts) == 1 && newParts[0].Type == "text" {
						req.Messages = append(req.Messages, CommonMessage{Role: "user", Content: newParts[0].Text})
					} else {
						req.Messages = append(req.Messages, CommonMessage{Role: "user", Content: newParts})
					}
				}
			}
			continue
		}

		if role == "assistant" {
			out := CommonMessage{Role: "assistant"}
			if s, ok := msgMap["content"].(string); ok {
				out.Content = s
			}
			if tcs, ok := msgMap["tool_calls"].([]interface{}); ok {
				for _, tc := range tcs {
					tcm, ok := tc.(map[string]interface{})
					if !ok {
						continue
					}
					if getString(tcm, "type") == "function" {
						if fn, ok := tcm["function"].(map[string]interface{}); ok {
							out.ToolCalls = append(out.ToolCalls, CommonToolCall{
								ID:   getString(tcm, "id"),
								Type: "function",
								Function: CommonToolCallFunction{
									Name:      getString(fn, "name"),
									Arguments: getString(fn, "arguments"),
								},
							})
						}
					}
				}
			}
			req.Messages = append(req.Messages, out)
			continue
		}

		if role == "tool" {
			req.Messages = append(req.Messages, CommonMessage{
				Role:    "tool",
				CallID:  getString(msgMap, "tool_call_id"),
				Content: msgMap["content"],
			})
			continue
		}
	}

	return req
}

func ToOaCompatibleRequest(req CommonRequest) map[string]interface{} {
	body := map[string]interface{}{
		"model": req.Model,
	}
	if req.MaxTokens != 0 {
		body["max_tokens"] = req.MaxTokens
	}
	if req.Temperature != 0 {
		body["temperature"] = req.Temperature
	}
	if req.TopP != 0 {
		body["top_p"] = req.TopP
	}
	if req.Stop != nil {
		body["stop"] = req.Stop
	}
	if req.Stream {
		body["stream"] = true
	}
	if strings.TrimSpace(req.Instructions) != "" {
		body["messages"] = []interface{}{
			map[string]interface{}{
				"role":    "system",
				"content": strings.TrimSpace(req.Instructions),
			},
		}
	}
	if req.ToolChoice != nil {
		body["tool_choice"] = req.ToolChoice
	}

	// Format passthrough
	// if req.ResponseFormat != nil ...

	// Tools
	if len(req.Tools) > 0 {
		tools := make([]map[string]interface{}, 0, len(req.Tools))
		for _, t := range req.Tools {
			if t.Type == "function" {
				tools = append(tools, map[string]interface{}{
					"type": "function",
					"function": map[string]interface{}{
						"name":        t.Function.Name,
						"description": t.Function.Description,
						"parameters":  t.Function.Parameters,
					},
				})
			}
		}
		body["tools"] = tools
	}

	var msgsOut []interface{}
	if seeded, ok := body["messages"].([]interface{}); ok && len(seeded) > 0 {
		msgsOut = append(msgsOut, seeded...)
	}

	for _, m := range req.Messages {
		if m.Role == "system" {
			if s, ok := m.Content.(string); ok && s != "" {
				msgsOut = append(msgsOut, map[string]interface{}{"role": "system", "content": s})
			}
			continue
		}

		if m.Role == "user" {
			switch content := SimplifyTextOnlyContent(m.Content).(type) {
			case string:
				if content != "" {
					msgsOut = append(msgsOut, map[string]interface{}{"role": "user", "content": content})
				}
			case []CommonContentPart:
				var newParts []interface{}
				for _, p := range content {
					if p.Type == "text" {
						newParts = append(newParts, map[string]interface{}{"type": "text", "text": p.Text})
					} else if p.Type == "image_url" && p.ImageURL != nil {
						newParts = append(newParts, map[string]interface{}{
							"type":      "image_url",
							"image_url": map[string]interface{}{"url": p.ImageURL.URL},
						})
					}
				}
				if len(newParts) > 0 {
					msgsOut = append(msgsOut, map[string]interface{}{"role": "user", "content": newParts})
				}
			}
			continue
		}

		if m.Role == "assistant" {
			out := map[string]interface{}{"role": "assistant"}
			if s, ok := m.Content.(string); ok {
				out["content"] = s
			}
			if len(m.ToolCalls) > 0 {
				var tcs []interface{}
				for _, tc := range m.ToolCalls {
					if tc.Type == "function" {
						tcs = append(tcs, map[string]interface{}{
							"type": "function",
							"id":   tc.ID,
							"function": map[string]interface{}{
								"name":      tc.Function.Name,
								"arguments": tc.Function.Arguments,
							},
						})
					}
				}
				out["tool_calls"] = tcs
			}
			msgsOut = append(msgsOut, out)
			continue
		}

		if m.Role == "tool" {
			msgsOut = append(msgsOut, map[string]interface{}{
				"role":         "tool",
				"tool_call_id": m.CallID,
				"content":      m.Content,
			})
			continue
		}
	}
	body["messages"] = msgsOut
	return body
}

func FromOaCompatibleChunk(chunk string) (CommonChunk, error) {
	if !strings.HasPrefix(chunk, "data: ") {
		return CommonChunk{}, fmt.Errorf("invalid chunk")
	}

	var jsonBody map[string]interface{}
	if err := json.Unmarshal([]byte(chunk[6:]), &jsonBody); err != nil {
		return CommonChunk{}, err
	}

	choices, ok := jsonBody["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return CommonChunk{}, fmt.Errorf("no choices")
	}

	out := CommonChunk{
		ID:      getString(jsonBody, "id"),
		Object:  "chat.completion.chunk",
		Created: int64(getFloat(jsonBody, "created")),
		Model:   getString(jsonBody, "model"),
		Choices: []CommonChunkChoice{},
	}

	choice0 := choices[0].(map[string]interface{})
	index := int(getFloat(choice0, "index"))
	delta, _ := choice0["delta"].(map[string]interface{})

	outChoice := CommonChunkChoice{
		Index:        index,
		Delta:        CommonChunkDelta{},
		FinishReason: getString(choice0, "finish_reason"),
	}

	if delta != nil {
		if content, ok := delta["content"].(string); ok {
			outChoice.Delta.Content = content
		}
		if role, ok := delta["role"].(string); ok {
			outChoice.Delta.Role = role
		}
		if tcs, ok := delta["tool_calls"].([]interface{}); ok {
			for _, tc := range tcs {
				if tcm, ok := tc.(map[string]interface{}); ok {
					ctc := CommonChunkToolCall{
						Index: int(getFloat(tcm, "index")),
						ID:    getString(tcm, "id"),
						Type:  getString(tcm, "type"),
					}
					if fn, ok := tcm["function"].(map[string]interface{}); ok {
						ctc.Function = &CommonToolCallFunction{
							Name:      getString(fn, "name"),
							Arguments: getString(fn, "arguments"),
						}
					}
					outChoice.Delta.ToolCalls = append(outChoice.Delta.ToolCalls, ctc)
				}
			}
		}
	}

	out.Choices = append(out.Choices, outChoice)

	// Usage is handled by helper parser, but can be extracted here too if needed
	// ...

	return out, nil
}
