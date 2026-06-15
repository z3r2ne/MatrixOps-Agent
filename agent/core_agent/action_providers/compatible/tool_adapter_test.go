package compatible

import (
	"testing"

	"matrixops.local/core_agent/streamtypes"
)

type mockChatClient struct {
	lastReq streamtypes.ChatRequest
}

func (m *mockChatClient) Chat(req streamtypes.ChatRequest) (streamtypes.ChatResponse, error) {
	m.lastReq = req
	return streamtypes.ChatResponse{
		Message: streamtypes.ModelMessage{
			Role:    "assistant",
			Content: `{"@action":"call_tool","data":{"name":"bash","params":{"command":"echo hello"}}}`,
		},
		Finish: "stop",
	}, nil
}

func TestToolPromptAdapterInjectsActionsAndTools(t *testing.T) {
	mock := &mockChatClient{}
	adapter := &ToolPromptAdapter{Inner: mock}

	tools := []streamtypes.ToolDefinition{
		{
			Name:        "bash",
			Description: "Run a shell command",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string"},
				},
			},
		},
	}
	actions := ToActionPromptSchemas(SessionActionSchemas(false))

	req := streamtypes.ChatRequest{
		Messages: []*streamtypes.ModelMessage{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "List files"},
		},
		Tools:         tools,
		ActionSchemas: actions,
	}

	_, err := adapter.Chat(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.lastReq.Tools != nil {
		t.Errorf("expected tools to be nil in forwarded request, got %d tools", len(mock.lastReq.Tools))
	}
	if len(mock.lastReq.ActionSchemas) != 0 {
		t.Errorf("expected action schemas to be cleared, got %d", len(mock.lastReq.ActionSchemas))
	}

	sysMsg := mock.lastReq.Messages[0]
	content, ok := sysMsg.Content.(string)
	if !ok {
		t.Fatalf("expected system content to be string")
	}
	if !contains(content, "<actions>") {
		t.Error("system prompt missing '<actions>'")
	}
	if !contains(content, "<name>call_tool</name>") {
		t.Error("system prompt missing call_tool action")
	}
	if !contains(content, "<tools>") {
		t.Error("system prompt missing '<tools>'")
	}
	if !contains(content, `"@action"`) {
		t.Error("system prompt missing @action format instruction")
	}
	if !contains(content, "You are helpful.") {
		t.Error("original system prompt not preserved")
	}
}

func TestToolPromptAdapterNoToolsPassthrough(t *testing.T) {
	mock := &mockChatClient{}
	adapter := &ToolPromptAdapter{Inner: mock}

	req := streamtypes.ChatRequest{
		Messages: []*streamtypes.ModelMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	_, err := adapter.Chat(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.lastReq.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(mock.lastReq.Messages))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || stringsContains(s, substr))
}

func stringsContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
