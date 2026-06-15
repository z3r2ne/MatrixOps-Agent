package provider

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"pkgs/db/models"
)

func FromAnthropicRequest(body map[string]interface{}) CommonRequest {
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
	if val, ok := body["stop_sequences"]; ok {
		req.Stop = val
	}
	if val, ok := body["stream"].(bool); ok {
		req.Stream = val
	}

	// TODO: Full implementation of FromAnthropicRequest if needed
	// (Current focus is on Client implementation: ToAnthropicRequest)

	return req
}

func ToAnthropicRequest(req CommonRequest) map[string]interface{} {
	body := map[string]interface{}{}
	if req.Model != "" {
		body["model"] = req.Model
	}
	if req.MaxTokens != 0 {
		body["max_tokens"] = req.MaxTokens
	} else {
		body["max_tokens"] = models.DefaultLLMMaxOutputTokens
	}
	if req.Temperature != 0 {
		body["temperature"] = req.Temperature
	}
	if req.TopP != 0 {
		body["top_p"] = req.TopP
	}
	if req.Stop != nil {
		body["stop_sequences"] = req.Stop
	}
	if req.Stream {
		body["stream"] = true
	}

	// System Messages
	var system []map[string]interface{}
	var msgsOut []map[string]interface{}
	if strings.TrimSpace(req.Instructions) != "" {
		system = append(system, map[string]interface{}{
			"type": "text",
			"text": strings.TrimSpace(req.Instructions),
		})
	}

	for _, m := range req.Messages {
		if m.Role == "system" {
			if s, ok := m.Content.(string); ok && s != "" {
				system = append(system, map[string]interface{}{"type": "text", "text": s})
			}
			continue
		}

		if m.Role == "user" {
			if s, ok := m.Content.(string); ok {
				msgsOut = append(msgsOut, map[string]interface{}{
					"role": "user",
					"content": []interface{}{
						map[string]interface{}{"type": "text", "text": s},
					},
				})
			} else if parts, ok := m.Content.([]CommonContentPart); ok {
				var newParts []interface{}
				for _, p := range parts {
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
			var content []interface{}
			if s, ok := m.Content.(string); ok && s != "" {
				content = append(content, map[string]interface{}{"type": "text", "text": s})
			}
			for _, tc := range m.ToolCalls {
				if tc.Type == "function" {
					var input interface{}
					// Try parsing JSON arguments
					if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
						input = tc.Function.Arguments
					}
					content = append(content, map[string]interface{}{
						"type":  "tool_use",
						"id":    tc.ID,
						"name":  tc.Function.Name,
						"input": input,
					})
				}
			}
			if len(content) > 0 {
				msgsOut = append(msgsOut, map[string]interface{}{"role": "assistant", "content": content})
			}
			continue
		}

		if m.Role == "tool" {
			msgsOut = append(msgsOut, map[string]interface{}{
				"role": "user",
				"content": []interface{}{
					map[string]interface{}{
						"type":        "tool_result",
						"tool_use_id": m.CallID,
						"content":     m.Content,
					},
				},
			})
			continue
		}
	}

	if len(system) > 0 {
		body["system"] = system
	}
	body["messages"] = msgsOut

	// Tools
	if len(req.Tools) > 0 {
		tools := make([]map[string]interface{}, 0, len(req.Tools))
		for _, t := range req.Tools {
			if t.Type == "function" {
				tools = append(tools, map[string]interface{}{
					"name":         t.Function.Name,
					"description":  t.Function.Description,
					"input_schema": t.Function.Parameters,
				})
			}
		}
		body["tools"] = tools
	}

	if req.ToolChoice != nil {
		// Simplified mapping
		if s, ok := req.ToolChoice.(string); ok {
			if s == "auto" {
				body["tool_choice"] = map[string]string{"type": "auto"}
			}
			if s == "required" {
				body["tool_choice"] = map[string]string{"type": "any"}
			}
		} else if m, ok := req.ToolChoice.(map[string]interface{}); ok {
			if fn, ok := m["function"].(map[string]interface{}); ok {
				if name, ok := fn["name"].(string); ok {
					body["tool_choice"] = map[string]interface{}{"type": "tool", "name": name}
				}
			}
		}
	}

	return body
}

func FromAnthropicChunk(chunk string) (CommonChunk, error) {
	lines := strings.Split(chunk, "\n")
	var dataLine string
	for _, l := range lines {
		if strings.HasPrefix(l, "data: ") {
			dataLine = l
			break
		}
	}
	if dataLine == "" {
		return CommonChunk{}, fmt.Errorf("no data line")
	}

	var jsonBody map[string]interface{}
	if err := json.Unmarshal([]byte(dataLine[6:]), &jsonBody); err != nil {
		return CommonChunk{}, err
	}

	out := CommonChunk{
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Choices: []CommonChunkChoice{},
	}

	// IDs from message object
	if msg, ok := jsonBody["message"].(map[string]interface{}); ok {
		out.ID = getString(msg, "id")
		out.Model = getString(msg, "model")
	}

	evtType := getString(jsonBody, "type")
	index := int(getFloat(jsonBody, "index"))

	if evtType == "content_block_start" {
		if cb, ok := jsonBody["content_block"].(map[string]interface{}); ok {
			cbType := getString(cb, "type")
			if cbType == "text" {
				out.Choices = append(out.Choices, CommonChunkChoice{
					Index: index,
					Delta: CommonChunkDelta{Role: "assistant", Content: ""},
				})
			} else if cbType == "tool_use" {
				out.Choices = append(out.Choices, CommonChunkChoice{
					Index: index,
					Delta: CommonChunkDelta{
						ToolCalls: []CommonChunkToolCall{{
							Index:    index,
							ID:       getString(cb, "id"),
							Type:     "function",
							Function: &CommonToolCallFunction{Name: getString(cb, "name")},
						}},
					},
				})
			}
		}
	} else if evtType == "content_block_delta" {
		if delta, ok := jsonBody["delta"].(map[string]interface{}); ok {
			dType := getString(delta, "type")
			if dType == "text_delta" {
				out.Choices = append(out.Choices, CommonChunkChoice{
					Index: index,
					Delta: CommonChunkDelta{Content: getString(delta, "text")},
				})
			} else if dType == "input_json_delta" {
				out.Choices = append(out.Choices, CommonChunkChoice{
					Index: index,
					Delta: CommonChunkDelta{
						ToolCalls: []CommonChunkToolCall{{
							Index:    index,
							Function: &CommonToolCallFunction{Arguments: getString(delta, "partial_json")},
						}},
					},
				})
			}
		}
	} else if evtType == "message_delta" {
		if delta, ok := jsonBody["delta"].(map[string]interface{}); ok {
			stopReason := getString(delta, "stop_reason")
			fr := ""
			if stopReason == "end_turn" {
				fr = "stop"
			} else if stopReason == "tool_use" {
				fr = "tool_calls"
			} else if stopReason == "max_tokens" {
				fr = "length"
			}

			out.Choices = append(out.Choices, CommonChunkChoice{
				Index:        0,
				Delta:        CommonChunkDelta{},
				FinishReason: fr,
			})
		}
	}

	return out, nil
}

// func getFloat(m map[string]interface{}, k string) float64 {
// 	if v, ok := m[k].(float64); ok {
// 		return v
// 	}
// 	return 0
// }
