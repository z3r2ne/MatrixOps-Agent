package tests

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"matrixops-agent/llm"
	"matrixops-agent/provider"
	"matrixops-agent/session"
	"matrixops-agent/tool"
)

type processV2SimpleTestTool struct{}

func (processV2SimpleTestTool) Name() string        { return "lookup_weather" }
func (processV2SimpleTestTool) VerbosName() string  { return "lookup_weather" }
func (processV2SimpleTestTool) Description() string { return "lookup weather" }
func (processV2SimpleTestTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"city": map[string]interface{}{"type": "string"},
		},
	}
}
func (processV2SimpleTestTool) Execute(ctx tool.Context, input map[string]interface{}) (tool.Result, error) {
	return tool.Result{
		Name:    "lookup_weather",
		Content: "Shanghai is sunny",
	}, nil
}

func TestRunProcessV2SimpleWithoutDB(t *testing.T) {
	registry := tool.NewRegistry()
	registry.Register(processV2SimpleTestTool{})

	client := provider.NewMockClient(
		provider.MockResponse{
			Text: `{"@action":"call_tool","data":{"tool_name":"lookup_weather","tool_input":{"city":"Shanghai"}}}`,
		},
		provider.MockResponse{
			Text: `{"@action":"answer","data":"Shanghai is sunny today."}`,
		},
	)

	eventCount := 0
	result, err := session.RunProcessV2Simple(session.ProcessV2SimpleInput{
		Context:      context.Background(),
		Client:       client,
		Model:        "mock-model",
		SystemPrompt: "You are helpful.",
		UserInput:    "What is the weather in Shanghai?",
		Tools:        registry,
		OnEvent: func(name string, payload interface{}) {
			eventCount++
		},
	})
	if err != nil {
		t.Fatalf("RunProcessV2Simple returned error: %v", err)
	}

	if result.Answer != "Shanghai is sunny today." {
		t.Fatalf("unexpected answer: %q", result.Answer)
	}
	if result.Memory == nil || len(result.Memory.History) < 4 {
		t.Fatalf("expected in-memory history to be built, got %#v", result.Memory)
	}
	if eventCount == 0 {
		t.Fatal("expected callback events to be emitted")
	}
}

func TestRunProcessV2Simple_PromptIncludesTaskLoopGuidance(t *testing.T) {
	registry := tool.NewRegistry()
	registry.Register(processV2SimpleTestTool{})

	client := provider.NewMockClient(
		provider.MockResponse{
			Text: `{"@action":"answer","data":"done"}`,
		},
	)

	_, err := session.RunProcessV2Simple(session.ProcessV2SimpleInput{
		Context:      context.Background(),
		Client:       client,
		Model:        "mock-model",
		SystemPrompt: "You are helpful.",
		UserInput:    "say done",
		Tools:        registry,
	})
	if err != nil {
		t.Fatalf("RunProcessV2Simple returned error: %v", err)
	}

	if client.LastRequest == nil {
		t.Fatalf("expected prompt request to be captured, got nil")
	}

	promptText := collectChatRequestPromptText(client.LastRequest)
	if !strings.Contains(promptText, "当前这轮只是整个任务中的一步") {
		t.Fatalf("expected prompt to include task loop guidance, got:\n%s", promptText)
	}
}

func collectChatRequestPromptText(req *llm.ChatRequest) string {
	if req == nil {
		return ""
	}
	var parts []string
	for _, msg := range req.Messages {
		if msg == nil {
			continue
		}
		switch v := msg.Content.(type) {
		case string:
			parts = append(parts, v)
		default:
			parts = append(parts, fmt.Sprint(v))
		}
	}
	return strings.Join(parts, "\n")
}
