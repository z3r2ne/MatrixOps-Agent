package session

import (
	"testing"

	"matrixops-agent/types"
)

func TestBuildLatestToolCall_ForToolResult(t *testing.T) {
	history := []*types.ChatHistoryItem{
		{Role: "user", Content: "帮我读取文件"},
		{Role: "assistant", Content: `{"call_tool":"call_tool","params":{"tool_name":"read_file","tool_input":{"path":"README.md"}}}`},
		{Role: "tool_call", Content: "[Tool Output]: README content"},
	}

	latestToolCall := buildLatestToolCall(history)

	if latestToolCall == nil {
		t.Fatal("expected latest tool call metadata to be built")
	}
	if latestToolCall.Role != "tool_call" {
		t.Fatalf("expected latest role tool_call, got %q", latestToolCall.Role)
	}
	if latestToolCall.ToolName != "read_file" {
		t.Fatalf("expected tool name read_file, got %q", latestToolCall.ToolName)
	}
	if latestToolCall.Content != "" {
		t.Fatalf("expected tool call latest hint content to be empty, got %q", latestToolCall.Content)
	}
	expectedInstruction := "你刚刚申请了调用工具 read_file，工具已经执行成功。请基于工具结果继续选择你的动作。"
	if latestToolCall.Instruction != expectedInstruction {
		t.Fatalf("expected instruction %q, got %q", expectedInstruction, latestToolCall.Instruction)
	}
	if len(history) != 3 {
		t.Fatalf("expected history length 3 to remain unchanged, got %d", len(history))
	}
}

func TestBuildLatestToolCall_ForAssistantMessage(t *testing.T) {
	history := []*types.ChatHistoryItem{
		{
			Role:      "assistant",
			Content:   "归一化后的文本",
			RawOutput: `{"call_tool":"message","params":{"message":"原始 AI 输出"}}`,
		},
	}

	latestToolCall := buildLatestToolCall(history)

	if latestToolCall == nil {
		t.Fatal("expected latest assistant message metadata to be built")
	}
	if latestToolCall.Role != "assistant" {
		t.Fatalf("expected latest role assistant, got %q", latestToolCall.Role)
	}
	if latestToolCall.Content != `{"call_tool":"message","params":{"message":"原始 AI 输出"}}` {
		t.Fatalf("expected raw output content, got %q", latestToolCall.Content)
	}
	expectedInstruction := "你刚刚输出了以下内容，现在继续。"
	if latestToolCall.Instruction != expectedInstruction {
		t.Fatalf("expected instruction %q, got %q", expectedInstruction, latestToolCall.Instruction)
	}
}

func TestBuildLatestToolCall_ForNativeToolRole(t *testing.T) {
	history := []*types.ChatHistoryItem{
		{Role: "user", Content: "列出文件"},
		{
			Role: "assistant",
			NativeToolCalls: []types.ChatHistoryNativeToolCall{
				{ID: "call-1", Name: "bash", Arguments: `{"command":"ls"}`},
			},
		},
		{Role: "tool", ToolCallID: "call-1", ToolName: "bash", Content: "a.txt"},
	}

	latestToolCall := buildLatestToolCall(history)
	if latestToolCall == nil {
		t.Fatal("expected latest tool call metadata")
	}
	if latestToolCall.Role != "tool" {
		t.Fatalf("expected latest role tool, got %q", latestToolCall.Role)
	}
	if latestToolCall.ToolName != "bash" {
		t.Fatalf("expected tool name bash, got %q", latestToolCall.ToolName)
	}
	if latestToolCall.Content != "a.txt" {
		t.Fatalf("expected tool output content, got %q", latestToolCall.Content)
	}
}

func TestBuildLatestToolCall_ForUserMessage(t *testing.T) {
	history := []*types.ChatHistoryItem{
		{Role: "user", Content: "请继续修复这个问题"},
	}

	latestToolCall := buildLatestToolCall(history)

	if latestToolCall == nil {
		t.Fatal("expected latest user message metadata to be built")
	}
	if latestToolCall.Role != "user" {
		t.Fatalf("expected latest role user, got %q", latestToolCall.Role)
	}
	if latestToolCall.Content != "请继续修复这个问题" {
		t.Fatalf("expected user content to be preserved, got %q", latestToolCall.Content)
	}
	expectedInstruction := "用户刚刚说了以下内容，需要你作答。"
	if latestToolCall.Instruction != expectedInstruction {
		t.Fatalf("expected instruction %q, got %q", expectedInstruction, latestToolCall.Instruction)
	}
}

func TestFormatToolCallResultForHistory_EmptyOutput(t *testing.T) {
	part := &types.Part{
		Type: "tool",
		Tool: &types.ToolPart{
			Name: "read_file",
			State: types.ToolState{
				Status: "success",
			},
		},
	}

	result := formatToolCallResultForHistory(part)

	if result != "工具输出为空" {
		t.Fatalf("expected empty tool output placeholder, got %q", result)
	}
}
