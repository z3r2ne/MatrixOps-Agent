package session

import (
	"context"
	"errors"
	"strings"

	coreagent "matrixops.local/core_agent"
	"matrixops-agent/llm"
	"matrixops-agent/provider"
)

func ToModelMessages(input []*WithParts) []*llm.ModelMessage {
	messages := []*llm.ModelMessage{}
	for _, msg := range input {
		switch msg.Info.Role {
		case RoleUser:
			content := buildUserContent(msg.Parts)
			if content == "" {
				continue
			}
			messages = append(messages, &llm.ModelMessage{Role: "user", Content: content})
		case RoleAssistant:
			if shouldSkipAssistant(msg) {
				continue
			}
			// content := buildAssistantContent(msg.Parts)
			// if content == "" {
			// 	continue
			// }
			// messages = append(messages, llm.ModelMessage{Role: "assistant", Content: content})
			modelMsgs := buildAssistantContentToMessages(msg.Parts)
			for _, modelMsg := range modelMsgs {
				modelMsg.Name = msg.Info.Worker
			}
			messages = append(messages, modelMsgs...)
		}
	}
	return messages
}

func buildUserContent(parts []*Part) string {
	out := []string{}
	for _, part := range parts {
		switch part.Type {
		case "text":
			if part.Ignored {
				continue
			}
			if part.Text != "" {
				out = append(out, part.Text)
			}
		case "file":
			if part.Mime == "text/plain" || part.Mime == "application/x-directory" {
				continue
			}
			if part.Filename != "" {
				out = append(out, "[file] "+part.Filename)
			} else if part.URL != "" {
				out = append(out, "[file] "+part.URL)
			}
		case "compaction":
			out = append(out, "What did we do so far?")
		case "subtask":
			out = append(out, "The following tool was executed by the user")
		}
	}
	return strings.Join(out, "\n")
}

func buildAssistantContentToMessages(parts []*Part) []*llm.ModelMessage {
	messages := []*llm.ModelMessage{}
	for _, part := range parts {
		switch part.Type {
		case "text":
			if part.Text != "" {
				messages = append(messages, &llm.ModelMessage{Role: "assistant", Content: part.Text})
			}
		case "tool":
			if part.Tool == nil {
				continue
			}
			messages = append(messages, &llm.ModelMessage{
				Role:       "assistant",
				ToolCallID: part.Tool.CallID,
				ToolCalls: []llm.ToolCall{
					{
						ID:        part.Tool.CallID,
						Name:      part.Tool.Name,
						Arguments: part.Tool.State.Input.(map[string]interface{}),
					},
				},
			})
			switch part.Tool.State.Status {
			case "completed":
				output := part.Tool.State.Output
				if part.Tool.State.Time.Compacted != 0 {
					output = "[Old tool result content cleared]"
				}
				resultContent := coreagent.BuildToolLLMContent(
					part.Tool.State.SystemMessage,
					output,
					false,
					part.Tool.State.Error,
				)
				if strings.TrimSpace(output) != "" || strings.TrimSpace(part.Tool.State.SystemMessage) != "" {
					messages = append(messages, &llm.ModelMessage{
						Role:       "tool",
						ToolCallID: part.Tool.CallID,
						Content:    resultContent,
					})
				}
			case "error":
				if part.Tool.State.Error != "" {
					resultContent := coreagent.BuildToolLLMContent(
						part.Tool.State.SystemMessage,
						part.Tool.State.Output,
						true,
						part.Tool.State.Error,
					)
					messages = append(messages, &llm.ModelMessage{
						Role:       "tool",
						ToolCallID: part.Tool.CallID,
						Content:    resultContent,
					})
				}
			case "pending", "running":
				messages = append(messages, &llm.ModelMessage{
					Role:       "tool",
					ToolCallID: part.Tool.CallID,
					Content:    "[Tool execution was interrupted]",
				})
			}
		case "reasoning":
			if part.Reasoning != "" {
				messages = append(messages, &llm.ModelMessage{Role: "assistant", Content: part.Reasoning})
			}
		}
	}
	return messages
}

func shouldSkipAssistant(msg *WithParts) bool {
	if msg.Info.Error == nil {
		return false
	}
	if msg.Info.Error.Name != "MessageAbortedError" {
		return false
	}
	for _, part := range msg.Parts {
		if part.Type != "step-start" && part.Type != "reasoning" {
			return false
		}
	}
	return true
}

func FromError(err error, providerID string) *MessageError {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, context.Canceled):
		return &MessageError{Name: "MessageAbortedError", Message: err.Error()}
	}
	switch typed := err.(type) {
	case *OutputLengthError:
		return &MessageError{Name: "MessageOutputLengthError", Message: typed.Error()}
	case *AuthError:
		return &MessageError{Name: "MessageAuthError", Message: typed.Message, ProviderID: typed.ProviderID}
	case *APIError:
		return &MessageError{
			Name:            "MessageAPIError",
			Message:         typed.Message,
			StatusCode:      typed.StatusCode,
			IsRetryable:     typed.IsRetryable,
			ResponseBody:    typed.ResponseBody,
			ResponseHeaders: typed.ResponseHeaders,
			Metadata:        typed.Metadata,
		}
	case *provider.APIError:
		return &MessageError{
			Name:            "MessageAPIError",
			Message:         typed.Message,
			ProviderID:      typed.ProviderID,
			StatusCode:      typed.StatusCode,
			IsRetryable:     typed.IsRetryable,
			ResponseBody:    typed.ResponseBody,
			ResponseHeaders: typed.ResponseHeaders,
		}
	default:
		return &MessageError{Name: "MessageUnknownError", Message: err.Error()}
	}
}
