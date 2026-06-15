package compatible

import (
	"encoding/json"
	"fmt"
	"strings"

	"matrixops.local/core_agent/streamtypes"
)

// ToolPromptAdapter wraps a ChatClient that does NOT support the tools field,
// injecting action schemas and worker tool definitions into the system prompt
// so the model can respond with JSON action envelopes ({"@action":"...","data":{...}}).
type ToolPromptAdapter struct {
	Inner streamtypes.ChatClient
}

var _ streamtypes.ChatClient = (*ToolPromptAdapter)(nil)
var _ streamtypes.StreamChatClient = (*ToolPromptAdapter)(nil)
var _ streamtypes.StreamChatClientWithOptions = (*ToolPromptAdapter)(nil)

func (a *ToolPromptAdapter) Chat(req streamtypes.ChatRequest) (streamtypes.ChatResponse, error) {
	req = a.injectPrompt(req)
	return a.Inner.Chat(req)
}

func (a *ToolPromptAdapter) StreamChat(req streamtypes.ChatRequest) (<-chan streamtypes.StreamEvent, error) {
	req = a.injectPrompt(req)
	if s, ok := a.Inner.(streamtypes.StreamChatClient); ok {
		return s.StreamChat(req)
	}
	return syncToStream(a.Inner, req), nil
}

func (a *ToolPromptAdapter) StreamChatWithOptions(req streamtypes.ChatRequest, opts ...streamtypes.StreamChatOption) (<-chan streamtypes.StreamEvent, error) {
	req = a.injectPrompt(req)
	if s, ok := a.Inner.(streamtypes.StreamChatClientWithOptions); ok {
		return s.StreamChatWithOptions(req, opts...)
	}
	if s, ok := a.Inner.(streamtypes.StreamChatClient); ok {
		return s.StreamChat(req)
	}
	return syncToStream(a.Inner, req), nil
}

func (a *ToolPromptAdapter) injectPrompt(req streamtypes.ChatRequest) streamtypes.ChatRequest {
	if len(req.Tools) == 0 && len(req.ActionSchemas) == 0 {
		return req
	}

	toolsPrompt := buildCompatiblePrompt(req.ActionSchemas, req.Tools)

	msgs := make([]*streamtypes.ModelMessage, 0, len(req.Messages)+1)
	injected := false
	for _, m := range req.Messages {
		if !injected && m.Role == "system" {
			clone := *m
			if s, ok := clone.Content.(string); ok {
				clone.Content = s + "\n\n" + toolsPrompt
			} else {
				clone.Content = toolsPrompt
			}
			msgs = append(msgs, &clone)
			injected = true
			continue
		}
		msgs = append(msgs, m)
	}
	if !injected {
		msgs = append([]*streamtypes.ModelMessage{{
			Role:    "system",
			Content: toolsPrompt,
		}}, msgs...)
	}

	req.Messages = msgs
	req.Tools = nil
	req.ActionSchemas = nil
	return req
}

func buildCompatiblePrompt(actions []streamtypes.ActionPromptSchema, tools []streamtypes.ToolDefinition) string {
	var b strings.Builder

	if len(actions) > 0 {
		b.WriteString("<actions>\n")
		for _, action := range actions {
			b.WriteString(fmt.Sprintf("<action>\n<name>%s</name>\n", action.ActionName))
			if action.Description != "" {
				b.WriteString(fmt.Sprintf("<description>%s</description>\n", action.Description))
			}
			if action.DataSchema != nil {
				if schema, ok := action.DataSchema.(map[string]interface{}); ok && len(schema) > 0 {
					encoded, _ := json.MarshalIndent(schema, "", "  ")
					b.WriteString("<data_schema>\n")
					b.Write(encoded)
					b.WriteString("\n</data_schema>\n")
				}
			}
			b.WriteString("</action>\n")
		}
		b.WriteString("</actions>\n\n")
	}

	if len(tools) > 0 {
		b.WriteString("<tools>\n")
		for _, tool := range tools {
			b.WriteString(fmt.Sprintf("<tool>\n<name>%s</name>\n", tool.Name))
			if tool.Description != "" {
				b.WriteString(fmt.Sprintf("<description>%s</description>\n", tool.Description))
			}
			if len(tool.Schema) > 0 {
				schema, _ := json.MarshalIndent(tool.Schema, "", "  ")
				b.WriteString("<schema>\n")
				b.Write(schema)
				b.WriteString("\n</schema>\n")
			}
			b.WriteString("</tool>\n")
		}
		b.WriteString("</tools>\n\n")
	}

	b.WriteString("<output_format>\n")
	b.WriteString("<requirement>你必须严格按照以下 JSON 格式输出（每个对象对应一次 action 调用）：</requirement>\n")
	b.WriteString("<format>\n{\n  \"@action\": \"<上方 <actions> 列表中的 action 名称>\",\n  \"data\": { }\n}\n</format>\n")
	b.WriteString("<important_notes>\n")
	b.WriteString("- 你的输出必须是有效的 JSON；`@action` 为 action 名，`data` 为该 action 在 data_schema 中定义的参数对象\n")
	b.WriteString("- 调用工具时使用 `@action\":\"call_tool\"`，`data` 为 `{\"name\":\"<tools> 中的工具名>\",\"params\":{...}}`\n")
	b.WriteString("- 当前这轮只是整个任务中的一步；工具执行结果会在后续轮次继续回传给你\n")
	b.WriteString("- 可连续输出多个 JSON 对象，每个对象是一次独立的 action\n")

	hasAnswer := false
	for _, action := range actions {
		if action.ActionName == "answer" {
			hasAnswer = true
			break
		}
	}
	if hasAnswer {
		b.WriteString("- 若本轮还有任何 action 需要执行，不要使用 `answer`\n")
		b.WriteString("- 仅当本轮不需要任何 action，且任务已完成或必须等待用户输入时，使用 `@action\":\"answer\"`，`data` 为 `{\"content\":\"...\"}`\n")
		b.WriteString("- 调用 `answer` 后本任务循环停止\n")
	}

	b.WriteString("- 顶层仅允许 `@action` 与 `data`\n")
	b.WriteString("- 不要输出 JSON 以外的文字\n")
	b.WriteString("</important_notes>\n")
	b.WriteString("</output_format>")

	return b.String()
}

// syncToStream converts a synchronous Chat call into a channel of StreamEvents.
func syncToStream(client streamtypes.ChatClient, req streamtypes.ChatRequest) <-chan streamtypes.StreamEvent {
	ch := make(chan streamtypes.StreamEvent, 2)
	go func() {
		defer close(ch)
		resp, err := client.Chat(req)
		if err != nil {
			ch <- streamtypes.StreamEvent{Type: "error", Error: err}
			return
		}
		if text := streamtypes.RenderContent(resp.Message.Content); text != "" {
			ch <- streamtypes.StreamEvent{Type: "text-delta", Text: text}
		}
		finish := resp.Finish
		if finish == "" {
			finish = "stop"
		}
		ch <- streamtypes.StreamEvent{Type: "finish", Finish: finish, Usage: resp.Usage}
	}()
	return ch
}
