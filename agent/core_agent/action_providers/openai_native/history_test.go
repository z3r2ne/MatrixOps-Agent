package openai_native

import (
	"encoding/json"
	"strings"
	"testing"

	"matrixops.local/core_agent/streamtypes"
)

func TestBuildOpenAIResponsesHistoryItemsInterleavesFunctionCallAndOutput(t *testing.T) {
	items := buildOpenAIResponsesHistoryItems([]*streamtypes.ModelMessage{
		{Role: "assistant", ToolCalls: []streamtypes.ToolCall{
			{ID: "c1", Name: "list", Arguments: map[string]interface{}{"path": "."}},
			{ID: "c2", Name: "glob", Arguments: map[string]interface{}{"pattern": "*.go"}},
		}},
		{Role: "tool", ToolCallID: "c1", Content: "out1"},
		{Role: "tool", ToolCallID: "c2", Content: "out2"},
	})
	raw, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(raw)
	if !strings.Contains(s, `"type":"function_call"`) {
		t.Fatalf("expected function_call in %s", s)
	}
	// Order: call c1 -> output c1 -> call c2 -> output c2
	c1Call := strings.Index(s, `"call_id":"c1","name":"list"`)
	if c1Call < 0 {
		t.Fatalf("missing first call: %s", s)
	}
	c1Out := strings.Index(s, `"call_id":"c1","output":"out1"`)
	c2Call := strings.Index(s, `"call_id":"c2","name":"glob"`)
	c2Out := strings.Index(s, `"call_id":"c2","output":"out2"`)
	if !(c1Call < c1Out && c1Out < c2Call && c2Call < c2Out) {
		t.Fatalf("want interleaved order, got c1Call=%d c1Out=%d c2Call=%d c2Out=%d\n%s", c1Call, c1Out, c2Call, c2Out, s)
	}
}

func TestBuildOpenAIResponsesHistoryItemsEasyMessageHasType(t *testing.T) {
	items := buildOpenAIResponsesHistoryItems([]*streamtypes.ModelMessage{
		{Role: "user", Content: "hello"},
	})
	raw, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"type":"message"`) || !strings.Contains(string(raw), `"role":"user"`) {
		t.Fatalf("expected type message + role user, got %s", raw)
	}
}
