package coreagent

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// slowTool 模拟一个执行时间较长的工具。
type slowTool struct {
	name      string
	delay     time.Duration
	cancelled atomic.Bool
	started   atomic.Bool
}

func (t *slowTool) Name() string        { return t.name }
func (t *slowTool) Description() string { return "slow tool" }
func (t *slowTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"properties":           map[string]interface{}{},
		"additionalProperties": true,
	}
}

func (t *slowTool) Execute(ctx ToolContext, input map[string]interface{}) (ToolResult, error) {
	t.started.Store(true)
	select {
	case <-time.After(t.delay):
		return ToolResult{Name: t.name, Content: "slow result"}, nil
	case <-ctx.Context.Done():
		t.cancelled.Store(true)
		return ToolResult{IsError: true, Name: t.name, Content: "cancelled"}, fmt.Errorf("tool cancelled")
	}
}

// contextCanceledSlowTool 模拟真实工具在看门狗取消后返回 context.Canceled。
type contextCanceledSlowTool struct {
	name      string
	delay     time.Duration
	cancelled atomic.Bool
	started   atomic.Bool
}

func (t *contextCanceledSlowTool) Name() string        { return t.name }
func (t *contextCanceledSlowTool) Description() string { return "slow tool returning context.Canceled" }
func (t *contextCanceledSlowTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"properties":           map[string]interface{}{},
		"additionalProperties": true,
	}
}

func (t *contextCanceledSlowTool) Execute(ctx ToolContext, _ map[string]interface{}) (ToolResult, error) {
	t.started.Store(true)
	select {
	case <-time.After(t.delay):
		return ToolResult{Name: t.name, Content: "slow result"}, nil
	case <-ctx.Context.Done():
		t.cancelled.Store(true)
		return ToolResult{IsError: true, Name: t.name, Content: "cancelled"}, context.Canceled
	}
}

// mockActionProviderForWatchdog 返回固定 JSON 决策。
type mockActionProviderForWatchdog struct {
	mu          sync.Mutex
	decision    string // "continue" or "cancel"
	reason      string
	waitSeconds int
	called      int
}

func (p *mockActionProviderForWatchdog) Stream(input StreamInput) (*StreamOutput, error) {
	p.mu.Lock()
	p.called++
	decision := p.decision
	reason := p.reason
	waitSeconds := p.waitSeconds
	p.mu.Unlock()

	rawJSON := fmt.Sprintf(`{"decision":"%s","reason":"%s"}`, decision, reason)
	if decision == "continue" {
		rawJSON = fmt.Sprintf(`{"decision":"%s","reason":"%s","wait_seconds":%d}`, decision, reason, waitSeconds)
	}

	return &StreamOutput{
		RawTextReader: strings.NewReader(rawJSON),
		Wait:          func() error { return nil },
	}, nil
}

func newRunnerWithWatchdog(timeout time.Duration, tool Tool, provider ActionProvider) (*Runner, *testEmitter, error) {
	emitter := &testEmitter{}
	runner, err := NewRunner(RunnerConfig{
		Emitter:              emitter,
		PromptBuilder:        func(state *RunState) (string, error) { return "prompt", nil },
		MaxSteps:             3,
		StallWatchdogTimeout: timeout,
		ActionProvider:       provider,
		Now:                  time.Now,
	})
	if err != nil {
		return nil, nil, err
	}
	if tool != nil {
		runner.RegisterTool(tool)
	}
	return runner, emitter, nil
}

func TestWatchdog_NotTriggeredForExemptTool(t *testing.T) {
	tool := &slowTool{name: RunWorkerTaskToolName, delay: 300 * time.Millisecond}
	provider := &mockActionProviderForWatchdog{decision: "cancel", reason: "should not be called"}
	runner, emitter, err := newRunnerWithWatchdog(100*time.Millisecond, tool, provider)
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	state := &RunState{
		Context:   context.Background(),
		SessionID: "session-1",
		Assistant: &Message{ID: "msg-1", SessionID: "session-1", Role: RoleAssistant, Time: MessageTime{Created: 1}},
		UserInput: "hello",
	}

	start := time.Now()
	parts, err := runner.executeToolCallRequest(state, &CallToolRequest{
		Name:      RunWorkerTaskToolName,
		Arguments: bytes.NewReader([]byte(`{}`)),
		RawJSON:   `{"@action":"call_tool","data":{"name":"run_worker_task","params":{}}`,
	})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("executeToolCallRequest: %v", err)
	}
	if elapsed < 250*time.Millisecond {
		t.Fatalf("expected exempt tool to run to completion, took %v", elapsed)
	}
	if parts[0].Tool.State.Status != "completed" {
		t.Fatalf("expected status completed, got %q", parts[0].Tool.State.Status)
	}
	if tool.cancelled.Load() {
		t.Fatal("exempt tool should not be cancelled by stall watchdog")
	}

	provider.mu.Lock()
	called := provider.called
	provider.mu.Unlock()
	if called != 0 {
		t.Fatalf("stall watchdog should not review exempt tool, provider called %d times", called)
	}
	for _, f := range emitter.footers {
		if strings.Contains(f.Text, "停滞") {
			t.Fatalf("unexpected footer: %q", f.Text)
		}
	}
}

func TestWatchdog_NotTriggeredWhenToolFinishesFast(t *testing.T) {
	tool := &slowTool{name: "slow", delay: 50 * time.Millisecond}
	provider := &mockActionProviderForWatchdog{decision: "continue", reason: "should not be called", waitSeconds: 1}
	runner, emitter, err := newRunnerWithWatchdog(200*time.Millisecond, tool, provider)
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	state := &RunState{
		Context:   context.Background(),
		SessionID: "session-1",
		Assistant: &Message{ID: "msg-1", SessionID: "session-1", Role: RoleAssistant, Time: MessageTime{Created: 1}},
		UserInput: "hello",
	}

	parts, err := runner.executeToolCallRequest(state, &CallToolRequest{
		Name:      "slow",
		Arguments: bytes.NewReader([]byte(`{}`)),
		RawJSON:   `{"@action":"call_tool","data":{"name":"slow","params":{}}`,
	})
	if err != nil {
		t.Fatalf("executeToolCallRequest: %v", err)
	}
	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	if parts[0].Tool == nil {
		t.Fatal("expected tool part")
	}
	if parts[0].Tool.State.Status != "completed" {
		t.Fatalf("expected status completed, got %q", parts[0].Tool.State.Status)
	}
	if tool.cancelled.Load() {
		t.Fatal("tool should not have been cancelled")
	}

	provider.mu.Lock()
	called := provider.called
	provider.mu.Unlock()
	if called != 0 {
		t.Fatalf("watchdog should not have been triggered, provider called %d times", called)
	}

	// footer 不应该有 watchdog 相关的状态
	for _, f := range emitter.footers {
		if strings.Contains(f.Text, "停滞") || strings.Contains(f.Text, "取消") {
			t.Fatalf("unexpected footer: %q", f.Text)
		}
	}
}

func TestWatchdog_CancelDoesNotAbortAgentRunWhenToolReturnsContextCanceled(t *testing.T) {
	tool := &contextCanceledSlowTool{name: "slow", delay: 5 * time.Second}
	provider := &mockActionProviderForWatchdog{decision: "cancel", reason: "taking too long"}
	runner, emitter, err := newRunnerWithWatchdog(100*time.Millisecond, tool, provider)
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	state := &RunState{
		Context:   context.Background(),
		SessionID: "session-1",
		Assistant: &Message{ID: "msg-1", SessionID: "session-1", Role: RoleAssistant, Time: MessageTime{Created: 1}},
		UserInput: "hello",
	}

	parts, err := runner.executeToolCallRequest(state, &CallToolRequest{
		Name:      "slow",
		Arguments: bytes.NewReader([]byte(`{}`)),
		RawJSON:   `{"@action":"call_tool","data":{"name":"slow","params":{}}`,
	})
	if err != nil {
		t.Fatalf("executeToolCallRequest should not return run-fatal error after watchdog cancel: %v", err)
	}
	if len(parts) != 1 || parts[0].Tool == nil {
		t.Fatalf("expected one tool part, got %#v", parts)
	}
	if parts[0].Tool.State.Status != "cancelled" {
		t.Fatalf("expected status cancelled, got %q", parts[0].Tool.State.Status)
	}
	if !hasToolResultCancelledBy("stall_watchdog", parts[0].Tool.State.MemoryMetadata) {
		t.Fatalf("expected stall_watchdog memory metadata, got %#v", parts[0].Tool.State.MemoryMetadata)
	}
	if !tool.cancelled.Load() {
		t.Fatal("tool should have been cancelled")
	}
	found := false
	for _, f := range emitter.footers {
		if strings.Contains(f.Text, "取消") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected footer with cancel status, got %#v", emitter.footers)
	}
}

func TestWatchdog_CancelsWhenModelDecidesCancel(t *testing.T) {
	tool := &slowTool{name: "slow", delay: 5 * time.Second}
	provider := &mockActionProviderForWatchdog{decision: "cancel", reason: "taking too long"}
	runner, emitter, err := newRunnerWithWatchdog(100*time.Millisecond, tool, provider)
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	state := &RunState{
		Context:   context.Background(),
		SessionID: "session-1",
		Assistant: &Message{ID: "msg-1", SessionID: "session-1", Role: RoleAssistant, Time: MessageTime{Created: 1}},
		UserInput: "hello",
	}

	start := time.Now()
	parts, err := runner.executeToolCallRequest(state, &CallToolRequest{
		Name:      "slow",
		Arguments: bytes.NewReader([]byte(`{}`)),
		RawJSON:   `{"@action":"call_tool","data":{"name":"slow","params":{}}`,
	})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("executeToolCallRequest: %v", err)
	}

	// 应该在 watchdog 超时后不久返回，而不是等 5 秒
	if elapsed > 2*time.Second {
		t.Fatalf("expected fast return after cancel, took %v", elapsed)
	}

	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	if parts[0].Tool == nil {
		t.Fatal("expected tool part")
	}
	if parts[0].Tool.State.Status != "cancelled" {
		t.Fatalf("expected status cancelled after watchdog cancel, got %q", parts[0].Tool.State.Status)
	}
	if !tool.cancelled.Load() {
		t.Fatal("tool should have been cancelled")
	}

	provider.mu.Lock()
	called := provider.called
	provider.mu.Unlock()
	if called < 1 {
		t.Fatalf("expected provider called at least once, got %d", called)
	}

	// footer 应该显示取消状态
	found := false
	for _, f := range emitter.footers {
		if strings.Contains(f.Text, "取消") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected footer with cancel status, got %#v", emitter.footers)
	}
}

func TestWatchdog_ContinuesWhenModelDecidesContinue(t *testing.T) {
	tool := &slowTool{name: "slow", delay: 350 * time.Millisecond}
	provider := &mockActionProviderForWatchdog{decision: "continue", reason: "almost done", waitSeconds: 1}
	runner, emitter, err := newRunnerWithWatchdog(100*time.Millisecond, tool, provider)
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	state := &RunState{
		Context:   context.Background(),
		SessionID: "session-1",
		Assistant: &Message{ID: "msg-1", SessionID: "session-1", Role: RoleAssistant, Time: MessageTime{Created: 1}},
		UserInput: "hello",
	}

	start := time.Now()
	parts, err := runner.executeToolCallRequest(state, &CallToolRequest{
		Name:      "slow",
		Arguments: bytes.NewReader([]byte(`{}`)),
		RawJSON:   `{"@action":"call_tool","data":{"name":"slow","params":{}}`,
	})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("executeToolCallRequest: %v", err)
	}

	// 工具 350ms 完成，watchdog 100ms，模型要求继续等待 1s，应该只触发一次 watchdog 并最终成功
	if elapsed < 300*time.Millisecond {
		t.Fatalf("expected tool to finish after ~350ms, took %v", elapsed)
	}
	if elapsed > 600*time.Millisecond {
		t.Fatalf("expected no extra delay, took %v", elapsed)
	}

	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	if parts[0].Tool.State.Status != "completed" {
		t.Fatalf("expected status completed, got %q", parts[0].Tool.State.Status)
	}
	if tool.cancelled.Load() {
		t.Fatal("tool should not have been cancelled")
	}

	provider.mu.Lock()
	called := provider.called
	provider.mu.Unlock()
	if called != 1 {
		t.Fatalf("expected provider called once, got %d", called)
	}

	// footer 应该显示继续等待状态
	found := false
	for _, f := range emitter.footers {
		if strings.Contains(f.Text, "继续等待 1s") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected footer with continue status, got %#v", emitter.footers)
	}
}

func TestWatchdog_DisabledWhenTimeoutIsZero(t *testing.T) {
	tool := &slowTool{name: "slow", delay: 200 * time.Millisecond}
	provider := &mockActionProviderForWatchdog{decision: "cancel", reason: "should not be called"}
	runner, _, err := newRunnerWithWatchdog(0, tool, provider)
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	state := &RunState{
		Context:   context.Background(),
		SessionID: "session-1",
		Assistant: &Message{ID: "msg-1", SessionID: "session-1", Role: RoleAssistant, Time: MessageTime{Created: 1}},
		UserInput: "hello",
	}

	start := time.Now()
	parts, err := runner.executeToolCallRequest(state, &CallToolRequest{
		Name:      "slow",
		Arguments: bytes.NewReader([]byte(`{}`)),
		RawJSON:   `{"@action":"call_tool","data":{"name":"slow","params":{}}`,
	})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("executeToolCallRequest: %v", err)
	}

	// 应该等工具自然完成
	if elapsed < 150*time.Millisecond {
		t.Fatalf("expected tool to finish naturally, took %v", elapsed)
	}
	if parts[0].Tool.State.Status != "completed" {
		t.Fatalf("expected status completed, got %q", parts[0].Tool.State.Status)
	}

	provider.mu.Lock()
	called := provider.called
	provider.mu.Unlock()
	if called != 0 {
		t.Fatalf("watchdog should be disabled, provider called %d times", called)
	}
}

func TestWatchdog_FallbackToWaitOnProviderError(t *testing.T) {
	tool := &slowTool{name: "slow", delay: 250 * time.Millisecond}
	provider := &stubActionProvider{
		stream: func(input StreamInput) (*StreamOutput, error) {
			return nil, fmt.Errorf("network error")
		},
	}
	runner, emitter, err := newRunnerWithWatchdog(100*time.Millisecond, tool, provider)
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	state := &RunState{
		Context:   context.Background(),
		SessionID: "session-1",
		Assistant: &Message{ID: "msg-1", SessionID: "session-1", Role: RoleAssistant, Time: MessageTime{Created: 1}},
		UserInput: "hello",
	}

	start := time.Now()
	parts, err := runner.executeToolCallRequest(state, &CallToolRequest{
		Name:      "slow",
		Arguments: bytes.NewReader([]byte(`{}`)),
		RawJSON:   `{"@action":"call_tool","data":{"name":"slow","params":{}}`,
	})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("executeToolCallRequest: %v", err)
	}

	// 即使 provider 报错，工具也应该最终完成
	if elapsed < 200*time.Millisecond {
		t.Fatalf("expected tool to finish, took %v", elapsed)
	}
	if parts[0].Tool.State.Status != "completed" {
		t.Fatalf("expected status completed, got %q", parts[0].Tool.State.Status)
	}

	// footer 应该显示 fallback 状态
	found := false
	for _, f := range emitter.footers {
		if strings.Contains(f.Text, "失败") || strings.Contains(f.Text, "继续等待") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected footer with fallback status, got %#v", emitter.footers)
	}
}

func TestWatchdog_ResolvesStallParsesJSONFromRawText(t *testing.T) {
	provider := &stubActionProvider{
		stream: func(input StreamInput) (*StreamOutput, error) {
			// 模拟模型输出带 markdown 代码块的 JSON
			raw := "```json\n{\"decision\":\"cancel\",\"reason\":\"too slow\"}\n```"
			return &StreamOutput{
				RawTextReader: strings.NewReader(raw),
				Wait:          func() error { return nil },
			}, nil
		},
	}
	runner, _, err := newRunnerWithWatchdog(100*time.Millisecond, nil, provider)
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	state := &RunState{
		Context:   context.Background(),
		SessionID: "session-1",
		Assistant: &Message{ID: "msg-1", SessionID: "session-1", Role: RoleAssistant, Time: MessageTime{Created: 1}},
		UserInput: "hello",
	}

	decision, reason, wait, err := runner.resolveStall(state, "test-tool", map[string]interface{}{"path": "/tmp"}, "some output", 5*time.Second)
	if err != nil {
		t.Fatalf("resolveStall: %v", err)
	}
	if decision != "cancel" {
		t.Fatalf("expected cancel, got %q", decision)
	}
	if !strings.Contains(reason, "too slow") {
		t.Fatalf("expected reason containing 'too slow', got %q", reason)
	}
	if wait != 0 {
		t.Fatalf("expected zero wait for cancel, got %v", wait)
	}
}

func TestWatchdog_ResolvesStallParsesJSONFromBraces(t *testing.T) {
	provider := &stubActionProvider{
		stream: func(input StreamInput) (*StreamOutput, error) {
			// 模拟模型输出混有解释文本的 JSON
			raw := "I think the tool is stuck. Let me decide:\n{\"decision\":\"continue\",\"reason\":\"almost done\",\"wait_seconds\":7}"
			return &StreamOutput{
				RawTextReader: strings.NewReader(raw),
				Wait:          func() error { return nil },
			}, nil
		},
	}
	runner, _, err := newRunnerWithWatchdog(100*time.Millisecond, nil, provider)
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	state := &RunState{
		Context:   context.Background(),
		SessionID: "session-1",
		Assistant: &Message{ID: "msg-1", SessionID: "session-1", Role: RoleAssistant, Time: MessageTime{Created: 1}},
		UserInput: "hello",
	}

	decision, reason, wait, err := runner.resolveStall(state, "test-tool", map[string]interface{}{}, "", 1*time.Second)
	if err != nil {
		t.Fatalf("resolveStall: %v", err)
	}
	if decision != "continue" {
		t.Fatalf("expected continue, got %q", decision)
	}
	if !strings.Contains(reason, "almost done") {
		t.Fatalf("expected reason containing 'almost done', got %q", reason)
	}
	if wait != 7*time.Second {
		t.Fatalf("expected wait 7s, got %v", wait)
	}
}

func TestWatchdog_ResolvesStallReturnsErrorOnInvalidJSON(t *testing.T) {
	provider := &stubActionProvider{
		stream: func(input StreamInput) (*StreamOutput, error) {
			raw := "invalid response without json"
			return &StreamOutput{
				RawTextReader: strings.NewReader(raw),
				Wait:          func() error { return nil },
			}, nil
		},
	}
	runner, _, err := newRunnerWithWatchdog(100*time.Millisecond, nil, provider)
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	state := &RunState{
		Context:   context.Background(),
		SessionID: "session-1",
		Assistant: &Message{ID: "msg-1", SessionID: "session-1", Role: RoleAssistant, Time: MessageTime{Created: 1}},
		UserInput: "hello",
	}

	_, _, _, err = runner.resolveStall(state, "test-tool", map[string]interface{}{}, "", 1*time.Second)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestWatchdog_ResolvesStallUsesRawTextReaderWhenContentReaderIsEmpty(t *testing.T) {
	provider := &stubActionProvider{
		stream: func(input StreamInput) (*StreamOutput, error) {
			return &StreamOutput{
				RawTextReader: strings.NewReader(`{"decision":"cancel","reason":"from raw"}`),
				ContentReader: strings.NewReader(""),
				Wait:          func() error { return nil },
			}, nil
		},
	}
	runner, _, err := newRunnerWithWatchdog(100*time.Millisecond, nil, provider)
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	state := &RunState{
		Context:   context.Background(),
		SessionID: "session-1",
		Assistant: &Message{ID: "msg-1", SessionID: "session-1", Role: RoleAssistant, Time: MessageTime{Created: 1}},
		UserInput: "hello",
	}

	decision, reason, wait, err := runner.resolveStall(state, "test-tool", map[string]interface{}{}, "", 1*time.Second)
	if err != nil {
		t.Fatalf("resolveStall: %v", err)
	}
	if decision != "cancel" {
		t.Fatalf("expected cancel, got %q", decision)
	}
	if reason != "from raw" {
		t.Fatalf("expected reason from raw, got %q", reason)
	}
	if wait != 0 {
		t.Fatalf("expected zero wait for cancel, got %v", wait)
	}
}

func TestWatchdog_ResolvesStallPrefersRawTextReaderOverContentReader(t *testing.T) {
	provider := &stubActionProvider{
		stream: func(input StreamInput) (*StreamOutput, error) {
			return &StreamOutput{
				RawTextReader: strings.NewReader(`{"decision":"continue","reason":"from raw","wait_seconds":9}`),
				ContentReader: strings.NewReader(`{"decision":"cancel","reason":"from content"}`),
				Wait:          func() error { return nil },
			}, nil
		},
	}
	runner, _, err := newRunnerWithWatchdog(100*time.Millisecond, nil, provider)
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	state := &RunState{
		Context:   context.Background(),
		SessionID: "session-1",
		Assistant: &Message{ID: "msg-1", SessionID: "session-1", Role: RoleAssistant, Time: MessageTime{Created: 1}},
		UserInput: "hello",
	}

	decision, reason, wait, err := runner.resolveStall(state, "test-tool", map[string]interface{}{}, "", 1*time.Second)
	if err != nil {
		t.Fatalf("resolveStall: %v", err)
	}
	if decision != "continue" {
		t.Fatalf("expected continue from raw reader, got %q", decision)
	}
	if reason != "from raw" {
		t.Fatalf("expected reason from raw reader, got %q", reason)
	}
	if wait != 9*time.Second {
		t.Fatalf("expected wait 9s from raw reader, got %v", wait)
	}
}

func TestWatchdog_ResolvesStallReturnsErrorOnInvalidContinueWait(t *testing.T) {
	provider := &stubActionProvider{
		stream: func(input StreamInput) (*StreamOutput, error) {
			return &StreamOutput{
				RawTextReader: strings.NewReader(`{"decision":"continue","reason":"missing wait","wait_seconds":0}`),
				Wait:          func() error { return nil },
			}, nil
		},
	}
	runner, _, err := newRunnerWithWatchdog(100*time.Millisecond, nil, provider)
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	state := &RunState{
		Context:   context.Background(),
		SessionID: "session-1",
		Assistant: &Message{ID: "msg-1", SessionID: "session-1", Role: RoleAssistant, Time: MessageTime{Created: 1}},
		UserInput: "hello",
	}

	_, _, _, err = runner.resolveStall(state, "test-tool", map[string]interface{}{}, "", 1*time.Second)
	if err == nil {
		t.Fatal("expected error for invalid continue wait, got nil")
	}
}
