package coreagent

import (
	"testing"

	"matrixops-agent/types"
)

func TestRenderV2TaskPrompt(t *testing.T) {
	memory := &types.Memory{
		GlobalPrompt:          "你是一个智能助手",
		OccupationPrompt:      "你是一个专业的代码助手",
		ProjectPrompt:         "当前项目是一个 Go 语言项目",
		WorkerPrompt:          "请帮助用户完成编程任务",
		SessionGuidancePrompt: "当前可调用的子 worker: explore, plan, verification。",
		OutputStylePrompt:     "先给结论，再给必要细节。",
		ToolPriorityPrompt:    "已知路径优先 read，内容搜索优先 rg。",
		EnvPrompt:             "操作系统: Linux",
		Entries: []*types.MemoryEntry{
			{EntryKind: "text", Role: "user", Content: "你好", Sequence: 1},
			{EntryKind: "text", Role: "assistant", Content: "你好！有什么可以帮助你的吗？", Sequence: 2},
			{
				EntryKind:          "tool_call",
				Role:               "assistant",
				Content:            `{"call_tool":"call_tool","params":{"tool_name":"read_file","tool_input":{"path":"/path/to/file.txt"}}}`,
				ToolName:           "read_file",
				ToolStatus:         "completed",
				ToolRequestRawJSON: `{"call_tool":"call_tool","params":{"tool_name":"read_file","tool_input":{"path":"/path/to/file.txt"}}}`,
				ToolInputJSON:      `{"path":"/path/to/file.txt"}`,
				ToolOutput:         "file content",
				Sequence:           3,
			},
		},
	}

	tools := []ToolDefinition{
		{
			Name:        "read_file",
			Description: "读取文件内容",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "文件路径",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "write_file",
			Description: "写入文件内容",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "文件路径",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "文件内容",
					},
				},
				"required": []string{"path", "content"},
			},
		},
	}

	data := V2TaskPromptData{
		Memory:    memory,
		Tools:     tools,
		UserInput: "请帮我读取 README.md 文件",
	}

	result, err := RenderV2TaskPrompt(data)
	if err != nil {
		t.Fatalf("RenderV2TaskPrompt failed: %v", err)
	}

	if result == "" {
		t.Fatal("Rendered prompt is empty")
	}

	expectedContents := []string{
		"<system_prompt>",
		"<global_prompt>",
		"你是一个智能助手",
		"<occupation_prompt>",
		"你是一个专业的代码助手",
		"<worker_prompt>",
		"请帮助用户完成编程任务",
		"<output_style_prompt>",
		"先给结论，再给必要细节。",
		"<developer_prompt>",
		"<session_guidance>",
		"当前可调用的子 worker: explore, plan, verification。",
		"<tool_priority>",
		"已知路径优先 read，内容搜索优先 rg。",
		"<project_prompt>",
		"当前项目是一个 Go 语言项目",
		"<environment_prompt>",
		"操作系统: Linux",
		"<memory>",
		"======",
		"[user]: 你好",
		"[assistant]: 你好！有什么可以帮助你的吗？",
		`[assistant]: {"call_tool":"call_tool","params":{"tool_name":"read_file","tool_input":{"path":"/path/to/file.txt"}}}`,
		`[call_tool_read_file]: file content`,
	}

	for _, expected := range expectedContents {
		if !containsPromptTest(result, expected) {
			t.Errorf("Expected content not found: %s", expected)
		}
	}
	if containsPromptTest(result, "<history>") {
		t.Error("history section should have been replaced by unified memory section")
	}
	if containsPromptTest(result, "<latest_message>") {
		t.Error("latest_message section should have been replaced by unified memory section")
	}
}

func TestRenderV2TaskPrompt_MinimalData(t *testing.T) {
	data := V2TaskPromptData{
		Memory: &types.Memory{
			GlobalPrompt: "你是一个助手",
		},
		UserInput: "帮我写代码",
	}

	result, err := RenderV2TaskPrompt(data)
	if err != nil {
		t.Fatalf("RenderV2TaskPrompt failed: %v", err)
	}

	if result == "" {
		t.Fatal("Rendered prompt is empty")
	}

	if !containsPromptTest(result, "你是一个助手") {
		t.Error("GlobalPrompt not found")
	}
}

func TestRenderV2TaskPrompt_NoTools(t *testing.T) {
	data := V2TaskPromptData{
		Memory: &types.Memory{
			GlobalPrompt: "你是一个助手",
		},
		Tools:     []ToolDefinition{},
		UserInput: "你好",
	}

	result, err := RenderV2TaskPrompt(data)
	if err != nil {
		t.Fatalf("RenderV2TaskPrompt failed: %v", err)
	}

	if !containsPromptTest(result, "你是一个助手") {
		t.Error("GlobalPrompt not found")
	}
}

func TestRenderV2TaskPrompt_DoesNotIncludeDelegationExamples(t *testing.T) {
	data := V2TaskPromptData{
		Memory: &types.Memory{
			GlobalPrompt: "你是一个助手",
		},
		Tools: []ToolDefinition{
			{
				Name:        "run_worker_task",
				Description: "执行子任务",
				Schema: map[string]interface{}{
					"type": "object",
				},
			},
		},
		UserInput: "看一下当前项目相关代码并给我方案",
	}

	result, err := RenderV2TaskPrompt(data)
	if err != nil {
		t.Fatalf("RenderV2TaskPrompt failed: %v", err)
	}

	if containsPromptTest(result, "<delegation_examples>") {
		t.Fatalf("expected prompt to not contain %q, got:\n%s", "<delegation_examples>", result)
	}
}

func TestRenderV2TaskPrompt_WithProjectFiles(t *testing.T) {
	data := V2TaskPromptData{
		Memory: &types.Memory{
			GlobalPrompt: "你是一个助手",
			ProjectFilePrompt: []types.FilePrompt{
				{
					Path:   "src/main.go",
					Prompt: "package main\n\nfunc main() {\n\tprintln(\"Hello\")\n}",
				},
				{
					Path:   "README.md",
					Prompt: "# Project Title\n\nThis is a test project.",
				},
			},
		},
		UserInput: "分析这些文件",
	}

	result, err := RenderV2TaskPrompt(data)
	if err != nil {
		t.Fatalf("RenderV2TaskPrompt failed: %v", err)
	}

	if !containsPromptTest(result, "<project_files>") {
		t.Error("Project files section not found")
	}
	if !containsPromptTest(result, "<path>src/main.go</path>") {
		t.Error("First file path not found")
	}
	if !containsPromptTest(result, "package main") {
		t.Error("First file content not found")
	}
	if !containsPromptTest(result, "<path>README.md</path>") {
		t.Error("Second file path not found")
	}
	if !containsPromptTest(result, "# Project Title") {
		t.Error("Second file content not found")
	}
}

func TestRenderV2TaskPrompt_MemoryEntryPreservesRawOutput(t *testing.T) {
	data := V2TaskPromptData{
		Memory: &types.Memory{
			GlobalPrompt: "你是一个助手",
			Entries: []*types.MemoryEntry{
				{
					EntryKind: "text",
					Role:      "assistant",
					Content:   "归一化后的文本",
					RawOutput: `{"call_tool":"message","params":{"message":"原始 AI 输出","next_step":"继续检查剩余问题"}}`,
					Sequence:  1,
				},
			},
		},
		UserInput: "继续处理剩余问题",
	}

	result, err := RenderV2TaskPrompt(data)
	if err != nil {
		t.Fatalf("RenderV2TaskPrompt failed: %v", err)
	}

	expectedContents := []string{
		"<memory>",
		"======",
		`[assistant]: {"call_tool":"message","params":{"message":"原始 AI 输出","next_step":"继续检查剩余问题"}}`,
	}

	for _, expected := range expectedContents {
		if !containsPromptTest(result, expected) {
			t.Errorf("Expected content not found: %s", expected)
		}
	}

	if containsPromptTest(result, "<latest_message>") {
		t.Error("latest_message section should have been replaced by unified memory section")
	}
}

func containsPromptTest(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && (s[0:len(substr)] == substr || containsPromptTest(s[1:], substr))))
}
