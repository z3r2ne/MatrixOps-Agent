package coreagent

import (
	"strings"
	"testing"

	agentmemory "matrixops.local/memory"
)

func TestCreatePromptBuilder_MaxStepsAnswerOnlyIncludesMemoryAndAnswerSchema(t *testing.T) {
	builder, err := CreatePromptBuilder(DefaultPromptBuilderName, PromptBuilderOptions{})
	if err != nil {
		t.Fatalf("CreatePromptBuilder: %v", err)
	}
	text, err := builder(&RunState{
		Memory: &agentmemory.Memory{
			Entries: []*agentmemory.MemoryEntry{{Role: "user", Content: "hello", Sequence: 1}},
		},
		UserInput:                  "do something",
		Tools:                      []ToolDefinition{AnswerToolDefinition()},
		MaxStepsExhaustedFinalPass: true,
	})
	if err != nil {
		t.Fatalf("builder: %v", err)
	}
	for _, expected := range []string{
		"<max_steps_exhausted>",
		"<user_request>",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected prompt to contain %q, got:\n%s", expected, text)
		}
	}
}

func TestCreatePromptBuilder_MemoryCompaction(t *testing.T) {
	builder, err := CreatePromptBuilder(MemoryCompactionPromptBuilderName, PromptBuilderOptions{})
	if err != nil {
		t.Fatalf("CreatePromptBuilder: %v", err)
	}
	text, err := builder(&RunState{
		Memory: &agentmemory.Memory{
			WorkerPrompt: "worker prompt",
			Entries:      []*agentmemory.MemoryEntry{{Role: "user", Content: "hello", Sequence: 1}},
		},
		UserInput: "只压缩最早的旧记忆",
	})
	if err != nil {
		t.Fatalf("builder returned error: %v", err)
	}
	for _, expected := range []string{
		"压缩优先级",
		"用户附加压缩要求",
		"<current_focus>",
		"你是摘要撰写者",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected prompt to contain %q, got:\n%s", expected, text)
		}
	}
	for _, unexpected := range []string{
		"<system_prompt>",
		"worker prompt",
		"<developer_prompt>",
		"<context_window>",
	} {
		if strings.Contains(text, unexpected) {
			t.Fatalf("expected prompt not to contain %q, got:\n%s", unexpected, text)
		}
	}
}

func TestRenderMemoryCompactionPromptPayloadSeparatesInstructionAndInput(t *testing.T) {
	payload, err := RenderMemoryCompactionPromptPayload(&agentmemory.Memory{
		Entries: []*agentmemory.MemoryEntry{{Role: "user", Content: "hello", Sequence: 1}},
	}, nil, "只压缩最早的旧记忆", "优先保留数据库迁移进度")
	if err != nil {
		t.Fatalf("RenderMemoryCompactionPromptPayload: %v", err)
	}
	if payload.Instruction != MemoryCompactionSystemPrompt {
		t.Fatalf("expected short system prompt %q, got:\n%s", MemoryCompactionSystemPrompt, payload.Instruction)
	}
	if strings.Contains(payload.Instruction, "MsgID:") {
		t.Fatalf("expected system prompt to omit transcript, got:\n%s", payload.Instruction)
	}
	if !strings.Contains(payload.UserInput, "MsgID: 1") {
		t.Fatalf("expected user input to contain transcript, got:\n%s", payload.UserInput)
	}
	if !strings.Contains(payload.UserInput, "压缩优先级") {
		t.Fatalf("expected user input to contain compaction task prompt, got:\n%s", payload.UserInput)
	}
	if !strings.Contains(payload.UserInput, "只压缩最早的旧记忆") {
		t.Fatalf("expected user input to contain custom instruction, got:\n%s", payload.UserInput)
	}
	if !strings.Contains(payload.UserInput, "优先保留数据库迁移进度") {
		t.Fatalf("expected user input to contain worker extra prompt, got:\n%s", payload.UserInput)
	}
	idxTranscript := strings.Index(payload.UserInput, "MsgID: 1")
	idxTask := strings.Index(payload.UserInput, "压缩优先级")
	if idxTask <= idxTranscript {
		t.Fatalf("expected task prompt after transcript, transcript@%d task@%d", idxTranscript, idxTask)
	}
}

func TestCreatePromptBuilder_V2TaskOmitsMemorySection(t *testing.T) {
	builder, err := CreatePromptBuilder(DefaultPromptBuilderName, PromptBuilderOptions{})
	if err != nil {
		t.Fatalf("CreatePromptBuilder: %v", err)
	}

	text, err := builder(&RunState{
		Memory: &agentmemory.Memory{
			Entries: []*agentmemory.MemoryEntry{{Role: "user", Content: "hello", Sequence: 1}},
		},
		UserInput: "继续处理",
	})
	if err != nil {
		t.Fatalf("builder returned error: %v", err)
	}

	if strings.Contains(text, "<memory>\n") {
		t.Fatalf("expected prompt to omit memory section in messages mode, got:\n%s", text)
	}
}

func TestCreatePromptBuilder_UsesV3TaskForNativeTools(t *testing.T) {
	builder, err := CreatePromptBuilder(DefaultPromptBuilderName, PromptBuilderOptions{})
	if err != nil {
		t.Fatalf("CreatePromptBuilder: %v", err)
	}

	text, err := builder(&RunState{
		Memory: &agentmemory.Memory{
			GlobalPrompt:          "global prompt",
			WorkerPrompt:          "worker prompt",
			SessionGuidancePrompt: "session guidance",
			OutputStylePrompt:     "output style",
			ToolPriorityPrompt:    "tool priority",
			EnvPrompt:             "env prompt",
			ProjectPrompt:         "project prompt",
			ProjectFilePrompt:     []agentmemory.FilePrompt{{Path: "AGENTS.md", Prompt: "agent file prompt"}},
			Entries:               []*agentmemory.MemoryEntry{{Role: "user", Content: "history content", Sequence: 1}},
		},
		NativeOpenAIToolCalls:   true,
		UserInput:               "继续处理",
		Tools:                   MergePromptToolDefinitions(nil, PromptToolMergeOptions{}),
		OmitAnswerInPromptMerge: true,
	})
	if err != nil {
		t.Fatalf("builder returned error: %v", err)
	}

	for _, expected := range []string{
		"<system_prompt>",
		"<global_prompt>",
		"global prompt",
		"<worker_prompt>",
		"worker prompt",
		"<output_style_prompt>",
		"output style",
		"<developer_prompt>",
		"<session_guidance>",
		"session guidance",
		"<environment_prompt>",
		"env prompt",
		"<tool_priority>",
		"tool priority",
		"<project_prompt>",
		"project prompt",
		"<project_files>",
		"agent file prompt",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected prompt to contain %q, got:\n%s", expected, text)
		}
	}

	for _, unexpected := range []string{
		"<memory>",
		"<tools>",
		"<output_format>",
		"<final_instruction>",
		"<context_window>",
		"history content",
		"继续处理",
	} {
		if strings.Contains(text, unexpected) {
			t.Fatalf("expected prompt not to contain %q, got:\n%s", unexpected, text)
		}
	}
}


