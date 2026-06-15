package session

import (
	"strings"
	"testing"

	"matrixops-agent/llm"
	"matrixops-agent/types"
	coreagent "matrixops.local/core_agent"
	"pkgs/db/models"
)

type noStreamChatClient struct{}

func (noStreamChatClient) Chat(request llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{}, nil
}

func (noStreamChatClient) StreamChat(request llm.ChatRequest) (<-chan llm.StreamEvent, error) {
	ch := make(chan llm.StreamEvent, 1)
	close(ch)
	return ch, nil
}

func TestMemoryCompactionSerializationFieldsIncludesBeforeAndAfter(t *testing.T) {
	before := []*types.MemoryEntry{{ID: 1, Role: "user", Content: "before"}}
	after := []*types.MemoryEntry{{ID: 2, Role: "assistant", Content: "after"}}
	fields := memoryCompactionSerializationFields(before, after)
	if len(fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(fields))
	}
	if fields[0].Key != "memoryBefore" || fields[1].Key != "memoryAfter" {
		t.Fatalf("unexpected field keys: %#v", fields)
	}
	if !strings.Contains(fields[0].Value, `"before"`) {
		t.Fatalf("unexpected before payload: %s", fields[0].Value)
	}
	if !strings.Contains(fields[1].Value, `"after"`) {
		t.Fatalf("unexpected after payload: %s", fields[1].Value)
	}
}

func TestBuildCompressedMemoryEntries(t *testing.T) {
	remaining := []*types.MemoryEntry{
		{
			SessionID: "session-1",
			EntryKind: "text",
			Role:      "assistant",
			Content:   "保留的后半段消息",
			Sequence:  200,
			Created:   300,
		},
	}

	entries := buildCompressedMemoryEntries("session-1", 100, 200, "这是压缩后的记忆", remaining)

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries after compression, got %d", len(entries))
	}

	if entries[0].EntryKind != "summary_user" || entries[0].Role != "user" {
		t.Fatalf("unexpected first synthetic entry: %#v", entries[0])
	}
	if entries[0].Content != "总结一下之前都做了什么。" {
		t.Fatalf("unexpected summary user content: %q", entries[0].Content)
	}
	if !entries[0].Synthetic {
		t.Fatal("expected summary user entry to be synthetic")
	}
	if entries[0].Sequence != 100 {
		t.Fatalf("unexpected summary user sequence: %d", entries[0].Sequence)
	}

	if entries[1].EntryKind != "summary_assistant" || entries[1].Role != "assistant" {
		t.Fatalf("unexpected second synthetic entry: %#v", entries[1])
	}
	if entries[1].Content != "这是压缩后的记忆" {
		t.Fatalf("unexpected summary assistant content: %q", entries[1].Content)
	}
	if entries[0].CompressionLevel != 2 || entries[1].CompressionLevel != 2 {
		t.Fatalf("expected compression level 2, got %d and %d", entries[0].CompressionLevel, entries[1].CompressionLevel)
	}
	if !entries[1].Synthetic {
		t.Fatal("expected summary assistant entry to be synthetic")
	}
	if entries[1].Sequence != 101 {
		t.Fatalf("unexpected summary assistant sequence: %d", entries[1].Sequence)
	}

	if entries[2] != remaining[0] {
		t.Fatal("expected remaining entries to be appended unchanged")
	}
}

func TestMemoryCompactionSamplingParamsAreOmitted(t *testing.T) {
	temperature, topP := MemoryCompactionSamplingParams()
	if temperature != 0 {
		t.Fatalf("temperature = %v, want 0", temperature)
	}
	if topP != 0 {
		t.Fatalf("topP = %v, want 0", topP)
	}
}

func TestMemoryCompactionMaxOutputTokensIsCapped(t *testing.T) {
	if got := MemoryCompactionMaxOutputTokens(); got != defaultMemoryCompactionMaxOutputTokens {
		t.Fatalf("max output tokens = %d, want %d", got, defaultMemoryCompactionMaxOutputTokens)
	}
	_, _, _, maxOut := memoryCompactionModelRequest(&MemoryCompactionRuntime{
		Worker: &models.Worker{Name: "compaction", Model: "compaction-model"},
	})
	if maxOut != defaultMemoryCompactionMaxOutputTokens {
		t.Fatalf("model request max output tokens = %d, want %d", maxOut, defaultMemoryCompactionMaxOutputTokens)
	}
}

func TestTruncateRepetitiveCompactionSummary(t *testing.T) {
	phrase := "这是一个大重构，我先创建计划，然后逐步执行"
	repeated := strings.Repeat(phrase, 20)
	truncated, changed := truncateRepetitiveCompactionSummary(repeated)
	if !changed {
		t.Fatal("expected repetitive summary to be truncated")
	}
	if truncated != phrase {
		t.Fatalf("truncated = %q, want %q", truncated, phrase)
	}

	normal := "用户要求将 llm-proxy 后端迁移到 GORM，已完成 traffic 分页与 analytics 指标采集。"
	if got, changed := truncateRepetitiveCompactionSummary(normal); changed || got != normal {
		t.Fatalf("normal summary changed unexpectedly: changed=%v got=%q", changed, got)
	}
}

func TestTruncateAgentPlanningLoop(t *testing.T) {
	first := "用户要求把数据库操作从原生 database/sql 改成 GORM。已完成 traffic 分页、analytics 指标与 NULL 修复，GORM 迁移尚未开始。"
	second := "用户要求将数据库操作从原生 database/sql 迁移到 GORM ORM。这是一个较大的重构任务，涉及模型定义、数据库初始化、所有 store 层和 handler 层的改动。"
	loop := first + second + strings.Repeat(second, 5)

	truncated, changed := truncateAgentPlanningLoop(loop)
	if !changed {
		t.Fatal("expected agent planning loop to be truncated")
	}
	if !strings.HasPrefix(truncated, first) {
		t.Fatalf("truncated = %q, want prefix %q", truncated, first)
	}
	if strings.Contains(truncated, "这是一个较大的重构任务") {
		t.Fatalf("truncated still contains planning loop: %q", truncated)
	}
}

func TestSanitizeMemoryCompactionSummaryRejectsPlanningLoop(t *testing.T) {
	planning := strings.Repeat("用户要求把 database/sql 改成 GORM。让我开始逐步重构，安装 GORM 依赖。", 4)
	if _, err := sanitizeMemoryCompactionSummary(planning); err == nil {
		t.Fatal("expected planning loop summary to be rejected")
	}
}

func TestSanitizeMemoryCompactionSummaryKeepsNormalSummary(t *testing.T) {
	normal := "用户要求 llm-proxy 增加 Analytics 与 Traffic 分页。已完成 schema 扩展、代理层 TTFB/token 采集、/api/analytics、前端 AnalyticsViewer；Traffic NULL model_name 已用 COALESCE 修复。待办：将 store 层迁移到 GORM。"
	got, err := sanitizeMemoryCompactionSummary(normal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != normal {
		t.Fatalf("summary changed unexpectedly: %q", got)
	}
}

func TestStreamMemoryCompactionSummaryTruncatesRepetitiveOutput(t *testing.T) {
	phrase := "这是一个大重构，我先创建计划，然后逐步执行"
	repeated := strings.Repeat(phrase, 10)
	client := &stubRepetitiveCompactionStreamClient{text: repeated}
	result, err := StreamMemoryCompactionSummary(client, llm.ChatRequest{
		Messages: []*llm.ModelMessage{
			{Role: "system", Content: "compress these memories"},
			{Role: "user", Content: "需要压缩的记忆"},
		},
		ProviderOptions: &models.LLMConfig{
			Name:                  "openai",
			Type:                  "openai",
			Model:                 "gpt-5.4",
			NativeOpenAIToolCalls: false,
		},
		Model: "gpt-5.4",
	}, MemoryCompactionStreamOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Summary != phrase {
		t.Fatalf("summary = %q, want %q", result.Summary, phrase)
	}
}

func TestStreamMemoryCompactionSummaryEmitsSummaryDeltas(t *testing.T) {
	client := &stubCompactionStreamClient{}
	var deltas []string
	result, err := StreamMemoryCompactionSummary(client, llm.ChatRequest{
		Messages: []*llm.ModelMessage{
			{Role: "system", Content: "compress these memories"},
			{Role: "user", Content: "需要压缩的记忆"},
		},
		ProviderOptions: &models.LLMConfig{
			Name:                  "openai",
			Type:                  "openai",
			Model:                 "gpt-5.4",
			NativeOpenAIToolCalls: false,
		},
		Model: "gpt-5.4",
	}, MemoryCompactionStreamOptions{
		OnSummaryDelta: func(summary string) {
			deltas = append(deltas, summary)
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deltas) == 0 {
		t.Fatal("expected at least one summary delta")
	}
	if deltas[len(deltas)-1] != "summary text" {
		t.Fatalf("last delta = %q, want %q", deltas[len(deltas)-1], "summary text")
	}
	if result.Summary != "summary text" {
		t.Fatalf("summary = %q, want %q", result.Summary, "summary text")
	}
}

func TestStreamMemoryCompactionSummaryUsesCompatibleStreamPathWhenNativeDisabled(t *testing.T) {
	client := &stubCompactionStreamClient{}
	result, err := StreamMemoryCompactionSummary(client, llm.ChatRequest{
		Messages: []*llm.ModelMessage{
			{Role: "system", Content: "compress these memories"},
			{Role: "user", Content: "需要压缩的记忆"},
		},
		ProviderOptions: &models.LLMConfig{
			Name:                  "openai",
			Type:                  "openai",
			Model:                 "gpt-5.4",
			NativeOpenAIToolCalls: false,
		},
		Model: "gpt-5.4",
	}, MemoryCompactionStreamOptions{
		ModelSettings: &models.ModelSettings{
			NativeOpenAIToolCalls: false,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Summary != "summary text" {
		t.Fatalf("summary = %q, want %q", result.Summary, "summary text")
	}
	if client.lastRequest == nil {
		t.Fatal("expected stream request to be captured")
	}
	if len(client.lastRequest.Messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(client.lastRequest.Messages))
	}
	if client.lastRequest.Messages[0].Role != "system" {
		t.Fatalf("first message role = %q, want system", client.lastRequest.Messages[0].Role)
	}
}

func TestStreamMemoryCompactionSummaryIgnoresNativeToolPathEvenWhenEnabled(t *testing.T) {
	client := &stubCompactionStreamClient{}
	result, err := StreamMemoryCompactionSummary(client, llm.ChatRequest{
		Messages: []*llm.ModelMessage{
			{Role: "system", Content: coreagent.MemoryCompactionSystemPrompt},
			{Role: "user", Content: "需要压缩的记忆\n\n压缩优先级"},
		},
		ProviderOptions: &models.LLMConfig{
			Name:                  "openai",
			Type:                  "openai",
			Model:                 "gpt-5.4",
			NativeOpenAIToolCalls: true,
		},
		Model: "gpt-5.4",
	}, MemoryCompactionStreamOptions{
		ModelSettings: &models.ModelSettings{
			NativeOpenAIToolCalls: true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Summary != "summary text" {
		t.Fatalf("summary = %q, want %q", result.Summary, "summary text")
	}
	if client.lastRequest == nil {
		t.Fatal("expected stream request to be captured")
	}
}

type stubCompactionStreamClient struct {
	lastRequest *llm.ChatRequest
}

func (s *stubCompactionStreamClient) Chat(request llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{}, nil
}

func (s *stubCompactionStreamClient) StreamChat(request llm.ChatRequest) (<-chan llm.StreamEvent, error) {
	return s.StreamChatWithOptions(request)
}

func (s *stubCompactionStreamClient) StreamChatWithOptions(request llm.ChatRequest, opts ...llm.StreamChatOption) (<-chan llm.StreamEvent, error) {
	reqCopy := request
	s.lastRequest = &reqCopy
	ch := make(chan llm.StreamEvent, 2)
	ch <- llm.StreamEvent{Type: string(llm.GeneratorMessageTypeTextDelta), Text: "summary text"}
	ch <- llm.StreamEvent{Type: string(llm.GeneratorMessageTypeFinish), Finish: "stop"}
	close(ch)
	return ch, nil
}

type stubRepetitiveCompactionStreamClient struct {
	text string
}

func (s *stubRepetitiveCompactionStreamClient) Chat(request llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{}, nil
}

func (s *stubRepetitiveCompactionStreamClient) StreamChat(request llm.ChatRequest) (<-chan llm.StreamEvent, error) {
	return s.StreamChatWithOptions(request)
}

func (s *stubRepetitiveCompactionStreamClient) StreamChatWithOptions(request llm.ChatRequest, opts ...llm.StreamChatOption) (<-chan llm.StreamEvent, error) {
	ch := make(chan llm.StreamEvent, 2)
	ch <- llm.StreamEvent{Type: string(llm.GeneratorMessageTypeTextDelta), Text: s.text}
	ch <- llm.StreamEvent{Type: string(llm.GeneratorMessageTypeFinish), Finish: "stop"}
	close(ch)
	return ch, nil
}
