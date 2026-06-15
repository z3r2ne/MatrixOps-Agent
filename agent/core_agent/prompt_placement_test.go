package coreagent

import "testing"

func TestPreparePromptPayloadUserInput(t *testing.T) {
	prompt := `<system_prompt>
<global_prompt>
system text
</global_prompt>
</system_prompt>

<developer_prompt>
developer text
</developer_prompt>`

	payload := PreparePromptPayload(prompt, SystemPromptPlacementUserInput)
	if payload.UserPrompt != prompt {
		t.Fatalf("expected full prompt to stay in user prompt, got %q", payload.UserPrompt)
	}
	if payload.SystemPrompt != "" {
		t.Fatalf("expected empty system prompt, got %q", payload.SystemPrompt)
	}
	if payload.Instruction != "" {
		t.Fatalf("expected empty instruction, got %q", payload.Instruction)
	}
}

func TestPreparePromptPayloadSystem(t *testing.T) {
	prompt := `<system_prompt>
<global_prompt>
system text
</global_prompt>
</system_prompt>

<developer_prompt>
developer text
</developer_prompt>

<memory>
memory text
</memory>`

	payload := PreparePromptPayload(prompt, SystemPromptPlacementSystem)
	if payload.SystemPrompt == "" {
		t.Fatal("expected system prompt to be extracted")
	}
	expectedUser := `<developer_prompt>
developer text
</developer_prompt>

<memory>
memory text
</memory>`
	if payload.UserPrompt != expectedUser {
		t.Fatalf("unexpected user prompt:\nwant:\n%s\n\ngot:\n%s", expectedUser, payload.UserPrompt)
	}
	if payload.Instruction != "" {
		t.Fatalf("expected empty instruction, got %q", payload.Instruction)
	}
}

func TestPreparePromptPayloadInstruction(t *testing.T) {
	prompt := `<system_prompt>
system text
</system_prompt>

<developer_prompt>
developer text
</developer_prompt>`

	payload := PreparePromptPayload(prompt, SystemPromptPlacementInstruction)
	if payload.Instruction == "" {
		t.Fatal("expected instruction to be extracted")
	}
	if payload.SystemPrompt != "" {
		t.Fatalf("expected empty system prompt, got %q", payload.SystemPrompt)
	}
	expectedUser := `<developer_prompt>
developer text
</developer_prompt>`
	if payload.UserPrompt != expectedUser {
		t.Fatalf("unexpected user prompt:\nwant:\n%s\n\ngot:\n%s", expectedUser, payload.UserPrompt)
	}
}

func TestPreparePromptPayloadFallbackWhenSystemSectionMissing(t *testing.T) {
	prompt := `<developer_prompt>
developer text
</developer_prompt>`

	payload := PreparePromptPayload(prompt, SystemPromptPlacementSystem)
	if payload.UserPrompt != prompt {
		t.Fatalf("expected fallback user prompt, got %q", payload.UserPrompt)
	}
	if payload.SystemPrompt != "" {
		t.Fatalf("expected empty system prompt, got %q", payload.SystemPrompt)
	}
}

func TestPrepareFullPromptPayloadSystem(t *testing.T) {
	payload := PrepareFullPromptPayload("<system_prompt>sys</system_prompt>\n<developer_prompt>dev</developer_prompt>", SystemPromptPlacementSystem, "用户输入")
	if payload.SystemPrompt != "<system_prompt>sys</system_prompt>\n<developer_prompt>dev</developer_prompt>" {
		t.Fatalf("unexpected system prompt: %q", payload.SystemPrompt)
	}
	if payload.UserPrompt != "用户输入" {
		t.Fatalf("unexpected user prompt: %q", payload.UserPrompt)
	}
}

func TestPrepareFullPromptPayloadInstruction(t *testing.T) {
	payload := PrepareFullPromptPayload("<system_prompt>sys</system_prompt>\n<developer_prompt>dev</developer_prompt>", SystemPromptPlacementInstruction, "用户输入")
	if payload.Instruction != "<system_prompt>sys</system_prompt>\n<developer_prompt>dev</developer_prompt>" {
		t.Fatalf("unexpected instruction: %q", payload.Instruction)
	}
	if payload.UserPrompt != "用户输入" {
		t.Fatalf("unexpected user prompt: %q", payload.UserPrompt)
	}
}

func TestPrepareFullPromptPayloadUserInput(t *testing.T) {
	payload := PrepareFullPromptPayload("<system_prompt>sys</system_prompt>\n<developer_prompt>dev</developer_prompt>", SystemPromptPlacementUserInput, "用户输入")
	expected := "<system_prompt>sys</system_prompt>\n<developer_prompt>dev</developer_prompt>\n\n用户输入"
	if payload.UserPrompt != expected {
		t.Fatalf("unexpected user prompt:\nwant: %q\ngot:  %q", expected, payload.UserPrompt)
	}
	if payload.SystemPrompt != "" {
		t.Fatalf("expected empty system prompt, got %q", payload.SystemPrompt)
	}
	if payload.Instruction != "" {
		t.Fatalf("expected empty instruction, got %q", payload.Instruction)
	}
}

func TestRemoveLastMatchingUserMessage(t *testing.T) {
	messages := []*ModelMessage{
		{Role: "user", Content: "用户输入"},
		{Role: "assistant", Content: "工具前回复"},
		{Role: "user", Content: "用户输入"},
		{Role: "assistant", Content: "工具后回复"},
	}

	result := removeLastMatchingUserMessage(messages, "用户输入")
	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}
	if result[0] != messages[0] {
		t.Fatal("expected the first matching user message to stay")
	}
	if result[1] != messages[1] || result[2] != messages[3] {
		t.Fatal("unexpected message order after removal")
	}
}
