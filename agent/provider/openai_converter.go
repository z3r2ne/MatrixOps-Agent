package provider

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// FromOpenAIRequest converts an arbitrary body map to CommonRequest
func FromOpenAIRequest(body map[string]interface{}) CommonRequest {
	req := CommonRequest{}

	if val, ok := body["model"].(string); ok {
		req.Model = val
	}
	if val, ok := body["max_tokens"].(float64); ok {
		req.MaxTokens = int(val)
	} else if val, ok := body["max_output_tokens"].(float64); ok {
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
	} else if val, ok := body["stop_sequences"]; ok {
		req.Stop = val
	}

	if val, ok := body["stream"].(bool); ok {
		req.Stream = val
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
				if name, ok := fn["name"].(string); ok {
					ct.Function.Name = name
				}
				if desc, ok := fn["description"].(string); ok {
					ct.Function.Description = desc
				}
				if params, ok := fn["parameters"].(map[string]interface{}); ok {
					ct.Function.Parameters = params
				}
				if strict, ok := fn["strict"].(bool); ok {
					ct.Function.Strict = strict
				}
			}
			req.Tools = append(req.Tools, ct)
		}
	}

	// Tool Choice
	if tc, ok := body["tool_choice"]; ok {
		if s, ok := tc.(string); ok {
			req.ToolChoice = s
		} else if m, ok := tc.(map[string]interface{}); ok {
			// Normalize tool choice object
			if typeStr, ok := m["type"].(string); ok && typeStr == "function" {
				if fn, ok := m["function"].(map[string]interface{}); ok {
					if name, ok := fn["name"].(string); ok {
						req.ToolChoice = map[string]interface{}{
							"type": "function",
							"function": map[string]interface{}{
								"name": name,
							},
						}
					}
				}
			}
		}
	}

	// Messages
	var inMsgs []interface{}
	if val, ok := body["input"].([]interface{}); ok {
		inMsgs = val
	} else if val, ok := body["messages"].([]interface{}); ok {
		inMsgs = val
	}

	for _, m := range inMsgs {
		msgMap, ok := m.(map[string]interface{})
		if !ok {
			continue
		}

		// Responses API items without role
		if _, hasRole := msgMap["role"]; !hasRole {
			if typeStr, ok := msgMap["type"].(string); ok {
				if typeStr == "function_call" {
					name := getString(msgMap, "name")
					argsVal := msgMap["arguments"]
					argsStr := ""
					if s, ok := argsVal.(string); ok {
						argsStr = s
					} else {
						b, _ := json.Marshal(argsVal)
						argsStr = string(b)
					}
					req.Messages = append(req.Messages, CommonMessage{
						Role: "assistant",
						ToolCalls: []CommonToolCall{{
							ID:   getString(msgMap, "id"),
							Type: "function",
							Function: CommonToolCallFunction{
								Name:      name,
								Arguments: argsStr,
							},
						}},
					})
				} else if typeStr == "function_call_output" {
					id := getString(msgMap, "call_id")
					out := msgMap["output"]
					content := ""
					if s, ok := out.(string); ok {
						content = s
					} else {
						b, _ := json.Marshal(out)
						content = string(b)
					}
					req.Messages = append(req.Messages, CommonMessage{
						Role:    "tool",
						CallID:  id,
						Content: content,
					})
				}
			}
			continue
		}

		role := getString(msgMap, "role")

		if role == "system" || role == "developer" {
			content := msgMap["content"]
			if s, ok := content.(string); ok && s != "" {
				req.Messages = append(req.Messages, CommonMessage{Role: "system", Content: s})
			} else if parts, ok := content.([]interface{}); ok {
				for _, p := range parts {
					pm, ok := p.(map[string]interface{})
					if !ok {
						continue
					}
					if txt, ok := pm["text"].(string); ok && txt != "" {
						req.Messages = append(req.Messages, CommonMessage{Role: "system", Content: txt})
						break
					}
				}
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
					if pType == "text" || pType == "input_text" {
						if txt, ok := pm["text"].(string); ok {
							newParts = append(newParts, CommonContentPart{Type: "text", Text: txt})
						}
					}
					// Image handling
					img := toOpenAIImage(pm)
					if img != nil {
						newParts = append(newParts, *img)
					}

					if pType == "tool_result" {
						id := getString(pm, "tool_call_id")
						c := pm["content"]
						cStr := ""
						if s, ok := c.(string); ok {
							cStr = s
						} else {
							b, _ := json.Marshal(c)
							cStr = string(b)
						}
						req.Messages = append(req.Messages, CommonMessage{Role: "tool", CallID: id, Content: cStr})
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
			content := msgMap["content"]
			if s, ok := content.(string); ok && s != "" {
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
							name := getString(fn, "name")
							argsVal := fn["arguments"]
							argsStr := ""
							if s, ok := argsVal.(string); ok {
								argsStr = s
							} else {
								b, _ := json.Marshal(argsVal)
								argsStr = string(b)
							}
							out.ToolCalls = append(out.ToolCalls, CommonToolCall{
								ID:   getString(tcm, "id"),
								Type: "function",
								Function: CommonToolCallFunction{
									Name:      name,
									Arguments: argsStr,
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

func toOpenAIImage(p map[string]interface{}) *CommonContentPart {
	if p == nil {
		return nil
	}
	pType := getString(p, "type")
	if pType == "image_url" {
		if urlData, ok := p["image_url"].(map[string]interface{}); ok {
			if url, ok := urlData["url"].(string); ok {
				return &CommonContentPart{Type: "image_url", ImageURL: &CommonImageURL{URL: url}}
			}
		}
	}
	if pType == "input_image" {
		// Responses API：image_url 为字符串；部分兼容实现仍用 {"url": "..."}
		if url, ok := p["image_url"].(string); ok && url != "" {
			return &CommonContentPart{Type: "image_url", ImageURL: &CommonImageURL{URL: url}}
		}
		if urlData, ok := p["image_url"].(map[string]interface{}); ok {
			if url, ok := urlData["url"].(string); ok {
				return &CommonContentPart{Type: "image_url", ImageURL: &CommonImageURL{URL: url}}
			}
		}
	}

	if s, ok := p["source"].(map[string]interface{}); ok {
		sType := getString(s, "type")
		if sType == "url" {
			if url, ok := s["url"].(string); ok {
				return &CommonContentPart{Type: "image_url", ImageURL: &CommonImageURL{URL: url}}
			}
		}
		if sType == "base64" {
			mediaType := getString(s, "media_type")
			data := getString(s, "data")
			if mediaType != "" && data != "" {
				url := fmt.Sprintf("data:%s;base64,%s", mediaType, data)
				return &CommonContentPart{Type: "image_url", ImageURL: &CommonImageURL{URL: url}}
			}
		}
	}
	return nil
}

const (
	InstructionTypeCodex   = "codex"
	InstructionTypeDefault = "default"
)

func modelToInstructionType(model string) string {
	if strings.HasSuffix(model, "-codex") {
		return InstructionTypeCodex
	}
	return InstructionTypeDefault
}

// ToOpenAIRequest converts CommonRequest back to OpenAI body
func ToOpenAIRequest(req CommonRequest) map[string]interface{} {
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
		body["instructions"] = strings.TrimSpace(req.Instructions)
	}

	// Tools
	if len(req.Tools) > 0 {
		tools := make([]map[string]interface{}, 0, len(req.Tools))
		for _, t := range req.Tools {
			if t.Type == "function" {
				fn := map[string]interface{}{
					"name": t.Function.Name,
					"type": "function",
				}
				if t.Function.Description != "" {
					fn["description"] = t.Function.Description
				}
				if t.Function.Parameters != nil {
					fn["parameters"] = t.Function.Parameters
				}
				if t.Function.Strict {
					fn["strict"] = true
				}
				tools = append(tools, fn)
			}
		}
		body["tools"] = tools
	}

	if req.ToolChoice != nil {
		body["tool_choice"] = req.ToolChoice
	}

	var input []interface{}

	for _, m := range req.Messages {
		if m.Role == "system" {
			instructionType := modelToInstructionType(req.Model)
			if instructionType == InstructionTypeDefault {
				if s, ok := m.Content.(string); ok {
					input = append(input, map[string]interface{}{"role": "system", "content": s, "type": "message"})
				}
				continue
			} else if instructionType == InstructionTypeCodex {
				if s, ok := m.Content.(string); ok {
					body["instructions"] = s
				}
			} else {
				if s, ok := m.Content.(string); ok {
					input = append(input, map[string]interface{}{"role": "system", "content": s, "type": "message"})
				}
				continue
			}

		}

		if m.Role == "user" {
			if s, ok := m.Content.(string); ok {
				input = append(input, map[string]interface{}{
					"role": "user",
					"content": []interface{}{
						map[string]interface{}{"type": "input_text", "text": s},
					},
					"type": "message",
				})
			} else if parts, ok := m.Content.([]CommonContentPart); ok {
				var newParts []interface{}
				for _, p := range parts {
					if p.Type == "text" {
						newParts = append(newParts, map[string]interface{}{"type": "input_text", "text": p.Text})
					} else if p.Type == "image_url" && p.ImageURL != nil {
						// Responses API要求 image_url 为 URL 字符串，不能是对象（否则会 invalid_type）
						newParts = append(newParts, map[string]interface{}{
							"type":      "input_image",
							"image_url": p.ImageURL.URL,
						})
					}
				}
				if len(newParts) > 0 {
					input = append(input, map[string]interface{}{"role": "user", "content": newParts, "type": "message"})
				}
			}
			continue
		}

		if m.Role == "assistant" {
			if s, ok := m.Content.(string); ok && s != "" {
				input = append(input, map[string]interface{}{
					"role": "assistant",
					"content": []interface{}{
						map[string]interface{}{"type": "output_text", "text": s},
					},
					"type": "message",
				})
			}
			if len(m.ToolCalls) > 0 {
				for _, tc := range m.ToolCalls {
					if tc.Type == "function" {
						input = append(input, map[string]interface{}{
							"type":      "function_call",
							"call_id":   tc.ID,
							"name":      tc.Function.Name,
							"arguments": tc.Function.Arguments,
						})
					}
				}
			}
			continue
		}

		if m.Role == "tool" {
			outStr := ""
			if s, ok := m.Content.(string); ok {
				outStr = s
			} else {
				outStr = fmt.Sprint(m.Content)
			}
			input = append(input, map[string]interface{}{
				"type":    "function_call_output",
				"call_id": m.CallID,
				"output":  outStr,
			})
			continue
		}
	}
	body["input"] = input

	// Other fields...
	if req.Include != nil {
		body["include"] = req.Include
	}
	if req.Truncation != nil {
		body["truncation"] = req.Truncation
	}
	if req.Metadata != nil {
		body["metadata"] = req.Metadata
	}
	if req.Store {
		body["store"] = true
	}
	if req.User != "" {
		body["user"] = req.User
	}

	// Default text verbosity and reasoning
	body["text"] = map[string]string{"verbosity": "medium"}
	if req.Model == "gpt-5-codex" {
		body["text"] = map[string]string{"verbosity": "medium"}
	}
	body["reasoning"] = map[string]string{"effort": "medium"}

	return body
}

func FromOpenAIChunk(chunk string) (CommonChunk, error) {
	parts := strings.Split(chunk, "\n")
	if len(parts) < 2 {
		return CommonChunk{}, fmt.Errorf("invalid chunk")
	}
	event := parts[0]
	data := parts[1]

	if !strings.HasPrefix(data, "data: ") {
		return CommonChunk{}, fmt.Errorf("invalid data prefix")
	}

	var jsonBody map[string]interface{}
	if err := json.Unmarshal([]byte(data[6:]), &jsonBody); err != nil {
		return CommonChunk{}, err
	}

	respObj, _ := jsonBody["response"].(map[string]interface{})
	if respObj == nil {
		respObj = jsonBody
	} // Fallback

	out := CommonChunk{
		ID:      getString(respObj, "id"),
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   getString(respObj, "model"),
		Choices: []CommonChunkChoice{},
	}
	if out.ID == "" {
		out.ID = getString(jsonBody, "id")
	}
	if out.Model == "" {
		out.Model = getString(jsonBody, "model")
	}

	evtName := strings.TrimPrefix(event, "event: ")
	evtName = strings.TrimSpace(evtName)

	if evtName == "response.output_text.delta" {
		d := ""
		if s, ok := jsonBody["delta"].(string); ok {
			d = s
		} else if s, ok := jsonBody["text"].(string); ok {
			d = s
		} else if s, ok := jsonBody["output_text_delta"].(string); ok {
			d = s
		}

		if d != "" {
			out.Choices = append(out.Choices, CommonChunkChoice{
				Index: 0,
				Delta: CommonChunkDelta{Content: d},
			})
		}
	}

	if evtName == "response.output_item.added" {
		item, _ := jsonBody["item"].(map[string]interface{})
		if item != nil && getString(item, "type") == "function_call" {
			name := getString(item, "name")
			id := getString(item, "id")
			if name != "" {
				out.Choices = append(out.Choices, CommonChunkChoice{
					Index: 0,
					Delta: CommonChunkDelta{
						ToolCalls: []CommonChunkToolCall{{
							Index:    0,
							ID:       id,
							Type:     "function",
							Function: &CommonToolCallFunction{Name: name, Arguments: ""},
						}},
					},
				})
			}
		}
	}

	if evtName == "response.function_call_arguments.delta" {
		a := ""
		if s, ok := jsonBody["delta"].(string); ok {
			a = s
		} else if s, ok := jsonBody["arguments_delta"].(string); ok {
			a = s
		}

		if a != "" {
			out.Choices = append(out.Choices, CommonChunkChoice{
				Index: 0,
				Delta: CommonChunkDelta{
					ToolCalls: []CommonChunkToolCall{{
						Index:    0,
						Function: &CommonToolCallFunction{Arguments: a},
					}},
				},
			})
		}
	}

	if evtName == "response.reasoning_summary_text.delta" {
		r := ""
		if s, ok := jsonBody["delta"].(string); ok {
			r = s
		}
		if r != "" {
			out.Choices = append(out.Choices, CommonChunkChoice{
				Index: 0,
				Delta: CommonChunkDelta{
					ReasoningContent: r,
				},
			})
		}
	}

	if evtName == "response.completed" {
		fr := ""
		sr := getString(respObj, "stop_reason")
		if sr == "" {
			sr = getString(jsonBody, "stop_reason")
		}

		if sr == "stop" {
			fr = "stop"
		} else if sr == "tool_call" || sr == "tool_calls" {
			fr = "tool_calls"
		} else if sr == "length" || sr == "max_output_tokens" {
			fr = "length"
		} else if sr == "content_filter" {
			fr = "content_filter"
		}
		if fr == "" {
			// Responses API 正常完成时有些实现不会返回 stop_reason，
			// 这里默认按一次正常 stop 处理，避免上层收不到 finish。
			fr = "stop"
		}

		out.Choices = append(out.Choices, CommonChunkChoice{
			Index:        0,
			Delta:        CommonChunkDelta{},
			FinishReason: fr,
		})

		// Usage handling in OpenAIHelper's parser
	}

	return out, nil
}

func getString(m map[string]interface{}, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}
