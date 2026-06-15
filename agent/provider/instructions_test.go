package provider

import "testing"

func TestToOpenAIRequestUsesInstructionsField(t *testing.T) {
	body := ToOpenAIRequest(CommonRequest{
		Model:        "gpt-5",
		Instructions: "<system_prompt>sys</system_prompt>",
		Messages: []CommonMessage{
			{Role: "user", Content: "hello"},
		},
	})

	if got, _ := body["instructions"].(string); got != "<system_prompt>sys</system_prompt>" {
		t.Fatalf("unexpected instructions field: %q", got)
	}
}

func TestToOaCompatibleRequestUsesStringForTextOnlyContentParts(t *testing.T) {
	body := ToOaCompatibleRequest(CommonRequest{
		Model: "test-model",
		Messages: []CommonMessage{
			{
				Role: "user",
				Content: []CommonContentPart{
					{Type: "text", Text: "hello"},
				},
			},
		},
	})

	messages, ok := body["messages"].([]interface{})
	if !ok {
		t.Fatalf("expected messages array, got %#v", body["messages"])
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	msg, _ := messages[0].(map[string]interface{})
	if content, ok := msg["content"].(string); !ok || content != "hello" {
		t.Fatalf("expected string content %q, got %#v", "hello", msg["content"])
	}
}

func TestToOaCompatibleRequestKeepsArrayForMultimodalContent(t *testing.T) {
	body := ToOaCompatibleRequest(CommonRequest{
		Model: "test-model",
		Messages: []CommonMessage{
			{
				Role: "user",
				Content: []CommonContentPart{
					{Type: "text", Text: "describe this"},
					{Type: "image_url", ImageURL: &CommonImageURL{URL: "https://example.com/a.png"}},
				},
			},
		},
	})

	messages, ok := body["messages"].([]interface{})
	if !ok {
		t.Fatalf("expected messages array, got %#v", body["messages"])
	}
	msg, _ := messages[0].(map[string]interface{})
	parts, ok := msg["content"].([]interface{})
	if !ok || len(parts) != 2 {
		t.Fatalf("expected multimodal content array with 2 parts, got %#v", msg["content"])
	}
}

func TestToOaCompatibleRequestFallsBackToSystemMessage(t *testing.T) {
	body := ToOaCompatibleRequest(CommonRequest{
		Model:        "test-model",
		Instructions: "<system_prompt>sys</system_prompt>",
		Messages: []CommonMessage{
			{Role: "user", Content: "hello"},
		},
	})

	messages, ok := body["messages"].([]interface{})
	if !ok {
		t.Fatalf("expected messages array, got %#v", body["messages"])
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	first, _ := messages[0].(map[string]interface{})
	if role, _ := first["role"].(string); role != "system" {
		t.Fatalf("expected first role=system, got %q", role)
	}
	if content, _ := first["content"].(string); content != "<system_prompt>sys</system_prompt>" {
		t.Fatalf("unexpected first content: %q", content)
	}
}

func TestToAnthropicRequestMapsInstructionsToSystem(t *testing.T) {
	body := ToAnthropicRequest(CommonRequest{
		Model:        "claude-test",
		Instructions: "<system_prompt>sys</system_prompt>",
		Messages: []CommonMessage{
			{Role: "user", Content: "hello"},
		},
	})

	system, ok := body["system"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected anthropic system array, got %#v", body["system"])
	}
	if len(system) != 1 {
		t.Fatalf("expected 1 system item, got %d", len(system))
	}
	if text, _ := system[0]["text"].(string); text != "<system_prompt>sys</system_prompt>" {
		t.Fatalf("unexpected system text: %q", text)
	}
}
