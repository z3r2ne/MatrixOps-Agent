package openai_native

import (
	"testing"

	"github.com/openai/openai-go"

	"matrixops.local/core_agent/streamtypes"
)

func TestMergeOpenAINativeToolCalls_splitNameAndArgsIndex(t *testing.T) {
	calls := []openai.ChatCompletionMessageToolCall{
		{
			ID: "call_isG0EtD1rgtOZ3kwRh8cS256",
			Function: openai.ChatCompletionMessageToolCallFunction{
				Name:      "message",
				Arguments: "",
			},
		},
		{
			Function: openai.ChatCompletionMessageToolCallFunction{
				Name:      "",
				Arguments: `{"message":"hi","next_step":"x"}`,
			},
		},
	}
	got := mergeOpenAINativeToolCalls(calls)
	if len(got) != 1 {
		t.Fatalf("got %d tool calls, want 1", len(got))
	}
	if got[0].Function.Name != "message" {
		t.Fatalf("name %q", got[0].Function.Name)
	}
	if got[0].Function.Arguments != `{"message":"hi","next_step":"x"}` {
		t.Fatalf("arguments %q", got[0].Function.Arguments)
	}
	if got[0].ID != "call_isG0EtD1rgtOZ3kwRh8cS256" {
		t.Fatalf("id %q", got[0].ID)
	}
}

func TestMergeOpenAINativeToolCalls_multipleArgOnlySlots(t *testing.T) {
	calls := []openai.ChatCompletionMessageToolCall{
		{Function: openai.ChatCompletionMessageToolCallFunction{Name: "f", Arguments: ""}},
		{Function: openai.ChatCompletionMessageToolCallFunction{Name: "", Arguments: `{"`}},
		{Function: openai.ChatCompletionMessageToolCallFunction{Name: "", Arguments: `a":1}`}},
	}
	got := mergeOpenAINativeToolCalls(calls)
	if len(got) != 1 || got[0].Function.Arguments != `{"a":1}` {
		t.Fatalf("got %+v", got)
	}
}

func TestMergeOpenAINativeToolCalls_twoNamedTools(t *testing.T) {
	calls := []openai.ChatCompletionMessageToolCall{
		{ID: "1", Function: openai.ChatCompletionMessageToolCallFunction{Name: "a", Arguments: ""}},
		{Function: openai.ChatCompletionMessageToolCallFunction{Name: "", Arguments: `{"x":1}`}},
		{ID: "2", Function: openai.ChatCompletionMessageToolCallFunction{Name: "b", Arguments: ""}},
		{Function: openai.ChatCompletionMessageToolCallFunction{Name: "", Arguments: `{"y":2}`}},
	}
	got := mergeOpenAINativeToolCalls(calls)
	if len(got) != 2 {
		t.Fatalf("len=%d %+v", len(got), got)
	}
	if got[0].Function.Name != "a" || got[0].Function.Arguments != `{"x":1}` {
		t.Fatalf("first %+v", got[0])
	}
	if got[1].Function.Name != "b" || got[1].Function.Arguments != `{"y":2}` {
		t.Fatalf("second %+v", got[1])
	}
}

func TestNormalizeOpenAINativeToolCalls_reassignSplitPayloadsToNamedSlots(t *testing.T) {
	tools := []streamtypes.ToolDefinition{
		streamtypes.ToolDefinition{Name: "message", Schema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"content": map[string]interface{}{"type": "string"}}}},
		{
			Name: "list",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{"type": "string"},
				},
			},
		},
		{
			Name: "glob",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"pattern": map[string]interface{}{"type": "string"},
					"root":    map[string]interface{}{"type": "string"},
				},
				"required": []interface{}{"pattern"},
			},
		},
		{
			Name: "tree",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path":  map[string]interface{}{"type": "string"},
					"depth": map[string]interface{}{"type": "number"},
				},
			},
		},
	}
	calls := []openai.ChatCompletionMessageToolCall{
		{
			ID: "tool-1",
			Function: openai.ChatCompletionMessageToolCallFunction{
				Name: "message",
				Arguments: `{"message":"我先快速扫一遍仓库结构和说明文件，给你一个项目概览。","next_step":"检查顶层目录、AGENTS.md、README 和主要模块分布"}` +
					`{"path":"/Users/patrick/Code/yaklang"}` +
					`{"pattern":"**/AGENTS.md","root":"/Users/patrick/Code/yaklang"}` +
					`{"pattern":"README*","root":"/Users/patrick/Code/yaklang"}` +
					`{"depth":2,"path":"/Users/patrick/Code/yaklang"}`,
			},
		},
		{ID: "tool-2", Function: openai.ChatCompletionMessageToolCallFunction{Name: "list"}},
		{ID: "tool-3", Function: openai.ChatCompletionMessageToolCallFunction{Name: "glob"}},
		{ID: "tool-4", Function: openai.ChatCompletionMessageToolCallFunction{Name: "glob"}},
		{ID: "tool-5", Function: openai.ChatCompletionMessageToolCallFunction{Name: "tree"}},
	}

	got := normalizeOpenAINativeToolCalls(calls, tools)
	if len(got) != 5 {
		t.Fatalf("got %d tool calls, want 5", len(got))
	}
	if got[0].Function.Name != "message" || got[0].Function.Arguments == "" {
		t.Fatalf("message call %+v", got[0])
	}
	if got[1].Function.Name != "list" || got[1].Function.Arguments != `{"path":"/Users/patrick/Code/yaklang"}` {
		t.Fatalf("list call %+v", got[1])
	}
	if got[2].Function.Name != "glob" || got[2].Function.Arguments != `{"pattern":"**/AGENTS.md","root":"/Users/patrick/Code/yaklang"}` {
		t.Fatalf("glob1 %+v", got[2])
	}
	if got[3].Function.Name != "glob" || got[3].Function.Arguments != `{"pattern":"README*","root":"/Users/patrick/Code/yaklang"}` {
		t.Fatalf("glob2 %+v", got[3])
	}
	if got[4].Function.Name != "tree" || got[4].Function.Arguments != `{"depth":2,"path":"/Users/patrick/Code/yaklang"}` {
		t.Fatalf("tree %+v", got[4])
	}
}

func TestNormalizeOpenAINativeToolCalls_inferNamesForAnonymousPayloads(t *testing.T) {
	tools := []streamtypes.ToolDefinition{
		streamtypes.ToolDefinition{Name: "message", Schema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"content": map[string]interface{}{"type": "string"}}}},
		{
			Name: "list",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{"type": "string"},
				},
			},
		},
		{
			Name: "glob",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"pattern": map[string]interface{}{"type": "string"},
					"root":    map[string]interface{}{"type": "string"},
				},
				"required": []interface{}{"pattern"},
			},
		},
		{
			Name: "tree",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path":  map[string]interface{}{"type": "string"},
					"depth": map[string]interface{}{"type": "number"},
				},
			},
		},
	}
	calls := []openai.ChatCompletionMessageToolCall{
		{
			ID: "tool-1",
			Function: openai.ChatCompletionMessageToolCallFunction{
				Name: "message",
				Arguments: `{"message":"hi","next_step":"next"}` +
					`{"path":"/tmp/work"}` +
					`{"pattern":"README*","root":"/tmp/work"}` +
					`{"depth":2,"path":"/tmp/work"}`,
			},
		},
	}

	got := normalizeOpenAINativeToolCalls(calls, tools)
	if len(got) != 4 {
		t.Fatalf("got %d tool calls, want 4", len(got))
	}
	if got[0].Function.Name != "message" {
		t.Fatalf("message %+v", got[0])
	}
	if got[1].Function.Name != "list" {
		t.Fatalf("list %+v", got[1])
	}
	if got[2].Function.Name != "glob" {
		t.Fatalf("glob %+v", got[2])
	}
	if got[3].Function.Name != "tree" {
		t.Fatalf("tree %+v", got[3])
	}
}
